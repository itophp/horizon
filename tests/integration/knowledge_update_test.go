//go:build integration

package tests

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"

	genai "horizon/foundation/gen/ai"
	gencapability "horizon/foundation/gen/capability"
	"horizon/internal/repository/dbgen"
)

func TestKnowledgeUpdatePermissionsAndFailedIndex(t *testing.T) {
	handler, pool, admin := iamFixture(t)
	root := loginIAM(t, handler, "root-admin")
	checkSucceeded(t, root.invoke(t, "identity.account.create", map[string]any{
		"username": "reader", "fullName": "Reader", "organizationId": admin.OrganizationID,
		"departmentId": admin.DepartmentID, "status": "active", "roleIds": []int64{},
		"overrides": []map[string]any{{"permission": "knowledge:read", "effect": "allow", "scope": "ALL"}},
		"password":  "TestOnly!2026Secure",
	}, true))
	reader := loginIAM(t, handler, "reader")
	q := dbgen.New(pool)
	insert := func(tenant string) int64 {
		t.Helper()
		id, err := q.KBCreateDocument(t.Context(), dbgen.KBCreateDocumentParams{
			TenantID: tenant, CreatedBy: admin.ID, Title: "Original", Content: "Original text", Source: "manual", Chunks: 1,
		})
		if err != nil {
			t.Fatal(err)
		}
		return id
	}
	localID, remoteID := insert("tenant-a"), insert("tenant-b")
	input := map[string]any{"id": localID, "expectedVersion": "", "title": "Updated", "content": "Updated text"}
	checkStatus(t, reader.invoke(t, "knowledge.update", input, true), http.StatusForbidden)
	checkKnowledgeUpdateFailure(t, root.invoke(t, "knowledge.update", input, true), "ai_embedding_incomplete")
	input["id"] = remoteID
	checkKnowledgeUpdateFailure(t, root.invoke(t, "knowledge.update", input, true), "not_found")
	row, err := q.KBReadDocument(t.Context(), dbgen.KBReadDocumentParams{TenantID: "tenant-a", ID: localID})
	if err != nil || row.Title != "Original" || row.Content != "Original text" || row.Version != "" {
		t.Fatalf("failed update changed the document: %v", err)
	}
}

func checkKnowledgeUpdateFailure(t *testing.T, response *httptest.ResponseRecorder, code string) {
	t.Helper()
	result := decodeBody[gencapability.CapabilityResult](t, response)
	if result.Status != "failed" || result.ErrorCode != code {
		t.Fatalf("expected %s, got status=%s code=%s", code, result.Status, result.ErrorCode)
	}
}

