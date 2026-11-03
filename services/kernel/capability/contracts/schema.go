package contracts

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"

	"github.com/eino-contrib/jsonschema"
)

// Schema is the compact JSON Schema subset a capability may declare as its
// input or output contract. It is intentionally small: the platform must be able to validate the
// same document it publishes to the UI, the agent and external adapters, so every
// keyword used here has one deterministic check.
type Schema struct {
	Type        string             `json:"type"`
	Description string             `json:"description,omitempty"`
	Properties  map[string]*Schema `json:"properties,omitempty"`
	Required    []string           `json:"required,omitempty"`
	Items       *Schema            `json:"items,omitempty"`
	Enum        []string           `json:"enum,omitempty"`
	MinLength   *int               `json:"minLength,omitempty"`
	MaxLength   *int               `json:"maxLength,omitempty"`
	Minimum     *float64           `json:"minimum,omitempty"`
	Maximum     *float64           `json:"maximum,omitempty"`
	MinItems    *int               `json:"minItems,omitempty"`
	MaxItems    *int               `json:"maxItems,omitempty"`
	// AdditionalProperties is always published explicitly. Capability inputs are
	// closed by default: an agent that invents a field is rejected instead of
	// having the value silently ignored.
	AdditionalProperties bool `json:"additionalProperties"`
}

// Validate checks a decoded argument object against the contract.
func (s *Schema) Validate(in Input) error {
	if s == nil || s.Type != "object" {
		return Problem("invalid")
	}
	return s.validate(map[string]any(in), "arguments")
}

func (s *Schema) validate(value any, path string) error {
	if s == nil {
		return Problem("invalid")
	}
	switch s.Type {
	case "object":
		object, ok := value.(map[string]any)
		if !ok {
			return Problem("invalid")
		}
		for _, key := range s.Required {
			if _, present := object[key]; !present {
				return Problem("invalid")
			}
		}
		for key, raw := range object {
			child, declared := s.Properties[key]
			if !declared {
				if s.AdditionalProperties {
					continue
				}
				return Problem("invalid")
			}
			if err := child.validate(raw, path+"."+key); err != nil {
				return err
			}
		}
	case "array":
		items, ok := value.([]any)
		if !ok {
			return Problem("invalid")
		}
		if s.MinItems != nil && len(items) < *s.MinItems {
			return Problem("invalid")
		}
		if s.MaxItems != nil && len(items) > *s.MaxItems {
			return Problem("invalid")
		}
		for _, item := range items {
			if err := s.Items.validate(item, path+"[]"); err != nil {
				return err
			}
		}
	case "string":
		text, ok := value.(string)
		if !ok {
			return Problem("invalid")
		}
		length := len([]rune(text))
		if s.MinLength != nil && length < *s.MinLength {
			return Problem("invalid")
		}
		if s.MaxLength != nil && length > *s.MaxLength {
			return Problem("invalid")
		}
		if len(s.Enum) > 0 {
			matched := false
			for _, allowed := range s.Enum {
				if allowed == text {
					matched = true
					break
				}
			}
			if !matched {
				return Problem("invalid")
			}
		}
	case "integer":
		number, ok := numeric(value)
		if !ok || math.Trunc(number) != number {
			return Problem("invalid")
		}
		return s.bounds(number)
	case "number":
		number, ok := numeric(value)
		if !ok {
			return Problem("invalid")
		}
		return s.bounds(number)
	case "boolean":
		if _, ok := value.(bool); !ok {
			return Problem("invalid")
		}
	default:
		return Problem("invalid")
	}
	return nil
}

func (s *Schema) bounds(value float64) error {
	if s.Minimum != nil && value < *s.Minimum {
		return Problem("invalid")
	}
	if s.Maximum != nil && value > *s.Maximum {
		return Problem("invalid")
	}
	return nil
}

func numeric(value any) (float64, bool) {
	switch typed := value.(type) {
	case float64:
		return typed, true
	case float32:
		return float64(typed), true
	case int:
		return float64(typed), true
	case int32:
		return float64(typed), true
	case int64:
		return float64(typed), true
	case json.Number:
		parsed, err := typed.Float64()
		return parsed, err == nil
	}
	return 0, false
}

// JSON renders the published contract. Map keys are sorted by encoding/json, so
// repeated renders are byte stable and the UI can diff them.
func (s *Schema) JSON() json.RawMessage {
	raw, err := json.Marshal(s)
	if err != nil {
		return json.RawMessage(`{}`)
	}
	return raw
}

// Document converts the contract to the JSON Schema type used by Eino tools and
// by external MCP adapters. The conversion is a round trip through JSON, which
// keeps a single source of truth for the contract.
func (s *Schema) Document() (*jsonschema.Schema, error) {
	raw, err := json.Marshal(s)
	if err != nil {
		return nil, fmt.Errorf("capability schema: %w", err)
	}
	document := &jsonschema.Schema{}
	if err = json.Unmarshal(raw, document); err != nil {
		return nil, fmt.Errorf("capability schema: %w", err)
	}
	return document, nil
}

// Object builds the root contract with required fields derived from the
// properties that must always be present.
func Object(properties map[string]*Schema, required ...string) *Schema {
	sort.Strings(required)
	return &Schema{Type: "object", Properties: properties, Required: required, AdditionalProperties: false}
}
