package memory

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"time"

	"baseline/services/api/internal/repository/dbgen"
	"baseline/services/api/kernel/capability/contracts"
	"baseline/services/api/kernel/capability/gateway"
	"baseline/services/api/system/identity"
	"github.com/jackc/pgx/v5"
)

func projectDefinitions() []contracts.Definition {
	partition := &contracts.Schema{Type: "string", Enum: []string{"agent", "activity", "exception", "business", "knowledge", "execution"}}
	projectID := contracts.Int(1, 1<<53)
	createInput := contracts.Object(map[string]*contracts.Schema{"projectId": projectID, "partition": partition, "memoryType": contracts.Str(1, 100), "title": contracts.Str(1, 200), "summary": contracts.Str(0, 4000), "sourceEventId": contracts.Str(1, 256)}, "projectId", "partition", "memoryType", "title", "summary")
	addValiditySchema(createInput)
	listInput := contracts.Object(map[string]*contracts.Schema{"projectId": projectID, "partition": partition, "beforeId": contracts.Int(0, 1<<53), "limit": contracts.Int(1, 100)}, "projectId")
	addListFilterSchema(listInput)
	getInput := contracts.Object(map[string]*contracts.Schema{"projectId": projectID, "id": contracts.Int(1, 1<<53)}, "projectId", "id")
	changeInput := contracts.Object(map[string]*contracts.Schema{"projectId": projectID, "id": contracts.Int(1, 1<<53), "version": contracts.Int(1, 1<<53)}, "projectId", "id", "version")
	correctInput := contracts.Object(map[string]*contracts.Schema{"projectId": projectID, "id": contracts.Int(1, 1<<53), "version": contracts.Int(1, 1<<53), "title": contracts.Str(1, 200), "summary": contracts.Str(0, 4000)}, "projectId", "id", "version", "title", "summary")
	listOutput := contracts.Object(map[string]*contracts.Schema{"items": {Type: "array", Items: itemSchema()}}, "items")
	source := contracts.Object(map[string]*contracts.Schema{"eventId": contracts.Str(1, 256), "eventType": contracts.Str(1, 256), "occurredAt": contracts.Str(1, 64), "correlationId": {Type: "string"}}, "eventId", "eventType", "occurredAt", "correlationId")
	sourcesOutput := contracts.Object(map[string]*contracts.Schema{"sources": {Type: "array", Items: source}}, "sources")
	outer := func(name, permission string, input, output *contracts.Schema, read bool) contracts.Definition {
		d := contracts.Definition{Layer: contracts.LayerOrchestration, Name: name, Version: "1.0.0", Title: name, Description: "Authorize the project and manage its memory through registered capabilities.", Domain: "memory", Entity: "memory_item", Permission: permission, Risk: contracts.RiskLow, Approval: contracts.ApprovalAuto, ReadOnly: read, TransientResult: read, Tags: []string{"manual-only"}, Executor: "go:" + name, Input: input, Output: output, Loggable: []string{"projectId", "beforeId", "limit"}}
		if !read {
			d.Risk = contracts.RiskMedium
			d.Idempotency = contracts.Idempotency{Required: true, Scope: "tenant+operation+key"}
			d.SuccessEvents = []string{name + ".completed"}
			d.Loggable = []string{"projectId"}
		}
		return d
	}
	data := func(name, parent, permission string, input, output *contracts.Schema, read bool) contracts.Definition {
		d := contracts.Definition{Layer: contracts.LayerAtomic, Name: name, Version: "1.0.0", Title: name, Description: "Internal tenant and project scoped memory operation authorized by its parent.", Domain: "memory", Entity: "memory_item", Permission: permission, Risk: contracts.RiskLow, Approval: contracts.ApprovalAuto, ReadOnly: read, TransientResult: read, Tags: []string{"internal-only", "parent:" + parent + "@1.0.0"}, Executor: "go:" + name, Input: input, Output: output, Loggable: []string{"projectId", "beforeId", "limit"}}
		if read {
			d.Entities = []string{"memory_item", "memory_access_audit"}
		} else {
			d.Risk = contracts.RiskMedium
			d.Idempotency = contracts.Idempotency{Required: true, Scope: "tenant+operation+key"}
			d.SuccessEvents = []string{name + ".completed"}
			d.Loggable = []string{"projectId"}
		}
		return d
	}
	type operation struct {
		name          string
		input, output *contracts.Schema
		read          bool
	}
	ops := []operation{{"create", createInput, mutationSchema(), false}, {"list", listInput, listOutput, true}, {"get", getInput, itemSchema(), true}, {"sources", getInput, sourcesOutput, true}, {"correct", correctInput, mutationSchema(), false}, {"archive", changeInput, mutationSchema(), false}, {"delete", changeInput, mutationSchema(), false}}
	result := make([]contracts.Definition, 0, len(ops)*2)
	for _, op := range ops {
		parent := "memory.project." + op.name
		permission := "memory:write"
		if op.read {
			permission = "memory:read"
		}
		a := outer(parent, permission, op.input, op.output, op.read)
		b := data("memory.project.data."+op.name, parent, permission, op.input, op.output, op.read)
		switch op.name {
		case "create":
			a.Entities = []string{"memory_item", "memory_source"}
			b.Entities = []string{"memory_item", "memory_source"}
		case "correct":
			a.Entities = []string{"memory_item", "memory_correction"}
			b.Entities = []string{"memory_item", "memory_correction"}
		case "delete":
			a.Entities = []string{"memory_item", "memory_source", "memory_relation", "memory_correction"}
			b.Entities = []string{"memory_item", "memory_source", "memory_relation", "memory_correction"}
		}
		result = append(result, a, b)
	}
	return result
}

