// Package automation implements user owned, time or interval driven agent
// tasks. A task is a capability contract: its schedule, instruction and owner
// are stored once, and a worker launches one workspace agent run per occurrence
// under the owner's re-resolved authority. The agent itself is the existing
// harness agent; this package owns scheduling, deduplication and history only.
package automation

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"horizon/foundation/kernel/capability/contracts"
	"horizon/foundation/kernel/capability/gateway"
	"horizon/foundation/kernel/capability/registry"
	"horizon/foundation/system/identity"
	"horizon/internal/repository/dbgen"
)

const (
	version              = "1.0.0"
	ReadPermission       = "automation:read"
	WritePermission      = "automation:write"
	maxNameLength        = 120
	maxDescriptionLength = 4000
	// maxTasksPerOwner bounds an account's automations. Agent conversations may
	// create tasks, so the cap is what stops a single account from turning a
	// conversation into an unbounded fleet of scheduled agent runs.
	maxTasksPerOwner = 20
)

// events names the fact each write capability produces. Declarations and
// executors read the same map so a typo cannot make the declared event a lie.
var events = map[string]string{
	"create":  "automation.task.created",
	"update":  "automation.task.updated",
	"enable":  "automation.task.enabled",
	"disable": "automation.task.disabled",
	"delete":  "automation.task.deleted",
	"run":     "automation.run.requested",
}

// Module owns the automation capabilities. The worker is constructed separately
// with the agent launcher, so the capability catalog never needs a model.
type Module struct{ Pool *pgxpool.Pool }

// New builds the module. A nil pool keeps catalog assembly and offline checks
// working while every executor reports unavailable.
func New(pool *pgxpool.Pool) *Module { return &Module{Pool: pool} }

// Entries registers every automation capability exactly once.
func (m *Module) Entries() []registry.Entry {
	return []registry.Entry{
		{Definition: m.createDefinition(), Handler: contracts.Handler{ExecuteTx: m.create}},
		{Definition: m.updateDefinition(), Handler: contracts.Handler{ExecuteTx: m.update}},
		{Definition: m.enableDefinition(), Handler: contracts.Handler{ExecuteTx: m.setEnabled}},
		{Definition: m.disableDefinition(), Handler: contracts.Handler{ExecuteTx: m.setEnabled}},
		{Definition: m.deleteDefinition(), Handler: contracts.Handler{ExecuteTx: m.remove}},
		{Definition: m.listDefinition(), Handler: contracts.Handler{Execute: m.list}},
		{Definition: m.getDefinition(), Handler: contracts.Handler{Execute: m.get}},
		{Definition: m.runNowDefinition(), Handler: contracts.Handler{ExecuteTx: m.runNow}},
		{Definition: m.runListDefinition(), Handler: contracts.Handler{Execute: m.runList}},
		{Definition: m.runGetDefinition(), Handler: contracts.Handler{Execute: m.runGet}},
	}
}

func (m *Module) createDefinition() contracts.Definition {
	return taskWriteDefinition("create", "Create an automation task")
}

func (m *Module) updateDefinition() contracts.Definition {
	return taskWriteDefinition("update", "Update an automation task")
}

func (m *Module) deleteDefinition() contracts.Definition {
	d := idWriteDefinition("delete", "Delete an automation task")
	return d
}
func (m *Module) enableDefinition() contracts.Definition {
	return idWriteDefinition("enable", "Enable an automation task")
}
func (m *Module) disableDefinition() contracts.Definition {
	return idWriteDefinition("disable", "Disable an automation task")
}

func (m *Module) listDefinition() contracts.Definition {
	d := contracts.Definition{
		Name: "automation.task.list", Version: version, Title: "List automation tasks",
		Description: "List the caller's own automation tasks.", Domain: "automation", Entity: "automation_task",
		Layer: contracts.LayerAtomic, Permission: ReadPermission, ReadOnly: true,
		Risk: contracts.RiskLow, Approval: contracts.ApprovalAuto, Executor: "go:automation.task.list",
		Input: contracts.Object(map[string]*contracts.Schema{
			"keyword":  contracts.Str(0, 200),
			"page":     contracts.Int(1, 1000000),
			"pageSize": contracts.Int(1, 100),
		}), Output: contracts.Object(map[string]*contracts.Schema{
			"items":    {Type: "array", Items: taskOutput()},
			"total":    {Type: "integer"},
			"page":     {Type: "integer"},
			"pageSize": {Type: "integer"},
		}, "items"),
		TransientResult: true, Tags: []string{"manual-only"}, Loggable: []string{},
	}
	return d
}

