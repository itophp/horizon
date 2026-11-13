package applicationruntime

import (
	"fmt"

	"horizon/foundation/kernel/capability/contracts"
)

type browserActionContract struct {
	input     *contracts.Schema
	formOnly  bool
	sensitive []string
}

var browserActions = map[string]browserActionContract{
	"webmcp.connect": {
		input:    contracts.Object(map[string]*contracts.Schema{"port": contracts.Int(1024, 65535), "token": contracts.Str(64, 64)}, "port", "token"),
		formOnly: true, sensitive: []string{"token"},
	},
	"webmcp.disconnect": {input: contracts.Object(nil)},
	"webmcp.install": {
		input: contracts.Object(map[string]*contracts.Schema{"client": contracts.Enum("codex", "workbuddy")}, "client"),
	},
	"remote-mcp.diagnose": {
		input:    contracts.Object(map[string]*contracts.Schema{"token": contracts.Str(1, 4096)}, "token"),
		formOnly: true, sensitive: []string{"token"},
	},
	"remote-mcp.prepare": {
		input: contracts.Object(map[string]*contracts.Schema{
			"client": contracts.Enum("codex", "workbuddy"), "key": contracts.Str(64, 64), "token": contracts.Str(1, 4096),
			"departmentId": contracts.Int(1, 1<<40),
		}, "client", "key", "token"),
		formOnly: true, sensitive: []string{"key", "token"},
	},
	"remote-mcp.install": {input: contracts.Object(nil)},
	"remote-mcp.cancel":  {input: contracts.Object(nil)},
}

func validateBrowserAction(component *ComponentSpec, event *EventBinding, allowed bool) error {
	contract, registered := browserActions[event.Target]
	if !allowed || !registered {
		return fmt.Errorf("browser operation %q is not available on this page", event.Target)
	}
	if event.ResultTarget != "" || event.Result != nil || event.Disclosure != nil || event.IdempotencyKey != nil || event.Reauthenticate != nil || event.Download != nil {
		return fmt.Errorf("browser operations cannot store a result or use a Gateway idempotency key")
	}
	if contract.formOnly && (component.Kind != "action-form" || event.Event != "submit") {
		return fmt.Errorf("browser operation %q requires form submission", event.Target)
	}
	for _, field := range contract.sensitive {
		found := false
		if event.Payload.Kind == "object" {
			for _, mapping := range event.Payload.Fields {
				if mapping.Target == field && mapping.Value.Kind == "path" && mapping.Value.Path == "event.values."+field {
					found = true
				}
			}
		}
		if !found {
			return fmt.Errorf("browser field %q must come from the current form", field)
		}
	}
	event.InputSchema = JSONSchema(contract.input)
	return nil
}
