package harness

import (
	"strings"
	"testing"

	"baseline/services/api/internal/platform/capability/contracts"
)

func TestEntityContextProjectsWritableFieldsOnly(t *testing.T) {
	entity := contracts.Entity{
		Name: "widget",
		Attributes: []contracts.Attribute{
			{Name: "id", Kind: contracts.AttributeSystem, Type: contracts.TypeInteger, Required: true},
			{Name: "title", Kind: contracts.AttributeBusiness, Type: contracts.TypeString, Required: true},
			{Name: "state", Kind: contracts.AttributeBusiness, Type: contracts.TypeEnum, Enum: []string{"on", "off"}},
			{Name: "status", Kind: contracts.AttributeState, Type: contracts.TypeEnum, Enum: []string{"draft", "live"}},
			{Name: "parentId", Kind: contracts.AttributeReference, Type: contracts.TypeReference},
		},
		Transitions: []contracts.Transition{
			{To: "draft"},
			{From: []string{"draft"}, To: "live"},
		},
		Relations: []contracts.Relation{{Name: "parent", Target: "widget"}},
	}
	got := entityContext(entity)
	if strings.Join(got.Fields, ",") != "title*,state(on|off),parentId" {
		t.Fatalf("fields = %v", got.Fields)
	}
	if got.State != "status: draft|live" {
		t.Fatalf("state = %q", got.State)
	}
	if strings.Join(got.Transitions, ";") != "new->draft;draft->live" {
		t.Fatalf("transitions = %v", got.Transitions)
	}
	if strings.Join(got.Relations, ",") != "parent->widget" {
		t.Fatalf("relations = %v", got.Relations)
	}
	if got.SystemCount != 1 {
		t.Fatalf("system count = %d", got.SystemCount)
	}
}

func TestEntityContextHandlesEntityWithoutAttributes(t *testing.T) {
	got := entityContext(contracts.Entity{Name: "settings"})
	if got.Name != "settings" || len(got.Fields) != 0 || got.State != "" {
		t.Fatalf("unexpected projection: %+v", got)
	}
}