func TestKnowledgeUpdateReplacesIndex(t *testing.T) {
	redisURL := os.Getenv("TEST_KNOWLEDGE_REDIS_URL")
	if redisURL == "" {
		t.Skip("set TEST_KNOWLEDGE_REDIS_URL to a disposable Redis Stack to verify real vector replacement")
	}
	model := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request struct {
			Input []string `json:"input"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil || len(request.Input) == 0 {
			http.Error(w, "invalid test input", http.StatusBadRequest)
			return
		}
		data := make([]map[string]any, 0, len(request.Input))
		for i, item := range request.Input {
			if strings.Contains(item, "Rejected text") {
				http.Error(w, "test provider refused input", http.StatusBadRequest)
				return
			}
			data = append(data, map[string]any{"object": "embedding", "embedding": []float64{1, 0}, "index": i})
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"object": "list", "data": data})
	}))
	defer model.Close()
	handler, pool, _ := iamFixture(t)
	root := loginIAM(t, handler, "root-admin")
	saveAIConfig(t, root, 0, map[string]any{
		"knowledge.embedding.profile": "custom-vector", "knowledge.vector.url": redisURL,
		"aiModels.custom-vector.type": "embedding", "aiModels.custom-vector.protocol": "openai-embedding",
		"aiModels.custom-vector.displayName": "Test vector", "aiModels.custom-vector.providerId": "test",
		"aiModels.custom-vector.baseUrl": model.URL + "/v1", "aiModels.custom-vector.model": "test-embedding",
		"aiModels.custom-vector.dimensions": 2, "aiModels.custom-vector.token": "test-only",
	})
	type document struct {
		ID                      int64
		Title, Content, Version string
	}
	originalText := strings.Repeat("Original material. ", 80)
	created := decodeData[document](t, root.invoke(t, "knowledge.create", map[string]any{"title": "Original", "content": originalText}, true))
	read := func() document {
		t.Helper()
		return decodeData[document](t, root.invoke(t, "knowledge.read", map[string]any{"id": created.ID}, true))
	}
	search := func(wantTitle, wantContent string, wantCount int) {
		t.Helper()
		type searchResult struct{ Results []document }
		found := decodeData[searchResult](t, root.invoke(t, "knowledge.search", map[string]any{"query": "test question", "limit": 4}, true))
		if len(found.Results) != wantCount {
			t.Fatalf("got %d search hits, want %d", len(found.Results), wantCount)
		}
		for _, hit := range found.Results {
			if hit.Title != wantTitle || !strings.Contains(hit.Content, wantContent) {
				t.Fatal("search returned a different document generation")
			}
		}
	}
	search("Original", "Original material", 3)
	input := map[string]any{"id": created.ID, "expectedVersion": "", "title": "Updated", "content": "Updated text"}
	checkSucceeded(t, root.invoke(t, "knowledge.update", input, true))
	updated := read()
	if updated.Content != "Updated text" || updated.Title != "Updated" || updated.Version == "" {
		t.Fatal("updated document is missing its content or version")
	}
	search("Updated", "Updated text", 1)
	var eventCount int
	if err := pool.QueryRow(t.Context(), "SELECT count(*) FROM cap_outbox WHERE event_type = 'knowledge.document_updated'").Scan(&eventCount); err != nil || eventCount != 1 {
		t.Fatalf("expected one committed update event: count=%d err=%v", eventCount, err)
	}
	checkKnowledgeUpdateFailure(t, root.invoke(t, "knowledge.update", input, true), "conflict")
	input["expectedVersion"], input["content"] = updated.Version, "Rejected text"
	failed := decodeBody[gencapability.CapabilityResult](t, root.invoke(t, "knowledge.update", input, true))
	if failed.Status != "failed" || read() != updated {
		t.Fatal("provider failure committed the new document")
	}
	search("Updated", "Updated text", 1)
	checkSucceeded(t, root.invoke(t, "knowledge.reindex", map[string]any{}, true))
	search("Updated", "Updated text", 1)

	const proposedText = "AI review replacement"
	cardID := requestAIKnowledgeChange(t, root, created.ID, updated.Version, proposedText)
	if read() != updated {
		t.Fatal("the AI agent changed the document before approval")
	}
	var storedApproval string
	if err := pool.QueryRow(t.Context(), "SELECT arguments::text FROM cap_approvals WHERE id=$1", cardID).Scan(&storedApproval); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(storedApproval, "enc:v1:") || strings.Contains(storedApproval, proposedText) || strings.Contains(storedApproval, "Updated text") {
		t.Fatal("the proposed document content was not sealed")
	}
	cards := root.request(t, "GET", "/api/capabilities/approvals", nil, false)
	checkStatus(t, cards, http.StatusOK)
	if strings.Contains(cards.Body.String(), proposedText) || strings.Contains(cards.Body.String(), "enc:v1:") {
		t.Fatal("approval cards exposed private arguments")
	}
	card := decodeBody[[]gencapability.CapabilityApproval](t, cards)
	if len(card) != 1 || card[0].ID != cardID || card[0].Arguments.(map[string]any)["id"] != float64(created.ID) {
		t.Fatal("approval card does not identify the document")
	}
	checkSucceeded(t, root.request(t, "POST", "/api/capabilities/approvals/"+itoa(cardID)+"/decisions", map[string]any{"approve": true}, true))
	approved := read()
	if approved.Title != updated.Title || approved.Content != proposedText || approved.Version == updated.Version {
		t.Fatal("approval did not apply the exact replacement")
	}
	search("Updated", proposedText, 1)
}

func requestAIKnowledgeChange(t *testing.T, root iamClient, id int64, version, proposed string) int64 {
	t.Helper()
	var calls atomic.Int32
	chat := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request struct {
			Tools    []struct{ Function struct{ Name string } }
			Messages []struct{ Role, Content string }
		}
		if json.NewDecoder(r.Body).Decode(&request) != nil {
			http.Error(w, "bad test request", http.StatusBadRequest)
			return
		}
		hasTool := func(name string) bool {
			for _, tool := range request.Tools {
				if tool.Function.Name == name {
					return true
				}
			}
			return false
		}
		message := map[string]any{"role": "assistant", "content": ""}
		finish := "tool_calls"
		call := func(name string, input map[string]any) {
			raw, _ := json.Marshal(input)
			message["tool_calls"] = []map[string]any{{
				"id": "kb_" + name, "type": "function",
				"function": map[string]any{"name": name, "arguments": string(raw)},
			}}
		}
		switch calls.Add(1) {
		case 1:
			if !(hasTool("knowledge_query") && hasTool("knowledge_read") && hasTool("knowledge_update")) {
				t.Error("the AI agent did not advertise the knowledge editing tools")
			}
			authoritativeTools := false
			for _, item := range request.Messages {
				if item.Role == "system" && strings.Contains(item.Content, "tool list in the current request is authoritative") {
					authoritativeTools = true
				}
			}
			if !authoritativeTools {
				t.Error("the AI instruction did not tell the model to trust the current tool list")
			}
			call("knowledge_query", map[string]any{"keyword": "Updated text"})
		case 2:
			call("knowledge_read", map[string]any{"id": id})
		case 3:
			readVersion := false
			for _, item := range request.Messages {
				if item.Role == "tool" && strings.Contains(item.Content, version) && strings.Contains(item.Content, "Updated text") {
					readVersion = true
				}
			}
			if !readVersion {
				t.Error("the edit tool did not receive the current document version and text")
			}
			call("knowledge_update", map[string]any{
				"id": id, "expectedVersion": version,
				"replacement": map[string]any{"find": "Updated text", "text": proposed},
			})
		default:
			finish = "stop"
			message["content"] = "The document edit is waiting for approval."
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id": "agent-test", "object": "chat.completion", "model": "deepseek-flash",
			"choices": []map[string]any{{"index": 0, "message": message, "finish_reason": finish}},
		})
	}))
	defer chat.Close()
	saveAIConfig(t, root, 1, map[string]any{
		"agent.enabled": true, "aiModels.deepseek.baseUrl": chat.URL + "/v1", "aiModels.deepseek.token": "test-only-chat",
	})
	response := root.request(t, "POST", "/api/ai/runs", map[string]any{"question": "Edit one exact passage in the test document."}, true)
	checkStatus(t, response, http.StatusOK)
	run := decodeBody[genai.AIRun](t, response)
	if run.Status != "awaiting_approval" || calls.Load() != 4 {
		t.Fatalf("expected a complete AI run waiting for confirmation: status=%s turns=%d", run.Status, calls.Load())
	}
	cards := decodeBody[[]gencapability.CapabilityApproval](t, root.request(t, "GET", "/api/capabilities/approvals", nil, false))
	if len(cards) != 1 || cards[0].Capability != "knowledge.update" || cards[0].Status != "pending" {
		t.Fatal("missing pending knowledge update")
	}
	return cards[0].ID
}
