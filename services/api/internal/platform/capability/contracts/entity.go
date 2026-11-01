package contracts

import (
	"fmt"
	"sort"
	"strings"
)

// AttributeKind says what a field means for the business, which is what decides
// whether it belongs in a capability input, a form, or neither.
type AttributeKind string

// Attribute kinds.
const (
	// AttributeBusiness is a field a user edits through a capability.
	AttributeBusiness AttributeKind = "business"
	// AttributeSystem is injected by the server and never accepted from a caller.
	AttributeSystem AttributeKind = "system"
	// AttributeState changes only through a declared Transition.
	AttributeState AttributeKind = "state"
	// AttributeReference points at another entity.
	AttributeReference AttributeKind = "reference"
)

// AttributeType is the storage independent type shown to the console and model.
type AttributeType string

// Attribute types. A type outside this set is a declaration bug, not a hint.
const (
	TypeString    AttributeType = "string"
	TypeInteger   AttributeType = "integer"
	TypeDecimal   AttributeType = "decimal"
	TypeFloat     AttributeType = "float"
	TypeBoolean   AttributeType = "boolean"
	TypeDate      AttributeType = "date"
	TypeTime      AttributeType = "time"
	TypeTimestamp AttributeType = "timestamp"
	TypeUUID      AttributeType = "uuid"
	TypeJSON      AttributeType = "json"
	TypeBytes     AttributeType = "bytes"
	TypeEnum      AttributeType = "enum"
	TypeReference AttributeType = "reference"
	TypeUnknown   AttributeType = "unknown"
)

// Attribute is one field of an entity.
type Attribute struct {
	// Name is the camelCase name used by capabilities, the API and the console.
	Name string
	// Column overrides the database column; empty means snake_case(Name).
	Column         string
	Kind           AttributeKind
	Type           AttributeType
	SQLType        string // only for TypeUnknown: the raw database type
	Required       bool
	Nullable       bool
	MaxLength      *int
	Precision      int
	Scale          int
	WithTimezone   bool
	Representation string // "number" (default) or "string" for big ids and money
	Enum           []string
	Shape          string // "object" | "array", required for TypeJSON
	Items          *Attribute
	Sensitive      bool
	Description    string
}

// DatabaseColumn is the column this attribute maps to.
func (a Attribute) DatabaseColumn() string {
	if a.Column != "" {
		return a.Column
	}
	return snakeCase(a.Name)
}

// Relation is a link from one entity to another.
type Relation struct {
	Name     string
	Target   string
	Kind     string // "one" | "many"
	Via      string // foreign key column or join table
	Required bool
}

// State is one value of the entity's state column.
type State struct {
	Name     string
	Terminal bool
}

// Transition is one legal state change and the capability that performs it.
type Transition struct {
	From       []string // empty means creation
	To         string
	Capability string // empty means no controlled entry point
}

// Entity is the business object model. Attributes, relations and states are
// declared; Capabilities and Events are derived from the capability catalog.
type Entity struct {
	Name        string
	Domain      string
	Table       string
	Description string
	Key         string
	Attributes  []Attribute
	Relations   []Relation
	States      []State
	Transitions []Transition
	// Derived by metadata.Build; declarations must not fill these in.
	Capabilities []string
	Events       []string
}

// Attribute finds an attribute by name.
func (e Entity) Attribute(name string) (Attribute, bool) {
	for _, attribute := range e.Attributes {
		if attribute.Name == name {
			return attribute, true
		}
	}
	return Attribute{}, false
}

// StateNames lists the declared states.
func (e Entity) StateNames() []string {
	out := make([]string, 0, len(e.States))
	for _, state := range e.States {
		out = append(out, state.Name)
	}
	return out
}

