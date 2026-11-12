package applicationruntime

import (
	"encoding/json"
	"fmt"
	"horizon/foundation/kernel/capability/contracts"
)

type FilterExpression struct {
	Kind     string             `json:"kind"`
	Field    string             `json:"field,omitempty"`
	Operator string             `json:"operator,omitempty"`
	Value    string             `json:"value,omitempty"`
	Children []FilterExpression `json:"children,omitempty"`
}

// CompileFilter maps declared operations to native capability parameters. Logical
// OR requires a provider's native filter object; it is never emulated by SQL.
func CompileFilter(filter FilterExpression, provider Provider) (contracts.Input, error) {
	count := 0
	var validate func(FilterExpression, int) error
	validate = func(f FilterExpression, depth int) error {
		count++
		if count > 20 || depth > 2 {
			return fmt.Errorf("invalid")
		}
		if f.Kind == "and" || f.Kind == "or" {
			if len(f.Children) == 0 {
				return fmt.Errorf("invalid")
			}
			if f.Kind == "or" && provider.FilterTarget == "" {
				return fmt.Errorf("invalid")
			}
			for _, child := range f.Children {
				if err := validate(child, depth+1); err != nil {
					return err
				}
			}
			return nil
		}
		if f.Kind != "condition" {
			return fmt.Errorf("invalid")
		}
		found := false
		for _, binding := range provider.QueryBindings {
			if binding.Field == f.Field && binding.Operator == f.Operator {
				found = true
			}
		}
		if !found || !json.Valid([]byte(f.Value)) {
			return fmt.Errorf("invalid")
		}
		return nil
	}
	if err := validate(filter, 0); err != nil {
		return nil, err
	}
	if provider.FilterTarget != "" {
		raw, err := json.Marshal(filter)
		if err != nil {
			return nil, err
		}
		var value map[string]any
		if err = json.Unmarshal(raw, &value); err != nil {
			return nil, err
		}
		return contracts.Input{provider.FilterTarget: value}, nil
	}
	out := contracts.Input{}
	var flatten func(FilterExpression) error
	flatten = func(f FilterExpression) error {
		if f.Kind == "and" {
			for _, child := range f.Children {
				if err := flatten(child); err != nil {
					return err
				}
			}
			return nil
		}
		for _, binding := range provider.QueryBindings {
			if binding.Field == f.Field && binding.Operator == f.Operator {
				if _, ok := out[binding.Target]; ok {
					return fmt.Errorf("invalid")
				}
				var value any
				if err := json.Unmarshal([]byte(f.Value), &value); err != nil {
					return err
				}
				out[binding.Target] = value
				return nil
			}
		}
		return fmt.Errorf("invalid")
	}
	if err := flatten(filter); err != nil {
		return nil, err
	}
	return out, nil
}
