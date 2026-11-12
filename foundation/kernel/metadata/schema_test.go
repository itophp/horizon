package metadata

import (
	"fmt"
	"os"
	"strings"
	"testing"
	"testing/fstest"
)

func loadTables(t *testing.T) map[string]*Table {
	t.Helper()
	tables, err := Load()
	if err != nil {
		t.Fatalf("load migrations: %v", err)
	}
	if len(tables) == 0 {
		t.Fatal("no tables parsed")
	}
	index := map[string]*Table{}
	for _, table := range tables {
		index[table.Name] = table
	}
	return index
}

func mustTable(t *testing.T, index map[string]*Table, name string) *Table {
	t.Helper()
	table, ok := index[name]
	if !ok {
		t.Fatalf("table %q missing; parsed %d tables", name, len(index))
	}
	return table
}

func TestExampleRecordsColumns(t *testing.T) {
	table := mustTable(t, loadTables(t), "example_records")
	want := []string{"id", "tenant_id", "title", "status", "created_by", "created_at", "updated_at"}
	if got := table.Names(); strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("columns = %v, want %v", got, want)
	}
	id, _ := table.Column("id")
	if !id.PrimaryKey || !id.Identity || !id.NotNull {
		t.Fatalf("id flags = %+v", id)
	}
	title, _ := table.Column("title")
	if title.MaxLength != 200 {
		t.Fatalf("title.MaxLength = %d, want 200", title.MaxLength)
	}
	status, _ := table.Column("status")
	if strings.Join(status.Enum, ",") != "draft,active,archived" {
		t.Fatalf("status enum = %v", status.Enum)
	}
	if status.NotNull != true {
		t.Fatalf("status should be NOT NULL")
	}
}

func TestMultiColumnLineIsSplit(t *testing.T) {
	table := mustTable(t, loadTables(t), "iam_accounts")
	for _, name := range []string{"id", "tenant_id", "username", "full_name", "password_hash", "is_admin"} {
		if _, ok := table.Column(name); !ok {
			t.Fatalf("column %q missing from iam_accounts (%v)", name, table.Names())
		}
	}
	status, _ := table.Column("status")
	if strings.Join(status.Enum, ",") != "active,inactive" {
		t.Fatalf("status enum = %v", status.Enum)
	}
}

func TestAlterTableApplied(t *testing.T) {
	table := mustTable(t, loadTables(t), "ai_runs")
	for _, name := range []string{"origin", "correlation_id", "causation_id", "trigger_depth", "trigger_policy", "event_id"} {
		column, ok := table.Column(name)
		if !ok {
			t.Fatalf("column %q missing after ALTER TABLE (%v)", name, table.Names())
		}
		if !column.NotNull {
			t.Fatalf("column %q should be NOT NULL", name)
		}
	}
	status, _ := table.Column("status")
	if len(status.Enum) != 5 {
		t.Fatalf("ai_runs.status enum = %v", status.Enum)
	}
}

func TestDroppedTablesAbsent(t *testing.T) {
	index := loadTables(t)
	for _, name := range []string{"ai_run_steps", "ai_approvals"} {
		if _, ok := index[name]; ok {
			t.Fatalf("table %q was dropped in the forward migrations but still parsed", name)
		}
	}
}

func TestGovernanceChecks(t *testing.T) {
	index := loadTables(t)
	deliveries := mustTable(t, index, "gov_deliveries")
	status, _ := deliveries.Column("status")
	if strings.Join(status.Enum, ",") != "pending,processing,sent,failed,skipped" {
		t.Fatalf("gov_deliveries.status = %v", status.Enum)
	}
	if _, ok := deliveries.Column("lease_until"); !ok {
		t.Fatal("gov_deliveries.lease_until missing")
	}
	schedules := mustTable(t, index, "gov_settings_schedules")
	scheduleStatus, _ := schedules.Column("status")
	if len(scheduleStatus.Enum) != 4 {
		t.Fatalf("gov_settings_schedules.status = %v", scheduleStatus.Enum)
	}
	temporary := mustTable(t, index, "iam_temporary_requests")
	temporaryStatus, _ := temporary.Column("status")
	if len(temporaryStatus.Enum) != 5 {
		t.Fatalf("iam_temporary_requests.status = %v", temporaryStatus.Enum)
	}
}

