package main

import (
	"encoding/json"
	"testing"
)

func TestSchemaGuidesDescribeRegisteredDefinitions(t *testing.T) {
	c, _, err := loadCatalog()
	if err != nil {
		t.Fatal(err)
	}
	for _, kind := range schemaKinds {
		t.Run(kind, func(t *testing.T) {
			guide, err := schemaFor(kind)
			if err != nil {
				t.Fatal(err)
			}
			if guide.Description == "" || len(guide.Rules) == 0 {
				t.Fatal("missing schema guidance")
			}
			for name, property := range guide.Schema.Properties {
				if property.Description == "" {
					t.Errorf("missing field description for %s", name)
				}
			}
			resolved, err := guide.Schema.Resolve(nil)
			if err != nil {
				t.Fatal(err)
			}
			count := 0
			for _, entry := range c.Items {
				if entry.Kind != kind {
					continue
				}
				raw, err := json.Marshal(entry.Definition)
				if err != nil {
					t.Fatal(err)
				}
				var document any
				if err = json.Unmarshal(raw, &document); err != nil {
					t.Fatal(err)
				}
				if err = resolved.Validate(document); err != nil {
					t.Errorf("%s@%s: %v", entry.ID, entry.Version, err)
				}
				count++
			}
			if count == 0 {
				t.Fatal("no registered definitions to validate")
			}
		})
	}
}