// projectFlow composes existing project authorization with memory operations.
// It never accesses business storage directly; Gateway preserves principal,
// correlation, depth and a stable child idempotency key.
func (m *Module) projectFlow(ctx context.Context, name string, in contracts.Input) (contracts.Output, error) {
	if !manualInvocation(ctx) {
		return contracts.Output{}, contracts.Problem("forbidden")
	}
	if m.gateway == nil {
		return contracts.Output{}, contracts.Problem("unavailable")
	}
	p, err := identity.Current(ctx)
	if err != nil {
		return contracts.Output{}, contracts.Problem("unauthenticated")
	}
	read := name == "memory.project.list" || name == "memory.project.get" || name == "memory.project.sources"
	if !read && !p.Can("projects:write") {
		return contracts.Output{}, contracts.Problem("forbidden")
	}
	projectID := valueInt(in["projectId"])
	if err := m.authorizeProject(ctx, projectID); err != nil {
		return contracts.Output{}, err
	}
	child := strings.Replace(name, "memory.project.", "memory.project.data.", 1)
	result, err := m.gateway.Invoke(ctx, gateway.Request{Name: child, Version: "1.0.0", Input: in, Origin: contracts.OriginCapability, IdempotencyKey: gateway.ChildKey(ctx, "project-memory")})
	if err != nil {
		return contracts.Output{}, err
	}
	if result.Status != contracts.StatusSucceeded {
		if result.ErrorCode == "" {
			return contracts.Output{}, contracts.Problem("unavailable")
		}
		return contracts.Output{}, contracts.Problem(result.ErrorCode)
	}
	if read {
		// A membership revoked while the data was fetched must not be returned.
		if err := m.authorizeProject(ctx, projectID); err != nil {
			return contracts.Output{}, err
		}
		return contracts.Output{Data: result.Data}, nil
	}
	return contracts.Output{Data: result.Data, Events: []contracts.Event{{Type: name + ".completed", AggregateType: "project", AggregateID: strconv.FormatInt(projectID, 10), Payload: map[string]any{}}}}, nil
}

func (m *Module) authorizeProject(ctx context.Context, id int64) error {
	result, err := m.gateway.Invoke(ctx, gateway.Request{Name: "project.get", Version: "1.0.0", Input: contracts.Input{"id": id}, Origin: contracts.OriginCapability})
	if err != nil {
		return err
	}
	if result.Status != contracts.StatusSucceeded {
		if result.ErrorCode == "" {
			return contracts.Problem("unavailable")
		}
		return contracts.Problem(result.ErrorCode)
	}
	return nil
}

