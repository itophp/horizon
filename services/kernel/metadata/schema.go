// Package metadata owns the business entity model: the derived catalog, the
// physical schema read from the embedded Goose migrations, and the checks that
// keep the two aligned.
//
// It exists so the business entity model can be validated against the database
// instead of being maintained by hand: migrations stay the only source of truth
// for columns, constraints and foreign keys, and everything declared in
// internal/modules/*/entities.go is checked against what this package reads.
//
// The parser is deliberately small but must handle the shapes this repository
// actually uses: several columns on one line, ALTER TABLE ADD/DROP COLUMN,
// CHECK constraints containing commas, and the `-- +goose Down` section which
// repeats DDL that must never be applied to the forward schema.
package metadata

import (
	"fmt"
	"io/fs"
	"regexp"
	"sort"
	"strings"

	migrations "baseline/services/api/db/migrations"
)

// Column is one physical column as declared by the migrations.
type Column struct {
	Name       string
	SQLType    string   // normalized base type, e.g. "text", "bigint", "timestamptz"
	Array      bool     // declared as T[]
	NotNull    bool     // NOT NULL
	PrimaryKey bool     // column level or table level primary key
	Identity   bool     // GENERATED ALWAYS AS IDENTITY
	Default    string   // literal or expression following DEFAULT
	Enum       []string // CHECK (col IN (...))
	Shape      string   // CHECK (jsonb_typeof(col) = 'array'|'object')
	MaxLength  int      // 0 when unbounded
	References string   // "table.column" from an inline or table level foreign key
}

// ForeignKey is one declared foreign key. Tenant scoped tables reference their
// parent on (tenant_id, id), so a relation is usually a composite: the tenant
// column keeps the reference inside one tenant and the remaining columns point
// at the row.
type ForeignKey struct {
	Name          string   // constraint name; future ALTERs address it by name
	Columns       []string // local columns, in declaration order
	TargetTable   string
	TargetColumns []string // target columns, aligned with Columns
}

// Table is one table with its final column set.
type Table struct {
	Name        string
	Columns     []Column
	ForeignKeys []ForeignKey
}

// ParentOf resolves the row a local column points at.
//
// Reference keys in this schema are composite because they carry the tenant:
// (tenant_id, x_id) references t(tenant_id, id). A pair whose two column names
// are equal is a scope guard, not the pointer, so it is skipped. Otherwise the
// pair whose names differ is the reference.
func (t *Table) ParentOf(column string) (string, bool) {
	if t == nil {
		return "", false
	}
	for _, fk := range t.ForeignKeys {
		for i, local := range fk.Columns {
			if local != column || i >= len(fk.TargetColumns) || fk.TargetColumns[i] == column {
				continue
			}
			return fk.TargetTable, true
		}
	}
	for _, fk := range t.ForeignKeys {
		if len(fk.Columns) == 1 && fk.Columns[0] == column {
			return fk.TargetTable, true
		}
	}
	return "", false
}

// ScopeTarget reports the table a column only guards, not references. When a
// key carries (organization_id) on both sides, the column keeps the row inside
// the same organization but the pointer itself is another column of that key.
func (t *Table) ScopeTarget(column string) (string, bool) {
	if t == nil {
		return "", false
	}
	for _, fk := range t.ForeignKeys {
		for i, local := range fk.Columns {
			if local == column && i < len(fk.TargetColumns) && fk.TargetColumns[i] == column {
				return fk.TargetTable, true
			}
		}
	}
	return "", false
}

// ChildrenOf lists the local columns that point at the given table.
func (t *Table) ChildrenOf(table string) []string {
	if t == nil {
		return nil
	}
	var out []string
	for _, fk := range t.ForeignKeys {
		if fk.TargetTable != table {
			continue
		}
		for i, local := range fk.Columns {
			if i < len(fk.TargetColumns) && fk.TargetColumns[i] != local {
				out = append(out, local)
			}
		}
	}
	return out
}