func (m *Module) getDefinition() contracts.Definition {
	d := contracts.Definition{
		Name: "automation.task.get", Version: version, Title: "Get an automation task",
		Description: "Read one of the caller's own automation tasks.", Domain: "automation", Entity: "automation_task",
		Layer: contracts.LayerAtomic, Permission: ReadPermission, ReadOnly: true,
		Risk: contracts.RiskLow, Approval: contracts.ApprovalAuto, Executor: "go:automation.task.get",
		Input:  contracts.Object(map[string]*contracts.Schema{"id": contracts.Int(1, 1<<53)}, "id"),
		Output: taskOutput(),
		Tags:   []string{"manual-only"}, Loggable: []string{"id"},
	}
	return d
}

func (m *Module) runNowDefinition() contracts.Definition {
	return contracts.Definition{
		Name: "automation.run.now", Version: version, Title: "Run an automation task now",
		Description: "Queue one immediate run of the caller's own automation task. The worker still resolves the caller's current authority.",
		Domain:      "automation", Entity: "automation_run",
		Layer: contracts.LayerAtomic, Permission: WritePermission,
		Risk: contracts.RiskMedium, Approval: contracts.ApprovalAuto,
		Input:         contracts.Object(map[string]*contracts.Schema{"id": contracts.Int(1, 1<<53)}, "id"),
		Output:        contracts.Object(map[string]*contracts.Schema{"id": {Type: "integer"}}, "id"),
		Idempotency:   contracts.Idempotency{Required: true, Scope: "tenant+task+key"},
		SuccessEvents: []string{events["run"]}, Executor: "go:automation.run.now",
		Tags: []string{"manual-only"}, Loggable: []string{"id"},
	}
}

func (m *Module) runListDefinition() contracts.Definition {
	d := contracts.Definition{
		Name: "automation.run.list", Version: version, Title: "List automation runs",
		Description: "List runs of the caller's own automation tasks.", Domain: "automation", Entity: "automation_run",
		Layer: contracts.LayerAtomic, Permission: ReadPermission, ReadOnly: true,
		Risk: contracts.RiskLow, Approval: contracts.ApprovalAuto, Executor: "go:automation.run.list",
		Input: contracts.Object(map[string]*contracts.Schema{
			"taskId":   contracts.Int(0, 1<<53),
			"page":     contracts.Int(1, 1000000),
			"pageSize": contracts.Int(1, 100),
		}), Output: contracts.Object(map[string]*contracts.Schema{
			"items":    {Type: "array", Items: runObject()},
			"total":    {Type: "integer"},
			"page":     {Type: "integer"},
			"pageSize": {Type: "integer"},
		}, "items"),
		TransientResult: true, Tags: []string{"manual-only"}, Loggable: []string{"taskId"},
	}
	return d
}

func (m *Module) runGetDefinition() contracts.Definition {
	return contracts.Definition{
		Name: "automation.run.get", Version: version, Title: "Get an automation run",
		Description: "Read one run of the caller's own automation task, including the stored agent answer.",
		Domain:      "automation", Entity: "automation_run",
		Layer: contracts.LayerAtomic, Permission: ReadPermission, ReadOnly: true,
		Risk: contracts.RiskLow, Approval: contracts.ApprovalAuto, Executor: "go:automation.run.get",
		Input:  contracts.Object(map[string]*contracts.Schema{"id": contracts.Int(1, 1<<53)}, "id"),
		Output: runObject(), Tags: []string{"manual-only"}, Loggable: []string{"id"},
	}
}

