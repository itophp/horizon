//go:build integration

package tests

import (
	filegen "baseline/services/api/gen/files"
	iamgen "baseline/services/api/gen/identity"
	noticegen "baseline/services/api/gen/notifications"
	gen "baseline/services/api/gen/settings"
	"baseline/services/api/internal/app"
	"baseline/services/api/internal/auth"
	"baseline/services/api/internal/authorization"
	"baseline/services/api/internal/dataaccess"
	"baseline/services/api/internal/governance"
	"baseline/services/api/internal/identity"
	"baseline/services/api/internal/notifications"
	"baseline/services/api/internal/transport"
	"bufio"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net"
	"net/mail"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestSMTPDurableRetryAndNoEmailIsolation(t *testing.T) {
	_, pool, rootAccount := iamFixture(t)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	var failing atomic.Bool
	failing.Store(true)
	messages := make(chan string, 10)
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			conn, e := listener.Accept()
			if e != nil {
				return
			}
			func() {
				defer conn.Close()
				conn.SetDeadline(time.Now().Add(10 * time.Second))
				reader := bufio.NewReader(conn)
				fmt.Fprint(conn, "220 isolated.test ESMTP\r\n")
				for {
					line, e := reader.ReadString('\n')
					if e != nil {
						return
					}
					switch {
					case strings.HasPrefix(line, "EHLO"), strings.HasPrefix(line, "HELO"):
						fmt.Fprint(conn, "250 isolated.test\r\n")
					case strings.HasPrefix(line, "MAIL"):
						if failing.Load() {
							fmt.Fprint(conn, "451 transient test failure\r\n")
						} else {
							fmt.Fprint(conn, "250 ok\r\n")
						}
					case strings.HasPrefix(line, "RCPT"):
						fmt.Fprint(conn, "250 ok\r\n")
					case strings.HasPrefix(line, "DATA"):
						fmt.Fprint(conn, "354 continue\r\n")
						var body strings.Builder
						for {
							data, e := reader.ReadString('\n')
							if e != nil {
								return
							}
							if data == ".\r\n" {
								break
							}
							body.WriteString(data)
						}
						messages <- body.String()
						fmt.Fprint(conn, "250 queued\r\n")
					case strings.HasPrefix(line, "QUIT"):
						fmt.Fprint(conn, "221 bye\r\n")
						return
					default:
						fmt.Fprint(conn, "250 ok\r\n")
					}
				}
			}()
		}
	}()
	defer func() { listener.Close(); <-done }()
	ctx := context.Background()
	if _, err = pool.Exec(ctx, "UPDATE iam_accounts SET email='root@example.test' WHERE id=$1", rootAccount.ID); err != nil {
		t.Fatal(err)
	}
	iam := identity.New(pool, false)
	engine, err := authorization.New()
	if err != nil {
		t.Fatal(err)
	}
	options := governance.Options{EncryptionKey: []byte("01234567890123456789012345678901"), StorageRoot: t.TempDir(), SMTPHosts: []string{"127.0.0.1"}, AllowTestSMTP: true}
	handler := transport.HandlerWithGovernance(app.System{}, app.Demo{Scopes: dataaccess.OwnResolver{}}, auth.DevelopmentToken{}, engine, options, iam)
	root := loginIAM(t, handler, "root-admin")
	port := listener.Addr().(*net.TCPAddr).Port
	w := root.request(t, "POST", "/api/admin/settings/drafts", &gen.DraftPayload{Target: &gen.ConfigTarget{Kind: "workspace"}, Reason: "Configure isolated SMTP server", Changes: []*gen.ChangeEntry{{Key: "smtp.enabled", Operation: "set", Value: "true"}, {Key: "smtp.host", Operation: "set", Value: `"127.0.0.1"`}, {Key: "smtp.port", Operation: "set", Value: strconv.Itoa(port)}, {Key: "smtp.from", Operation: "set", Value: `"sender@example.test"`}}}, true)
	checkStatus(t, w, 201)
	draft := decodeBody[gen.ChangeRecord](t, w)
	w = root.request(t, "POST", "/api/auth/reauthenticate", &gen.ReauthenticatePayload{Password: "TestOnly!2026Secure", Operation: "settings.publish", ResourceID: draft.ID}, true)
	checkStatus(t, w, 200)
	proof := decodeBody[gen.ReauthProof](t, w)
	checkStatus(t, root.request(t, "POST", fmt.Sprintf("/api/admin/settings/drafts/%d/publish", draft.ID), &gen.PublishPayload{Proof: proof.Proof}, true), 200)
	w = root.request(t, "POST", "/api/admin/accounts", &iamgen.CreateAccountPayload{Username: "smtp-user", FullName: "SMTP User", OrganizationID: rootAccount.OrganizationID, DepartmentID: rootAccount.DepartmentID, Password: "TestOnly!2026Secure", Status: "active", RoleIds: []int64{}, Overrides: []*iamgen.PermissionGrant{}}, true)
	checkStatus(t, w, 201)
	user := loginIAM(t, handler, "smtp-user")
	w = user.request(t, "POST", "/api/admin/temporary-access", &iamgen.RequestTemporaryAccessPayload{Minutes: 30, Reason: "Need temporary access", Grants: []*iamgen.PermissionGrant{{Permission: "accounts:read", Effect: "allow", Scope: "OWN", DepartmentIds: []int64{}}}}, true)
	checkStatus(t, w, 200)
	request := decodeBody[iamgen.TemporaryAccessRecord](t, w)
	worker := notifications.New(governance.New(pool, iam, options))
	if err = worker.ProcessOnce(ctx); err != nil {
		t.Fatal(err)
	}
	var status, code string
	var attempts int
	if err = pool.QueryRow(ctx, "SELECT status,error_code,attempts FROM gov_deliveries ORDER BY id LIMIT 1").Scan(&status, &code, &attempts); err != nil || status != "pending" || code != "smtp_failed" || attempts != 1 {
		t.Fatal("failure not persisted", status, code, attempts, err)
	}
	failing.Store(false)
	if _, err = pool.Exec(ctx, "UPDATE gov_deliveries SET next_attempt_at=now()-interval '1 second'"); err != nil {
		t.Fatal(err)
	}
	if err = worker.ProcessOnce(ctx); err != nil {
		t.Fatal(err)
	}
	select {
	case message := <-messages:
		if !strings.Contains(message, "Message-ID:") || !strings.Contains(message, "Content-Transfer-Encoding: base64") {
			t.Fatal("unsafe MIME")
		}
	case <-time.After(time.Second):
		t.Fatal("SMTP not delivered")
	}
	if err = pool.QueryRow(ctx, "SELECT status,attempts FROM gov_deliveries ORDER BY id LIMIT 1").Scan(&status, &attempts); err != nil || status != "sent" || attempts != 2 {
		t.Fatal("retry state", status, attempts, err)
	}
	checkStatus(t, root.request(t, "POST", fmt.Sprintf("/api/admin/temporary-access/%d/review", request.ID), &iamgen.ReviewTemporaryAccessPayload{Decision: "reject", Minutes: 30, Note: "Not required", Grants: []*iamgen.PermissionGrant{}}, true), 200)
	if err = worker.ProcessOnce(ctx); err != nil {
		t.Fatal(err)
	}
	inbox := user.request(t, "GET", "/api/auth/notifications", nil, false)
	checkStatus(t, inbox, 200)
	if !strings.Contains(inbox.Body.String(), "rejected") {
		t.Fatal("missing in-app notice for no-email user")
	}
	if err = pool.QueryRow(ctx, "SELECT status,error_code FROM gov_deliveries ORDER BY id DESC LIMIT 1").Scan(&status, &code); err != nil || status != "skipped" || code != "email_missing" {
		t.Fatal("email absence affected inbox", status, code, err)
	}
	upload := root.request(t, "POST", "/api/admin/files", &filegen.UploadPayload{Name: "smtp-note.txt", Purpose: "attachment", Content: []byte("mail attachment")}, true)
	checkStatus(t, upload, 201)
	file := decodeBody[filegen.ManagedFile](t, upload)
	manual := root.request(t, "POST", "/api/admin/notifications/send", &noticegen.SendPayload{ClientKey: "08c97f8b-0401-419e-91d5-e5a2425a0185", Title: "Manual SMTP notice", Markdown: fmt.Sprintf("# Human message\n[file](file:%d)", file.ID), FileIds: []int64{file.ID}, Email: true, Targets: []*noticegen.NotificationTarget{{Kind: "account", ID: rootAccount.ID}}}, true)
	checkStatus(t, manual, 201)
	if err = worker.ProcessOnce(ctx); err != nil {
		t.Fatal(err)
	}
	select {
	case raw := <-messages:
		message, e := mail.ReadMessage(strings.NewReader(raw))
		if e != nil {
			t.Fatal(e)
		}
		encoded, e := io.ReadAll(message.Body)
		if e != nil {
			t.Fatal(e)
		}
		decoded, e := base64.StdEncoding.DecodeString(string(encoded))
		if e != nil {
			t.Fatal(e)
		}
		if !strings.Contains(string(decoded), "# Human message") || !strings.Contains(string(decoded), "/notifications?message=") || strings.Contains(string(decoded), "file:") {
			t.Fatal("manual mail did not preserve body/protected detail link", string(decoded))
		}
	case <-time.After(time.Second):
		t.Fatal("manual SMTP message not delivered")
	}

}
