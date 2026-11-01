package metadata

import (
	"strings"
	"testing"

	"baseline/services/api/internal/platform/capability/contracts"
)

func sampleTables() []*Table {
	return []*Table{{
		Name: "widgets",
		Columns: []Column{
			{Name: "id", SQLType: "bigint", NotNull: true, PrimaryKey: true},
			{Name: "tenant_id", SQLType: "text", NotNull: true},
			{Name: "title", SQLType: "text", NotNull: true},
			{Name: "status", SQLType: "text", NotNull: true, Enum: []string{"draft", "done"}},
			{Name: "payload", SQLType: "jsonb", NotNull: true, Shape: "array"},
			{Name: "note", SQLType: "text"},
			{Name: "created_at", SQLType: "timestamptz", NotNull: true},
		},
	}}
}

func baseEntity() contracts.Entity {
	return contracts.Entity{
		Name:  "widget",
		Table: "widgets",
		Key:   "id",
		Attributes: []contracts.Attribute{
			{Name: "id", Kind: contracts.AttributeSystem, Type: contracts.TypeInteger, Required: true},
			{Name: "title", Kind: contracts.AttributeBusiness, Type: contracts.TypeString, Required: true},
			{Name: "status", Kind: contracts.AttributeState, Type: contracts.TypeEnum, Required: true,
				Enum: []string{"draft", "done"}},
			{Name: "payload", Kind: contracts.AttributeSystem, Type: contracts.TypeJSON, Shape: "array", Required: true},
			{Name: "note", Kind: contracts.AttributeBusiness, Type: contracts.TypeString, Nullable: true},
		},
		States: []contracts.State{{Name: "draft"}, {Name: "done", Terminal: true}},
	}
}

func codes(issues []Issue) string {
	out := make([]string, 0, len(issues))
	for _, issue := range issues {
		out = append(out, issue.Code)
	}
	return strings.Join(out, ",")
}

func TestValidatePassesForConsistentDeclarations(t *testing.T) {
	if issues := Validate(sampleTables(), []contracts.Entity{baseEntity()}); len(issues) != 0 {
		t.Fatalf("unexpected issues: %v", issues)
	}
}

func TestValidateReportsMissingTable(t *testing.T) {
	entity := baseEntity()
	entity.Table = "missing"
	issues := Validate(sampleTables(), []contracts.Entity{entity})
	if codes(issues) != "E1" {
		t.Fatalf("issues = %v", issues)
	}
}

func TestValidateReportsMissingColumn(t *testing.T) {
	entity := baseEntity()
	entity.Attributes = append(entity.Attributes, contracts.Attribute{
		Name: "ghost", Kind: contracts.AttributeBusiness, Type: contracts.TypeString,
	})
	issues := Validate(sampleTables(), []contracts.Entity{entity})
	if !strings.Contains(codes(issues), "E2") {
		t.Fatalf("issues = %v", issues)
	}
}

func TestValidateReportsUndeclaredColumn(t *testing.T) {
	entity := baseEntity()
	kept := entity.Attributes[:0]
	for _, attribute := range entity.Attributes {
		if attribute.Name != "title" {
			kept = append(kept, attribute)
		}
	}
	entity.Attributes = kept
	issues := Validate(sampleTables(), []contracts.Entity{entity})
	if codes(issues) != "E3" {
		t.Fatalf("issues = %v", issues)
	}
	// Housekeeping columns never need an attribute.
	for _, issue := range issues {
		if strings.Contains(issue.Message, "tenant_id") || strings.Contains(issue.Message, "created_at") {
			t.Fatalf("housekeeping column was reported: %v", issue)
		}
	}
}

func TestValidateReportsEnumDrift(t *testing.T) {
	entity := baseEntity()
	for i := range entity.Attributes {
		if entity.Attributes[i].Name == "status" {
			entity.Attributes[i].Enum = []string{"draft", "done", "archived"}
		}
	}
	issues := Validate(sampleTables(), []contracts.Entity{entity})
	if codes(issues) != "E4" {
		t.Fatalf("issues = %v", issues)
	}
}

