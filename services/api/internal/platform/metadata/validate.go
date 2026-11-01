package metadata

import (
	"fmt"
	"sort"
	"strings"

	"baseline/services/api/internal/platform/capability/contracts"
)

// Severity separates a broken declaration from a future-looking hint.
type Severity string

// Severities.
const (
	SeverityError   Severity = "error"
	SeverityWarning Severity = "warning"
)

// Issue is one failed check.
type Issue struct {
	Code     string
	Entity   string
	Message  string
	Severity Severity
}

func (i Issue) String() string {
	return fmt.Sprintf("%s %s: %s", i.Code, i.Entity, i.Message)
}

// HousekeepingColumns are present on every table and carry no business meaning,
// so they need no attribute. Anything else NOT NULL must be declared, otherwise
// a new column silently escapes the model.
var HousekeepingColumns = map[string]bool{
	"tenant_id":  true,
	"created_at": true,
	"updated_at": true,
}

// Validate checks declared entities against the parsed physical schema.
//
// Implemented rules: E1 table exists, E2 attribute column exists, E3 every
// business column is declared, E4 enum matches the CHECK constraint, E5 the
// state attribute matches the column, E6 relations match real foreign keys,
// E10 json shape matches jsonb_typeof. E8 (capability covers transition)
// arrives with its inputs.
func Validate(tables []*Table, entities []contracts.Entity) []Issue {
	byName := make(map[string]*Table, len(tables))
	for _, table := range tables {
		byName[table.Name] = table
	}
	entityByTable := make(map[string]string, len(entities))
	for _, entity := range entities {
		if entity.Table != "" {
			entityByTable[entity.Table] = entity.Name
		}
	}
	var issues []Issue
	for _, entity := range entities {
		table, ok := byName[entity.Table]
		if !ok {
			issues = append(issues, Issue{
				Code: "E1", Entity: entity.Name,
				Message:  fmt.Sprintf("table %q does not exist", entity.Table),
				Severity: SeverityError,
			})
			continue
		}
		declared := map[string]bool{}
		for _, attribute := range entity.Attributes {
			column := attribute.DatabaseColumn()
			declared[column] = true
			target, ok := table.Column(column)
			if !ok {
				issues = append(issues, Issue{
					Code: "E2", Entity: entity.Name,
					Message:  fmt.Sprintf("attribute %s maps to missing column %q", attribute.Name, column),
					Severity: SeverityError,
				})
				continue
			}
			issues = append(issues, checkAttribute(entity, attribute, target)...)
		}
		for _, target := range table.Columns {
			if declared[target.Name] || HousekeepingColumns[target.Name] || !target.NotNull {
				continue
			}
			issues = append(issues, Issue{
				Code: "E3", Entity: entity.Name,
				Message:  fmt.Sprintf("column %q is NOT NULL but has no attribute", target.Name),
				Severity: SeverityError,
			})
		}
		issues = append(issues, checkRelations(entity, table, byName, entityByTable)...)
	}
	sort.SliceStable(issues, func(i, j int) bool {
		if issues[i].Entity == issues[j].Entity {
			return issues[i].Code < issues[j].Code
		}
		return issues[i].Entity < issues[j].Entity
	})
	return issues
}

// ReferenceAliases maps a table onto the entity whose rows it points at.
//
// gov_identity_refs exists so a historical reference survives the account row
// being deleted: it is an identity pointer, not a separate business object. A
// foreign key to it therefore means "points at an account", and relations must
// be allowed to say so.
var ReferenceAliases = map[string]string{
	"gov_identity_refs": "account",
}

// entityOf resolves a physical table to the entity registered for it, following
// the alias map when the table is a pointer rather than an object.
func entityOf(table string, entityByTable map[string]string) (string, bool) {
	if name, ok := entityByTable[table]; ok {
		return name, true
	}
	if alias, ok := ReferenceAliases[table]; ok {
		return alias, true
	}
	return "", false
}

