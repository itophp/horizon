package settings

import (
	"context"
	"fmt"

	"baseline/services/api/internal/identity"
	"baseline/services/api/internal/platform/capability/contracts"
	"baseline/services/api/internal/platform/capability/registry"
)

// Event types published by the settings module.
const (
	PreferencesUpdated = "settings.preferences_updated"
	DraftCreated       = "settings.draft_created"
	Published          = "settings.published"
	RolledBack         = "settings.rolled_back"
	Scheduled          = "settings.scheduled"
	ScheduleCancelled  = "settings.schedule_cancelled"
)

const version = "1.0.0"

// Entries is the capability catalog contributed by the settings module. Every
// console button and every agent tool calls these definitions; the module still
// owns the transaction, validation and revision locking.
func (s *Service) Entries() []registry.Entry {
	return []registry.Entry{
		{Definition: s.workspaceDefinition(), Handler: contracts.Handler{Execute: s.readWorkspace}},
		{Definition: s.effectiveDefinition(), Handler: contracts.Handler{Execute: s.readEffective}},
		{Definition: s.preferencesDefinition(), Handler: contracts.Handler{Execute: s.updatePreferences}},
		{Definition: s.draftDefinition(), Handler: contracts.Handler{Execute: s.createDraft}},
		{Definition: s.publishDefinition(), Handler: contracts.Handler{Execute: s.publishDraft}},
		{Definition: s.rollbackDefinition(), Handler: contracts.Handler{Execute: s.rollbackVersion}},
		{Definition: s.schedulesDefinition(), Handler: contracts.Handler{Execute: s.listSchedules}},
		{Definition: s.scheduleDefinition(), Handler: contracts.Handler{Execute: s.scheduleDraft}},
		{Definition: s.cancelScheduleDefinition(), Handler: contracts.Handler{Execute: s.cancelSchedule}},
		{Definition: s.exportDefinition(), Handler: contracts.Handler{Execute: s.exportEntries}},
		{Definition: s.logoDefinition(), Handler: contracts.Handler{Execute: s.readLogo}},
		{Definition: s.verifyDefinition(), Handler: contracts.Handler{Execute: s.verifyPassword}},
	}
}

func principalID(ctx context.Context) int64 {
	p, err := identity.Current(ctx)
	if err != nil || p.Account == nil {
		return 0
	}
	return p.Account.ID
}

func targetSchema() *contracts.Schema {
	return contracts.Object(map[string]*contracts.Schema{
		"kind": contracts.Enum("workspace"),
		"id":   contracts.Int(0, 0),
	}, "kind")
}

func changeSchema() *contracts.Schema {
	return contracts.Object(map[string]*contracts.Schema{
		"key":       contracts.Str(1, 100),
		"operation": contracts.Enum("set", "reset", "clear"),
		"value":     contracts.Str(0, 1048576),
	}, "key", "operation")
}

func scopeFields() map[string]*contracts.Schema {
	return map[string]*contracts.Schema{
		"targetKind": contracts.Enum("workspace"),
		"targetId":   contracts.Int(0, 0),
	}
}

func (s *Service) workspaceDefinition() contracts.Definition {
	return contracts.Definition{Layer: contracts.LayerAtomic,
		Name: "settings.workspace.read", Version: version, Title: "Read the settings workspace",
		Description: "Return the configuration definitions, effective entries, drafts and history of one scope.",
		Domain:      "settings", Entity: "settings", Permission: "settings:read",
		Risk: contracts.RiskLow, Approval: contracts.ApprovalAuto, ReadOnly: true,
		Executor: "go:settings.workspace", Tags: []string{"settings", "read", "agent"},
		Input:  contracts.Object(scopeFields()),
		Output: contracts.Object(map[string]*contracts.Schema{"revision": {Type: "integer"}}),
	}
}

func (s *Service) effectiveDefinition() contracts.Definition {
	return contracts.Definition{Layer: contracts.LayerAtomic,
		Name: "settings.effective.read", Version: version, Title: "Read effective configuration",
		Description: "Return the configuration the current account actually sees, including personal preferences.",
		Domain:      "settings", Entity: "settings", Permission: "",
		Risk: contracts.RiskLow, Approval: contracts.ApprovalAuto, ReadOnly: true,
		Executor: "go:settings.effective", Tags: []string{"settings", "read", "agent"},
		Input:  contracts.Object(map[string]*contracts.Schema{}),
		Output: contracts.Object(map[string]*contracts.Schema{"revision": {Type: "string"}}),
	}
}

