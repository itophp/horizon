package contracts

import "testing"

func TestAccessSchemaIsDerivedAndGatedOutputMustBeOptional(t *testing.T) {
	schema := Object(map[string]*Schema{"title": Str(1, 100), "private": Str(0, 100)}, "title")
	rules := []FieldAccess{{Path: "private", Permission: "articles:review"}}
	result := AuthorizedSchema(schema, rules, func(string) bool { return false })
	if result == nil || result.Properties["private"] != nil || schema.Properties["private"] == nil {
		t.Fatal("projection drift")
	}
	d := Definition{Output: schema, OutputAccess: rules}
	if d.validateAccess() != nil {
		t.Fatal("valid policy rejected")
	}
	schema.Required = append(schema.Required, "private")
	if d.validateAccess() == nil {
		t.Fatal("required conditional output accepted")
	}
	if AuthorizedSchema(schema, rules, func(string) bool { return false }) != nil {
		t.Fatal("required denied input still discoverable")
	}
}