func TestValidateReportsEnumDeclaredWithoutConstraint(t *testing.T) {
	entity := baseEntity()
	for i := range entity.Attributes {
		if entity.Attributes[i].Name == "title" {
			entity.Attributes[i].Type = contracts.TypeEnum
			entity.Attributes[i].Enum = []string{"a"}
		}
	}
	issues := Validate(sampleTables(), []contracts.Entity{entity})
	if !strings.Contains(codes(issues), "E4") {
		t.Fatalf("issues = %v", issues)
	}
}

func TestValidateReportsStateWithoutConstraint(t *testing.T) {
	tables := sampleTables()
	for i := range tables[0].Columns {
		if tables[0].Columns[i].Name == "status" {
			tables[0].Columns[i].Enum = nil
		}
	}
	entity := baseEntity()
	for i := range entity.Attributes {
		if entity.Attributes[i].Name == "status" {
			entity.Attributes[i].Type = contracts.TypeString
		}
	}
	issues := Validate(tables, []contracts.Entity{entity})
	if !strings.Contains(codes(issues), "E5") {
		t.Fatalf("issues = %v", issues)
	}
}

func TestValidateReportsJSONShapeDrift(t *testing.T) {
	entity := baseEntity()
	for i := range entity.Attributes {
		if entity.Attributes[i].Name == "payload" {
			entity.Attributes[i].Shape = "object"
		}
	}
	issues := Validate(sampleTables(), []contracts.Entity{entity})
	if codes(issues) != "E10" {
		t.Fatalf("issues = %v", issues)
	}
}

func TestEntityProblemsCatchesStructuralMistakes(t *testing.T) {
	entity := baseEntity()
	entity.Transitions = []contracts.Transition{{To: "unknown"}}
	entity.Relations = []contracts.Relation{{Name: "x", Target: "", Kind: "many"}}
	problems := strings.Join(entity.Problems(), "\n")
	for _, want := range []string{"undeclared state", "relation without name or target"} {
		if !strings.Contains(problems, want) {
			t.Fatalf("problems missing %q: %s", want, problems)
		}
	}
}

// relationTables builds a parent/child pair joined by a composite key, matching
// the shape every tenant scoped table uses.
func relationTables() []*Table {
	return []*Table{
		{
			Name: "owners",
			Columns: []Column{
				{Name: "id", SQLType: "bigint", NotNull: true, PrimaryKey: true},
				{Name: "tenant_id", SQLType: "text", NotNull: true},
			},
		},
		{
			Name: "toys",
			Columns: []Column{
				{Name: "id", SQLType: "bigint", NotNull: true, PrimaryKey: true},
				{Name: "tenant_id", SQLType: "text", NotNull: true},
				{Name: "owner_id", SQLType: "bigint", NotNull: true},
				{Name: "spare_id", SQLType: "bigint"},
			},
			ForeignKeys: []ForeignKey{{
				Name: "toys_tenant_id_owner_id_fkey", Columns: []string{"tenant_id", "owner_id"},
				TargetTable: "owners", TargetColumns: []string{"tenant_id", "id"},
			}},
		},
	}
}

func toyEntity(relations ...contracts.Relation) contracts.Entity {
	return contracts.Entity{
		Name: "toy", Table: "toys", Key: "id",
		Attributes: []contracts.Attribute{
			{Name: "id", Kind: contracts.AttributeSystem, Type: contracts.TypeInteger, Required: true},
			{Name: "ownerId", Kind: contracts.AttributeReference, Type: contracts.TypeReference, Required: true},
			{Name: "spareId", Kind: contracts.AttributeReference, Type: contracts.TypeReference, Nullable: true},
		},
		Relations: relations,
	}
}

func relationIssues(t *testing.T, tables []*Table, entities ...contracts.Entity) string {
	t.Helper()
	var out []string
	for _, issue := range Validate(tables, entities) {
		if issue.Code == "E6" {
			out = append(out, string(issue.Severity)+": "+issue.Message)
		}
	}
	return strings.Join(out, " | ")
}

