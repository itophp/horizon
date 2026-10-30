//go:build integration

package tests

import (
	genidentity "baseline/services/api/gen/identity"
	gen "baseline/services/api/gen/mcp_management"
	"baseline/services/api/internal/identity"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestMCPTokenLifecycleAndIsolation(t *testing.T) {
	handler, pool, root := iamFixture(t)
	owner := loginIAM(t, handler, "root-admin")
	w := owner.request(t, "POST", "/api/auth/mcp/tokens", map[string]interface{}{"password": "TestOnly!2026Secure", "name": "Codex", "departmentId": root.DepartmentID, "days": 30, "tools": []string{"context_get", "capabilities_list"}}, true)
	checkStatus(t, w, 201)
	created := decodeBody[gen.MCPTokenCreated](t, w)
	if len(created.Secret) != 69 {
		t.Fatal("missing one-time credential")
	}
	list := owner.request(t, "GET", "/api/auth/mcp/tokens", nil, false)
	checkStatus(t, list, 200)
	if strings.Contains(list.Body.String(), created.Secret) || strings.Contains(list.Body.String(), "tokenHash") {
		t.Fatal("secret leaked into list")
	}
	invoke := func(secret, body string) *httptest.ResponseRecorder {
		request := httptest.NewRequest(http.MethodPost, "http://console.test/mcp", strings.NewReader(body))
		request.Header.Set("Authorization", "Bearer "+secret)
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set("Accept", "application/json, text/event-stream")
		request.Header.Set("MCP-Protocol-Version", "2025-11-25")
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		return response
	}
	response := invoke(created.Secret, `{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}`)
	checkStatus(t, response, 200)
	if !strings.Contains(response.Body.String(), "context_get") || strings.Contains(response.Body.String(), "notifications_list") {
		t.Fatalf("unexpected tool visibility: %s", response.Body.String())
	}
	_, err := identity.Bootstrap(context.Background(), pool, "tenant-b", "other-admin", "TestOnly!2026Secure")
	if err != nil {
		t.Fatal(err)
	}
	other := loginIAM(t, handler, "other-admin")
	checkStatus(t, other.request(t, "DELETE", "/api/auth/mcp/tokens/"+strconv.FormatInt(created.Token.ID, 10), map[string]string{"password": "TestOnly!2026Secure"}, true), 404)
	rotatedResponse := owner.request(t, "POST", "/api/auth/mcp/tokens/"+strconv.FormatInt(created.Token.ID, 10)+"/rotate", map[string]string{"password": "TestOnly!2026Secure"}, true)
	checkStatus(t, rotatedResponse, 201)
	rotated := decodeBody[gen.MCPTokenCreated](t, rotatedResponse)
	checkStatus(t, invoke(created.Secret, `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`), 401)
	checkStatus(t, invoke(rotated.Secret, `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"context_get","arguments":{}}}`), 200)
	checkStatus(t, owner.request(t, "DELETE", "/api/auth/mcp/tokens/"+strconv.FormatInt(rotated.Token.ID, 10), map[string]string{"password": "TestOnly!2026Secure"}, true), 204)
	checkStatus(t, invoke(rotated.Secret, `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`), 401)
}

func TestMCPSecurityBoundaries(t *testing.T) {
	handler, pool, root := iamFixture(t)
	owner := loginIAM(t, handler, "root-admin")
	ctx := context.Background()
	create := func(department int64, password string, tools []string) *httptest.ResponseRecorder {
		return owner.request(t, "POST", "/api/auth/mcp/tokens", map[string]interface{}{"password": password, "name": "security", "departmentId": department, "days": 30, "tools": tools}, true)
	}
	invoke := func(secret string) *httptest.ResponseRecorder {
		r := httptest.NewRequest("POST", "http://console.test/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`))
		r.Header.Set("Authorization", "Bearer "+secret)
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("Accept", "application/json, text/event-stream")
		r.Header.Set("MCP-Protocol-Version", "2025-11-25")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		return w
	}
	checkStatus(t, create(root.DepartmentID, "wrong-password", []string{"context_get"}), 403)
	checkStatus(t, create(root.DepartmentID, "TestOnly!2026Secure", []string{"arbitrary_shell"}), 400)
	checkStatus(t, create(root.DepartmentID, "TestOnly!2026Secure", []string{"context_get", "context_get"}), 400)
	checkStatus(t, create(999999, "TestOnly!2026Secure", []string{"context_get"}), 403)
	token := decodeBody[gen.MCPTokenCreated](t, create(root.DepartmentID, "TestOnly!2026Secure", []string{"context_get"}))
	t.Run("concurrent rotation has exactly one winner", func(t *testing.T) {
		statuses := make(chan int, 2)
		for range 2 {
			go func() {
				r := owner.request(t, "POST", "/api/auth/mcp/tokens/"+strconv.FormatInt(token.Token.ID, 10)+"/rotate", map[string]string{"password": "TestOnly!2026Secure"}, true)
				statuses <- r.Code
			}()
		}
		one, two := <-statuses, <-statuses
		if !((one == 201 && two == 404) || (one == 404 && two == 201)) {
			t.Fatalf("unexpected concurrent statuses %d %d", one, two)
		}
		checkStatus(t, invoke(token.Secret), 401)
	})
	t.Run("expiry uses database time", func(t *testing.T) {
		value := decodeBody[gen.MCPTokenCreated](t, create(root.DepartmentID, "TestOnly!2026Secure", []string{"context_get"}))
		if _, err := pool.Exec(ctx, "UPDATE mcp_tokens SET created_at=now()-interval '2 days',expires_at=now()-interval '1 day' WHERE id=$1", value.Token.ID); err != nil {
			t.Fatal(err)
		}
		checkStatus(t, invoke(value.Secret), 401)
	})
	t.Run("removed department cannot revive old token on rejoin", func(t *testing.T) {
		var dept int64
		if err := pool.QueryRow(ctx, "SELECT id FROM iam_departments WHERE tenant_id=$1 AND organization_id=$2 AND id<>$3 LIMIT 1", "tenant-a", root.OrganizationID, root.DepartmentID).Scan(&dept); err != nil {
			t.Fatal(err)
		}
		if _, err := pool.Exec(ctx, "INSERT INTO iam_department_memberships(tenant_id,account_id,organization_id,department_id) VALUES($1,$2,$3,$4)", "tenant-a", root.ID, root.OrganizationID, dept); err != nil {
			t.Fatal(err)
		}
		value := decodeBody[gen.MCPTokenCreated](t, create(dept, "TestOnly!2026Secure", []string{"context_get"}))
		checkStatus(t, invoke(value.Secret), 200)
		if _, err := pool.Exec(ctx, "DELETE FROM iam_department_memberships WHERE tenant_id=$1 AND account_id=$2 AND department_id=$3", "tenant-a", root.ID, dept); err != nil {
			t.Fatal(err)
		}
		if _, err := pool.Exec(ctx, "INSERT INTO iam_department_memberships(tenant_id,account_id,organization_id,department_id) VALUES($1,$2,$3,$4)", "tenant-a", root.ID, root.OrganizationID, dept); err != nil {
			t.Fatal(err)
		}
		checkStatus(t, invoke(value.Secret), 401)
	})
	t.Run("disabled account invalidates token permanently", func(t *testing.T) {
		value := decodeBody[gen.MCPTokenCreated](t, create(root.DepartmentID, "TestOnly!2026Secure", []string{"context_get"}))
		if _, err := pool.Exec(ctx, "UPDATE iam_accounts SET status='inactive' WHERE tenant_id=$1 AND id=$2", "tenant-a", root.ID); err != nil {
			t.Fatal(err)
		}
		checkStatus(t, invoke(value.Secret), 401)
		if _, err := pool.Exec(ctx, "UPDATE iam_accounts SET status='active' WHERE tenant_id=$1 AND id=$2", "tenant-a", root.ID); err != nil {
			t.Fatal(err)
		}
		checkStatus(t, invoke(value.Secret), 401)
	})
}

func TestMCPDoesNotInheritTemporaryGrants(t *testing.T) {
	handler, _, admin := iamFixture(t)
	root := loginIAM(t, handler, "root-admin")
	grant := &genidentity.PermissionGrant{Permission: "dictionaries:use", Effect: "allow", Scope: "ALL", DepartmentIds: []int64{}}
	made := root.request(t, "POST", "/api/admin/accounts", &genidentity.CreateAccountPayload{Username: "mcp-formal-reader", FullName: "MCP reader", Status: "active", OrganizationID: admin.OrganizationID, DepartmentID: admin.DepartmentID, Password: "TestOnly!2026Secure", RoleIds: []int64{}, Overrides: []*genidentity.PermissionGrant{grant}}, true)
	checkStatus(t, made, 201)
	account := decodeBody[genidentity.AccountRecord](t, made)
	client := loginIAM(t, handler, account.Username)
	body := map[string]interface{}{"name": "formal reader", "password": "TestOnly!2026Secure", "days": 30, "departmentId": account.DepartmentID, "tools": []string{"context_get", "dictionary_options"}}
	response := client.request(t, "POST", "/api/auth/mcp/tokens", body, true)
	checkStatus(t, response, 201)
	token := decodeBody[gen.MCPTokenCreated](t, response)
	checkStatus(t, root.request(t, "PUT", "/api/admin/accounts/"+strconv.FormatInt(account.ID, 10), &genidentity.UpdateAccountPayload{ID: account.ID, Username: account.Username, FullName: account.FullName, Status: "active", OrganizationID: account.OrganizationID, DepartmentID: account.DepartmentID, RoleIds: []int64{}, Overrides: []*genidentity.PermissionGrant{}}, true), 200)
	requested := client.request(t, "POST", "/api/admin/temporary-access", &genidentity.RequestTemporaryAccessPayload{Minutes: 30, Reason: "Read dictionary for an assigned task", Grants: []*genidentity.PermissionGrant{grant}}, true)
	checkStatus(t, requested, 200)
	request := decodeBody[genidentity.TemporaryAccessRecord](t, requested)
	checkStatus(t, root.request(t, "POST", "/api/admin/temporary-access/"+strconv.FormatInt(request.ID, 10)+"/review", &genidentity.ReviewTemporaryAccessPayload{Decision: "approve", Minutes: 30, Note: "Temporary read", Grants: []*genidentity.PermissionGrant{grant}}, true), 200)
	me := client.request(t, "GET", "/api/auth/me", nil, false)
	checkStatus(t, me, 200)
	profile := decodeBody[genidentity.SessionProfile](t, me)
	hasTemporary := false
	for _, permission := range profile.Permissions {
		hasTemporary = hasTemporary || permission == "dictionaries:use"
	}
	if !hasTemporary {
		t.Fatal("temporary grant fixture did not become effective")
	}
	// Existing credential still contains the tool, but its live permanent identity cannot expose it.
	r := httptest.NewRequest("POST", "http://console.test/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`))
	r.Header.Set("Authorization", "Bearer "+token.Secret)
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Accept", "application/json, text/event-stream")
	r.Header.Set("MCP-Protocol-Version", "2025-11-25")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)
	checkStatus(t, w, 200)
	if strings.Contains(w.Body.String(), "dictionary_options") {
		t.Fatal("MCP inherited temporary authorization")
	}
	checkStatus(t, client.request(t, "POST", "/api/auth/mcp/tokens", body, true), 403)
}

func TestMCPNotificationToolsEnforceRecipientScope(t *testing.T) {
	handler, pool, root := iamFixture(t)
	ctx := context.Background()
	owner := loginIAM(t, handler, "root-admin")
	other, err := identity.Bootstrap(ctx, pool, "tenant-b", "other-notification-admin", "TestOnly!2026Secure")
	if err != nil {
		t.Fatal(err)
	}
	notice := func(tenant string, account, dept int64, title string) int64 {
		var announcement, id int64
		err := pool.QueryRow(ctx, "INSERT INTO gov_announcements(tenant_id,sender_id,department_id,client_key,payload_hash,title,markdown,targets,email,recipient_count) VALUES($1,$2,$3,'mcp-scope','test-digest',$4,$4,'[]',false,1) RETURNING id", tenant, account, dept, title).Scan(&announcement)
		if err != nil {
			t.Fatal(err)
		}
		payload, encodeErr := json.Marshal(map[string]interface{}{"title": title, "markdown": title, "inApp": true, "event": "announcement"})
		if encodeErr != nil {
			t.Fatal(encodeErr)
		}
		if err = pool.QueryRow(ctx, "INSERT INTO gov_notifications(tenant_id,recipient_id,department_id,payload,announcement_id) VALUES($1,$2,$3,$4,$5) RETURNING id", tenant, account, dept, payload, announcement).Scan(&id); err != nil {
			t.Fatal(err)
		}
		return id
	}
	ownID := notice("tenant-a", root.ID, root.DepartmentID, "Own notice")
	foreignID := notice("tenant-b", other.ID, other.DepartmentID, "Foreign private sentinel")
	made := owner.request(t, "POST", "/api/auth/mcp/tokens", map[string]interface{}{"name": "recipient scope", "password": "TestOnly!2026Secure", "days": 1, "departmentId": root.DepartmentID, "tools": []string{"notifications_get", "notifications_list"}}, true)
	checkStatus(t, made, 201)
	token := decodeBody[gen.MCPTokenCreated](t, made)
	call := func(name string, args map[string]interface{}) string {
		raw, e := json.Marshal(map[string]interface{}{"jsonrpc": "2.0", "id": 1, "method": "tools/call", "params": map[string]interface{}{"name": name, "arguments": args}})
		if e != nil {
			t.Fatal(e)
		}
		r := httptest.NewRequest("POST", "http://console.test/mcp", strings.NewReader(string(raw)))
		r.Header.Set("Authorization", "Bearer "+token.Secret)
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("Accept", "application/json, text/event-stream")
		r.Header.Set("MCP-Protocol-Version", "2025-11-25")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		checkStatus(t, w, 200)
		return w.Body.String()
	}
	own := call("notifications_get", map[string]interface{}{"id": ownID})
	if !strings.Contains(own, "Own notice") || !strings.Contains(own, "structuredContent") {
		t.Fatal("authorized detail or structured output missing")
	}
	denied := call("notifications_get", map[string]interface{}{"id": foreignID})
	if strings.Contains(denied, "Foreign private sentinel") || !strings.Contains(denied, "not_found") {
		t.Fatal("foreign notification was not denied")
	}
	list := call("notifications_list", map[string]interface{}{"offset": 0, "limit": 1})
	if strings.Contains(list, "Foreign private sentinel") || !strings.Contains(list, "Own notice") {
		t.Fatal("list scope or pagination failed")
	}
}

func TestMCPPasswordResetCannotRaceNewCredential(t *testing.T) {
	handler, pool, root := iamFixture(t)
	owner := loginIAM(t, handler, "root-admin")
	ctx := context.Background()
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, "SELECT id FROM iam_accounts WHERE tenant_id=$1 AND id=$2 FOR UPDATE", "tenant-a", root.ID); err != nil {
		t.Fatal(err)
	}
	result := make(chan int, 1)
	go func() {
		response := owner.request(t, "POST", "/api/auth/mcp/tokens", map[string]interface{}{"name": "reset race", "password": "TestOnly!2026Secure", "departmentId": root.DepartmentID, "days": 1, "tools": []string{"context_get"}}, true)
		result <- response.Code
	}()
	waiting := false
	deadline := time.NewTimer(10 * time.Second)
	defer deadline.Stop()
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for !waiting {
		select {
		case <-deadline.C:
			t.Fatal("credential request did not reach owner lock")
		case <-ticker.C:
			if err = pool.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE wait_event_type='Lock' AND query LIKE '%MCPLockOwner%')").Scan(&waiting); err != nil {
				t.Fatal(err)
			}
		}
	}
	replacement, err := identity.HashPassword("ReplacementOnly!2026Secure")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec(ctx, "UPDATE iam_accounts SET password_hash=$3 WHERE tenant_id=$1 AND id=$2", "tenant-a", root.ID, replacement); err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	select {
	case status := <-result:
		if status != 403 {
			t.Fatalf("old password created credential after reset: %d", status)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("credential request did not finish")
	}
	var count int
	if err = pool.QueryRow(ctx, "SELECT count(*) FROM mcp_tokens WHERE tenant_id=$1 AND account_id=$2", "tenant-a", root.ID).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatal("racing credential persisted")
	}
}
