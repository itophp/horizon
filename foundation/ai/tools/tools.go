// Package tools adapts registered capabilities into Eino tools. The adapter is
// thin on purpose: every call goes through the capability gateway, so an agent
// gets exactly the same authorization, approval, idempotency, outbox and audit as
// a human clicking a button.
package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"

	"horizon/foundation/kernel/capability/contracts"
	"horizon/foundation/kernel/capability/gateway"
	"horizon/foundation/kernel/capability/registry"
	"horizon/foundation/system/governance"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
)

// maxResult bounds what one tool result may add to the model context.
const maxResult = 12000

// Adapter builds the tool set for one run.
type Adapter struct {
	AllowedKeys []string
	Gateway     *gateway.Gateway
	Registry    *registry.Registry
	Caller      contracts.Caller
	TraceID     string
	RunID       int64
	Depth       int
	Observe     func(capability, status string)
}

// Tools returns the capabilities the caller may use.
func (a *Adapter) Tools(contexts ...context.Context) []tool.BaseTool {
	definitions := a.Registry.DiscoverAutomated(a.Caller)
	if len(contexts) > 0 && a.Gateway != nil {
		var err error
		if a.AllowedKeys != nil {
			definitions, err = a.Gateway.DefinitionsFor(contexts[0], a.AllowedKeys)
		} else {
			definitions, err = a.Gateway.Definitions(contexts[0])
		}
		if err != nil {
			return nil
		}
	}
	out := make([]tool.BaseTool, 0, len(definitions))
	for _, definition := range definitions {
		if slices.Contains(definition.Tags, "system-only") || slices.Contains(definition.Tags, "manual-only") || slices.Contains(definition.Tags, "internal-only") || !definition.AllowsOrigin(contracts.OriginAgent) {
			continue
		}
		out = append(out, &capabilityTool{definition: definition, adapter: a})
	}
	return out
}

type capabilityTool struct {
	definition contracts.Definition
	adapter    *Adapter
}

func (t *capabilityTool) Info(context.Context) (*schema.ToolInfo, error) {
	document, err := t.definition.Input.Document()
	if err != nil {
		return nil, err
	}
	return &schema.ToolInfo{
		Name:        t.definition.ToolName(),
		Desc:        t.definition.Description,
		ParamsOneOf: schema.NewParamsOneOfByJSONSchema(document),
	}, nil
}

// InvokableRun returns business failures as data so the model can explain them
// instead of aborting the whole turn.
func (t *capabilityTool) InvokableRun(ctx context.Context, argumentsInJSON string, _ ...tool.Option) (string, error) {
	observe := func(status string) {
		if t.adapter.Observe != nil {
			t.adapter.Observe(t.definition.Name, status)
		}
	}
	input := contracts.Input{}
	if argumentsInJSON != "" {
		if err := json.Unmarshal([]byte(argumentsInJSON), &input); err != nil {
			return envelope{OK: false, Status: "failed", ErrorCode: "invalid"}.String(), nil
		}
	}
	key := ""
	if t.definition.Idempotency.Required {
		key = fmt.Sprintf("run:%d:%s:%s", t.adapter.RunID, t.definition.Name, contracts.Digest(input))
	}
	observe("running")
	if t.adapter.RunID > 0 {
		ctx = gateway.WithTaskReference(ctx, gateway.TaskReference{RunID: t.adapter.RunID, TenantID: t.adapter.Caller.TenantID, AccountID: t.adapter.Caller.AccountID, DepartmentID: t.adapter.Caller.DepartmentID})
	}
	result, err := t.adapter.Gateway.Invoke(ctx, gateway.Request{
		Name: t.definition.Name, Version: t.definition.Version, Input: input,
		IdempotencyKey: key, Origin: contracts.OriginAgent,
		CorrelationID: t.adapter.TraceID,
		CausationID:   fmt.Sprintf("run:%d", t.adapter.RunID),
		TriggerDepth:  t.adapter.Depth,
	})
	if err != nil {
		observe("failed")
		return envelope{OK: false, Status: "failed", ErrorCode: mapCode(err)}.String(), nil
	}
	observe(string(result.Status))
	out := envelope{
		OK: result.Status == contracts.StatusSucceeded, Status: string(result.Status),
		InvocationID: result.InvocationID, Data: result.Data, EventIDs: result.EventIDs,
		ErrorCode: result.ErrorCode, ApprovalID: result.ApprovalID,
	}
	if result.Status == contracts.StatusAwaitingApproval {
		out.OK = false
		out.ErrorCode = "approval_required"
	}
	return out.String(), nil
}

type envelope struct {
	OK           bool     `json:"ok"`
	Status       string   `json:"status"`
	InvocationID string   `json:"invocationId,omitempty"`
	Data         any      `json:"data,omitempty"`
	EventIDs     []string `json:"eventIds,omitempty"`
	ErrorCode    string   `json:"errorCode,omitempty"`
	ApprovalID   int64    `json:"approvalId,omitempty"`
}

func (e envelope) String() string {
	raw, err := json.Marshal(e)
	if err != nil {
		return `{"ok":false,"status":"failed","errorCode":"internal"}`
	}
	if len(raw) > maxResult {
		trimmed, _ := json.Marshal(envelope{OK: false, Status: "failed", InvocationID: e.InvocationID, ErrorCode: "result_too_large"})
		return string(trimmed)
	}
	return string(raw)
}

func mapCode(err error) string {
	if err == nil {
		return ""
	}
	var coded *contracts.Error
	if errors.As(err, &coded) {
		return coded.Code
	}
	var domain *governance.Error
	if errors.As(err, &domain) {
		return domain.Code
	}
	return "internal"
}