// checkRelations verifies that each declared relation exists in the schema.
//
// A relation is either forward (Via names a local column backed by a foreign
// key) or reverse (Via names the child table). Declaring a relation that no
// foreign key supports would turn the object model into documentation, so both
// shapes are checked against the parsed keys.
func checkRelations(entity contracts.Entity, table *Table, byName map[string]*Table, entityByTable map[string]string) []Issue {
	var issues []Issue
	report := func(severity Severity, format string, args ...any) {
		issues = append(issues, Issue{
			Code: "E6", Entity: entity.Name,
			Message:  fmt.Sprintf(format, args...),
			Severity: severity,
		})
	}
	for _, relation := range entity.Relations {
		if relation.Via == "" {
			report(SeverityError, "relation %s has no via (column or child table)", relation.Name)
			continue
		}
		if column, isColumn := table.Column(relation.Via); isColumn {
			target, ok := table.ParentOf(relation.Via)
			if !ok {
				if scope, guarded := table.ScopeTarget(relation.Via); guarded {
					report(SeverityWarning,
						"relation %s: %q only guards the key to %s; the same organization is implied by that key",
						relation.Name, relation.Via, scope)
					continue
				}
				report(SeverityError, "relation %s: column %q has no foreign key", relation.Name, relation.Via)
				continue
			}
			if actual, declared := entityOf(target, entityByTable); declared && actual != relation.Target {
				report(SeverityError, "relation %s: %q points at %s (%s), not %s",
					relation.Name, relation.Via, target, actual, relation.Target)
			}
			if relation.Required && !column.NotNull {
				report(SeverityError, "relation %s is required but %q is nullable", relation.Name, relation.Via)
			}
			if !relation.Required && column.NotNull {
				report(SeverityWarning, "relation %s is optional but %q is NOT NULL", relation.Name, relation.Via)
			}
			continue
		}
		// Reverse relation: Via names the child table.
		child, ok := byName[relation.Via]
		if !ok {
			report(SeverityError, "relation %s: table %q does not exist", relation.Name, relation.Via)
			continue
		}
		if actual, declared := entityByTable[child.Name]; declared && actual != relation.Target {
			report(SeverityError, "relation %s: %s holds %s, not %s",
				relation.Name, child.Name, actual, relation.Target)
		}
		if len(child.ChildrenOf(table.Name)) == 0 {
			report(SeverityError, "relation %s: %s has no foreign key to %s", relation.Name, child.Name, table.Name)
		}
	}
	return issues
}

func checkAttribute(entity contracts.Entity, attribute contracts.Attribute, column *Column) []Issue {
	var issues []Issue
	report := func(code, format string, args ...any) {
		issues = append(issues, Issue{
			Code: code, Entity: entity.Name,
			Message:  fmt.Sprintf(format, args...),
			Severity: SeverityError,
		})
	}
	if len(column.Enum) > 0 {
		if attribute.Type != contracts.TypeEnum {
			report("E4", "%s: column %q is an enum but the attribute type is %q", attribute.Name, column.Name, attribute.Type)
		} else if !sameSet(attribute.Enum, column.Enum) {
			report("E4", "%s: enum %v does not match constraint %v", attribute.Name, attribute.Enum, column.Enum)
		}
	} else if attribute.Type == contracts.TypeEnum {
		report("E4", "%s: declared as enum but column %q has no CHECK constraint", attribute.Name, column.Name)
	}
	if attribute.Kind == contracts.AttributeState && len(column.Enum) == 0 {
		report("E5", "%s: state column %q has no CHECK constraint", attribute.Name, column.Name)
	}
	if attribute.Type == contracts.TypeJSON && column.Shape != "" && attribute.Shape != column.Shape {
		report("E10", "%s: shape %q does not match constraint %q", attribute.Name, attribute.Shape, column.Shape)
	}
	return issues
}

func sameSet(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	a := append([]string{}, left...)
	b := append([]string{}, right...)
	sort.Strings(a)
	sort.Strings(b)
	return strings.Join(a, ",") == strings.Join(b, ",")
}

// Errors filters the blocking issues.
func Errors(issues []Issue) []Issue {
	out := make([]Issue, 0, len(issues))
	for _, issue := range issues {
		if issue.Severity == SeverityError {
			out = append(out, issue)
		}
	}
	return out
}