func (s *Service) preferencesDefinition() contracts.Definition {
	return contracts.Definition{Layer: contracts.LayerAtomic,
		Name: "settings.preferences.update", Version: version, Title: "Update personal preferences",
		Description: "Change the caller's own display and notification preferences.",
		Domain:      "settings", Entity: "settings_account", Permission: "",
		Risk: contracts.RiskLow, Approval: contracts.ApprovalAuto,
		Idempotency:   contracts.Idempotency{Required: true, Scope: "tenant+account+key"},
		SuccessEvents: []string{PreferencesUpdated}, Executor: "go:settings.preferences",
		Tags:  []string{"settings", "write"},
		Input: contracts.Object(map[string]*contracts.Schema{"changes": contracts.Array(changeSchema(), 1, 20)}, "changes"),
		Output: contracts.Object(map[string]*contracts.Schema{
			"entries":  {Type: "array", Items: contracts.FreeObject()},
			"revision": {Type: "string"},
		}),
		Loggable: []string{"changes"},
	}
}

func (s *Service) draftDefinition() contracts.Definition {
	return contracts.Definition{Layer: contracts.LayerAtomic,
		Name: "settings.draft.create", Version: version, Title: "Create a settings draft",
		Description: "Stage configuration changes against a revision. Nothing takes effect until publish.",
		Domain:      "settings", Entity: "settings_draft", Permission: "settings:update",
		Risk: contracts.RiskMedium, Approval: contracts.ApprovalAuto,
		Idempotency:   contracts.Idempotency{Required: true, Scope: "tenant+scope+key"},
		SuccessEvents: []string{DraftCreated}, Executor: "go:settings.draft",
		Tags: []string{"settings", "write"},
		Input: contracts.Object(map[string]*contracts.Schema{
			"target":       targetSchema(),
			"baseRevision": contracts.Int(0, 1<<40),
			"reason":       contracts.Str(5, 500),
			"changes":      contracts.Array(changeSchema(), 1, 100),
		}, "target", "baseRevision", "changes"),
		Output:   contracts.Object(map[string]*contracts.Schema{"id": {Type: "integer"}, "status": {Type: "string"}}),
		Loggable: []string{"target", "baseRevision", "reason"},
	}
}

func (s *Service) publishDefinition() contracts.Definition {
	return contracts.Definition{Layer: contracts.LayerAtomic,
		Name: "settings.draft.publish", Version: version, Title: "Publish a settings draft",
		Description: "Apply a reviewed draft. Requires a short lived reauthentication proof.",
		Domain:      "settings", Entity: "settings_draft", Permission: "settings:publish",
		// The action is performed on a draft and produces a version. Publishing a
		// sensitive change also re-binds files and can clamp live sessions.
		Entities: []string{"settings_draft", "settings_version", "settings", "file", "session"},
		Risk:     contracts.RiskHigh, Approval: contracts.ApprovalConfirm,
		Idempotency:   contracts.Idempotency{Required: true, Scope: "tenant+draft+key"},
		SuccessEvents: []string{Published}, Executor: "go:settings.publish",
		Tags: []string{"settings", "write"},
		Input: contracts.Object(map[string]*contracts.Schema{
			"id":           contracts.Int(1, 1<<40),
			"baseRevision": contracts.Int(0, 1<<40),
			"proof":        contracts.Str(0, 200),
		}, "id", "baseRevision", "proof"),
		Output:   contracts.Object(map[string]*contracts.Schema{"id": {Type: "integer"}, "revision": {Type: "integer"}}),
		Loggable: []string{"id", "baseRevision"},
	}
}

func (s *Service) rollbackDefinition() contracts.Definition {
	return contracts.Definition{Layer: contracts.LayerAtomic,
		Name: "settings.history.rollback", Version: version, Title: "Roll settings back",
		Description: "Create a new revision from a published settings version.",
		Domain:      "settings", Entity: "settings_version", Permission: "settings:rollback",
		Risk: contracts.RiskHigh, Approval: contracts.ApprovalConfirm,
		Idempotency:   contracts.Idempotency{Required: true, Scope: "tenant+version+key"},
		SuccessEvents: []string{RolledBack}, Executor: "go:settings.rollback",
		Tags: []string{"settings", "write"},
		Input: contracts.Object(map[string]*contracts.Schema{
			"id":           contracts.Int(1, 1<<40),
			"baseRevision": contracts.Int(0, 1<<40),
			"proof":        contracts.Str(0, 200),
			"reason":       contracts.Str(5, 500),
		}, "id", "baseRevision", "proof", "reason"),
		Output:   contracts.Object(map[string]*contracts.Schema{"id": {Type: "integer"}, "revision": {Type: "integer"}}),
		Loggable: []string{"id", "baseRevision", "reason"},
	}
}