func TestJSONShapeAndForeignKey(t *testing.T) {
	index := loadTables(t)
	roles := mustTable(t, index, "iam_roles")
	grants, ok := roles.Column("grants")
	if !ok {
		t.Fatal("iam_roles.grants missing")
	}
	if grants.SQLType != "jsonb" || grants.Shape != "array" {
		t.Fatalf("grants type=%q shape=%q", grants.SQLType, grants.Shape)
	}
	if !grants.NotNull {
		t.Fatal("grants should be NOT NULL")
	}
	accounts := mustTable(t, index, "iam_accounts")
	overrides, ok := accounts.Column("overrides")
	if !ok || overrides.Shape != "array" {
		t.Fatalf("overrides = %+v", overrides)
	}
}

func TestGooseDownIsIgnored(t *testing.T) {
	fsys := fstest.MapFS{
		"00001_test.sql": &fstest.MapFile{Data: []byte(`
-- +goose Up
CREATE TABLE kept (id bigint PRIMARY KEY, note text NOT NULL,
  state text NOT NULL CHECK (state IN ('a','b')));
-- +goose Down
DROP TABLE kept;
CREATE TABLE resurrected (id bigint);
`)},
	}
	tables, err := Parse(fsys)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(tables) != 1 || tables[0].Name != "kept" {
		t.Fatalf("tables = %v", tableNames(tables))
	}
	note, _ := tables[0].Column("note")
	if note == nil || !note.NotNull {
		t.Fatalf("note = %+v", note)
	}
	state, _ := tables[0].Column("state")
	if strings.Join(state.Enum, ",") != "a,b" {
		t.Fatalf("state enum = %v", state.Enum)
	}
}

func TestAlterDropAndDefaultInFixture(t *testing.T) {
	fsys := fstest.MapFS{
		"00001_test.sql": &fstest.MapFile{Data: []byte(`
-- +goose Up
CREATE TABLE demo (id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  amount numeric(12,2) NOT NULL DEFAULT 0,
  payload jsonb NOT NULL DEFAULT '{}'::jsonb CHECK (jsonb_typeof(payload) = 'object'),
  gone text NOT NULL DEFAULT '');
ALTER TABLE demo ADD COLUMN origin text NOT NULL DEFAULT 'human-ui';
ALTER TABLE demo DROP COLUMN gone;
ALTER TABLE demo ALTER COLUMN amount DROP NOT NULL;
`)},
	}
	tables, err := Parse(fsys)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	table := tables[0]
	if _, ok := table.Column("gone"); ok {
		t.Fatal("gone should be dropped")
	}
	origin, ok := table.Column("origin")
	if !ok || !origin.NotNull || origin.Default != "'human-ui'" {
		t.Fatalf("origin = %+v", origin)
	}
	amount, _ := table.Column("amount")
	if amount.NotNull {
		t.Fatal("amount should be nullable after ALTER COLUMN DROP NOT NULL")
	}
	if amount.SQLType != "numeric" {
		t.Fatalf("amount type = %q", amount.SQLType)
	}
	payload, _ := table.Column("payload")
	if payload.Shape != "object" {
		t.Fatalf("payload shape = %q", payload.Shape)
	}
}

func TestViewsAreNotTables(t *testing.T) {
	// 00008 defines two reporting views next to the tables it creates. They are
	// not base tables and must not be offered as entity targets.
	index := loadTables(t)
	for _, name := range []string{"gov_notification_members", "gov_notification_targets"} {
		if _, ok := index[name]; ok {
			t.Fatalf("view %q must not be parsed as a table", name)
		}
	}
}

func TestCompositeForeignKeyResolvesPointerNotScopeColumn(t *testing.T) {
	index := loadTables(t)

	// iam_accounts references iam_organizations on (tenant_id, organization_id).
	accounts := mustTable(t, index, "iam_accounts")
	if target, ok := accounts.ParentOf("organization_id"); !ok || target != "iam_organizations" {
		t.Fatalf("ParentOf(organization_id) = %q, %v", target, ok)
	}

	// gov_files carries organization_id only as a guard inside the department
	// key, so it must not resolve to a direct organization reference.
	files := mustTable(t, index, "gov_files")
	if target, ok := files.ParentOf("organization_id"); ok {
		t.Fatalf("organization_id should only guard the key, got %q", target)
	}
	if target, ok := files.ScopeTarget("organization_id"); !ok || target != "iam_departments" {
		t.Fatalf("ScopeTarget(organization_id) = %q, %v", target, ok)
	}
	if target, ok := files.ParentOf("department_id"); !ok || target != "iam_departments" {
		t.Fatalf("ParentOf(department_id) = %q, %v", target, ok)
	}
}

