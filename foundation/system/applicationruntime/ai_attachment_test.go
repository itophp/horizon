package applicationruntime

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"horizon/foundation/ai/automation"
	gen "horizon/foundation/gen/identity"
	ar "horizon/foundation/kernel/applicationruntime"
	"horizon/foundation/kernel/capability/contracts"
	"horizon/foundation/system/identity"
)

func TestRuntimeAIFormUsesAttachmentContent(t *testing.T) {
	app := automation.RuntimeApplication()
	module := &Module{Definitions: map[string]contracts.Definition{}}
	for _, entry := range automation.New(nil).Entries() {
		module.Definitions[entry.Definition.Key()] = entry.Definition
	}
	principal, err := identity.NewPrincipal(&gen.AccountRecord{ID: 1, IsAdmin: true}, "test-tenant", nil)
	if err != nil {
		t.Fatal(err)
	}
	component, bindings := ar.FindAIComponent(app.Protocol.Pages[0].Root, "automation-task-list")
	input := contracts.Input{
		"sceneId": "automation.task.form", "mode": "create", "recordId": 0, "draft": map[string]any{},
	}
	const suggestion = `{"suggestions":[{"field":"description","value":"Summarize weekly sales."}]}`
	var modelData map[string]any
	module.GenerateAI = func(_ context.Context, _, data string) (string, error) {
		if err := json.Unmarshal([]byte(data), &modelData); err != nil {
			t.Fatal(err)
		}
		return suggestion, nil
	}

	var word bytes.Buffer
	writer := zip.NewWriter(&word)
	for name, content := range map[string]string{
		"[Content_Types].xml": "<Types/>",
		"word/document.xml":   `<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:body><w:p><w:r><w:t>Weekly sales</w:t></w:r></w:p></w:body></w:document>`,
	} {
		part, err := writer.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := part.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	excel, err := os.ReadFile("../../ai/workbook/testdata/projects-ai-import.xlsx")
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range []struct {
		name string
		raw  []byte
		want string
	}{
		{"weekly.docx", word.Bytes(), "Weekly sales"},
		{"weekly.xlsx", excel, "WEEK-201"},
		{"fake.docx", []byte("not a document"), ""},
	} {
		t.Run(file.name, func(t *testing.T) {
			modelData = nil
			input["attachment"] = map[string]any{
				"name": file.name, "content": base64.StdEncoding.EncodeToString(file.raw),
			}
			_, err := module.suggestAI(t.Context(), principal, app, component, bindings, input)
			if file.want == "" {
				if err == nil || modelData != nil {
					t.Fatal("invalid file reached the model")
				}
				return
			}
			content, _ := modelData["attachmentText"].(string)
			if err != nil || !strings.Contains(content, file.want) {
				t.Fatalf("model received %v, suggestion failed with %v", modelData, err)
			}
		})
	}

	t.Run("multimodal image", func(t *testing.T) {
		const png = "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+/l0QAAAAASUVORK5CYII="
		input["attachment"] = map[string]any{"name": "chart.png", "content": png}
		called := false
		module.GenerateAIWithImage = func(_ context.Context, _, _, mime, content string) (string, error) {
			called = true
			if mime != "image/png" || content != png {
				t.Fatal("model did not receive the attached image")
			}
			return suggestion, nil
		}
		if _, err := module.suggestAI(t.Context(), principal, app, component, bindings, input); err != nil || !called {
			t.Fatalf("image generation failed: %v", err)
		}
	})
}