func (s *Service) schedulesDefinition() contracts.Definition {
	return contracts.Definition{Layer: contracts.LayerAtomic,
		Name: "settings.schedule.list", Version: version, Title: "List scheduled publications",
		Description: "List pending and finished settings publications of the tenant.",
		Domain:      "settings", Entity: "settings_schedule", Permission: "settings:read",
		Risk: contracts.RiskLow, Approval: contracts.ApprovalAuto, ReadOnly: true,
		Executor: "go:settings.schedules", Tags: []string{"settings", "read"},
		Input:  contracts.Object(map[string]*contracts.Schema{}),
		Output: contracts.Object(map[string]*contracts.Schema{"schedules": {Type: "array", Items: contracts.FreeObject()}}),
	}
}

func (s *Service) scheduleDefinition() contracts.Definition {
	return contracts.Definition{Layer: contracts.LayerAtomic,
		Name: "settings.schedule.create", Version: version, Title: "Schedule a publication",
		Description: "Publish a draft at a future time.",
		Domain:      "settings", Entity: "settings_schedule", Permission: "settings:publish",
		Risk: contracts.RiskHigh, Approval: contracts.ApprovalConfirm,
		Idempotency:   contracts.Idempotency{Required: true, Scope: "tenant+draft+key"},
		SuccessEvents: []string{Scheduled}, Executor: "go:settings.schedule",
		Tags: []string{"settings", "write"},
		Input: contracts.Object(map[string]*contracts.Schema{
			"id":           contracts.Int(1, 1<<40),
			"baseRevision": contracts.Int(0, 1<<40),
			"proof":        contracts.Str(0, 200),
			"scheduledAt":  contracts.Str(1, 40),
		}, "id", "baseRevision", "proof", "scheduledAt"),
		Output:   contracts.Object(map[string]*contracts.Schema{"id": {Type: "integer"}, "status": {Type: "string"}}),
		Loggable: []string{"id", "scheduledAt"},
	}
}

func (s *Service) cancelScheduleDefinition() contracts.Definition {
	return contracts.Definition{Layer: contracts.LayerAtomic,
		Name: "settings.schedule.cancel", Version: version, Title: "Cancel a scheduled publication",
		Description: "Cancel a pending scheduled settings publication.",
		Domain:      "settings", Entity: "settings_schedule", Permission: "settings:publish",
		Risk: contracts.RiskMedium, Approval: contracts.ApprovalAuto,
		Idempotency:   contracts.Idempotency{Required: true, Scope: "tenant+schedule+key"},
		SuccessEvents: []string{ScheduleCancelled}, Executor: "go:settings.cancel_schedule",
		Tags:     []string{"settings", "write"},
		Input:    contracts.Object(map[string]*contracts.Schema{"id": contracts.Int(1, 1<<40)}, "id"),
		Output:   contracts.Object(map[string]*contracts.Schema{"id": {Type: "integer"}}),
		Loggable: []string{"id"},
	}
}

func (s *Service) exportDefinition() contracts.Definition {
	return contracts.Definition{Layer: contracts.LayerAtomic,
		Name: "settings.export.read", Version: version, Title: "Export settings",
		Description: "Export the effective entries of one scope as a change list.",
		Domain:      "settings", Entity: "settings", Permission: "settings:export",
		Risk: contracts.RiskLow, Approval: contracts.ApprovalAuto, ReadOnly: true,
		Executor: "go:settings.export", Tags: []string{"settings", "read"},
		Input:  contracts.Object(scopeFields()),
		Output: contracts.Object(map[string]*contracts.Schema{"entries": {Type: "array", Items: contracts.FreeObject()}}),
	}
}

