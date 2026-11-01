package orchestration

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestGeneratedWorkflowSpecificationRejectsMalformedDeclarations(t *testing.T) {
	raw, err := json.Marshal(LifecycleExample())
	if err != nil {
		t.Fatal(err)
	}
	if _, err = DecodeDefinition(raw); err != nil {
		t.Fatal(err)
	}
	cases := map[string]func(map[string]any){
		"unknown workflow field": func(v map[string]any) { v["executor"] = "shell" },
		"null steps":             func(v map[string]any) { v["steps"] = nil },
		"unknown node field":     func(v map[string]any) { v["steps"].([]any)[0].(map[string]any)["bypassGateway"] = true },
		"unbounded poll": func(v map[string]any) {
			v["steps"].([]any)[1].(map[string]any)["poll"].(map[string]any)["maxSteps"] = MaxPollSteps + 1
		},
		"unpinned agent tool": func(v map[string]any) {
			v["steps"].([]any)[2].(map[string]any)["agent"].(map[string]any)["tools"] = []any{"example.record.get"}
		},
		"mixed node kinds": func(v map[string]any) {
			v["steps"].([]any)[0].(map[string]any)["agent"] = map[string]any{"instruction": "do", "tools": []any{"example.record.get@1.0.0"}, "maxIterations": 3}
		},
		"unsupported input keyword": func(v map[string]any) { v["input"].(map[string]any)["$ref"] = "https://untrusted.invalid/schema" },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			var v map[string]any
			if err = json.Unmarshal(raw, &v); err != nil {
				t.Fatal(err)
			}
			mutate(v)
			invalid, err := json.Marshal(v)
			if err != nil {
				t.Fatal(err)
			}
			if _, err = DecodeDefinition(invalid); err == nil {
				t.Fatal("invalid declaration accepted")
			}
		})
	}
}
func TestGeneratedWorkflowArtifactsMatchRuntime(t *testing.T) {
	schema, err := SpecificationSchema()
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.MarshalIndent(schema, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	checked, err := os.ReadFile("../../../../../docs/generated/workflows/workflow.schema.json")
	if err != nil {
		t.Fatal(err)
	}
	if string(checked) != string(append(raw, '\n')) {
		t.Fatal("workflow schema drift; run make generate-workflows")
	}
	for _, d := range Builtins() {
		raw, err := os.ReadFile(filepath.Join("../../../../../docs/generated/workflows", d.Name+".workflow.json"))
		if err != nil {
			t.Fatal(err)
		}
		decoded, err := DecodeDefinition(raw)
		if err != nil {
			t.Fatal(err)
		}
		if decoded.Hash() != d.Hash() {
			t.Fatalf("generated definition drift: %s", d.Name)
		}
	}
}
