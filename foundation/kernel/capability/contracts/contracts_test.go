package contracts

import (
	"strings"
	"testing"
)

func base() Definition {
	return Definition{Layer: LayerAtomic,
		Name: "example.record.create", Version: "1.0.0", Title: "Create",
		Description: "creates", Domain: "example", Entity: "example_record",
		Permission: "example:create", Risk: RiskMedium, Approval: ApprovalAuto,
		Idempotency: Idempotency{Required: true, Scope: "tenant"}, SuccessEvents: []string{"example.record.created"},
		Executor: "go:example.create", Loggable: []string{"title"},
		Input: Object(map[string]*Schema{
			"title": {Type: "string", MinLength: intPointer(1), MaxLength: intPointer(10)},
			"items": {
				Type: "array", MaxItems: intPointer(1),
				Items: Object(map[string]*Schema{"code": {Type: "string", Enum: []string{"a", "b"}}}, "code"),
			},
		}, "title"),
	}
}

func intPointer(value int) *int { return &value }

func TestDefinitionValidation(t *testing.T) {
	if err := base().Validate(); err != nil {
		t.Fatalf("valid definition rejected: %v", err)
	}
	cases := map[string]func(*Definition){
		"missing input":       func(d *Definition) { d.Input = nil },
		"bad risk":            func(d *Definition) { d.Risk = "extreme" },
		"bad approval":        func(d *Definition) { d.Approval = "maybe" },
		"write without key":   func(d *Definition) { d.Idempotency.Required = false },
		"write without event": func(d *Definition) { d.SuccessEvents = nil },
	}
	for name, mutate := range cases {
		definition := base()
		mutate(&definition)
		if err := definition.Validate(); err == nil {
			t.Fatalf("%s must be rejected", name)
		}
	}
}

func TestSchemaValidation(t *testing.T) {
	schema := base().Input
	if err := schema.Validate(Input{"title": "ok", "items": []any{map[string]any{"code": "a"}}}); err != nil {
		t.Fatalf("valid input rejected: %v", err)
	}
	invalid := map[string]Input{
		"missing required": {"items": []any{}},
		"unknown field":    {"title": "ok", "extra": true},
		"too long":         {"title": strings.Repeat("x", 11)},
		"bad enum":         {"title": "ok", "items": []any{map[string]any{"code": "c"}}},
		"too many items":   {"title": "ok", "items": []any{map[string]any{"code": "a"}, map[string]any{"code": "b"}}},
	}
	for name, input := range invalid {
		if err := schema.Validate(input); err == nil {
			t.Fatalf("%s must be rejected", name)
		}
	}
}

func TestToolNameAndDigest(t *testing.T) {
	definition := base()
	if got := definition.ToolName(); got != "example_record_create" {
		t.Fatalf("unexpected tool name %q", got)
	}
	definition.Name = strings.Repeat("a", 80)
	if got := definition.ToolName(); len(got) != 64 {
		t.Fatalf("tool name must be capped, got %d", len(got))
	}
	if Digest(Input{"a": "1", "b": "2"}) != Digest(Input{"b": "2", "a": "1"}) {
		t.Fatal("digest must be canonical")
	}
	if Digest(Input{"a": "1"}) == Digest(Input{"a": "2"}) {
		t.Fatal("different payloads must differ")
	}
}

func TestRedactKeepsAllowlistOnly(t *testing.T) {
	definition := base()
	redacted := definition.Redact(Input{"title": strings.Repeat("x", 300), "items": []any{}})
	if len(redacted) != 1 {
		t.Fatalf("unexpected projection %+v", redacted)
	}
	if length := len([]rune(redacted["title"].(string))); length != 200 {
		t.Fatalf("loggable strings must be truncated, got %d", length)
	}
}

func TestProblemsNameTheOffendingField(t *testing.T) {
	valid := base()
	if got := valid.Problems(); len(got) != 0 {
		t.Fatalf("valid definition reported problems: %v", got)
	}

	// The primary entity must appear in Entities; the message has to name it,
	// otherwise a failed check only says "invalid" and the caller guesses.
	broken := base()
	broken.Entities = []string{"example_record", "file"}
	broken.Entity = "example_record"
	if got := broken.Problems(); len(got) != 0 {
		t.Fatalf("primary entity inside Entities must pass: %v", got)
	}
	broken.Entities = []string{"file"}
	problems := broken.Problems()
	if len(problems) != 1 || !strings.Contains(problems[0], "example_record") || !strings.Contains(problems[0], "Entities") {
		t.Fatalf("problems = %v", problems)
	}
	if err := broken.Validate(); err == nil {
		t.Fatal("Validate must reject a definition with problems")
	}

	cases := []struct {
		name   string
		mutate func(*Definition)
		want   string
	}{
		{"name", func(d *Definition) { d.Name = "" }, "name and version"},
		{"input", func(d *Definition) { d.Input = nil }, "object schema"},
		{"risk", func(d *Definition) { d.Risk = "extreme" }, "risk must be"},
		{"approval", func(d *Definition) { d.Approval = "sometimes" }, "approval must be"},
		{"idempotency", func(d *Definition) { d.Idempotency = Idempotency{} }, "idempotency key"},
		{"events", func(d *Definition) { d.SuccessEvents = nil }, "success event"},
	}
	for _, test := range cases {
		definition := base()
		test.mutate(&definition)
		joined := strings.Join(definition.Problems(), "; ")
		if !strings.Contains(joined, test.want) {
			t.Fatalf("%s: problems = %q, want %q", test.name, joined, test.want)
		}
	}

	// A read only capability legitimately has neither key nor events.
	read := base()
	read.ReadOnly = true
	read.Idempotency = Idempotency{}
	read.SuccessEvents = nil
	if got := read.Problems(); len(got) != 0 {
		t.Fatalf("read only definition reported problems: %v", got)
	}
}

func TestTaxonomyIsConservative(t *testing.T) {
	d := base()
	d.Layer = ""
	if d.Validate() == nil {
		t.Fatal("missing layer accepted")
	}
	d = base().Normalized()
	if d.Effect != EffectIrreversible || d.RetrySafe {
		t.Fatal("committed write treated as reversible or retry safe")
	}
	d.ReadOnly = true
	d.Writes = nil
	d.Effect = ""
	d = d.Normalized()
	if d.Effect != EffectNone || !d.RetrySafe {
		t.Fatal("read defaults incorrect")
	}
}