// taskWriteDefinition builds create and update together so their schemas cannot
// drift. The description is the agent instruction, so it shares the 4000
// character bound of the AI run question: a saved task never needs truncation.
func taskWriteDefinition(action, title string) contracts.Definition {
	description := contracts.Str(1, maxDescriptionLength)
	description.Description = "Markdown instruction the workspace agent executes on every occurrence. The AI form assistant can draft a structured brief from a short description."
	schedule := contracts.Enum(ScheduleKinds...)
	schedule.Description = "once runs at runAt; hourly, daily, weekly and monthly reuse its local wall clock (minute, hour or day) in timezone."
	runAt := contracts.Str(0, 40)
	runAt.Description = "Required for once: RFC3339 instant, or local YYYY-MM-DDTHH:MM interpreted in timezone. The instant must be in the future."
	timeOfDay := contracts.Str(0, 5)
	timeOfDay.Description = "Required for daily, weekly and monthly: local wall-clock time HH:MM."
	minute := contracts.Enum(numberRange(0, 59)...)
	minute.Description = "Required for hourly: local minute as a string, 0-59."
	second := contracts.Enum(numberRange(0, 59)...)
	second.Description = "Optional for hourly: local second as a string, 0-59 (default 0)."
	weekday := contracts.Enum("1", "2", "3", "4", "5", "6", "7")
	weekday.Description = "Required for weekly: ISO weekday as a string, 1 = Monday and 7 = Sunday."
	dayOfMonth := contracts.Enum(numberRange(1, 31)...)
	dayOfMonth.Description = "Required for monthly: day of month as a string, 1-31. Shorter months clamp to their last day without changing the stored day."
	zone := contracts.Enum(Timezones...)
	zone.Description = "IANA time zone used for the schedule; defaults to Asia/Shanghai."
	input := map[string]*contracts.Schema{
		"name":         contracts.Str(1, maxNameLength),
		"description":  description,
		"scheduleKind": schedule,
		"runAt":        runAt,
		"time":         timeOfDay,
		"minute":       minute,
		"second":       second,
		"weekday":      weekday,
		"dayOfMonth":   dayOfMonth,
		"timezone":     zone,
	}
	required := []string{"name", "description", "scheduleKind"}
	if action == "update" {
		input["id"] = contracts.Int(1, 1<<53)
		required = append(required, "id")
	}
	d := contracts.Definition{
		Name: "automation.task." + action, Version: version, Title: title,
		Description: "Create or change an automation task. The description is the agent instruction used on every occurrence.",
		Domain:      "automation", Entity: "automation_task",
		Layer: contracts.LayerAtomic, Permission: WritePermission,
		Risk: contracts.RiskMedium, Approval: contracts.ApprovalAuto,
		Input:         contracts.Object(input, required...),
		Output:        contracts.Object(map[string]*contracts.Schema{"id": {Type: "integer"}}, "id"),
		Idempotency:   contracts.Idempotency{Required: true, Scope: "tenant+operation+key"},
		SuccessEvents: []string{events[action]}, Executor: "go:automation." + action,
		Loggable: []string{"id", "name", "scheduleKind"},
	}
	if action == "create" {
		// Agent conversations may create an automation, so this one capability is
		// deliberately not manual-only. The handler rejects creation from a run
		// launched by automation itself, which keeps the loop bounded.
		d.Description = "Create an automation task that makes the workspace agent execute a described job on a schedule. The task then runs under the caller's current authority; an agent created task may not create further automations when it runs."
		return d
	}
	d.Tags = []string{"manual-only"}
	return d
}

func idWriteDefinition(action, title string) contracts.Definition {
	return contracts.Definition{
		Name: "automation.task." + action, Version: version, Title: title,
		Description: "Enable, disable or delete one of the caller's own automation tasks.",
		Domain:      "automation", Entity: "automation_task",
		Layer: contracts.LayerAtomic, Permission: WritePermission,
		Risk: contracts.RiskMedium, Approval: contracts.ApprovalAuto,
		Input:         contracts.Object(map[string]*contracts.Schema{"id": contracts.Int(1, 1<<53)}, "id"),
		Output:        contracts.Object(map[string]*contracts.Schema{"id": {Type: "integer"}}, "id"),
		Idempotency:   contracts.Idempotency{Required: true, Scope: "tenant+operation+key"},
		SuccessEvents: []string{events[action]}, Executor: "go:automation." + action,
		Tags: []string{"manual-only"}, Loggable: []string{"id"},
	}
}

