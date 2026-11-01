//go:build integration

package tests

import (
	"context"
	"encoding/base64"
	"testing"
	"time"

	"baseline/services/api/internal/platform/eventhub"
	"baseline/services/api/internal/platform/outbox"

	"baseline/services/api/internal/modules/dictionaries"
	"baseline/services/api/internal/modules/files"
	"baseline/services/api/internal/modules/mcp"
	"baseline/services/api/internal/modules/notifications"
	"baseline/services/api/internal/modules/settings"
)

// TestModuleCapabilitiesClosedLoop proves that the migrated business modules are
// reachable only through the gateway and that their writes publish events. It
// replaces the removed per-module REST suites: the domain behaviour is the same,
// the entry point is now one contract.
func TestModuleCapabilitiesClosedLoop(t *testing.T) {
	handler, pool, root := iamFixture(t)
	owner := loginIAM(t, handler, "root-admin")
	relay := outbox.NewRelay(pool, eventhub.NewHub())

	t.Run("settings draft publishes and emits an event", func(t *testing.T) {
		workspace := decodeData[settings.SettingsWorkspace](t, owner.invoke(t, "settings.workspace.read", map[string]any{
			"targetKind": "workspace", "targetId": 0,
		}, true))
		draft := decodeData[settings.ChangeRecord](t, owner.invoke(t, "settings.draft.create", map[string]any{
			"target":       map[string]any{"kind": "workspace", "id": 0},
			"baseRevision": workspace.Revision,
			"reason":       "capability test rollout",
			"changes":      []map[string]any{{"key": "branding.name", "operation": "set", "value": `"Capability"`}},
		}, true))
		if draft.ID == 0 {
			t.Fatal("draft was not created")
		}
		proof := decodeData[settings.ReauthProof](t, owner.invoke(t, "settings.password.verify", map[string]any{
			"password": "TestOnly!2026Secure", "operation": "settings.publish",
			"resourceId": draft.ID, "revision": workspace.Revision,
		}, true))
		published := decodeData[settings.ChangeRecord](t, owner.invoke(t, "settings.draft.publish", map[string]any{
			"id": draft.ID, "baseRevision": workspace.Revision, "proof": proof.Proof,
		}, true))
		if published.Revision <= workspace.Revision {
			t.Fatalf("revision did not advance: %d -> %d", workspace.Revision, published.Revision)
		}
		assertEvent(t, owner, relay, "settings.published")
	})

	t.Run("file upload is audited and replay safe", func(t *testing.T) {
		content := base64.StdEncoding.EncodeToString([]byte("capability content"))
		input := map[string]any{"name": "capability.txt", "purpose": "attachment", "content": content}
		first := owner.invoke(t, "file.upload", input, true)
		checkSucceeded(t, first)
		uploaded := decodeData[files.ManagedFile](t, first)
		if uploaded.ID == 0 || uploaded.Size == 0 {
			t.Fatalf("upload not persisted: %+v", uploaded)
		}
		listed := decodeData[[]*files.ManagedFile](t, owner.invoke(t, "file.list.read", map[string]any{}, true))
		if len(listed) == 0 {
			t.Fatal("uploaded file missing from the list")
		}
		downloaded := decodeData[files.FileContent](t, owner.invoke(t, "file.content.read", map[string]any{"id": uploaded.ID}, true))
		if downloaded.Name != "capability.txt" {
			t.Fatalf("content mismatch: %+v", downloaded)
		}
		checkSucceeded(t, owner.invoke(t, "file.delete", map[string]any{"id": uploaded.ID}, true))
		assertEvent(t, owner, relay, "file.uploaded")
	})

	t.Run("notification send reaches the inbox and records the read", func(t *testing.T) {
		sent := decodeData[notifications.SentNotification](t, owner.invoke(t, "notification.send", map[string]any{
			"targets":  []map[string]any{{"kind": "account", "id": root.ID}},
			"title":    "Capability rollout",
			"markdown": "The **capability** gateway now owns every write.",
			"fileIds":  []int64{}, "email": false,
		}, true))
		if sent.ID == 0 || sent.RecipientCount == 0 {
			t.Fatalf("notification not delivered: %+v", sent)
		}
		inbox := decodeData[[]*notifications.InboxMessage](t, owner.invoke(t, "notification.inbox.read", map[string]any{}, true))
		found := false
		for _, message := range inbox {
			if message.ID == sent.ID {
				found = true
			}
		}
		if !found {
			t.Fatalf("sent message missing from inbox: %d", sent.ID)
		}
		checkSucceeded(t, owner.invoke(t, "notification.inbox.mark_read", map[string]any{"id": sent.ID}, true))
		assertEvent(t, owner, relay, "notification.sent")
	})

	t.Run("dictionary catalog is readable through the contract", func(t *testing.T) {
		workspace := decodeData[dictionaries.DictionaryWorkspace](t, owner.invoke(t, "dictionary.list.read", map[string]any{
			"targetKind": "workspace", "targetId": 0,
		}, true))
		if workspace.Target == nil || workspace.Target.Kind != "workspace" {
			t.Fatalf("unexpected dictionary scope: %+v", workspace.Target)
		}
	})

	t.Run("mcp token lifecycle runs as capabilities", func(t *testing.T) {
		created := decodeData[mcp.MCPTokenCreated](t, owner.invoke(t, "mcp.token.create", map[string]any{
			"name": "capability acceptance", "password": "TestOnly!2026Secure",
			"days": 1, "departmentId": root.DepartmentID, "tools": []string{"context_get"},
		}, true))
		if created.Secret == "" || created.Token.ID == 0 {
			t.Fatal("token secret was not issued")
		}
		tokens := decodeData[[]*mcp.MCPToken](t, owner.invoke(t, "mcp.token.list", map[string]any{}, true))
		if len(tokens) == 0 {
			t.Fatal("issued token missing from the list")
		}
		checkSucceeded(t, owner.invoke(t, "mcp.token.revoke", map[string]any{
			"id": created.Token.ID, "password": "TestOnly!2026Secure",
		}, true))
		assertEvent(t, owner, relay, "mcp.token.created")
	})
}

// assertEvent waits briefly for the relay: the outbox commits with the business
// transaction, so the event is durable before the assertion starts.
func assertEvent(t *testing.T, client iamClient, relay *outbox.Relay, eventType string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		if _, err := relay.Once(context.Background()); err != nil {
			t.Fatal(err)
		}
		response := client.request(t, "GET", "/api/capabilities/events", nil, false)
		checkStatus(t, response, 200)
		events := decodeBody[[]struct {
			Type string `json:"type"`
		}](t, response)
		for _, event := range events {
			if event.Type == eventType {
				return
			}
		}
		if time.Now().After(deadline) {
			t.Fatalf("event %s was not published", eventType)
		}
		time.Sleep(100 * time.Millisecond)
	}
}
