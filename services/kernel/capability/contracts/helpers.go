package contracts

import "encoding/json"

// Bind decodes a capability argument object into a module owned payload type.
// The gateway has already validated the schema, so a decode failure here means
// the declared contract and the executor drifted apart.
func Bind(in Input, out any) error {
	raw, err := json.Marshal(in)
	if err != nil {
		return Problem("invalid")
	}
	if err = json.Unmarshal(raw, out); err != nil {
		return Problem("invalid")
	}
	return nil
}

// Str builds a bounded string property.
func Str(min, max int) *Schema {
	low, high := min, max
	return &Schema{Type: "string", MinLength: &low, MaxLength: &high}
}

// Text builds an unbounded string property.
func Text() *Schema { return &Schema{Type: "string"} }

// Enum builds a closed string property.
func Enum(values ...string) *Schema { return &Schema{Type: "string", Enum: values} }

// Int builds a bounded integer property.
func Int(min, max float64) *Schema {
	low, high := min, max
	return &Schema{Type: "integer", Minimum: &low, Maximum: &high}
}

// Bool builds a boolean property.
func Bool() *Schema { return &Schema{Type: "boolean"} }

// Array builds a bounded array property.
func Array(items *Schema, min, max int) *Schema {
	low, high := min, max
	return &Schema{Type: "array", Items: items, MinItems: &low, MaxItems: &high}
}

// FreeObject accepts any object. It is only used for reviewed payloads whose
// detailed shape a module validates itself (for example dictionary documents).
func FreeObject() *Schema { return &Schema{Type: "object", AdditionalProperties: true} }