func taskOutput() *contracts.Schema {
	return contracts.Object(map[string]*contracts.Schema{
		"id":           {Type: "integer"},
		"name":         contracts.Str(1, maxNameLength),
		"description":  contracts.Str(0, maxDescriptionLength),
		"scheduleKind": contracts.Enum(ScheduleKinds...),
		"runAt":        contracts.Str(0, 40),
		"time":         contracts.Str(0, 5),
		"minute":       contracts.Enum(numberRange(0, 59)...),
		"second":       contracts.Enum(numberRange(0, 59)...),
		"weekday":      contracts.Enum("1", "2", "3", "4", "5", "6", "7"),
		"dayOfMonth":   contracts.Enum(numberRange(1, 31)...),
		"timezone":     contracts.Str(1, 100),
		"enabled":      {Type: "boolean"},
		"nextRunAt":    {Type: "string"},
		"lastRunAt":    {Type: "string"},
		"failureCount": {Type: "integer"},
		"createdAt":    {Type: "string"},
		"updatedAt":    {Type: "string"},
	}, "id", "name", "description", "scheduleKind", "runAt", "time", "minute", "second", "weekday", "dayOfMonth", "timezone", "enabled", "nextRunAt", "lastRunAt", "failureCount", "createdAt", "updatedAt")
}

func runObject() *contracts.Schema {
	return contracts.Object(map[string]*contracts.Schema{
		"id":           {Type: "integer"},
		"taskId":       {Type: "integer"},
		"trigger":      contracts.Enum("schedule", "manual"),
		"status":       contracts.Enum("queued", "running", "awaiting_approval", "completed", "failed", "skipped", "cancelled"),
		"scheduledFor": {Type: "string"},
		"errorCode":    {Type: "string"},
		"aiRunId":      {Type: "integer"},
		"startedAt":    {Type: "string"},
		"finishedAt":   {Type: "string"},
		"createdAt":    {Type: "string"},
		"answer":       contracts.Str(0, 4000),
		"toolCalls":    contracts.Int(0, 1<<31),
		"aiStatus":     {Type: "string"},
	}, "id", "taskId", "trigger", "status", "scheduledFor", "errorCode", "aiRunId", "startedAt", "finishedAt", "createdAt", "answer", "toolCalls", "aiStatus")
}

// ---- handlers ----

func principal(ctx context.Context) (*identity.Principal, error) { return identity.Current(ctx) }

func integer(in contracts.Input, key string) int64 {
	switch value := in[key].(type) {
	case float64:
		return int64(value)
	case int64:
		return value
	case int:
		return int64(value)
	}
	return 0
}

func text(in contracts.Input, key string) string {
	value, _ := in[key].(string)
	return strings.TrimSpace(value)
}

func timeValue(value pgtype.Timestamptz) string {
	if !value.Valid {
		return ""
	}
	return value.Time.UTC().Format(time.RFC3339)
}

func location(name string) (*time.Location, error) {
	if name == "" {
		name = DefaultTimezone
	}
	loc, err := time.LoadLocation(name)
	if err != nil {
		return nil, contracts.Problem("invalid")
	}
	return loc, nil
}

func (m *Module) list(ctx context.Context, in contracts.Input) (contracts.Output, error) {
	p, err := principal(ctx)
	if err != nil {
		return contracts.Output{}, err
	}
	page, size := integer(in, "page"), integer(in, "pageSize")
	if page <= 0 {
		page = 1
	}
	if size <= 0 {
		size = 20
	}
	q := dbgen.New(m.Pool)
	rows, err := q.AutomationTaskList(ctx, dbgen.AutomationTaskListParams{
		TenantID: p.TenantID, OwnerID: p.Account.ID, Keyword: text(in, "keyword"),
		PageLimit: int32(size), PageOffset: int32((page - 1) * size),
	})
	if err != nil {
		return contracts.Output{}, err
	}
	total, err := q.AutomationTaskCount(ctx, dbgen.AutomationTaskCountParams{TenantID: p.TenantID, OwnerID: p.Account.ID, Keyword: text(in, "keyword")})
	if err != nil {
		return contracts.Output{}, err
	}
	items := []map[string]any{}
	for _, row := range rows {
		items = append(items, taskView(row.ID, row.Name, row.Description, row.ScheduleKind, row.RunAt, row.Timezone, row.ScheduleDay, row.Enabled, row.NextRunAt, row.LastRunAt, row.FailureCount, row.CreatedAt, row.UpdatedAt))
	}
	return contracts.Output{Data: map[string]any{"items": items, "total": total, "page": page, "pageSize": size}}, nil
}