func (s *Service) logoDefinition() contracts.Definition {
	return contracts.Definition{Layer: contracts.LayerAtomic,
		Name: "settings.logo.read", Version: version, Title: "Read the workspace logo",
		Description: "Return the configured logo as base64 content.",
		Domain:      "settings", Entity: "settings", Permission: "",
		Risk: contracts.RiskLow, Approval: contracts.ApprovalAuto, ReadOnly: true,
		Executor: "go:settings.logo", Tags: []string{"settings", "read"},
		Input:  contracts.Object(map[string]*contracts.Schema{}),
		Output: contracts.Object(map[string]*contracts.Schema{"name": {Type: "string"}, "mime": {Type: "string"}}),
	}
}

func (s *Service) verifyDefinition() contracts.Definition {
	return contracts.Definition{Layer: contracts.LayerAtomic,
		Name: "settings.password.verify", Version: version, Title: "Reauthenticate",
		Description: "Verify the caller's password and issue a short lived proof for a sensitive publication.",
		Domain:      "settings", Entity: "settings", Permission: "",
		Risk: contracts.RiskMedium, Approval: contracts.ApprovalAuto, ReadOnly: true,
		Executor: "go:settings.reauthenticate", Tags: []string{"settings", "auth"},
		Input: contracts.Object(map[string]*contracts.Schema{
			"password":   contracts.Str(1, 128),
			"operation":  contracts.Enum("settings.publish", "settings.schedule", "settings.rollback", "dictionaries.rollback"),
			"resourceId": contracts.Int(1, 1<<40),
			"revision":   contracts.Int(0, 1<<40),
		}, "password", "operation", "resourceId", "revision"),
		Output: contracts.Object(map[string]*contracts.Schema{"proof": {Type: "string"}, "expiresAt": {Type: "string"}}, "proof"),
	}
}

func (s *Service) readWorkspace(ctx context.Context, in contracts.Input) (contracts.Output, error) {
	payload := WorkspacePayload{}
	if err := contracts.Bind(in, &payload); err != nil {
		return contracts.Output{}, err
	}
	if payload.TargetKind == "" {
		payload.TargetKind = "workspace"
	}
	out, err := s.Workspace(ctx, &payload)
	if err != nil {
		return contracts.Output{}, err
	}
	return contracts.Output{Data: out, Summary: fmt.Sprintf("revision=%d", out.Revision)}, nil
}

func (s *Service) readEffective(ctx context.Context, in contracts.Input) (contracts.Output, error) {
	if err := contracts.Bind(in, &EffectivePayload{}); err != nil {
		return contracts.Output{}, err
	}
	out, err := s.Effective(ctx, &EffectivePayload{})
	if err != nil {
		return contracts.Output{}, err
	}
	return contracts.Output{Data: out, Summary: fmt.Sprintf("entries=%d", len(out.Entries))}, nil
}

func (s *Service) updatePreferences(ctx context.Context, in contracts.Input) (contracts.Output, error) {
	payload := PreferencesPayload{}
	if err := contracts.Bind(in, &payload); err != nil {
		return contracts.Output{}, err
	}
	out, err := s.Preferences(ctx, &payload)
	if err != nil {
		return contracts.Output{}, err
	}
	event := contracts.Event{
		Type: PreferencesUpdated, AggregateType: "settings_account", AggregateID: fmt.Sprintf("%d", principalID(ctx)),
		Payload: map[string]any{"keys": len(payload.Changes)},
	}
	return contracts.Output{Data: out, Summary: fmt.Sprintf("changes=%d", len(payload.Changes)), Events: []contracts.Event{event}}, nil
}

func (s *Service) createDraft(ctx context.Context, in contracts.Input) (contracts.Output, error) {
	payload := DraftPayload{}
	if err := contracts.Bind(in, &payload); err != nil {
		return contracts.Output{}, err
	}
	out, err := s.Draft(ctx, &payload)
	if err != nil {
		return contracts.Output{}, err
	}
	return contracts.Output{Data: out, Summary: fmt.Sprintf("draft=%d", out.ID), Events: []contracts.Event{{
		Type: DraftCreated, AggregateType: "settings_draft", AggregateID: fmt.Sprintf("%d", out.ID),
		Payload: map[string]any{"draftId": out.ID, "changes": len(payload.Changes)},
	}}}, nil
}

func (s *Service) publishDraft(ctx context.Context, in contracts.Input) (contracts.Output, error) {
	payload := PublishPayload{}
	if err := contracts.Bind(in, &payload); err != nil {
		return contracts.Output{}, err
	}
	out, err := s.Publish(ctx, &payload)
	if err != nil {
		return contracts.Output{}, err
	}
	return contracts.Output{Data: out, Summary: fmt.Sprintf("revision=%d", out.Revision), Events: []contracts.Event{{
		Type: Published, AggregateType: "settings_version", AggregateID: fmt.Sprintf("%d", out.ID),
		Payload: map[string]any{"versionId": out.ID, "revision": out.Revision, "draftId": payload.ID},
	}}}, nil
}