func TestAlterAddForeignKeyIsApplied(t *testing.T) {
	// gov_notifications gains announcement_id in 00008 with ALTER TABLE ... ADD
	// FOREIGN KEY, not inside CREATE TABLE.
	notifications := mustTable(t, loadTables(t), "gov_notifications")
	if target, ok := notifications.ParentOf("announcement_id"); !ok || target != "gov_announcements" {
		t.Fatalf("ParentOf(announcement_id) = %q, %v", target, ok)
	}
}

func TestDroppedConstraintStopsResolving(t *testing.T) {
	// 00006 drops the iam_accounts keys and re-points them at gov_identity_refs.
	// The parser must follow the drop, otherwise the old target wins.
	files := mustTable(t, loadTables(t), "gov_files")
	if target, ok := files.ParentOf("owner_id"); !ok || target != "gov_identity_refs" {
		t.Fatalf("ParentOf(owner_id) = %q, %v", target, ok)
	}
	if got := len(files.ForeignKeys); got != 3 {
		t.Fatalf("gov_files should keep 3 keys after the re-point, got %d", got)
	}
}

func TestForeignKeyDeclarationKeepsConstraintName(t *testing.T) {
	fsys := fstest.MapFS{"00001.sql": {Data: []byte(`
-- +goose Up
CREATE TABLE parents (id bigint PRIMARY KEY, tenant_id text NOT NULL);
CREATE TABLE children (
  id bigint PRIMARY KEY,
  tenant_id text NOT NULL,
  parent_id bigint NOT NULL,
  CONSTRAINT children_parent_fk FOREIGN KEY(tenant_id, parent_id) REFERENCES parents(tenant_id, id)
);
ALTER TABLE children DROP CONSTRAINT children_parent_fk;
ALTER TABLE children ADD CONSTRAINT children_parent_fk FOREIGN KEY(tenant_id, parent_id) REFERENCES parents(tenant_id, id) ON DELETE CASCADE;
-- +goose Down
DROP TABLE children;
`)}}
	tables, err := Parse(fsys)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	var children *Table
	for _, table := range tables {
		if table.Name == "children" {
			children = table
		}
	}
	if children == nil {
		t.Fatal("children table missing")
	}
	if len(children.ForeignKeys) != 1 {
		t.Fatalf("re-adding under the same name must not duplicate: %+v", children.ForeignKeys)
	}
	if children.ForeignKeys[0].Name != "children_parent_fk" {
		t.Fatalf("constraint name = %q", children.ForeignKeys[0].Name)
	}
}

func TestDumpSchema(t *testing.T) {
	if os.Getenv("ENTITY_DUMP") == "" {
		t.Skip("set ENTITY_DUMP=1 to print the parsed schema")
	}
	tables, err := Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	columns := 0
	for _, table := range tables {
		columns += len(table.Columns)
	}
	fmt.Printf("tables=%d columns=%d\n", len(tables), columns)
	for _, table := range tables {
		names := make([]string, 0, len(table.Columns))
		for _, column := range table.Columns {
			names = append(names, column.Name+":"+column.SQLType)
		}
		fmt.Printf("%-28s %2d  %s\n", table.Name, len(table.Columns), strings.Join(names, " "))
	}
}

// TestDumpForeignKeys prints the parsed keys so they can be diffed against
// information_schema or pg_constraint by hand. It is how the parser was
// validated against the live database.
func TestDumpForeignKeys(t *testing.T) {
	if os.Getenv("FK_DUMP") == "" {
		t.Skip("set FK_DUMP=1 to print the parsed foreign keys")
	}
	tables, err := Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	for _, table := range tables {
		for _, fk := range table.ForeignKeys {
			fmt.Printf("%s\t%s\t%s(%s)\n",
				table.Name, strings.Join(fk.Columns, ","), fk.TargetTable, strings.Join(fk.TargetColumns, ","))
		}
	}
}

func tableNames(tables []*Table) []string {
	out := make([]string, 0, len(tables))
	for _, table := range tables {
		out = append(out, table.Name)
	}
	return out
}

func TestConstraintKeywordRequiresTokenBoundary(t *testing.T) {
	for _, column := range []string{"checkpoint_key text NOT NULL", "unique_label text", "excluded_count integer"} {
		if isConstraint(column) {
			t.Fatalf("column parsed as constraint: %s", column)
		}
	}
	for _, constraint := range []string{"CHECK (length(payload)>0)", "UNIQUE(id)", "PRIMARY KEY (id)"} {
		if !isConstraint(constraint) {
			t.Fatalf("constraint parsed as column: %s", constraint)
		}
	}
}