func TestRelationPassesWhenForeignKeyMatches(t *testing.T) {
	tables := relationTables()
	entities := []contracts.Entity{
		toyEntity(contracts.Relation{Name: "owner", Target: "owner", Kind: "one", Via: "owner_id", Required: true}),
		{Name: "owner", Table: "owners", Key: "id", Attributes: []contracts.Attribute{
			{Name: "id", Kind: contracts.AttributeSystem, Type: contracts.TypeInteger, Required: true},
		}},
	}
	if got := relationIssues(t, tables, entities...); got != "" {
		t.Fatalf("expected no relation issues, got %s", got)
	}
}

func TestRelationWithoutForeignKeyFails(t *testing.T) {
	got := relationIssues(t, relationTables(), toyEntity(
		contracts.Relation{Name: "spare", Target: "owner", Kind: "one", Via: "spare_id"},
	))
	if !strings.Contains(got, "has no foreign key") {
		t.Fatalf("expected missing key error, got %q", got)
	}
}

func TestRelationPointingAtWrongEntityFails(t *testing.T) {
	entities := []contracts.Entity{
		toyEntity(contracts.Relation{Name: "owner", Target: "factory", Kind: "one", Via: "owner_id", Required: true}),
		{Name: "owner", Table: "owners", Key: "id", Attributes: []contracts.Attribute{
			{Name: "id", Kind: contracts.AttributeSystem, Type: contracts.TypeInteger, Required: true},
		}},
	}
	got := relationIssues(t, relationTables(), entities...)
	if !strings.Contains(got, "not factory") {
		t.Fatalf("expected target mismatch, got %q", got)
	}
}

func TestRequiredRelationOnNullableColumnFails(t *testing.T) {
	got := relationIssues(t, relationTables(), toyEntity(
		contracts.Relation{Name: "spare", Target: "owner", Kind: "one", Via: "spare_id", Required: true},
	))
	// spare_id has no key at all, so the missing-key error wins; assert the
	// nullability rule separately with a keyed but nullable column.
	tables := relationTables()
	tables[1].Columns[3].NotNull = false
	tables[1].ForeignKeys = append(tables[1].ForeignKeys, ForeignKey{
		Columns: []string{"tenant_id", "spare_id"}, TargetTable: "owners", TargetColumns: []string{"tenant_id", "id"},
	})
	got = relationIssues(t, tables, toyEntity(
		contracts.Relation{Name: "spare", Target: "owner", Kind: "one", Via: "spare_id", Required: true},
	))
	if !strings.Contains(got, "is nullable") {
		t.Fatalf("expected nullability error, got %q", got)
	}
}

func TestReverseRelationNeedsChildKey(t *testing.T) {
	owner := contracts.Entity{
		Name: "owner", Table: "owners", Key: "id",
		Attributes: []contracts.Attribute{
			{Name: "id", Kind: contracts.AttributeSystem, Type: contracts.TypeInteger, Required: true},
		},
		Relations: []contracts.Relation{{Name: "toys", Target: "toy", Kind: "many", Via: "toys"}},
	}
	toy := contracts.Entity{
		Name: "toy", Table: "toys", Key: "id",
		Attributes: []contracts.Attribute{
			{Name: "id", Kind: contracts.AttributeSystem, Type: contracts.TypeInteger, Required: true},
			{Name: "ownerId", Kind: contracts.AttributeReference, Type: contracts.TypeReference, Required: true},
			{Name: "spareId", Kind: contracts.AttributeReference, Type: contracts.TypeReference, Nullable: true},
		},
	}
	if got := relationIssues(t, relationTables(), owner, toy); got != "" {
		t.Fatalf("reverse relation is backed by the child key, got %q", got)
	}

	tables := relationTables()
	tables[1].ForeignKeys = nil
	got := relationIssues(t, tables, owner, toy)
	if !strings.Contains(got, "has no foreign key to") {
		t.Fatalf("expected reverse key error, got %q", got)
	}
}