func (s *Service) rollbackVersion(ctx context.Context, in contracts.Input) (contracts.Output, error) {
	payload := RollbackPayload{}
	if err := contracts.Bind(in, &payload); err != nil {
		return contracts.Output{}, err
	}
	out, err := s.Rollback(ctx, &payload)
	if err != nil {
		return contracts.Output{}, err
	}
	return contracts.Output{Data: out, Summary: fmt.Sprintf("revision=%d", out.Revision), Events: []contracts.Event{{
		Type: RolledBack, AggregateType: "settings_version", AggregateID: fmt.Sprintf("%d", out.ID),
		Payload: map[string]any{"versionId": out.ID, "revision": out.Revision, "reason": payload.Reason},
	}}}, nil
}

func (s *Service) listSchedules(ctx context.Context, in contracts.Input) (contracts.Output, error) {
	if err := contracts.Bind(in, &SchedulesPayload{}); err != nil {
		return contracts.Output{}, err
	}
	out, err := s.Schedules(ctx, &SchedulesPayload{})
	if err != nil {
		return contracts.Output{}, err
	}
	return contracts.Output{Data: out, Summary: fmt.Sprintf("schedules=%d", len(out))}, nil
}

func (s *Service) scheduleDraft(ctx context.Context, in contracts.Input) (contracts.Output, error) {
	payload := SchedulePayload{}
	if err := contracts.Bind(in, &payload); err != nil {
		return contracts.Output{}, err
	}
	out, err := s.Schedule(ctx, &payload)
	if err != nil {
		return contracts.Output{}, err
	}
	return contracts.Output{Data: out, Summary: fmt.Sprintf("schedule=%d", out.ID), Events: []contracts.Event{{
		Type: Scheduled, AggregateType: "settings_schedule", AggregateID: fmt.Sprintf("%d", out.ID),
		Payload: map[string]any{"scheduleId": out.ID, "draftId": out.DraftID, "scheduledAt": out.ScheduledAt},
	}}}, nil
}

func (s *Service) cancelSchedule(ctx context.Context, in contracts.Input) (contracts.Output, error) {
	payload := CancelSchedulePayload{}
	if err := contracts.Bind(in, &payload); err != nil {
		return contracts.Output{}, err
	}
	if err := s.CancelSchedule(ctx, &payload); err != nil {
		return contracts.Output{}, err
	}
	return contracts.Output{Data: map[string]any{"id": payload.ID}, Summary: fmt.Sprintf("schedule=%d", payload.ID), Events: []contracts.Event{{
		Type: ScheduleCancelled, AggregateType: "settings_schedule", AggregateID: fmt.Sprintf("%d", payload.ID),
		Payload: map[string]any{"scheduleId": payload.ID},
	}}}, nil
}

func (s *Service) exportEntries(ctx context.Context, in contracts.Input) (contracts.Output, error) {
	payload := ExportPayload{}
	if err := contracts.Bind(in, &payload); err != nil {
		return contracts.Output{}, err
	}
	if payload.TargetKind == "" {
		payload.TargetKind = "workspace"
	}
	out, err := s.Export(ctx, &payload)
	if err != nil {
		return contracts.Output{}, err
	}
	return contracts.Output{Data: out, Summary: fmt.Sprintf("entries=%d", len(out))}, nil
}

func (s *Service) readLogo(ctx context.Context, in contracts.Input) (contracts.Output, error) {
	if err := contracts.Bind(in, &LogoPayload{}); err != nil {
		return contracts.Output{}, err
	}
	out, err := s.Logo(ctx, &LogoPayload{})
	if err != nil {
		return contracts.Output{}, err
	}
	return contracts.Output{Data: out, Summary: "logo"}, nil
}

func (s *Service) verifyPassword(ctx context.Context, in contracts.Input) (contracts.Output, error) {
	payload := ReauthenticatePayload{}
	if err := contracts.Bind(in, &payload); err != nil {
		return contracts.Output{}, err
	}
	out, err := s.Reauthenticate(ctx, &payload)
	if err != nil {
		return contracts.Output{}, err
	}
	return contracts.Output{Data: out, Summary: "verified"}, nil
}
