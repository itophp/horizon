package memory

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"horizon/foundation/kernel/capability/contracts"
	"horizon/foundation/kernel/capability/gateway"
	"horizon/foundation/system/identity"
	"horizon/internal/repository/dbgen"
)

func groupDefinitions() []contracts.Definition {
	scopeType := &contracts.Schema{Type: "string", Enum: []string{"department", "organization"}}
	scopeID := contracts.Int(1, 1<<53)
	partition := &contracts.Schema{Type: "string", Enum: []string{"agent", "activity", "exception", "business", "knowledge", "execution"}}
	create := contracts.Object(map[string]*contracts.Schema{"scopeType": scopeType, "scopeId": scopeID, "partition": partition, "memoryType": contracts.Str(1, 100), "title": contracts.Str(1, 200), "summary": contracts.Str(0, 4000), "sourceEventId": contracts.Str(1, 256)}, "scopeType", "scopeId", "partition", "memoryType", "title", "summary")
	addValiditySchema(create)
	list := contracts.Object(map[string]*contracts.Schema{"scopeType": scopeType, "scopeId": scopeID, "partition": partition, "beforeId": contracts.Int(0, 1<<53), "limit": contracts.Int(1, 100)}, "scopeType", "scopeId")
	addListFilterSchema(list)
	get := contracts.Object(map[string]*contracts.Schema{"scopeType": scopeType, "scopeId": scopeID, "id": contracts.Int(1, 1<<53)}, "scopeType", "scopeId", "id")
	change := contracts.Object(map[string]*contracts.Schema{"scopeType": scopeType, "scopeId": scopeID, "id": contracts.Int(1, 1<<53), "version": contracts.Int(1, 1<<53)}, "scopeType", "scopeId", "id", "version")
	correct := contracts.Object(map[string]*contracts.Schema{"scopeType": scopeType, "scopeId": scopeID, "id": contracts.Int(1, 1<<53), "version": contracts.Int(1, 1<<53), "title": contracts.Str(1, 200), "summary": contracts.Str(0, 4000)}, "scopeType", "scopeId", "id", "version", "title", "summary")
	source := contracts.Object(map[string]*contracts.Schema{"eventId": contracts.Str(1, 256), "eventType": contracts.Str(1, 256), "occurredAt": contracts.Str(1, 64), "correlationId": {Type: "string"}}, "eventId", "eventType", "occurredAt", "correlationId")
	type operation struct {
		name          string
		input, output *contracts.Schema
		read          bool
	}
	ops := []operation{{"create", create, mutationSchema(), false}, {"list", list, contracts.Object(map[string]*contracts.Schema{"items": {Type: "array", Items: itemSchema()}}, "items"), true}, {"get", get, itemSchema(), true}, {"sources", get, contracts.Object(map[string]*contracts.Schema{"sources": {Type: "array", Items: source}}, "sources"), true}, {"correct", correct, mutationSchema(), false}, {"archive", change, mutationSchema(), false}, {"delete", change, mutationSchema(), false}}
	out := make([]contracts.Definition, 0, len(ops))
	for _, op := range ops {
		name := "memory.group." + op.name
		permission := "memory:write"
		if op.read {
			permission = "memory:read"
		}
		d := contracts.Definition{Layer: contracts.LayerAtomic, Name: name, Version: "1.0.0", Title: name, Description: "Manage memory within an IAM-authorized department or organization.", Domain: "memory", Entity: "memory_item", Permission: permission, Risk: contracts.RiskLow, Approval: contracts.ApprovalAuto, ReadOnly: op.read, TransientResult: op.read, Tags: []string{"manual-only"}, Executor: "go:" + name, Input: op.input, Output: op.output, Loggable: []string{"scopeType", "scopeId", "id", "version", "beforeId", "limit"}}
		if op.read {
			d.Entities = []string{"memory_item", "memory_access_audit"}
		} else {
			d.Risk = contracts.RiskMedium
			d.Idempotency = contracts.Idempotency{Required: true, Scope: "tenant+operation+key"}
			d.SuccessEvents = []string{name + ".completed"}
		}
		switch op.name {
		case "create":
			d.Entities = []string{"memory_item", "memory_source"}
		case "correct":
			d.Entities = []string{"memory_item", "memory_correction"}
		case "delete":
			d.Entities = []string{"memory_item", "memory_source", "memory_relation", "memory_correction"}
		}
		out = append(out, d)
	}
	return out
}

type groupScope struct {
	read, write identity.Scope
	kind, id    string
}

