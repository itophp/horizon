package memory

import (
	"context"
	"strconv"
	"strings"

	"baseline/services/api/kernel/capability/contracts"
	"baseline/services/api/kernel/capability/gateway"
)

// entityDefinitions extends the same lifecycle to the registered example
// entity. Additional resource types must add both a Gateway adapter and an SQL
// scope predicate; unknown types never become readable by default.
func entityDefinitions() []contracts.Definition {
	groups := groupDefinitions()
	out := make([]contracts.Definition, 0, len(groups)*2)
	for _, group := range groups {
		op := strings.TrimPrefix(group.Name, "memory.group.")
		input := *group.Input
		input.Properties = make(map[string]*contracts.Schema, len(group.Input.Properties)+1)
		for key, value := range group.Input.Properties {
			input.Properties[key] = value
		}
		input.Properties["scopeType"] = &contracts.Schema{Type: "string", Enum: []string{"business_entity"}}
		input.Properties["resourceType"] = &contracts.Schema{Type: "string", Enum: []string{"example_record"}}
		input.Required = append(append([]string(nil), group.Input.Required...), "resourceType")
		parent := group
		parent.Name = "memory.entity." + op
		parent.Title = parent.Name
		parent.Layer = contracts.LayerOrchestration
		parent.Description = "Authorize the source entity through its capability before accessing memory."
		parent.Executor = "go:" + parent.Name
		parent.Input = &input
		parent.Tags = []string{"manual-only"}
		if !parent.ReadOnly {
			parent.SuccessEvents = []string{parent.Name + ".completed"}
		}
		child := group
		child.Name = "memory.entity.data." + op
		child.Title = child.Name
		child.Description = "Internal memory data operation authorized by the pinned parent."
		child.Executor = "go:" + child.Name
		child.Input = &input
		child.Tags = []string{"internal-only", "parent:" + parent.Name + "@1.0.0"}
		if !child.ReadOnly {
			child.SuccessEvents = []string{child.Name + ".completed"}
		}
		out = append(out, parent, child)
	}
	return out
}

func (m *Module) authorizeEntity(ctx context.Context, in contracts.Input) error {
	resource, _ := in["resourceType"].(string)
	if resource != "example_record" {
		return contracts.Problem("forbidden")
	}
	r, err := m.gateway.Invoke(ctx, gateway.Request{Name: "example.record.get", Version: "1.0.0", Input: contracts.Input{"id": valueInt(in["scopeId"])}, Origin: contracts.OriginCapability})
	if err != nil {
		return err
	}
	if r.Status != contracts.StatusSucceeded {
		if r.ErrorCode == "" {
			return contracts.Problem("unavailable")
		}
		return contracts.Problem(r.ErrorCode)
	}
	return nil
}

func (m *Module) entityFlow(ctx context.Context, name string, in contracts.Input) (contracts.Output, error) {
	if !manualInvocation(ctx) {
		return contracts.Output{}, contracts.Problem("forbidden")
	}
	if m.gateway == nil {
		return contracts.Output{}, contracts.Problem("unavailable")
	}
	if err := m.authorizeEntity(ctx, in); err != nil {
		return contracts.Output{}, err
	}
	op := strings.TrimPrefix(name, "memory.entity.")
	child := "memory.entity.data." + op
	r, err := m.gateway.Invoke(ctx, gateway.Request{Name: child, Version: "1.0.0", Input: in, Origin: contracts.OriginCapability, IdempotencyKey: gateway.ChildKey(ctx, "entity-memory")})
	if err != nil {
		return contracts.Output{}, err
	}
	if r.Status != contracts.StatusSucceeded {
		if r.ErrorCode == "" {
			return contracts.Output{}, contracts.Problem("unavailable")
		}
		return contracts.Output{}, contracts.Problem(r.ErrorCode)
	}
	if op == "list" || op == "get" || op == "sources" {
		if err := m.authorizeEntity(ctx, in); err != nil {
			return contracts.Output{}, err
		}
		return contracts.Output{Data: r.Data}, nil
	}
	return contracts.Output{Data: r.Data, Events: []contracts.Event{{Type: name + ".completed", AggregateType: "example_record", AggregateID: strconv.FormatInt(valueInt(in["scopeId"]), 10), Payload: map[string]any{}}}}, nil
}