func (m *Module) get(ctx context.Context, in contracts.Input) (contracts.Output, error) {
	p, err := principal(ctx)
	if err != nil {
		return contracts.Output{}, err
	}
	row, err := dbgen.New(m.Pool).AutomationTaskGet(ctx, dbgen.AutomationTaskGetParams{TenantID: p.TenantID, OwnerID: p.Account.ID, ID: integer(in, "id")})
	if err != nil {
		return contracts.Output{}, contracts.Problem("not_found")
	}
	return contracts.Output{Data: taskView(row.ID, row.Name, row.Description, row.ScheduleKind, row.RunAt, row.Timezone, row.ScheduleDay, row.Enabled, row.NextRunAt, row.LastRunAt, row.FailureCount, row.CreatedAt, row.UpdatedAt)}, nil
}

func (m *Module) create(ctx context.Context, tx pgx.Tx, in contracts.Input) (contracts.Output, error) {
	p, err := principal(ctx)
	if err != nil {
		return contracts.Output{}, err
	}
	meta, _ := gateway.CurrentInvocation(ctx)
	if meta.Origin == contracts.OriginAgent && strings.HasPrefix(meta.CorrelationID, "automation:") {
		// An automation-launched agent may not create more automations; that is
		// how an unattended chain would otherwise grow without a human in it.
		return contracts.Output{}, contracts.Problem("forbidden")
	}
	name, instruction := text(in, "name"), text(in, "description")
	kind := text(in, "scheduleKind")
	loc, err := location(text(in, "timezone"))
	if err != nil {
		return contracts.Output{}, err
	}
	spec, err := scheduleSpec(in, loc)
	if err != nil {
		return contracts.Output{}, err
	}
	anchor, monthlyDay, err := BuildAnchor(spec, time.Now().UTC())
	if err != nil {
		return contracts.Output{}, err
	}
	q := dbgen.New(tx)
	count, err := q.AutomationTaskCount(ctx, dbgen.AutomationTaskCountParams{TenantID: p.TenantID, OwnerID: p.Account.ID})
	if err != nil {
		return contracts.Output{}, err
	}
	if count >= maxTasksPerOwner {
		return contracts.Output{}, contracts.Problem("conflict")
	}
	row, err := q.AutomationTaskCreate(ctx, dbgen.AutomationTaskCreateParams{
		TenantID: p.TenantID, OwnerID: p.Account.ID, DepartmentID: p.Account.DepartmentID,
		Name: name, Description: instruction, ScheduleKind: kind, RunAt: pgtype.Timestamptz{Time: anchor, Valid: true},
		Timezone: loc.String(), NextRunAt: pgtype.Timestamptz{Time: anchor, Valid: true}, ScheduleDay: scheduleDayValue(monthlyDay),
	})
	if err != nil {
		return contracts.Output{}, err
	}
	return contracts.Output{Data: map[string]any{"id": row}, Summary: fmt.Sprintf("automation=%d", row), Events: []contracts.Event{{
		Type: events["create"], AggregateType: "automation_task", AggregateID: fmt.Sprint(row),
		Payload: map[string]any{"id": row, "scheduleKind": kind},
	}}}, nil
}

func (m *Module) update(ctx context.Context, tx pgx.Tx, in contracts.Input) (contracts.Output, error) {
	p, err := principal(ctx)
	if err != nil {
		return contracts.Output{}, err
	}
	q := dbgen.New(tx)
	id := integer(in, "id")
	current, err := q.AutomationTaskGet(ctx, dbgen.AutomationTaskGetParams{TenantID: p.TenantID, OwnerID: p.Account.ID, ID: id})
	if err != nil {
		return contracts.Output{}, contracts.Problem("not_found")
	}
	name, instruction := text(in, "name"), text(in, "description")
	kind := text(in, "scheduleKind")
	loc, err := location(text(in, "timezone"))
	if err != nil {
		return contracts.Output{}, err
	}
	spec, err := scheduleSpec(in, loc)
	if err != nil {
		return contracts.Output{}, err
	}
	anchor, monthlyDay, err := BuildAnchor(spec, time.Now().UTC())
	if err != nil {
		return contracts.Output{}, err
	}
	nextRunAt := pgtype.Timestamptz{}
	if current.Enabled {
		nextRunAt = pgtype.Timestamptz{Time: anchor, Valid: true}
	}
	affected, err := q.AutomationTaskUpdate(ctx, dbgen.AutomationTaskUpdateParams{
		TenantID: p.TenantID, OwnerID: p.Account.ID, ID: id,
		Name: name, Description: instruction, ScheduleKind: kind,
		RunAt: pgtype.Timestamptz{Time: anchor, Valid: true}, Timezone: loc.String(), NextRunAt: nextRunAt,
		ScheduleDay: scheduleDayValue(monthlyDay),
	})
	if err != nil {
		return contracts.Output{}, err
	}
	if affected != 1 {
		return contracts.Output{}, contracts.Problem("conflict")
	}
	return contracts.Output{Data: map[string]any{"id": id}, Summary: fmt.Sprintf("automation=%d", id), Events: []contracts.Event{{
		Type: events["update"], AggregateType: "automation_task", AggregateID: fmt.Sprint(id),
		Payload: map[string]any{"id": id, "scheduleKind": kind},
	}}}, nil
}