func (m *Module) groupScope(ctx context.Context, tx pgx.Tx, p *identity.Principal, in contracts.Input, write bool) (groupScope, error) {
	if m.identity == nil {
		return groupScope{}, contracts.Problem("unavailable")
	}
	kind, _ := in["scopeType"].(string)
	if kind == "business_entity" {
		resource, _ := in["resourceType"].(string)
		if resource != "example_record" || !p.Can("example:read") {
			return groupScope{}, contracts.Problem("forbidden")
		}
		return groupScope{kind: kind, id: resource + ":" + strconv.FormatInt(valueInt(in["scopeId"]), 10), read: identity.Scope{All: true}, write: identity.Scope{All: true}}, nil
	}
	resource := "departments"
	if kind == "organization" {
		resource = "organizations"
	} else if kind != "department" {
		return groupScope{}, contracts.Problem("invalid")
	}
	resolve := func(permission string) (identity.Scope, error) {
		if tx != nil {
			return m.identity.ScopeForInTx(ctx, tx, p, permission, true)
		}
		return m.identity.ScopeFor(ctx, p, permission, true)
	}
	read, err := resolve(resource + ":read")
	if err != nil {
		return groupScope{}, err
	}
	allowed := read
	if write {
		allowed, err = resolve(resource + ":update")
		if err != nil {
			return groupScope{}, err
		}
	}
	return groupScope{read: read, write: allowed, kind: kind, id: strconv.FormatInt(valueInt(in["scopeId"]), 10)}, nil
}

