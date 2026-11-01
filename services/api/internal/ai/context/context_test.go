package aicontext

import (
	"strings"
	"testing"
)

func TestPromptRendersEntityBlock(t *testing.T) {
	context := Context{
		AccountName: "Ada", Username: "ada", DepartmentID: 3,
		Permissions: []string{"example:archive"},
		Page:        Page{Route: "/capabilities", Entity: "example_record", EntityID: 7},
		Entities: []EntityContext{
			{Name: "example_record", Fields: []string{"title*"}, State: "status: draft|active|archived",
				Transitions: []string{"new->draft", "draft|active->archived"}, SystemCount: 2},
			{Name: "account", Fields: []string{"username*"}, SystemCount: 5},
		},
	}
	prompt := context.Prompt()
	for _, want := range []string{
		"<entities>",
		"example_record | fields: title* | status: draft|active|archived",
		"transitions: new->draft; draft|active->archived",
		"+2 system",
		"account | fields: username*",
		"</entities>",
	} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("prompt missing %q:\n%s", want, prompt)
		}
	}
	if strings.Contains(prompt, "known_entities") {
		t.Fatalf("legacy entity line must be gone:\n%s", prompt)
	}
}

func TestPromptPutsPageEntityFirst(t *testing.T) {
	context := Context{
		Page: Page{Route: "/x", Entity: "account"},
		Entities: []EntityContext{
			{Name: "example_record"},
			{Name: "account"},
		},
	}
	prompt := context.Prompt()
	if strings.Index(prompt, "- account") > strings.Index(prompt, "- example_record") {
		t.Fatalf("page entity should render first:\n%s", prompt)
	}
}

func TestPromptBudgetStopsRendering(t *testing.T) {
	entities := make([]EntityContext, 0, 40)
	for i := 0; i < 40; i++ {
		entities = append(entities, EntityContext{
			Name:   strings.Repeat("e", 20),
			Fields: []string{strings.Repeat("f", 200)},
		})
	}
	prompt := Context{Entities: entities}.Prompt()
	if !strings.Contains(prompt, "(remaining entities omitted)") {
		t.Fatalf("budget not applied:\n%s", prompt)
	}
	if len([]rune(prompt)) > entityBlockBudget+400 {
		t.Fatalf("prompt exceeded budget: %d runes", len([]rune(prompt)))
	}
}

func TestEntityBlockIsSingleLine(t *testing.T) {
	context := Context{Entities: []EntityContext{{
		Name:   "bad\nentity",
		Fields: []string{"a\r\nb"},
	}}}
	prompt := context.Prompt()
	block := prompt[strings.Index(prompt, "<entities>"):]
	if strings.Contains(block, "bad\nentity") || strings.Contains(block, "a\r\nb") {
		t.Fatalf("newlines leaked into the block:\n%s", block)
	}
}