// Column returns a column by name.
func (t *Table) Column(name string) (*Column, bool) {
	if t == nil {
		return nil, false
	}
	for i := range t.Columns {
		if t.Columns[i].Name == name {
			return &t.Columns[i], true
		}
	}
	return nil, false
}

// Names lists column names in declaration order.
func (t *Table) Names() []string {
	if t == nil {
		return nil
	}
	out := make([]string, 0, len(t.Columns))
	for _, column := range t.Columns {
		out = append(out, column.Name)
	}
	return out
}

// Load parses the migrations embedded in the binary.
func Load() ([]*Table, error) { return Parse(migrations.Files) }

// Parse applies the migrations in file order and returns the forward schema.
func Parse(fsys fs.FS) ([]*Table, error) {
	names, err := fs.Glob(fsys, "*.sql")
	if err != nil {
		return nil, err
	}
	sort.Strings(names)
	byName := map[string]*Table{}
	var order []string
	for _, name := range names {
		raw, err := fs.ReadFile(fsys, name)
		if err != nil {
			return nil, err
		}
		for _, statement := range splitStatements(stripComments(upSection(string(raw)))) {
			if err := apply(byName, &order, statement); err != nil {
				return nil, fmt.Errorf("%s: %w", name, err)
			}
		}
	}
	out := make([]*Table, 0, len(order))
	for _, name := range order {
		if table, ok := byName[name]; ok {
			out = append(out, table)
		}
	}
	return out, nil
}

func upSection(raw string) string {
	if index := strings.Index(raw, "-- +goose Down"); index >= 0 {
		return raw[:index]
	}
	return raw
}

// stripComments removes line comments while leaving string literals intact.
func stripComments(source string) string {
	var out strings.Builder
	inQuote := false
	for i := 0; i < len(source); i++ {
		if inQuote {
			out.WriteByte(source[i])
			if source[i] == '\'' {
				if i+1 < len(source) && source[i+1] == '\'' {
					out.WriteByte(source[i+1])
					i++
					continue
				}
				inQuote = false
			}
			continue
		}
		if source[i] == '-' && i+1 < len(source) && source[i+1] == '-' {
			for i < len(source) && source[i] != '\n' {
				i++
			}
			out.WriteByte('\n')
			continue
		}
		if source[i] == '\'' {
			inQuote = true
		}
		out.WriteByte(source[i])
	}
	return out.String()
}

// splitStatements splits on semicolons outside quotes and parentheses.
func splitStatements(source string) []string {
	var (
		out     []string
		current strings.Builder
		depth   int
		inQuote bool
	)
	flush := func() {
		if statement := strings.TrimSpace(current.String()); statement != "" {
			out = append(out, statement)
		}
		current.Reset()
	}
	for i := 0; i < len(source); i++ {
		char := source[i]
		if inQuote {
			current.WriteByte(char)
			if char == '\'' {
				if i+1 < len(source) && source[i+1] == '\'' {
					current.WriteByte(source[i+1])
					i++
					continue
				}
				inQuote = false
			}
			continue
		}
		switch char {
		case '\'':
			inQuote = true
		case '(':
			depth++
		case ')':
			if depth > 0 {
				depth--
			}
		case ';':
			if depth == 0 {
				flush()
				continue
			}
		}
		current.WriteByte(char)
	}
	flush()
	return out
}

