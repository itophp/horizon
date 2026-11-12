package main

import (
	"encoding/json"
	"fmt"
	ar "horizon/foundation/kernel/applicationruntime"
	"os"
	"strings"
)

func validateLabels(apps []ar.ApplicationV2) error {
	raw, err := json.Marshal(apps)
	if err != nil {
		return err
	}
	var data any
	if err = json.Unmarshal(raw, &data); err != nil {
		return err
	}
	labels := map[string]bool{}
	var visit func(any)
	visit = func(value any) {
		switch x := value.(type) {
		case map[string]any:
			for key, v := range x {
				if key == "labelKey" || key == "titleKey" || key == "confirmKey" || key == "successKey" {
					if s, ok := v.(string); ok && s != "" {
						labels[s] = true
					}
				}
				visit(v)
			}
		case []any:
			for _, v := range x {
				visit(v)
			}
		}
	}
	visit(data)
	for _, lang := range []string{"en", "zh-CN"} {
		for label := range labels {
			parts := strings.SplitN(label, ":", 2)
			if len(parts) != 2 {
				return fmt.Errorf("invalid label %s", label)
			}
			raw, err := os.ReadFile("../apps/web/src/i18n/locales/" + lang + "/" + parts[0] + ".json")
			if err != nil {
				return err
			}
			var resource any
			if err = json.Unmarshal(raw, &resource); err != nil {
				return err
			}
			for _, part := range strings.Split(parts[1], ".") {
				object, ok := resource.(map[string]any)
				if !ok {
					return fmt.Errorf("missing label %s/%s", lang, label)
				}
				resource, ok = object[part]
				if !ok {
					return fmt.Errorf("missing label %s/%s", lang, label)
				}
			}
		}
	}
	return nil
}