func (m *Module) readGroup(ctx context.Context, name string, in contracts.Input) (contracts.Output, error) {
	if !groupInvocationAllowed(ctx, name) {
		return contracts.Output{}, contracts.Problem("forbidden")
	}
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
	s, err := m.groupScope(ctx, nil, p, in, false)
	if err != nil {
		return contracts.Output{}, err
	}
	q := dbgen.New(m.pool)
	base := dbgen.MemoryGetGroupParams{TenantID: p.TenantID, ScopeType: s.kind, ScopeID: s.id, ID: valueInt(in["id"]), ReadAll: s.read.All, ReadOrgs: s.read.OrgIDs, ReadDepts: s.read.DeptIDs, WriteAll: s.write.All, WriteOrgs: s.write.OrgIDs, WriteDepts: s.write.DeptIDs, ActorID: p.Account.ID}
	op := strings.Replace(name, "memory.entity.data.", "memory.group.", 1)
	if op == "memory.group.get" || op == "memory.group.sources" {
		row, err := q.MemoryGetGroup(ctx, base)
		if errors.Is(err, pgx.ErrNoRows) {
			return contracts.Output{}, contracts.Problem("not_found")
		}
		if err != nil {
			return contracts.Output{}, err
		}
		if op == "memory.group.get" {
			return auditedRead(ctx, q, p, name, row.ID, itemProjection(row.ID, row.Partition, row.MemoryType, row.Title, row.Summary, row.Status, row.VerificationStatus, row.Version, row.Sensitivity, row.ValidFrom, row.ValidTo))
		}
		rows, err := q.MemoryGroupSources(ctx, dbgen.MemoryGroupSourcesParams(base))
		if err != nil {
			return contracts.Output{}, err
		}
		sources := make([]map[string]any, 0, len(rows))
		for _, r := range rows {
			sources = append(sources, map[string]any{"eventId": r.EventID, "eventType": r.EventType, "occurredAt": r.OccurredAt.Time.UTC().Format(time.RFC3339Nano), "correlationId": r.CorrelationID})
		}
		return auditedRead(ctx, q, p, name, row.ID, map[string]any{"sources": sources})
	}
	if op != "memory.group.list" {
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
	rows, err := q.MemoryListGroup(ctx, dbgen.MemoryListGroupParams{TenantID: p.TenantID, ScopeType: s.kind, ScopeID: s.id, OwnerFilter: valueInt(in["ownerId"]), PartitionFilter: partition, StatusFilter: status, CreatedFrom: from, CreatedBefore: before, BeforeID: valueInt(in["beforeId"]), MaxRows: int32(limit), ReadAll: s.read.All, ReadOrgs: s.read.OrgIDs, ReadDepts: s.read.DeptIDs, WriteAll: s.write.All, WriteOrgs: s.write.OrgIDs, WriteDepts: s.write.DeptIDs, ActorID: p.Account.ID})
	if err != nil {
		return contracts.Output{}, err
	}
	items := make([]map[string]any, 0, len(rows))
	for _, r := range rows {
		items = append(items, itemProjection(r.ID, r.Partition, r.MemoryType, r.Title, r.Summary, r.Status, r.VerificationStatus, r.Version, r.Sensitivity, r.ValidFrom, r.ValidTo))
	}
	return auditedRead(ctx, q, p, name, 0, map[string]any{"items": items})
}

func (m *Module) writeGroup(ctx context.Context, tx pgx.Tx, name string, in contracts.Input) (contracts.Output, error) {
	if !groupInvocationAllowed(ctx, name) {
		return contracts.Output{}, contracts.Problem("forbidden")
	}
	p, err := identity.Current(ctx)
	if err != nil {
		return contracts.Output{}, contracts.Problem("unauthenticated")
	}
	if !p.Can("memory:write") {
		return contracts.Output{}, contracts.Problem("forbidden")
	}
	s, err := m.groupScope(ctx, tx, p, in, true)
	if err != nil {
		return contracts.Output{}, err
	}
	q := dbgen.New(tx)
	if err := lockGroupResource(ctx, q, p, s); err != nil {
		return contracts.Output{}, err
	}
	id, version := valueInt(in["id"]), valueInt(in["version"])
	status := "active"
	base := dbgen.MemoryGetGroupParams{TenantID: p.TenantID, ScopeType: s.kind, ScopeID: s.id, ID: id, ReadAll: s.read.All, ReadOrgs: s.read.OrgIDs, ReadDepts: s.read.DeptIDs, WriteAll: s.write.All, WriteOrgs: s.write.OrgIDs, WriteDepts: s.write.DeptIDs, ActorID: p.Account.ID}
	op := strings.Replace(name, "memory.entity.data.", "memory.group.", 1)
	if op == "memory.group.create" {
		partition, _ := in["partition"].(string)
		kind, _ := in["memoryType"].(string)
		title, _ := in["title"].(string)
		summary, _ := in["summary"].(string)
		from, to, err := validity(in)
		if err != nil {
			return contracts.Output{}, err
		}
		row, err := q.MemoryCreateGroup(ctx, dbgen.MemoryCreateGroupParams{TenantID: p.TenantID, ScopeType: s.kind, ScopeID: s.id, OwnerID: p.Account.ID, Partition: partition, MemoryType: kind, Title: title, Summary: summary, ValidFrom: from, ValidTo: to, ReadAll: s.read.All, ReadOrgs: s.read.OrgIDs, ReadDepts: s.read.DeptIDs, WriteAll: s.write.All, WriteOrgs: s.write.OrgIDs, WriteDepts: s.write.DeptIDs, ActorID: p.Account.ID})
		if errors.Is(err, pgx.ErrNoRows) {
			return contracts.Output{}, contracts.Problem("not_found")
		}
		if err != nil {
			return contracts.Output{}, err
		}
		id, version, base.ID = row.ID, row.Version, row.ID
		if source, ok := in["sourceEventId"].(string); ok {
			count, err := q.MemoryGroupAttachEvent(ctx, dbgen.MemoryGroupAttachEventParams{EventID: source, TenantID: p.TenantID, ScopeType: s.kind, ScopeID: s.id, ID: id, ReadAll: s.read.All, ReadOrgs: s.read.OrgIDs, ReadDepts: s.read.DeptIDs, WriteAll: s.write.All, WriteOrgs: s.write.OrgIDs, WriteDepts: s.write.DeptIDs, ActorID: p.Account.ID})
			if err != nil {
				return contracts.Output{}, err
			}
			if count != 1 {
				return contracts.Output{}, contracts.Problem("not_found")
			}
		}
	} else {
		old, err := q.MemoryLockGroup(ctx, dbgen.MemoryLockGroupParams(base))
		if errors.Is(err, pgx.ErrNoRows) {
			return contracts.Output{}, contracts.Problem("not_found")
		}
		if err != nil {
			return contracts.Output{}, err
		}
		if old.Version != version {
			return contracts.Output{}, contracts.Problem("conflict")
		}
		var count int64
		switch op {
		case "memory.group.correct":
			if old.Status != "active" {
				return contracts.Output{}, contracts.Problem("conflict")
			}
			count, err = q.MemoryGroupCorrection(ctx, dbgen.MemoryGroupCorrectionParams{TenantID: p.TenantID, ScopeType: s.kind, ScopeID: s.id, ID: id, Version: version, ReadAll: s.read.All, ReadOrgs: s.read.OrgIDs, ReadDepts: s.read.DeptIDs, WriteAll: s.write.All, WriteOrgs: s.write.OrgIDs, WriteDepts: s.write.DeptIDs, ActorID: p.Account.ID})
			if err != nil {
				return contracts.Output{}, err
			}
			if count != 1 {
				return contracts.Output{}, contracts.Problem("conflict")
			}
			title, _ := in["title"].(string)
			summary, _ := in["summary"].(string)
			count, err = q.MemoryCorrectGroup(ctx, dbgen.MemoryCorrectGroupParams{TenantID: p.TenantID, ScopeType: s.kind, ScopeID: s.id, ID: id, Version: version, Title: title, Summary: summary, ReadAll: s.read.All, ReadOrgs: s.read.OrgIDs, ReadDepts: s.read.DeptIDs, WriteAll: s.write.All, WriteOrgs: s.write.OrgIDs, WriteDepts: s.write.DeptIDs, ActorID: p.Account.ID})
		case "memory.group.archive":
			status = "archived"
			count, err = q.MemoryArchiveGroup(ctx, dbgen.MemoryArchiveGroupParams{TenantID: p.TenantID, ScopeType: s.kind, ScopeID: s.id, ID: id, Version: version, ReadAll: s.read.All, ReadOrgs: s.read.OrgIDs, ReadDepts: s.read.DeptIDs, WriteAll: s.write.All, WriteOrgs: s.write.OrgIDs, WriteDepts: s.write.DeptIDs, ActorID: p.Account.ID})
		case "memory.group.delete":
			status = "deleted"
			count, err = q.MemoryDeleteGroup(ctx, dbgen.MemoryDeleteGroupParams{TenantID: p.TenantID, ScopeType: s.kind, ScopeID: s.id, ID: id, Version: version, ReadAll: s.read.All, ReadOrgs: s.read.OrgIDs, ReadDepts: s.read.DeptIDs, WriteAll: s.write.All, WriteOrgs: s.write.OrgIDs, WriteDepts: s.write.DeptIDs, ActorID: p.Account.ID})
			if err == nil && count == 1 {
				err = q.MemoryGroupDeleteSources(ctx, dbgen.MemoryGroupDeleteSourcesParams(base))
			}
			if err == nil && count == 1 {
				err = q.MemoryGroupDeleteCorrections(ctx, dbgen.MemoryGroupDeleteCorrectionsParams(base))
			}
			if err == nil && count == 1 {
				err = q.MemoryGroupDeleteRelations(ctx, dbgen.MemoryGroupDeleteRelationsParams(base))
			}
		default:
			return contracts.Output{}, contracts.Problem("not_found")
		}
		if err != nil {
			return contracts.Output{}, err
		}
		if count != 1 {
			return contracts.Output{}, contracts.Problem("conflict")
		}
		version++
	}
	return contracts.Output{Data: map[string]any{"id": id, "version": version, "status": status}, Events: []contracts.Event{{Type: name + ".completed", AggregateType: "memory_item", AggregateID: strconv.FormatInt(id, 10), Payload: map[string]any{"id": id, "version": version, "status": status}}}}, nil
}

func lockGroupResource(ctx context.Context, q *dbgen.Queries, p *identity.Principal, s groupScope) error {
	base := dbgen.MemoryLockGroupDepartmentParams{TenantID: p.TenantID, ScopeID: s.id, ReadAll: s.read.All, ReadOrgs: s.read.OrgIDs, ReadDepts: s.read.DeptIDs, WriteAll: s.write.All, WriteOrgs: s.write.OrgIDs, WriteDepts: s.write.DeptIDs, ActorID: p.Account.ID}
	var err error
	switch s.kind {
	case "department":
		_, err = q.MemoryLockGroupDepartment(ctx, base)
	case "organization":
		_, err = q.MemoryLockGroupOrganization(ctx, dbgen.MemoryLockGroupOrganizationParams(base))
	case "business_entity":
		_, err = q.MemoryLockGroupEntity(ctx, dbgen.MemoryLockGroupEntityParams(base))
	default:
		return contracts.Problem("invalid")
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return contracts.Problem("not_found")
	}
	return err
}

func groupInvocationAllowed(ctx context.Context, name string) bool {
	if strings.HasPrefix(name, "memory.entity.data.") {
		meta, ok := gateway.CurrentInvocation(ctx)
		return ok && meta.Origin == contracts.OriginCapability
	}
	return manualInvocation(ctx)
}