// setEnabled serves both enable and disable. The action is derived from the
// invocation name rather than a client supplied flag, so a caller cannot use
// one contract to perform the other action.
func (m *Module) setEnabled(ctx context.Context, tx pgx.Tx, in contracts.Input) (contracts.Output, error) {
	p, err := principal(ctx)
	if err != nil {
		return contracts.Output{}, err
	}
	meta, _ := gateway.CurrentInvocation(ctx)
	action := "enable"
	if strings.HasSuffix(meta.Capability, ".disable") {
		action = "disable"
	}
	q := dbgen.New(tx)
	id := integer(in, "id")
	current, err := q.AutomationTaskGet(ctx, dbgen.AutomationTaskGetParams{TenantID: p.TenantID, OwnerID: p.Account.ID, ID: id})
	if err != nil {
		return contracts.Output{}, contracts.Problem("not_found")
	}
	enabled := action == "enable"
	nextRunAt := pgtype.Timestamptz{}
	if enabled {
		loc, err := time.LoadLocation(current.Timezone)
		if err != nil {
			return contracts.Output{}, contracts.Problem("invalid")
		}
		next, ok := NextOccurrence(current.ScheduleKind, current.RunAt.Time, loc, time.Now().UTC(), scheduleDayInt(current.ScheduleDay))
		if !ok {
			return contracts.Output{}, contracts.Problem("conflict")
		}
		nextRunAt = pgtype.Timestamptz{Time: next, Valid: true}
	}
	affected, err := q.AutomationTaskSetEnabled(ctx, dbgen.AutomationTaskSetEnabledParams{
		TenantID: p.TenantID, OwnerID: p.Account.ID, ID: id, Enabled: enabled, NextRunAt: nextRunAt,
	})
	if err != nil {
		return contracts.Output{}, err
	}
	if affected != 1 {
		return contracts.Output{}, contracts.Problem("conflict")
	}
	return contracts.Output{Data: map[string]any{"id": id}, Summary: fmt.Sprintf("automation=%d enabled=%v", id, enabled), Events: []contracts.Event{{
		Type: events[action], AggregateType: "automation_task", AggregateID: fmt.Sprint(id),
		Payload: map[string]any{"id": id},
	}}}, nil
}

func (m *Module) remove(ctx context.Context, tx pgx.Tx, in contracts.Input) (contracts.Output, error) {
	p, err := principal(ctx)
	if err != nil {
		return contracts.Output{}, err
	}
	id := integer(in, "id")
	affected, err := dbgen.New(tx).AutomationTaskDelete(ctx, dbgen.AutomationTaskDeleteParams{TenantID: p.TenantID, OwnerID: p.Account.ID, ID: id})
	if err != nil {
		return contracts.Output{}, err
	}
	if affected != 1 {
		return contracts.Output{}, contracts.Problem("not_found")
	}
	return contracts.Output{Data: map[string]any{"id": id}, Summary: fmt.Sprintf("automation=%d", id), Events: []contracts.Event{{
		Type: events["delete"], AggregateType: "automation_task", AggregateID: fmt.Sprint(id),
		Payload: map[string]any{"id": id},
	}}}, nil
}

