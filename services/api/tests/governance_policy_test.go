//go:build integration

package tests

import (
	iamgen "baseline/services/api/gen/identity"
	gen "baseline/services/api/gen/settings"
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

func TestPublishedSecurityPolicyCannotReviveTerminatedSessions(t *testing.T) {
	_, pool, account := iamFixture(t)
	engine, err := authorization.New()
	if err != nil {
		t.Fatal(err)
	}
	options := governance.Options{EncryptionKey: []byte("01234567890123456789012345678901"), StorageRoot: t.TempDir()}
	handler := transport.HandlerWithGovernance(app.System{}, app.Demo{Scopes: dataaccess.OwnResolver{}}, auth.DevelopmentToken{}, engine, options, identity.New(pool, false))
	root := loginIAM(t, handler, "root-admin")
	old := loginIAM(t, handler, "root-admin")
	ctx := context.Background()
	if _, err = pool.Exec(ctx, "UPDATE iam_sessions SET created_at=now()-interval '30 minutes' WHERE token_hash=$1", governance.Hash(old.cookie.Value)); err != nil {
		t.Fatal(err)
	}
	publish := func(base int64, changes ...*gen.ChangeEntry) {
		w := root.request(t, "POST", "/api/admin/settings/drafts", &gen.DraftPayload{Target: &gen.ConfigTarget{Kind: "workspace"}, BaseRevision: base, Reason: "Apply security policy to real consumers", Changes: changes}, true)
		checkStatus(t, w, 201)
		d := decodeBody[gen.ChangeRecord](t, w)
		w = root.request(t, "POST", "/api/auth/reauthenticate", &gen.ReauthenticatePayload{Password: "TestOnly!2026Secure", Operation: "settings.publish", ResourceID: d.ID, Revision: base}, true)
		checkStatus(t, w, 200)
		proof := decodeBody[gen.ReauthProof](t, w)
		checkStatus(t, root.request(t, "POST", fmt.Sprintf("/api/admin/settings/drafts/%d/publish", d.ID), &gen.PublishPayload{BaseRevision: base, Proof: proof.Proof}, true), 200)
	}
	publish(0, &gen.ChangeEntry{Key: "security.sessionMinutes", Operation: "set", Value: "15"}, &gen.ChangeEntry{Key: "security.passwordMin", Operation: "set", Value: "24"}, &gen.ChangeEntry{Key: "security.requireDigit", Operation: "set", Value: "true"})
	checkStatus(t, old.request(t, "GET", "/api/auth/me", nil, false), 401)
	publish(1, &gen.ChangeEntry{Key: "security.sessionMinutes", Operation: "set", Value: "480"})
	checkStatus(t, old.request(t, "GET", "/api/auth/me", nil, false), 401)
	var retained bool
	if err = pool.QueryRow(ctx, "SELECT expires_at<=now() FROM iam_sessions WHERE token_hash=$1", governance.Hash(old.cookie.Value)).Scan(&retained); err != nil || !retained {
		t.Fatal("signed expiry was extended", err)
	}
	payload := &iamgen.CreateAccountPayload{Username: "policy-user", FullName: "Policy user", OrganizationID: account.OrganizationID, DepartmentID: account.DepartmentID, Status: "active", RoleIds: []int64{}, Overrides: []*iamgen.PermissionGrant{}, Password: "TestOnly!2026Secure"}
	checkStatus(t, root.request(t, "POST", "/api/admin/accounts", payload, true), 400)
	payload.Password = "longenoughbutwithoutdigits"
	checkStatus(t, root.request(t, "POST", "/api/admin/accounts", payload, true), 400)
	payload.Password = "LongEnough!1234567890Secure"
	checkStatus(t, root.request(t, "POST", "/api/admin/accounts", payload, true), 201)
	// The stricter password policy applies to creation/reset, not an existing login.
	idle := loginIAM(t, handler, "root-admin")
	if _, err = pool.Exec(ctx, "UPDATE iam_sessions SET last_seen_at=now()-interval '1 hour' WHERE token_hash=$1", governance.Hash(idle.cookie.Value)); err != nil {
		t.Fatal(err)
	}
	publish(2, &gen.ChangeEntry{Key: "security.idleMinutes", Operation: "set", Value: "10"})
	checkStatus(t, idle.request(t, "GET", "/api/auth/me", nil, false), 401)
	publish(3, &gen.ChangeEntry{Key: "security.idleMinutes", Operation: "set", Value: "0"})
	checkStatus(t, idle.request(t, "GET", "/api/auth/me", nil, false), 401)
	publish(4, &gen.ChangeEntry{Key: "security.lockAttempts", Operation: "set", Value: "3"}, &gen.ChangeEntry{Key: "security.lockMinutes", Operation: "set", Value: "5"})
	anonymous := iamClient{handler: handler}
	for n := 0; n < 3; n++ {
		checkStatus(t, anonymous.request(t, "POST", "/api/auth/login", map[string]string{"username": "policy-user", "password": "incorrect"}, false), 401)
	}
	checkStatus(t, anonymous.request(t, "POST", "/api/auth/login", map[string]string{"username": "policy-user", "password": payload.Password}, false), 401)
	var locks int
	if err = pool.QueryRow(ctx, "SELECT count(*) FROM iam_login_attempts WHERE locked_until>now()+interval '4 minutes'").Scan(&locks); err != nil || locks < 1 {
		t.Fatal("configured lockout not applied", locks, err)
	}
	if _, err = pool.Exec(ctx, "UPDATE iam_login_attempts SET locked_until=now()-interval '1 second'"); err != nil {
		t.Fatal(err)
	}
	checkStatus(t, anonymous.request(t, "POST", "/api/auth/login", map[string]string{"username": "policy-user", "password": payload.Password}, false), 200)

}
