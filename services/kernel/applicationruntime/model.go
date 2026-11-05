// Package applicationruntime adapts the existing business object model to bounded UI definitions.
package applicationruntime

import (
	"baseline/services/api/kernel/capability/contracts"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
)

const Version = "1.0"

type Field struct {
	Link             bool     `json:"link"`
	ValueLabelPrefix string   `json:"valueLabelPrefix"`
	Name             string   `json:"name"`
	LabelKey         string   `json:"labelKey"`
	Component        string   `json:"component"`
	Kind             string   `json:"kind"`
	Type             string   `json:"type"`
	Required         bool     `json:"required"`
	Sensitive        bool     `json:"sensitive"`
	Hidden           bool     `json:"hidden"`
	ReadOnly         bool     `json:"readOnly"`
	MaxLength        int      `json:"maxLength"`
	Enum             []string `json:"enum"`
}
type Relation struct {
	Name   string `json:"name"`
	Target string `json:"target"`
	Kind   string `json:"kind"`
}
type Page struct {
	EditableStates   []string   `json:"editableStates"`
	Relations        []Relation `json:"relations"`
	SchemaVersion    string     `json:"schemaVersion"`
	ID               string     `json:"id"`
	Type             string     `json:"type"`
	TitleKey         string     `json:"titleKey"`
	EntityRef        string     `json:"entityRef"`
	Layout           string     `json:"layout"`
	Fields           []Field    `json:"fields"`
	ReadCapability   string     `json:"readCapability"`
	CreateCapability string     `json:"createCapability"`
	UpdateCapability string     `json:"updateCapability"`
}
type Application struct {
	ID              string `json:"id"`
	Version         string `json:"version"`
	MetadataVersion string `json:"metadataVersion"`
	LabelKey        string `json:"labelKey"`
	Route           string `json:"route"`
	Pages           []Page `json:"pages"`
}

// Adapt resolves each displayed field against both the entity and the capability
// projection. Storage identifiers never cross this boundary.
func Adapt(page Page, entity contracts.Entity, read, write contracts.Definition) (Page, error) {
	if page.SchemaVersion != Version {
		return Page{}, fmt.Errorf("schema_version_unsupported")
	}
	if entity.Name != page.EntityRef || len(entity.Attributes) == 0 {
		return Page{}, fmt.Errorf("schema_invalid")
	}
	if page.Layout != "standard" || (page.Type != "list" && page.Type != "detail" && page.Type != "form" && page.Type != "custom") {
		return Page{}, fmt.Errorf("schema_invalid")
	}
	projection := read.Output
	if page.Type == "list" && projection != nil && projection.Properties["items"] != nil {
		projection = projection.Properties["items"].Items
	}
	page.EditableStates = []string{}
	for _, transition := range entity.Transitions {
		if transition.Capability == write.Name {
			page.EditableStates = append(page.EditableStates, transition.From...)
		}
	}
	page.Relations = []Relation{}
	for _, r := range entity.Relations {
		page.Relations = append(page.Relations, Relation{Name: r.Name, Target: r.Target, Kind: r.Kind})
	}
	seen := map[string]bool{}
	for i, f := range page.Fields {
		a, ok := entity.Attribute(f.Name)
		if !ok || seen[f.Name] || projection == nil || projection.Properties[f.Name] == nil {
			return Page{}, fmt.Errorf("field_not_allowed")
		}
		seen[f.Name] = true
		if f.LabelKey == "" || (f.Component != "text" && f.Component != "textarea" && f.Component != "status") {
			return Page{}, fmt.Errorf("component_not_registered")
		}
		f.Kind = string(a.Kind)
		f.Type = string(a.Type)
		f.Sensitive = a.Sensitive
		f.Enum = append([]string{}, a.Enum...)
		f.ReadOnly = true
		if page.Type == "list" && a.Sensitive {
			return Page{}, fmt.Errorf("field_not_allowed")
		}
		if page.Type == "form" && write.Input != nil {
			if s, exists := write.Input.Properties[f.Name]; exists && a.Kind == contracts.AttributeBusiness {
				f.ReadOnly = false
				if s.MaxLength != nil {
					f.MaxLength = *s.MaxLength
				}
				for _, r := range write.Input.Required {
					if r == f.Name {
						f.Required = true
					}
				}
			}
		}
		page.Fields[i] = f
	}
	return page, nil
}
func Stamp(app Application) Application {
	app.MetadataVersion = ""
	raw, _ := json.Marshal(app)
	sum := sha256.Sum256(raw)
	app.MetadataVersion = hex.EncodeToString(sum[:])
	return app
}

// Schema and the TypeScript generator derive structural contracts from the same Go types.
func Schema(value any) *contracts.Schema {
	return schemaType(reflect.TypeOf(value), map[reflect.Type]bool{})
}
func schemaType(t reflect.Type, stack map[reflect.Type]bool) *contracts.Schema {
	if t.Kind() == reflect.Pointer {
		return schemaType(t.Elem(), stack)
	}
	switch t.Kind() {
	case reflect.String:
		return &contracts.Schema{Type: "string"}
	case reflect.Bool:
		return &contracts.Schema{Type: "boolean"}
	case reflect.Int, reflect.Int64:
		return &contracts.Schema{Type: "integer"}
	case reflect.Float64:
		return &contracts.Schema{Type: "number"}
	case reflect.Slice:
		return &contracts.Schema{Type: "array", Items: schemaType(t.Elem(), stack)}
	case reflect.Struct:
		if stack[t] {
			return &contracts.Schema{Type: "object", AdditionalProperties: true}
		}
		stack[t] = true
		defer delete(stack, t)
		props := map[string]*contracts.Schema{}
		required := []string{}
		for i := 0; i < t.NumField(); i++ {
			f := t.Field(i)
			parts := strings.Split(f.Tag.Get("json"), ",")
			name := parts[0]
			props[name] = schemaType(f.Type, stack)
			if len(parts) == 1 {
				required = append(required, name)
			}
		}
		return contracts.Object(props, required...)
	default:
		panic("unsupported runtime contract type")
	}
}