func (m *Module) readProjectData(ctx context.Context, name string, in contracts.Input) (contracts.Output, error) {
	p, err := identity.Current(ctx)
	if err != nil {
		return contracts.Output{}, contracts.Problem("unauthenticated")
	}
	if !p.Can("memory:read") {
		return contracts.Output{}, contracts.Problem("forbidden")
	}
	if m.pool == nil {
		return contracts.Output{}, contracts.Problem("unavailable")
	}
	q := dbgen.New(m.pool)
	scope := strconv.FormatInt(valueInt(in["projectId"]), 10)
	if name == "memory.project.data.get" || name == "memory.project.data.sources" {
		id := valueInt(in["id"])
		row, err := q.MemoryGetProject(ctx, dbgen.MemoryGetProjectParams{TenantID: p.TenantID, AccountID: p.Account.ID, ScopeID: scope, ID: id})
		if errors.Is(err, pgx.ErrNoRows) {
			return contracts.Output{}, contracts.Problem("not_found")
		}
		if err != nil {
			return contracts.Output{}, err
		}
		if name == "memory.project.data.get" {
			return auditedRead(ctx, q, p, "memory.project.get", id, itemProjection(row.ID, row.Partition, row.MemoryType, row.Title, row.Summary, row.Status, row.VerificationStatus, row.Version, row.Sensitivity, row.ValidFrom, row.ValidTo))
		}
		rows, err := q.MemoryListProjectSources(ctx, dbgen.MemoryListProjectSourcesParams{TenantID: p.TenantID, AccountID: p.Account.ID, ScopeID: scope, MemoryID: id})
		if err != nil {
			return contracts.Output{}, err
		}
		sources := make([]map[string]any, 0, len(rows))
		for _, r := range rows {
			sources = append(sources, map[string]any{"eventId": r.EventID, "eventType": r.EventType, "occurredAt": r.OccurredAt.Time.UTC().Format(time.RFC3339Nano), "correlationId": r.CorrelationID})
		}
		return auditedRead(ctx, q, p, "memory.project.sources", id, map[string]any{"sources": sources})
	}
	if name != "memory.project.data.list" {
		return contracts.Output{}, contracts.Problem("not_found")
	}
	limit := valueInt(in["limit"])
	if limit == 0 {
		limit = 30
	}
	partition, _ := in["partition"].(string)
	status, from, before, err := listFilters(in)
	if err != nil {
		return contracts.Output{}, err
	}
	rows, err := q.MemoryListProject(ctx, dbgen.MemoryListProjectParams{TenantID: p.TenantID, AccountID: p.Account.ID, OwnerFilter: valueInt(in["ownerId"]), ScopeID: scope, PartitionFilter: partition, StatusFilter: status, CreatedFrom: from, CreatedBefore: before, BeforeID: valueInt(in["beforeId"]), MaxRows: int32(limit)})
	if err != nil {
		return contracts.Output{}, err
	}
	items := make([]map[string]any, 0, len(rows))
	for _, r := range rows {
		items = append(items, itemProjection(r.ID, r.Partition, r.MemoryType, r.Title, r.Summary, r.Status, r.VerificationStatus, r.Version, r.Sensitivity, r.ValidFrom, r.ValidTo))
	}
	return auditedRead(ctx, q, p, "memory.project.list", 0, map[string]any{"items": items})
}

func (m *Module) createProjectData(ctx context.Context, tx pgx.Tx, in contracts.Input) (contracts.Output, error) {
	p, err := identity.Current(ctx)
	if err != nil {
		return contracts.Output{}, contracts.Problem("unauthenticated")
	}
	if !p.Can("memory:write") || !p.Can("projects:write") {
		return contracts.Output{}, contracts.Problem("forbidden")
	}
	partition, _ := in["partition"].(string)
	kind, _ := in["memoryType"].(string)
	title, _ := in["title"].(string)
	summary, _ := in["summary"].(string)
	from, to, err := validity(in)
	if err != nil {
		return contracts.Output{}, err
	}
	projectID := valueInt(in["projectId"])
	if err := lockProjectMembership(ctx, dbgen.New(tx), p, projectID); err != nil {
		return contracts.Output{}, err
	}
	row, err := dbgen.New(tx).MemoryCreateProject(ctx, dbgen.MemoryCreateProjectParams{TenantID: p.TenantID, Partition: partition, MemoryType: kind, ScopeID: strconv.FormatInt(projectID, 10), OwnerID: p.Account.ID, Title: title, Summary: summary, ValidFrom: from, ValidTo: to})
	if errors.Is(err, pgx.ErrNoRows) {
		return contracts.Output{}, contracts.Problem("not_found")
	}
	if err != nil {
		return contracts.Output{}, err
	}
	if source, ok := in["sourceEventId"].(string); ok {
		count, err := dbgen.New(tx).MemoryAttachProjectEvent(ctx, dbgen.MemoryAttachProjectEventParams{TenantID: p.TenantID, AccountID: p.Account.ID, ScopeID: strconv.FormatInt(projectID, 10), MemoryID: row.ID, EventID: source})
		if err != nil {
			return contracts.Output{}, err
		}
		if count != 1 {
			return contracts.Output{}, contracts.Problem("not_found")
		}
	}
	return contracts.Output{Data: map[string]any{"id": row.ID, "version": row.Version, "status": "active"}, Events: []contracts.Event{{Type: "memory.project.data.create.completed", AggregateType: "memory_item", AggregateID: strconv.FormatInt(row.ID, 10), Payload: map[string]any{"projectId": projectID, "id": row.ID}}}}, nil
}