func (m *Module) runNow(ctx context.Context, tx pgx.Tx, in contracts.Input) (contracts.Output, error) {
	p, err := principal(ctx)
	if err != nil {
		return contracts.Output{}, err
	}
	id := integer(in, "id")
	q := dbgen.New(tx)
	if _, err = q.AutomationTaskGet(ctx, dbgen.AutomationTaskGetParams{TenantID: p.TenantID, OwnerID: p.Account.ID, ID: id}); err != nil {
		return contracts.Output{}, contracts.Problem("not_found")
	}
	meta, _ := gateway.CurrentInvocation(ctx)
	key := meta.IdempotencyKey
	if key == "" {
		key = meta.InvocationID
	}
	occurrence := "manual:" + key
	now := time.Now().UTC()
	runID, err := q.AutomationRunQueue(ctx, dbgen.AutomationRunQueueParams{
		TenantID: p.TenantID, TaskID: id, OccurrenceKey: occurrence,
		ScheduledFor: pgtype.Timestamptz{Time: now, Valid: true}, TriggerKind: "manual", Status: "queued", ErrorCode: "",
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			existing, lookupErr := q.AutomationRunByKey(ctx, dbgen.AutomationRunByKeyParams{TenantID: p.TenantID, TaskID: id, OccurrenceKey: occurrence})
			if lookupErr != nil {
				return contracts.Output{}, lookupErr
			}
			return contracts.Output{Data: map[string]any{"id": existing}, Summary: fmt.Sprintf("run=%d", existing)}, nil
		}
		return contracts.Output{}, err
	}
	return contracts.Output{Data: map[string]any{"id": runID}, Summary: fmt.Sprintf("run=%d", runID), Events: []contracts.Event{{
		Type: events["run"], AggregateType: "automation_run", AggregateID: fmt.Sprint(runID),
		Payload: map[string]any{"id": runID, "taskId": id},
	}}}, nil
}

func (m *Module) runList(ctx context.Context, in contracts.Input) (contracts.Output, error) {
	p, err := principal(ctx)
	if err != nil {
		return contracts.Output{}, err
	}
	page, size := integer(in, "page"), integer(in, "pageSize")
	if page <= 0 {
		page = 1
	}
	if size <= 0 {
		size = 20
	}
	taskID := integer(in, "taskId")
	q := dbgen.New(m.Pool)
	rows, err := q.AutomationRunList(ctx, dbgen.AutomationRunListParams{
		TenantID: p.TenantID, OwnerID: p.Account.ID, TaskID: taskID,
		PageLimit: int32(size), PageOffset: int32((page - 1) * size),
	})
	if err != nil {
		return contracts.Output{}, err
	}
	total, err := q.AutomationRunCount(ctx, dbgen.AutomationRunCountParams{TenantID: p.TenantID, OwnerID: p.Account.ID, TaskID: taskID})
	if err != nil {
		return contracts.Output{}, err
	}
	items := []map[string]any{}
	for _, row := range rows {
		run := runView(row.ID, row.TaskID, row.TriggerKind, row.Status, row.ErrorCode, row.AiRunID, row.ScheduledFor, row.CreatedAt, row.StartedAt, row.FinishedAt)
		run["answer"], run["toolCalls"], run["aiStatus"] = "", int32(0), ""
		items = append(items, run)
	}
	return contracts.Output{Data: map[string]any{"items": items, "total": total, "page": page, "pageSize": size}}, nil
}

func (m *Module) runGet(ctx context.Context, in contracts.Input) (contracts.Output, error) {
	p, err := principal(ctx)
	if err != nil {
		return contracts.Output{}, err
	}
	row, err := dbgen.New(m.Pool).AutomationRunGet(ctx, dbgen.AutomationRunGetParams{TenantID: p.TenantID, OwnerID: p.Account.ID, ID: integer(in, "id")})
	if err != nil {
		return contracts.Output{}, contracts.Problem("not_found")
	}
	run := runView(row.ID, row.TaskID, row.TriggerKind, row.Status, row.ErrorCode, row.AiRunID, row.ScheduledFor, row.CreatedAt, row.StartedAt, row.FinishedAt)
	run["answer"] = row.Answer
	run["toolCalls"] = row.ToolCalls
	run["aiStatus"] = row.AiStatus
	return contracts.Output{Data: run}, nil
}

