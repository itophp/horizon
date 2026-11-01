package app

import (
	"testing"

	"baseline/services/api/internal/platform/metadata"
)

// TestDeclaredEntitiesMatchSchema is the gate: every declared entity must line
// up with the columns the migrations actually produce. It runs with the normal
// unit tests, so a drifting declaration fails `make test` and not a review.
func TestDeclaredEntitiesMatchSchema(t *testing.T) {
	tables, err := metadata.Load()
	if err != nil {
		t.Fatalf("parse migrations: %v", err)
	}
	declared := Entities()
	if len(declared) < 18 {
		t.Fatalf("expected at least 18 declarations, got %d", len(declared))
	}
	for _, item := range declared {
		for _, problem := range item.Problems() {
			t.Errorf("%s", problem)
		}
	}
	for _, issue := range metadata.Errors(metadata.Validate(tables, declared)) {
		t.Errorf("%s", issue.String())
	}
}

func TestDeclaredEntitiesAreNamed(t *testing.T) {
	seen := map[string]bool{}
	for _, item := range Entities() {
		if item.Name == "" || item.Table == "" {
			t.Fatalf("entity without name or table: %+v", item)
		}
		if seen[item.Name] {
			t.Fatalf("entity %q declared twice", item.Name)
		}
		seen[item.Name] = true
	}
	for _, want := range []string{
		"example_record", "account", "session", "organization", "department", "role",
		"announcement", "notification", "notification_delivery",
		"file", "dictionary", "dictionary_item",
		"settings", "settings_draft", "settings_schedule", "settings_version",
		"mcp_token", "mcp_audit",
	} {
		if !seen[want] {
			t.Fatalf("entity %q missing from declarations: %v", want, seen)
		}
	}
}