// matchParen returns the index of the parenthesis closing the one at open.
func matchParen(source string, open int) int {
	depth := 0
	inQuote := false
	for i := open; i < len(source); i++ {
		char := source[i]
		if inQuote {
			if char == '\'' {
				if i+1 < len(source) && source[i+1] == '\'' {
					i++
					continue
				}
				inQuote = false
			}
			continue
		}
		switch char {
		case '\'':
			inQuote = true
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}

// splitTopLevel splits a definition list on commas outside quotes and parens.
func splitTopLevel(source string) []string {
	var (
		out     []string
		current strings.Builder
		depth   int
		inQuote bool
	)
	for i := 0; i < len(source); i++ {
		char := source[i]
		if inQuote {
			current.WriteByte(char)
			if char == '\'' {
				if i+1 < len(source) && source[i+1] == '\'' {
					current.WriteByte(source[i+1])
					i++
					continue
				}
				inQuote = false
			}
			continue
		}
		switch char {
		case '\'':
			inQuote = true
		case '(':
			depth++
		case ')':
			if depth > 0 {
				depth--
			}
		case ',':
			if depth == 0 {
				out = append(out, strings.TrimSpace(current.String()))
				current.Reset()
				continue
			}
		}
		current.WriteByte(char)
	}
	if tail := strings.TrimSpace(current.String()); tail != "" {
		out = append(out, tail)
	}
	return out
}

var (
	identifierRe = regexp.MustCompile(`^[a-z_][a-z0-9_]*`)
	typeRe       = regexp.MustCompile(`(?i)^(double\s+precision|timestamp\s+with(?:out)?\s+time\s+zone|time\s+with(?:out)?\s+time\s+zone|character\s+varying|character|smallint|integer|bigint|int|serial|bigserial|text|varchar|char|bpchar|boolean|bool|timestamptz|timestamp|date|time|timetz|jsonb|json|bytea|uuid|numeric|decimal|real|money|inet|cidr|macaddr|interval|tsvector|xml)\s*(?:\(\s*\d+\s*(?:,\s*\d+\s*)?\))?\s*(\[\])?`)
	typeLenRe    = regexp.MustCompile(`(?i)^(?:varchar|char|bpchar|character\s+varying|character)\s*\(\s*(\d+)`)
	enumCheckRe  = regexp.MustCompile(`(?is)^\s*([a-z_][a-z0-9_]*)\s+IN\s*\((.+)\)\s*$`)
	shapeCheckRe = regexp.MustCompile(`(?is)jsonb_typeof\s*\(\s*([a-z_][a-z0-9_]*)\s*\)\s*=\s*'(array|object)'`)
	maxLenRe     = regexp.MustCompile(`(?is)(?:char_length|length)\s*\(\s*([a-z_][a-z0-9_]*)\s*\)\s*BETWEEN\s+(\d+)\s+AND\s+(\d+)`)
	inlineFKRe   = regexp.MustCompile(`(?is)\bREFERENCES\s+([a-z_][a-z0-9_]*)\s*\(\s*([a-z_][a-z0-9_]*)\s*\)`)
	tableFKRe    = regexp.MustCompile(`(?is)^FOREIGN\s+KEY\s*\(([^)]*)\)\s*REFERENCES\s+([a-z_][a-z0-9_]*)\s*\(([^)]*)\)`)
	defaultRe    = regexp.MustCompile(`(?is)\bDEFAULT\s+('(?:[^']|'')*'(?:::[a-zA-Z_]+)?|[a-zA-Z_][\w.]*(?:\([^)]*\))?(?:::[a-zA-Z_]+)?)`)
)

func apply(byName map[string]*Table, order *[]string, statement string) error {
	head := strings.ToUpper(statement)
	switch {
	case strings.HasPrefix(head, "CREATE TABLE"):
		return applyCreate(byName, order, statement)
	case strings.HasPrefix(head, "ALTER TABLE"):
		return applyAlter(byName, statement)
	case strings.HasPrefix(head, "DROP TABLE"):
		applyDrop(byName, order, statement)
	}
	return nil
}

func applyCreate(byName map[string]*Table, order *[]string, statement string) error {
	rest := strings.TrimSpace(statement[len("CREATE TABLE"):])
	rest = trimKeyword(rest, "IF NOT EXISTS")
	name, rest := leadingIdentifier(rest)
	if name == "" {
		return fmt.Errorf("create table without a name")
	}
	open := strings.Index(rest, "(")
	if open < 0 {
		return fmt.Errorf("create table %s without a column list", name)
	}
	closeAt := matchParen(rest, open)
	if closeAt < 0 {
		return fmt.Errorf("create table %s with an unterminated column list", name)
	}
	table := &Table{Name: name}
	for _, part := range splitTopLevel(rest[open+1 : closeAt]) {
		trimmed := strings.TrimSpace(part)
		if trimmed == "" {
			continue
		}
		if isConstraint(trimmed) {
			applyConstraintPart(table, trimmed)
			continue
		}
		column := parseColumn(trimmed)
		table.Columns = append(table.Columns, column)
		// An inline REFERENCES declares an implicit single column key.
		if match := inlineFKRe.FindStringSubmatch(trimmed); match != nil {
			columns := []string{column.Name}
			table.ForeignKeys = append(table.ForeignKeys, ForeignKey{
				Name: table.fkName(columns), Columns: columns,
				TargetTable: match[1], TargetColumns: []string{match[2]},
			})
		}
	}
	if _, exists := byName[name]; !exists {
		*order = append(*order, name)
	}
	byName[name] = table
	return nil
}

// isConstraint reports whether a table body entry declares a constraint rather
// than a column. Columns always start with a name followed by a type, so the
// constraint keywords are unambiguous.
func isConstraint(part string) bool {
	upper := strings.ToUpper(strings.TrimSpace(part))
	for _, keyword := range []string{"CONSTRAINT ", "PRIMARY KEY", "FOREIGN KEY", "UNIQUE", "CHECK", "EXCLUDE"} {
		if strings.HasPrefix(upper, keyword) && (len(upper) == len(keyword) || strings.HasSuffix(keyword, " ") || upper[len(keyword)] == ' ' || upper[len(keyword)] == '(') {
			return true
		}
	}
	return false
}

// applyConstraintPart applies one table constraint, with or without a leading
// CONSTRAINT <name>. Adding a key after the fact (ALTER TABLE ... ADD) goes
// through the same path, so both spellings stay consistent.
func applyConstraintPart(table *Table, part string) {
	trimmed := strings.TrimSpace(part)
	name := ""
	if strings.HasPrefix(strings.ToUpper(trimmed), "CONSTRAINT") {
		name, trimmed = leadingIdentifier(strings.TrimSpace(trimmed[len("CONSTRAINT"):]))
	}
	upper := strings.ToUpper(trimmed)
	switch {
	case strings.HasPrefix(upper, "PRIMARY KEY"):
		for _, column := range splitTopLevel(strings.TrimSpace(trimmed[len("PRIMARY KEY"):])) {
			if target, ok := table.Column(strings.Trim(strings.TrimSpace(column), "()")); ok {
				target.PrimaryKey = true
				target.NotNull = true
			}
		}
	case strings.HasPrefix(upper, "FOREIGN KEY"):
		applyForeignKeyDecl(table, trimmed, name)
	case strings.HasPrefix(upper, "UNIQUE"), strings.HasPrefix(upper, "CHECK"), strings.HasPrefix(upper, "EXCLUDE"):
		for _, inner := range checkExpressions(trimmed) {
			applyCheckExpression(table, inner)
		}
	}
}

// applyForeignKeyDecl records one foreign key, replacing any key already known
// under the same constraint name. Migrations re-point a column by dropping the
// old constraint and adding a new one, so the name is the identity here.
func applyForeignKeyDecl(table *Table, part, name string) {
	match := tableFKRe.FindStringSubmatch(part)
	if match == nil {
		return
	}
	local := identifierList(match[1])
	target := identifierList(match[3])
	if len(local) == 0 {
		return
	}
	if name == "" {
		name = table.fkName(local)
	}
	table.removeForeignKey(name)
	table.ForeignKeys = append(table.ForeignKeys, ForeignKey{
		Name: name, Columns: local, TargetTable: match[2], TargetColumns: target,
	})
	// Keep the flat References hint for simple single column keys.
	if len(local) == 1 && len(target) == 1 {
		if column, ok := table.Column(local[0]); ok {
			column.References = match[2] + "." + target[0]
		}
	}
}

// fkName reproduces PostgreSQL's default constraint name so a later
// DROP CONSTRAINT can be matched without tracking it separately.
func (t *Table) fkName(columns []string) string {
	return t.Name + "_" + strings.Join(columns, "_") + "_fkey"
}

func (t *Table) removeForeignKey(name string) {
	kept := t.ForeignKeys[:0]
	for _, fk := range t.ForeignKeys {
		if fk.Name != name {
			kept = append(kept, fk)
		}
	}
	t.ForeignKeys = kept
}

// identifierList parses "a, b, c" into column names.
func identifierList(source string) []string {
	out := []string{}
	for _, part := range splitTopLevel(source) {
		name, _ := leadingIdentifier(strings.TrimSpace(part))
		if name != "" {
			out = append(out, name)
		}
	}
	return out
}

func applyCheckExpression(table *Table, inner string) {
	if match := enumCheckRe.FindStringSubmatch(strings.TrimSpace(inner)); match != nil {
		if column, ok := table.Column(match[1]); ok {
			column.Enum = parseEnumValues(match[2])
		}
		return
	}
	if match := shapeCheckRe.FindStringSubmatch(inner); match != nil {
		if column, ok := table.Column(match[1]); ok {
			column.Shape = strings.ToLower(match[2])
		}
		return
	}
	if match := maxLenRe.FindStringSubmatch(inner); match != nil {
		if column, ok := table.Column(match[1]); ok {
			if limit := atoi(match[3]); limit > column.MaxLength {
				column.MaxLength = limit
			}
		}
	}
}

func parseColumn(part string) Column {
	name, rest := leadingIdentifier(strings.TrimSpace(part))
	column := Column{Name: name}
	if match := typeRe.FindStringSubmatch(rest); match != nil {
		column.SQLType = normalizeType(match[1])
		column.Array = match[2] != ""
	}
	if match := typeLenRe.FindStringSubmatch(rest); match != nil {
		column.MaxLength = atoi(match[1])
	}
	upper := strings.ToUpper(rest)
	column.NotNull = strings.Contains(upper, "NOT NULL")
	column.PrimaryKey = strings.Contains(upper, "PRIMARY KEY")
	if column.PrimaryKey {
		// PostgreSQL enforces NOT NULL on primary keys without spelling it out.
		column.NotNull = true
	}
	column.Identity = strings.Contains(upper, "GENERATED ALWAYS AS IDENTITY")
	if match := defaultRe.FindStringSubmatch(rest); match != nil {
		column.Default = match[1]
	}
	if match := inlineFKRe.FindStringSubmatch(rest); match != nil {
		column.References = match[1] + "." + match[2]
	}
	for _, inner := range checkExpressions(rest) {
		if match := enumCheckRe.FindStringSubmatch(strings.TrimSpace(inner)); match != nil && match[1] == name {
			column.Enum = parseEnumValues(match[2])
			continue
		}
		if match := shapeCheckRe.FindStringSubmatch(inner); match != nil && match[1] == name {
			column.Shape = strings.ToLower(match[2])
			continue
		}
		if match := maxLenRe.FindStringSubmatch(inner); match != nil && match[1] == name {
			if limit := atoi(match[3]); limit > column.MaxLength {
				column.MaxLength = limit
			}
		}
	}
	return column
}

// checkExpressions returns the inner text of every CHECK (...) on a definition.
func checkExpressions(source string) []string {
	var out []string
	upper := strings.ToUpper(source)
	for offset := 0; offset < len(source); {
		index := strings.Index(upper[offset:], "CHECK")
		if index < 0 {
			break
		}
		start := offset + index + len("CHECK")
		open := strings.Index(source[start:], "(")
		if open < 0 {
			break
		}
		open += start
		closeAt := matchParen(source, open)
		if closeAt < 0 {
			break
		}
		out = append(out, source[open+1:closeAt])
		offset = closeAt + 1
	}
	return out
}

func applyAlter(byName map[string]*Table, statement string) error {
	rest := strings.TrimSpace(statement[len("ALTER TABLE"):])
	rest = trimKeyword(rest, "IF EXISTS")
	name, rest := leadingIdentifier(rest)
	table, exists := byName[name]
	if !exists {
		// An ALTER for a table this parser never saw is a parser bug, not a
		// migration bug; surface it instead of silently dropping the change.
		return fmt.Errorf("alter table %s before create", name)
	}
	upper := strings.ToUpper(rest)
	switch {
	case strings.Contains(upper, "ADD COLUMN"):
		index := strings.Index(upper, "ADD COLUMN")
		definition := strings.TrimSpace(rest[index+len("ADD COLUMN"):])
		definition = trimKeyword(definition, "IF NOT EXISTS")
		column := parseColumn(definition)
		if existing, ok := table.Column(column.Name); ok {
			if column.NotNull {
				existing.NotNull = true
			}
			return nil
		}
		table.Columns = append(table.Columns, column)
	case strings.Contains(upper, "DROP COLUMN"):
		index := strings.Index(upper, "DROP COLUMN")
		tail := strings.TrimSpace(rest[index+len("DROP COLUMN"):])
		tail = trimKeyword(tail, "IF EXISTS")
		if column, _ := leadingIdentifier(tail); column != "" {
			removeColumn(table, column)
		}
	case strings.Contains(upper, "ALTER COLUMN"):
		index := strings.Index(upper, "ALTER COLUMN")
		tail := strings.TrimSpace(rest[index+len("ALTER COLUMN"):])
		columnName, remainder := leadingIdentifier(tail)
		column, ok := table.Column(columnName)
		if !ok {
			return nil
		}
		if strings.Contains(strings.ToUpper(remainder), "SET NOT NULL") {
			column.NotNull = true
		}
		if strings.Contains(strings.ToUpper(remainder), "DROP NOT NULL") {
			column.NotNull = false
		}
	case strings.Contains(upper, "DROP CONSTRAINT"):
		index := strings.Index(upper, "DROP CONSTRAINT")
		name, _ := leadingIdentifier(strings.TrimSpace(rest[index+len("DROP CONSTRAINT"):]))
		if name != "" {
			table.removeForeignKey(name)
		}
	case strings.Contains(upper, "ADD CONSTRAINT"), strings.Contains(upper, "ADD FOREIGN KEY"),
		strings.Contains(upper, "ADD CHECK"), strings.Contains(upper, "ADD UNIQUE"),
		strings.Contains(upper, "ADD PRIMARY KEY"):
		// A key or check added after the fact carries no column definition, so
		// only the constraint itself is applied. Without this branch an
		// ALTER ... ADD FOREIGN KEY would be silently dropped from the schema.
		index := strings.Index(upper, "ADD ")
		applyConstraintPart(table, strings.TrimSpace(rest[index+len("ADD "):]))
	}
	return nil
}

func applyDrop(byName map[string]*Table, order *[]string, statement string) {
	rest := strings.TrimSpace(statement[len("DROP TABLE"):])
	rest = trimKeyword(rest, "IF EXISTS")
	name, _ := leadingIdentifier(rest)
	delete(byName, name)
	kept := (*order)[:0]
	for _, item := range *order {
		if item != name {
			kept = append(kept, item)
		}
	}
	*order = kept
}

func removeColumn(table *Table, name string) {
	kept := table.Columns[:0]
	for _, column := range table.Columns {
		if column.Name != name {
			kept = append(kept, column)
		}
	}
	table.Columns = kept
}

func leadingIdentifier(source string) (string, string) {
	source = strings.TrimSpace(source)
	match := identifierRe.FindString(source)
	if match == "" {
		return "", source
	}
	return match, strings.TrimSpace(source[len(match):])
}

func trimKeyword(source, keyword string) string {
	if len(source) >= len(keyword) && strings.EqualFold(source[:len(keyword)], keyword) {
		return strings.TrimSpace(source[len(keyword):])
	}
	return source
}

func normalizeType(value string) string {
	return strings.Join(strings.Fields(strings.ToLower(value)), " ")
}

func parseEnumValues(list string) []string {
	parts := splitTopLevel(list)
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		value := strings.Trim(strings.TrimSpace(part), "'")
		if value != "" {
			out = append(out, value)
		}
	}
	return out
}

func atoi(value string) int {
	result := 0
	for _, char := range value {
		if char < '0' || char > '9' {
			return result
		}
		result = result*10 + int(char-'0')
	}
	return result
}