func TestRelationThroughIdentityReferenceResolvesToAccount(t *testing.T) {
	tables := []*Table{
		{
			Name: "gov_identity_refs",
			Columns: []Column{
				{Name: "tenant_id", SQLType: "text", NotNull: true},
				{Name: "account_id", SQLType: "bigint", NotNull: true},
			},
		},
		{
			Name: "notes",
			Columns: []Column{
				{Name: "id", SQLType: "bigint", NotNull: true, PrimaryKey: true},
				{Name: "tenant_id", SQLType: "text", NotNull: true},
				{Name: "sender_id", SQLType: "bigint", NotNull: true},
			},
			ForeignKeys: []ForeignKey{{
				Columns:     []string{"tenant_id", "sender_id"},
				TargetTable: "gov_identity_refs", TargetColumns: []string{"tenant_id", "account_id"},
			}},
		},
	}
	note := contracts.Entity{
		Name: "note", Table: "notes", Key: "id",
		Attributes: []contracts.Attribute{
			{Name: "id", Kind: contracts.AttributeSystem, Type: contracts.TypeInteger, Required: true},
			{Name: "senderId", Kind: contracts.AttributeReference, Type: contracts.TypeReference, Required: true},
		},
		Relations: []contracts.Relation{{Name: "sender", Target: "account", Kind: "one", Via: "sender_id", Required: true}},
	}
	if got := relationIssues(t, tables, note); got != "" {
		t.Fatalf("gov_identity_refs must satisfy an account relation, got %s", got)
	}
}

func TestScopeColumnRelationIsHintNotError(t *testing.T) {
	tables := []*Table{
		{Name: "orgs", Columns: []Column{
			{Name: "id", SQLType: "bigint", NotNull: true, PrimaryKey: true},
			{Name: "tenant_id", SQLType: "text", NotNull: true},
		}},
		{Name: "depts", Columns: []Column{
			{Name: "id", SQLType: "bigint", NotNull: true, PrimaryKey: true},
			{Name: "tenant_id", SQLType: "text", NotNull: true},
			{Name: "organization_id", SQLType: "bigint", NotNull: true},
		}},
		{
			Name: "toys",
			Columns: []Column{
				{Name: "id", SQLType: "bigint", NotNull: true, PrimaryKey: true},
				{Name: "tenant_id", SQLType: "text", NotNull: true},
				{Name: "department_id", SQLType: "bigint", NotNull: true},
				{Name: "organization_id", SQLType: "bigint", NotNull: true},
			},
			ForeignKeys: []ForeignKey{{
				Columns:     []string{"tenant_id", "department_id", "organization_id"},
				TargetTable: "depts", TargetColumns: []string{"tenant_id", "id", "organization_id"},
			}},
		},
	}
	toy := contracts.Entity{
		Name: "toy", Table: "toys", Key: "id",
		Attributes: []contracts.Attribute{
			{Name: "id", Kind: contracts.AttributeSystem, Type: contracts.TypeInteger, Required: true},
			{Name: "departmentId", Kind: contracts.AttributeReference, Type: contracts.TypeReference, Required: true},
			{Name: "organizationId", Kind: contracts.AttributeReference, Type: contracts.TypeReference, Required: true},
		},
		Relations: []contracts.Relation{
			{Name: "organization", Target: "org", Kind: "one", Via: "organization_id", Required: true},
		},
	}
	org := contracts.Entity{
		Name: "org", Table: "orgs", Key: "id",
		Attributes: []contracts.Attribute{
			{Name: "id", Kind: contracts.AttributeSystem, Type: contracts.TypeInteger, Required: true},
		},
	}
	issues := Validate(tables, []contracts.Entity{toy, org})
	hints := 0
	for _, issue := range issues {
		if issue.Code != "E6" {
			continue
		}
		if issue.Severity != SeverityWarning || !strings.Contains(issue.Message, "only guards") {
			t.Fatalf("scope column should be a hint, got %s: %s", issue.Severity, issue.Message)
		}
		hints++
	}
	if hints != 1 {
		t.Fatalf("expected one hint, got %d: %v", hints, issues)
	}
	if len(Errors(issues)) != 0 {
		t.Fatalf("hint must not block the check: %v", Errors(issues))
	}
}
