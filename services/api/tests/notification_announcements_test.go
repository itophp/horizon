//go:build integration

package tests

import (
	filegen "baseline/services/api/gen/files"
	iamgen "baseline/services/api/gen/identity"
	gen "baseline/services/api/gen/notifications"
	"baseline/services/api/internal/app"
	"baseline/services/api/internal/auth"
	"baseline/services/api/internal/authorization"
	"baseline/services/api/internal/dataaccess"
	"baseline/services/api/internal/governance"
	"baseline/services/api/internal/identity"
	"baseline/services/api/internal/transport"
	"context"
	"fmt"
	"testing"
)

func TestNotificationAnnouncementsRecipientsFilesAndIsolation(t *testing.T) {
	_, pool, admin := iamFixture(t)
	engine, err := authorization.New()
	if err != nil {
		t.Fatal(err)
	}
	handler := transport.HandlerWithGovernance(app.System{}, app.Demo{Scopes: dataaccess.OwnResolver{}}, auth.DevelopmentToken{}, engine, governance.Options{StorageRoot: t.TempDir()}, identity.New(pool, false))
	root := loginIAM(t, handler, "root-admin")
	checkStatus(t, root.request(t, "POST", "/api/admin/notifications/preview", &gen.PreviewPayload{Targets: []*gen.NotificationTarget{nil}}, true), 400)
	checkStatus(t, root.request(t, "POST", "/api/admin/notifications/send", &gen.SendPayload{ClientKey: "bb901ce5-75f8-43ae-83ae-d508991266e8", Title: "Reject null target", Markdown: "Body", Targets: []*gen.NotificationTarget{nil}, FileIds: []int64{}, Email: false}, true), 400)

	snapshot := decodeBody[iamgen.AdminSnapshot](t, root.request(t, "GET", "/api/admin/snapshot", nil, false))
	var other int64
	for _, d := range snapshot.Departments {
		if d.ID != admin.DepartmentID {
			other = d.ID
		}
	}
	create := func(name string, dept int64, grants []*iamgen.PermissionGrant) iamgen.AccountRecord {
		w := root.request(t, "POST", "/api/admin/accounts", &iamgen.CreateAccountPayload{Username: name, FullName: name, Password: "TestOnly!2026Secure", OrganizationID: admin.OrganizationID, DepartmentID: dept, Status: "active", RoleIds: []int64{}, Overrides: grants}, true)
		checkStatus(t, w, 201)
		return decodeBody[iamgen.AccountRecord](t, w)
	}
	alice := create("notice-alice", admin.DepartmentID, []*iamgen.PermissionGrant{})
	create("notice-bob", other, []*iamgen.PermissionGrant{})
	create("notice-sender", admin.DepartmentID, []*iamgen.PermissionGrant{{Permission: "notifications:send", Effect: "allow", Scope: "DEPARTMENT", DepartmentIds: []int64{}}})
	recipient := loginIAM(t, handler, "notice-alice")
	outsider := loginIAM(t, handler, "notice-bob")
	sender := loginIAM(t, handler, "notice-sender")
	for _, path := range []string{"/api/admin/notifications/audience", "/api/admin/notifications/sent", "/api/admin/notifications/deliveries"} {
		checkStatus(t, recipient.request(t, "GET", path, nil, false), 403)
	}
	checkStatus(t, root.request(t, "POST", "/api/admin/notifications/preview", &gen.PreviewPayload{Targets: []*gen.NotificationTarget{{Kind: "account", ID: alice.ID}}}, false), 403)
	audience := decodeBody[[]gen.NotificationAudience](t, root.request(t, "GET", "/api/admin/notifications/audience", nil, false))
	targets := []*gen.NotificationTarget{{Kind: "account", ID: alice.ID}, {Kind: "department", ID: admin.DepartmentID}}
	for _, a := range audience {
		if a.Kind == "enterprise" || a.Kind == "company" {
			targets = append(targets, &gen.NotificationTarget{Kind: a.Kind, ID: a.ID})
		}
	}
	preview := root.request(t, "POST", "/api/admin/notifications/preview", &gen.PreviewPayload{Targets: targets}, true)
	checkStatus(t, preview, 200)
	if decodeBody[gen.PreviewResult](t, preview).RecipientCount != 4 {
		t.Fatal("overlapping groups not deduplicated", preview.Body.String())
	}
	checkStatus(t, sender.request(t, "POST", "/api/admin/notifications/preview", &gen.PreviewPayload{Targets: []*gen.NotificationTarget{{Kind: "department", ID: other}}}, true), 403)
	scoped := sender.request(t, "GET", "/api/admin/notifications/audience", nil, false)
	checkStatus(t, scoped, 200)
	for _, a := range decodeBody[[]gen.NotificationAudience](t, scoped) {
		if a.Kind == "company" || a.Kind == "enterprise" || a.Kind == "department" && a.ID == other {
			t.Fatal("partially authorized group offered")
		}
	}
	upload := root.request(t, "POST", "/api/admin/files", &filegen.UploadPayload{Name: "notice.txt", Purpose: "attachment", Content: []byte("private notification content")}, true)
	checkStatus(t, upload, 201)
	file := decodeBody[filegen.ManagedFile](t, upload)
	body := &gen.SendPayload{ClientKey: "bde33720-8b86-4fd9-8f47-bdd3fe87cb15", Title: "Team notice", Markdown: fmt.Sprintf("# Heading\n\n**Hello**\n\n[File](file:%d)", file.ID), FileIds: []int64{file.ID}, Targets: targets, Email: true}
	w := root.request(t, "POST", "/api/admin/notifications/send", body, true)
	checkStatus(t, w, 201)
	sent := decodeBody[gen.SentNotification](t, w)
	if sent.RecipientCount != 4 || len(sent.Attachments) != 1 {
		t.Fatal("incomplete send", w.Body.String())
	}
	repeat := root.request(t, "POST", "/api/admin/notifications/send", body, true)
	checkStatus(t, repeat, 201)
	if decodeBody[gen.SentNotification](t, repeat).ID != sent.ID {
		t.Fatal("duplicate campaign")
	}
	var messages, deliveries int
	ctx := context.Background()
	if err = pool.QueryRow(ctx, "SELECT count(*) FROM gov_notifications WHERE announcement_id=$1", sent.ID).Scan(&messages); err != nil || messages != 4 {
		t.Fatal("duplicate recipient", messages, err)
	}
	if err = pool.QueryRow(ctx, "SELECT count(*) FROM gov_deliveries d JOIN gov_notifications n ON n.id=d.message_id WHERE n.announcement_id=$1", sent.ID).Scan(&deliveries); err != nil || deliveries != 8 {
		t.Fatal("queue not atomic/deduplicated", deliveries, err)
	}
	body.Title = "Different content"
	checkStatus(t, root.request(t, "POST", "/api/admin/notifications/send", body, true), 409)
	body.ClientKey = "192fbf8d-22db-43d6-b16f-bb8b5071baca"
	body.FileIds = []int64{}
	checkStatus(t, root.request(t, "POST", "/api/admin/notifications/send", body, true), 400)
	inbox := recipient.request(t, "GET", "/api/auth/notifications", nil, false)
	checkStatus(t, inbox, 200)
	items := decodeBody[[]gen.InboxMessage](t, inbox)
	if len(items) != 1 || items[0].Title != "Team notice" || len(items[0].Attachments) != 1 {
		t.Fatal("message not received", inbox.Body.String())
	}
	messageID := items[0].ID
	checkStatus(t, recipient.request(t, "GET", fmt.Sprintf("/api/auth/notifications/%d", messageID), nil, false), 200)
	checkStatus(t, outsider.request(t, "GET", fmt.Sprintf("/api/auth/notifications/%d", messageID), nil, false), 404)
	path := fmt.Sprintf("/api/auth/notifications/%d/files/%d", messageID, file.ID)
	checkStatus(t, recipient.request(t, "GET", fmt.Sprintf("/api/admin/files/%d/content", file.ID), nil, false), 403)
	content := recipient.request(t, "GET", path, nil, false)
	checkStatus(t, content, 200)
	if string(decodeBody[gen.FileContent](t, content).Content) != "private notification content" {
		t.Fatal("attachment mismatch")
	}
	checkStatus(t, outsider.request(t, "GET", path, nil, false), 404)
	checkStatus(t, outsider.request(t, "POST", fmt.Sprintf("/api/auth/notifications/%d/read", messageID), nil, true), 404)
	checkStatus(t, recipient.request(t, "POST", fmt.Sprintf("/api/auth/notifications/%d/read", messageID), nil, true), 204)
	checkStatus(t, root.request(t, "DELETE", fmt.Sprintf("/api/admin/files/%d", file.ID), nil, true), 404)
	if _, err = identity.Bootstrap(ctx, pool, "notice-tenant-b", "notice-foreign", "TestOnly!2026Secure"); err != nil {
		t.Fatal(err)
	}
	foreign := loginIAM(t, handler, "notice-foreign")
	checkStatus(t, foreign.request(t, "GET", path, nil, false), 404)
	checkStatus(t, foreign.request(t, "POST", "/api/admin/notifications/preview", &gen.PreviewPayload{Targets: []*gen.NotificationTarget{{Kind: "account", ID: alice.ID}}}, true), 403)

	checkStatus(t, recipient.request(t, "PUT", "/api/auth/preferences", map[string][]map[string]string{"changes": {{"key": "notifications.inApp", "operation": "set", "value": "false"}}}, true), 200)
	emailOnly := *body
	emailOnly.ClientKey = "d50308ca-d0ec-4512-b2f7-4cd29d20e8dd"
	emailOnly.Title = "Email-only notice"
	emailOnly.Markdown = "Email-only detail"
	emailOnly.FileIds = []int64{}
	emailOnly.Targets = []*gen.NotificationTarget{{Kind: "account", ID: alice.ID}}
	emailOnly.Email = true
	sentEmail := root.request(t, "POST", "/api/admin/notifications/send", &emailOnly, true)
	checkStatus(t, sentEmail, 201)
	emailCampaign := decodeBody[gen.SentNotification](t, sentEmail)
	var emailMessage int64
	if err = pool.QueryRow(ctx, "SELECT id FROM gov_notifications WHERE announcement_id=$1", emailCampaign.ID).Scan(&emailMessage); err != nil {
		t.Fatal(err)
	}
	checkStatus(t, recipient.request(t, "GET", fmt.Sprintf("/api/auth/notifications/%d", emailMessage), nil, false), 200)
	hidden := recipient.request(t, "GET", "/api/auth/notifications", nil, false)
	checkStatus(t, hidden, 200)
	for _, n := range decodeBody[[]gen.InboxMessage](t, hidden) {
		if n.ID == emailMessage {
			t.Fatal("disabled in-app preference ignored")
		}
	}
	// A scoped sender cannot attach another sender's file; the whole campaign rolls back.
	body.ClientKey = "cc25e3ac-359f-4dbf-a93a-d423e603dca6"
	body.Targets = []*gen.NotificationTarget{{Kind: "account", ID: alice.ID}}
	body.FileIds = []int64{file.ID}
	checkStatus(t, sender.request(t, "POST", "/api/admin/notifications/send", body, true), 404)
	body.ClientKey = "abf0684a-4c3c-43b1-9542-77e5e502b43c"
	body.Markdown = "Scoped announcement"
	body.FileIds = []int64{}
	body.Email = false
	checkStatus(t, sender.request(t, "POST", "/api/admin/notifications/send", body, true), 201)
	// Sending and deleting the same file must never leave a successful notice with a missing attachment.
	raceUpload := root.request(t, "POST", "/api/admin/files", &filegen.UploadPayload{Name: "race.txt", Purpose: "attachment", Content: []byte("race")}, true)
	checkStatus(t, raceUpload, 201)
	raceFile := decodeBody[filegen.ManagedFile](t, raceUpload)
	raceBody := &gen.SendPayload{ClientKey: "b09c7cb9-c275-4af8-9761-c9d79a382e23", Title: "Concurrent attachment", Markdown: "File binding race", FileIds: []int64{raceFile.ID}, Targets: []*gen.NotificationTarget{{Kind: "account", ID: admin.ID}}, Email: false}
	start := make(chan struct{})
	sendResult := make(chan int, 1)
	deleteResult := make(chan int, 1)
	go func() {
		<-start
		sendResult <- root.request(t, "POST", "/api/admin/notifications/send", raceBody, true).Code
	}()
	go func() {
		<-start
		deleteResult <- root.request(t, "DELETE", fmt.Sprintf("/api/admin/files/%d", raceFile.ID), nil, true).Code
	}()
	close(start)
	sentStatus, deletedStatus := <-sendResult, <-deleteResult
	if !((sentStatus == 201 && deletedStatus == 404) || (sentStatus == 404 && deletedStatus == 204)) {
		t.Fatalf("inconsistent file binding/delete: send %d delete %d", sentStatus, deletedStatus)
	}

	// Removing the selected historical membership removes cross-context inbox/file access.
	if _, err = pool.Exec(ctx, "UPDATE iam_accounts SET department_id=$1 WHERE tenant_id='tenant-a' AND id=$2", other, alice.ID); err != nil {
		t.Fatal(err)
	}
	moved := loginIAM(t, handler, "notice-alice")
	checkStatus(t, moved.request(t, "GET", path, nil, false), 404)
	remaining := moved.request(t, "GET", "/api/auth/notifications", nil, false)
	checkStatus(t, remaining, 200)
	if len(decodeBody[[]gen.InboxMessage](t, remaining)) != 0 {
		t.Fatal("revoked membership retained notification access")
	}

}