// Problems reports declaration mistakes that need no database to detect.
func (e Entity) Problems() []string {
	var problems []string
	report := func(format string, args ...any) {
		problems = append(problems, fmt.Sprintf(format, args...))
	}
	if e.Name == "" {
		report("entity name is empty")
	}
	if e.Table == "" {
		report("%s: table is empty", e.Name)
	}
	seen := map[string]bool{}
	for _, attribute := range e.Attributes {
		if attribute.Name == "" {
			report("%s: attribute without a name", e.Name)
			continue
		}
		key := strings.ToLower(attribute.Name)
		if seen[key] {
			report("%s: duplicate attribute %q", e.Name, attribute.Name)
		}
		seen[key] = true
		switch attribute.Kind {
		case AttributeBusiness, AttributeSystem, AttributeState, AttributeReference:
		default:
			report("%s.%s: unknown kind %q", e.Name, attribute.Name, attribute.Kind)
		}
		switch attribute.Type {
		case TypeString, TypeInteger, TypeDecimal, TypeFloat, TypeBoolean, TypeDate,
			TypeTime, TypeTimestamp, TypeUUID, TypeJSON, TypeBytes, TypeEnum,
			TypeReference, TypeUnknown:
		default:
			report("%s.%s: unknown type %q", e.Name, attribute.Name, attribute.Type)
		}
		if attribute.Type == TypeEnum && len(attribute.Enum) == 0 {
			report("%s.%s: enum type without values", e.Name, attribute.Name)
		}
		if attribute.Type == TypeJSON && attribute.Shape != "object" && attribute.Shape != "array" {
			report("%s.%s: json type needs shape object or array", e.Name, attribute.Name)
		}
		if attribute.Type == TypeDecimal && attribute.Precision == 0 {
			report("%s.%s: decimal type needs precision", e.Name, attribute.Name)
		}
		if attribute.Type == TypeUnknown && attribute.SQLType == "" {
			report("%s.%s: unknown type needs sqlType", e.Name, attribute.Name)
		}
		if attribute.Required && attribute.Nullable {
			report("%s.%s: required and nullable at once", e.Name, attribute.Name)
		}
	}

	states := map[string]bool{}
	for _, state := range e.States {
		if state.Name == "" {
			report("%s: state without a name", e.Name)
			continue
		}
		if states[state.Name] {
			report("%s: duplicate state %q", e.Name, state.Name)
		}
		states[state.Name] = true
	}
	var stateAttributes []Attribute
	for _, attribute := range e.Attributes {
		if attribute.Kind == AttributeState {
			stateAttributes = append(stateAttributes, attribute)
		}
	}
	switch {
	case len(e.States) > 0 && len(stateAttributes) == 0:
		report("%s: states declared without a state attribute", e.Name)
	case len(stateAttributes) > 1:
		report("%s: more than one state attribute", e.Name)
	case len(stateAttributes) == 1 && len(e.States) > 0:
		declared := append([]string{}, stateAttributes[0].Enum...)
		listed := e.StateNames()
		sort.Strings(declared)
		sort.Strings(listed)
		if strings.Join(declared, ",") != strings.Join(listed, ",") {
			report("%s: state attribute enum %v does not match states %v", e.Name, declared, listed)
		}
	}

	for _, transition := range e.Transitions {
		if transition.To == "" {
			report("%s: transition without a target state", e.Name)
			continue
		}
		if !states[transition.To] {
			report("%s: transition to undeclared state %q", e.Name, transition.To)
		}
		for _, from := range transition.From {
			if !states[from] {
				report("%s: transition from undeclared state %q", e.Name, from)
			}
		}
	}
	for _, relation := range e.Relations {
		if relation.Name == "" || relation.Target == "" {
			report("%s: relation without name or target", e.Name)
		}
		if relation.Kind != "one" && relation.Kind != "many" {
			report("%s.%s: relation kind must be one or many", e.Name, relation.Name)
		}
	}
	return problems
}

// snakeCase converts a camelCase field name to its database column.
func snakeCase(value string) string {
	var out strings.Builder
	for i, char := range value {
		if char >= 'A' && char <= 'Z' {
			if i > 0 {
				out.WriteByte('_')
			}
			out.WriteRune(char + ('a' - 'A'))
			continue
		}
		out.WriteRune(char)
	}
	return out.String()
}
