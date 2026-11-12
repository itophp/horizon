package projects

import (
	"reflect"
	"testing"

	"horizon/foundation/kernel/plugin/plugintest"
)

func TestPluginContractPreservesModuleDeclarations(t *testing.T) {
	module := New(nil)
	manager := plugintest.Contract(t, Plugin{Module: module})
	direct := module.Entries()
	if len(manager.Catalog.Entries) != len(direct) {
		t.Fatalf("entry count: got %d want %d", len(manager.Catalog.Entries), len(direct))
	}
	for i, entry := range direct {
		got := manager.Catalog.Entries[i]
		if !reflect.DeepEqual(got.Definition, entry.Definition) || (got.Handler.Execute == nil) != (entry.Handler.Execute == nil) || (got.Handler.ExecuteTx == nil) != (entry.Handler.ExecuteTx == nil) {
			t.Fatalf("entry %d changed", i)
		}
	}
	if !reflect.DeepEqual(manager.Catalog.Entities, Entities()) {
		t.Fatal("entity declarations changed")
	}
	counts := map[string]int{}
	for _, extension := range manager.Catalog.Extensions {
		counts[extension.Type]++
	}
	for _, kind := range []string{"capability", "entity", "permission", "event"} {
		if counts[kind] == 0 {
			t.Errorf("missing %s extensions", kind)
		}
	}
}
