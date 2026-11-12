package applicationruntime

import "testing"

func protocolApplication(pages ...PageV3) ApplicationV2 {
	return ApplicationV2{ID: "a", Source: "code", Protocol: &ApplicationV3{Pages: pages}}
}

func TestRebaseUsesProtocolStableIDs(t *testing.T) {
	base := protocolApplication(PageV3{ID: "one", TitleKey: "old"}, PageV3{ID: "two", TitleKey: "old"})
	current := Canonical(base)
	current.Protocol.Pages[0].TitleKey = "custom"
	next := Canonical(base)
	next.Version = "2.0.0"
	next.Protocol.Version = "2.0.0"
	next.Protocol.Pages[1].TitleKey = "deployed"
	merged, conflicts, err := RebaseDefinition(base, current, next)
	if err != nil || len(conflicts) != 0 || merged.Protocol.Pages[0].TitleKey != "custom" || merged.Protocol.Pages[1].TitleKey != "deployed" {
		t.Fatal(merged, conflicts, err)
	}
	next.Protocol.Pages[0].TitleKey = "different"
	_, conflicts, err = RebaseDefinition(base, current, next)
	if err != nil || len(conflicts) != 1 || conflicts[0] != "protocol.pages[one].titleKey" {
		t.Fatal(conflicts, err)
	}
}

func TestRebaseProtocolOrderingAndDeletionConflicts(t *testing.T) {
	base := protocolApplication(PageV3{ID: "one"}, PageV3{ID: "two"}, PageV3{ID: "three"})
	current := Canonical(base)
	current.Protocol.Pages[0], current.Protocol.Pages[1] = current.Protocol.Pages[1], current.Protocol.Pages[0]
	next := Canonical(base)
	next.Protocol.Pages = append(next.Protocol.Pages, PageV3{ID: "four"})
	merged, conflicts, err := RebaseDefinition(base, current, next)
	if err != nil || len(conflicts) != 0 || len(merged.Protocol.Pages) != 4 || merged.Protocol.Pages[0].ID != "two" || merged.Protocol.Pages[3].ID != "four" {
		t.Fatal(merged.Protocol.Pages, conflicts, err)
	}
	next.Protocol.Pages[1], next.Protocol.Pages[2] = next.Protocol.Pages[2], next.Protocol.Pages[1]
	_, conflicts, err = RebaseDefinition(base, current, next)
	if err != nil || len(conflicts) != 1 || conflicts[0] != "protocol.pages.order" {
		t.Fatal(conflicts, err)
	}
	current = Canonical(base)
	current.Protocol.Pages = current.Protocol.Pages[1:]
	next = Canonical(base)
	next.Protocol.Pages[0].TitleKey = "changed"
	_, conflicts, err = RebaseDefinition(base, current, next)
	if err != nil || len(conflicts) != 1 || conflicts[0] != "protocol.pages[one]" {
		t.Fatal(conflicts, err)
	}
}