func (m *Module) writeProjectData(ctx context.Context, tx pgx.Tx, name string, in contracts.Input) (contracts.Output, error) {
	if name == "memory.project.data.create" {
		return m.createProjectData(ctx, tx, in)
	}
	p, err := identity.Current(ctx)
	if err != nil {
		return contracts.Output{}, contracts.Problem("unauthenticated")
	}
	if !p.Can("memory:write") || !p.Can("projects:write") {
		return contracts.Output{}, contracts.Problem("forbidden")
	}
	q := dbgen.New(tx)
	id, version := valueInt(in["id"]), valueInt(in["version"])
	scope := strconv.FormatInt(valueInt(in["projectId"]), 10)
	if err := lockProjectMembership(ctx, q, p, valueInt(in["projectId"])); err != nil {
		return contracts.Output{}, err
	}
	old, err := q.MemoryLockProject(ctx, dbgen.MemoryLockProjectParams{TenantID: p.TenantID, AccountID: p.Account.ID, ScopeID: scope, ID: id})
	if errors.Is(err, pgx.ErrNoRows) {
		return contracts.Output{}, contracts.Problem("not_found")
	}
	if err != nil {
		return contracts.Output{}, err
	}
	if old.Version != version {
		return contracts.Output{}, contracts.Problem("conflict")
	}
	var affected int64
	status := "active"
	switch name {
	case "memory.project.data.correct":
		if old.Status != "active" {
			return contracts.Output{}, contracts.Problem("conflict")
		}
		count, err := q.MemoryRecordCorrection(ctx, dbgen.MemoryRecordCorrectionParams{TenantID: p.TenantID, ActorID: p.Account.ID, MemoryID: id, PreviousVersion: version})
		if err != nil {
			return contracts.Output{}, err
		}
		if count != 1 {
			return contracts.Output{}, contracts.Problem("conflict")
		}
		title, _ := in["title"].(string)
		summary, _ := in["summary"].(string)
		affected, err = q.MemoryCorrectProject(ctx, dbgen.MemoryCorrectProjectParams{TenantID: p.TenantID, AccountID: p.Account.ID, ScopeID: scope, ID: id, Version: version, Title: title, Summary: summary})
	case "memory.project.data.archive":
		status = "archived"
		affected, err = q.MemoryArchiveProject(ctx, dbgen.MemoryArchiveProjectParams{TenantID: p.TenantID, AccountID: p.Account.ID, ScopeID: scope, ID: id, Version: version})
	case "memory.project.data.delete":
		status = "deleted"
		affected, err = q.MemoryDeleteProject(ctx, dbgen.MemoryDeleteProjectParams{TenantID: p.TenantID, AccountID: p.Account.ID, ScopeID: scope, ID: id, Version: version})
		if err == nil && affected == 1 {
			err = q.MemoryDeleteSources(ctx, dbgen.MemoryDeleteSourcesParams{TenantID: p.TenantID, MemoryID: id, ActorID: p.Account.ID})
		}
		if err == nil && affected == 1 {
			err = q.MemoryDeleteCorrections(ctx, dbgen.MemoryDeleteCorrectionsParams{TenantID: p.TenantID, MemoryID: id, ActorID: p.Account.ID})
		}
		if err == nil && affected == 1 {
			err = q.MemoryDeleteRelations(ctx, dbgen.MemoryDeleteRelationsParams{TenantID: p.TenantID, MemoryID: id, ActorID: p.Account.ID})
		}
	default:
		return contracts.Output{}, contracts.Problem("not_found")
	}
	if err != nil {
		return contracts.Output{}, err
	}
	if affected != 1 {
		return contracts.Output{}, contracts.Problem("conflict")
	}
	version++
	return contracts.Output{Data: map[string]any{"id": id, "version": version, "status": status}, Events: []contracts.Event{{Type: name + ".completed", AggregateType: "memory_item", AggregateID: strconv.FormatInt(id, 10), Payload: map[string]any{"id": id, "version": version, "status": status}}}}, nil
}

func lockProjectMembership(ctx context.Context, q *dbgen.Queries, p *identity.Principal, projectID int64) error {
	_, err := q.MemoryLockProjectMembership(ctx, dbgen.MemoryLockProjectMembershipParams{TenantID: p.TenantID, AccountID: p.Account.ID, ScopeID: strconv.FormatInt(projectID, 10)})
	if errors.Is(err, pgx.ErrNoRows) {
		return contracts.Problem("not_found")
	}
	return err
}
