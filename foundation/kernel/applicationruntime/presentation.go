package applicationruntime

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

var messageKeyPattern = regexp.MustCompile(`^[A-Za-z0-9_.-]+$`)
var themeValuePattern = regexp.MustCompile(`^(?:#[\da-fA-F]{3,8}|-?\d+(?:\.\d+)?(?:px|rem|em|%)?|var\(--[a-z0-9-]+\))$`)

// BindApplicationMessages replaces only the reserved application-local namespace.
func BindApplicationMessages(app ApplicationV2) (ApplicationV2, error) {
	raw, err := json.Marshal(app)
	if err != nil {
		return app, err
	}
	var document any
	if err = json.Unmarshal(raw, &document); err != nil {
		return app, err
	}
	var visit func(any)
	visit = func(value any) {
		switch node := value.(type) {
		case map[string]any:
			for key, value := range node {
				if key == "labelKey" || key == "titleKey" || key == "confirmKey" || key == "successKey" || key == "valueLabelPrefix" || key == "key" {
					if text, ok := value.(string); ok && strings.HasPrefix(text, "$app:") {
						node[key] = app.ID + "_local" + strings.TrimPrefix(text, "$app")
					}
				} else {
					visit(value)
				}
			}
		case []any:
			for _, item := range node {
				visit(item)
			}
		}
	}
	visit(document)
	raw, err = json.Marshal(document)
	if err != nil {
		return app, err
	}
	err = json.Unmarshal(raw, &app)
	return app, err
}
func validatePresentation(app ApplicationV2) error {
	if len(app.Messages) > 500 || len(app.Theme) > 50 {
		return fmt.Errorf("schema_invalid: resource budget")
	}
	seen := map[string]bool{}
	for _, message := range app.Messages {
		prefix := app.ID + "_local:"
		key := strings.TrimPrefix(message.Key, prefix)
		if !strings.HasPrefix(message.Key, prefix) || !messageKeyPattern.MatchString(key) || seen[message.Key] || message.En == "" || message.ZhCN == "" || len(message.En) > 4000 || len(message.ZhCN) > 4000 {
			return fmt.Errorf("schema_invalid: message resource")
		}
		for _, part := range strings.Split(key, ".") {
			if part == "__proto__" || part == "prototype" || part == "constructor" {
				return fmt.Errorf("schema_invalid: message resource")
			}
		}
		seen[message.Key] = true
	}
	for _, token := range app.Theme {
		if !strings.HasPrefix(token.Name, "--plugin-"+app.ID+"-") || !messageKeyPattern.MatchString(token.Name) || !themeValuePattern.MatchString(token.Value) {
			return fmt.Errorf("schema_invalid: theme resource")
		}
	}
	for _, key := range MessageReferences(app) {
		if strings.HasPrefix(key, app.ID+"_local:") && !seen[key] {
			return fmt.Errorf("schema_invalid: missing application message")
		}
	}
	return nil
}

// MessageReferences lists actual label references, including declared enum values.
// It does not inspect or expose message bodies or business records.
func MessageReferences(app ApplicationV2) []string {
	raw, err := json.Marshal(app)
	if err != nil {
		return nil
	}
	var document any
	if json.Unmarshal(raw, &document) != nil {
		return nil
	}
	keys := []string{}
	var visit func(any)
	visit = func(value any) {
		switch node := value.(type) {
		case map[string]any:
			for key, child := range node {
				if key == "labelKey" || key == "titleKey" || key == "confirmKey" || key == "successKey" {
					if text, ok := child.(string); ok && text != "" {
						keys = append(keys, text)
					}
				}
				if key == "valueLabelPrefix" {
					if prefix, ok := child.(string); ok && prefix != "" {
						values, _ := node["enum"].([]any)
						if len(values) == 0 {
							keys = append(keys, prefix)
						}
						for _, value := range values {
							if text, ok := value.(string); ok {
								keys = append(keys, prefix+text)
							}
						}
					}
				}
				visit(child)
			}
		case []any:
			for _, child := range node {
				visit(child)
			}
		}
	}
	visit(document)
	return keys
}
