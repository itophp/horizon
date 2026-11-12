package drive

import (
	"testing"

	"horizon/foundation/kernel/capability/registry"
)

func TestCapabilityCatalog(t *testing.T) {
	r := registry.New(New(nil).Entries())
	if err := r.Err(); err != nil {
		t.Fatalf("invalid cloud drive catalog: %v", err)
	}
	r.RegisterEntities(Entities())
	if err := r.Err(); err != nil {
		t.Fatalf("invalid cloud drive entities: %v", err)
	}
}