// scheduleSpec reads only the components the picked kind needs. The form uses
// native time and select controls; every discrete value arrives as a string.
func scheduleSpec(in contracts.Input, loc *time.Location) (ScheduleSpec, error) {
	kind := text(in, "scheduleKind")
	spec := ScheduleSpec{Kind: kind, Timezone: loc, RunAt: text(in, "runAt")}
	switch kind {
	case ScheduleHourly:
		minute, err := enumInt(in, "minute", 0, 59)
		if err != nil {
			return spec, err
		}
		spec.Minute = minute
		if raw, ok := in["second"].(string); ok && raw != "" {
			second, err := intInRange(raw, 0, 59)
			if err != nil {
				return spec, err
			}
			spec.Second = &second
		}
	case ScheduleDaily, ScheduleWeekly, ScheduleMonthly:
		hour, minute, err := parseClock(text(in, "time"))
		if err != nil {
			return spec, err
		}
		spec.Hour, spec.Minute = &hour, &minute
		if kind == ScheduleWeekly {
			weekday, err := enumInt(in, "weekday", 1, 7)
			if err != nil {
				return spec, err
			}
			spec.Weekday = weekday
		}
		if kind == ScheduleMonthly {
			day, err := enumInt(in, "dayOfMonth", 1, 31)
			if err != nil {
				return spec, err
			}
			spec.DayOfMonth = day
		}
	}
	return spec, nil
}

func parseClock(value string) (int, int, error) {
	parts := strings.Split(value, ":")
	if len(parts) != 2 {
		return 0, 0, contracts.Problem("invalid")
	}
	hour, err := intInRange(parts[0], 0, 23)
	if err != nil {
		return 0, 0, err
	}
	minute, err := intInRange(parts[1], 0, 59)
	if err != nil {
		return 0, 0, err
	}
	return hour, minute, nil
}

func enumInt(in contracts.Input, key string, low, high int) (*int, error) {
	value, ok := in[key].(string)
	if !ok || value == "" {
		return nil, contracts.Problem("invalid")
	}
	number, err := intInRange(value, low, high)
	if err != nil {
		return nil, err
	}
	return &number, nil
}

func intInRange(value string, low, high int) (int, error) {
	number, err := strconv.Atoi(value)
	if err != nil || number < low || number > high {
		return 0, contracts.Problem("invalid")
	}
	return number, nil
}

// numberRange lists selectable values without manual numeric entry.
func numberRange(low, high int) []string {
	out := make([]string, 0, high-low+1)
	for value := low; value <= high; value++ {
		out = append(out, strconv.Itoa(value))
	}
	return out
}

func scheduleDayValue(day int) pgtype.Int4 {
	if day <= 0 {
		return pgtype.Int4{}
	}
	return pgtype.Int4{Int32: int32(day), Valid: true}
}

func scheduleDayInt(day pgtype.Int4) int {
	if !day.Valid {
		return 0
	}
	return int(day.Int32)
}

// taskView projects the wall-clock components the edit form needs. Monthly
// tasks report the stored day, not the clamped anchor day.
func taskView(id int64, name, description, kind string, runAt pgtype.Timestamptz, timezone string, day pgtype.Int4, enabled bool, nextRunAt, lastRunAt pgtype.Timestamptz, failures int32, createdAt, updatedAt pgtype.Timestamptz) map[string]any {
	loc, _ := time.LoadLocation(timezone)
	if loc == nil {
		loc = time.UTC
	}
	local := runAt.Time.In(loc)
	dayOfMonth := scheduleDayInt(day)
	if dayOfMonth <= 0 {
		dayOfMonth = local.Day()
	}
	return map[string]any{
		"id": id, "name": name, "description": description,
		"scheduleKind": kind, "runAt": FormatRunAt(runAt.Time, loc), "timezone": timezone,
		"time":   fmt.Sprintf("%02d:%02d", local.Hour(), local.Minute()),
		"minute": strconv.Itoa(local.Minute()), "second": strconv.Itoa(local.Second()),
		"weekday": strconv.Itoa(isoWeekday(local)), "dayOfMonth": strconv.Itoa(dayOfMonth),
		"enabled": enabled, "nextRunAt": timeValue(nextRunAt), "lastRunAt": timeValue(lastRunAt),
		"failureCount": failures, "createdAt": timeValue(createdAt), "updatedAt": timeValue(updatedAt),
	}
}

func runView(id, taskID int64, trigger, status, errorCode string, aiRunID pgtype.Int8, scheduledFor, createdAt, startedAt, finishedAt pgtype.Timestamptz) map[string]any {
	runID := int64(0)
	if aiRunID.Valid {
		runID = aiRunID.Int64
	}
	return map[string]any{
		"id": id, "taskId": taskID, "trigger": trigger, "status": status,
		"errorCode": errorCode, "aiRunId": runID,
		"scheduledFor": timeValue(scheduledFor), "createdAt": timeValue(createdAt),
		"startedAt": timeValue(startedAt), "finishedAt": timeValue(finishedAt),
	}
}
