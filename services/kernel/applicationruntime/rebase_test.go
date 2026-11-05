package applicationruntime

import "testing"

func TestRebaseUsesStableIDs(t *testing.T) {
	base := ApplicationV2{ID: "a", Source: "code", Pages: []PageV2{{ID: "one", TitleKey: "old"}, {ID: "two", TitleKey: "old"}}}
	current := Canonical(base)
	current.Pages[0].TitleKey = "custom"
	next := Canonical(base)
	next.Version = "2.0.0"
	next.Pages[1].TitleKey = "deployed"
	merged, conflicts, err := RebaseDefinition(base, current, next)
	if err != nil || len(conflicts) != 0 || merged.Pages[0].TitleKey != "custom" || merged.Pages[1].TitleKey != "deployed" {
		t.Fatal(merged, conflicts, err)
	}
	next.Pages[0].TitleKey = "different"
	_, conflicts, err = RebaseDefinition(base, current, next)
	if err != nil || len(conflicts) != 1 || conflicts[0] != "pages[one].titleKey" {
		t.Fatal(conflicts, err)
	}
}

func TestRebaseOrderingAndDeletionConflicts(t *testing.T) {
	base := ApplicationV2{ID: "a", Source: "code", Pages: []PageV2{{ID: "one"}, {ID: "two"}, {ID: "three"}}}
	current := Canonical(base)
	current.Pages[0], current.Pages[1] = current.Pages[1], current.Pages[0]
	next := Canonical(base)
	next.Pages = append(next.Pages, PageV2{ID: "four"})
	merged, conflicts, err := RebaseDefinition(base, current, next)
	if err != nil || len(conflicts) != 0 || len(merged.Pages) != 4 || merged.Pages[0].ID != "two" || merged.Pages[3].ID != "four" {
		t.Fatal(merged.Pages, conflicts, err)
	}
	next.Pages[1], next.Pages[2] = next.Pages[2], next.Pages[1]
	_, conflicts, err = RebaseDefinition(base, current, next)
	if err != nil || len(conflicts) != 1 || conflicts[0] != "pages.order" {
		t.Fatal(conflicts, err)
	}
	current = Canonical(base)
	current.Pages = current.Pages[1:]
	next = Canonical(base)
	next.Pages[0].TitleKey = "changed"
	_, conflicts, err = RebaseDefinition(base, current, next)
	if err != nil || len(conflicts) != 1 || conflicts[0] != "pages[one]" {
		t.Fatal(conflicts, err)
	}
}
