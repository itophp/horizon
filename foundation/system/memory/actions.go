package memory

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"horizon/foundation/kernel/capability/contracts"
	"horizon/foundation/kernel/capability/gateway"
	"horizon/foundation/system/identity"
	"horizon/internal/repository/dbgen"
)

func valueInt(v any) int64 {
	switch n := v.(type) {
	case float64:
		return int64(n)
	case int:
		return int64(n)
	case int64:
		return n
	case json.Number:
		x, _ := n.Int64()
		return x
	default:
		return 0
	}
}

// listFilters treats an omitted status as active and never exposes deleted rows.
func listFilters(in contracts.Input) (string, pgtype.Timestamptz, pgtype.Timestamptz, error) {
	status, _ := in["status"].(string)
	if status == "" {
		status = "active"
	}
	if status != "active" && status != "archived" {
		return "", pgtype.Timestamptz{}, pgtype.Timestamptz{}, contracts.Problem("invalid")
	}
	var from, before pgtype.Timestamptz
	for key, target := range map[string]*pgtype.Timestamptz{"createdFrom": &from, "createdBefore": &before} {
		if raw, ok := in[key].(string); ok {
			parsed, err := time.Parse(time.RFC3339Nano, raw)
			if err != nil {
				return "", from, before, contracts.Problem("invalid")
			}
			*target = pgtype.Timestamptz{Time: parsed.UTC(), Valid: true}
		}
	}
	if from.Valid && before.Valid && !from.Time.Before(before.Time) {
		return "", from, before, contracts.Problem("invalid")
	}
	return status, from, before, nil
}

func validity(in contracts.Input) (pgtype.Timestamptz, pgtype.Timestamptz, error) {
	var from, to pgtype.Timestamptz
	for key, target := range map[string]*pgtype.Timestamptz{"validFrom": &from, "validTo": &to} {
		if raw, ok := in[key].(string); ok {
			value, err := time.Parse(time.RFC3339Nano, raw)
			if err != nil {
				return from, to, contracts.Problem("invalid")
			}
			*target = pgtype.Timestamptz{Time: value.UTC(), Valid: true}
		}
	}
	if to.Valid && ((!from.Valid && !to.Time.After(time.Now())) || (from.Valid && !to.Time.After(from.Time))) {
		return from, to, contracts.Problem("invalid")
	}
	return from, to, nil
}

func itemProjection(id int64, partition, kind, title, summary, status, verification string, version int64, sensitivity string, from, to pgtype.Timestamptz) map[string]any {
	validTo := ""
	if to.Valid {
		validTo = to.Time.UTC().Format(time.RFC3339Nano)
	}
	return map[string]any{"id": id, "partition": partition, "memoryType": kind, "title": title, "summary": summary, "status": status, "verificationStatus": verification, "version": version, "sensitivity": sensitivity, "validFrom": from.Time.UTC().Format(time.RFC3339Nano), "validTo": validTo}
}

func (m *Module) read(ctx context.Context, name string, in contracts.Input) (contracts.Output, error) {
	if !manualInvocation(ctx) {
		return contracts.Output{}, contracts.Problem("forbidden")
	}
	p, err := identity.Current(ctx)
	if err != nil {
		return contracts.Output{}, contracts.Problem("unauthenticated")
	}
	permission := "memory:read"
	if name == "memory.ingestion.failures" {
		permission = "memory:replay"
	}
	if name == "memory.policy.list" {
		permission = "memory:manage"
	}
	if !p.Can(permission) {
		return contracts.Output{}, contracts.Problem("forbidden")
	}
	if m.pool == nil {
		return contracts.Output{}, contracts.Problem("unavailable")
	}
	q, scope := dbgen.New(m.pool), strconv.FormatInt(p.Account.ID, 10)
	switch name {
	case "memory.policy.list":
		rows, err := q.MemoryListPolicies(ctx, p.TenantID)
		if err != nil {
			return contracts.Output{}, err
		}
		policies := make([]map[string]any, 0, len(rows))
		for _, row := range rows {
			policies = append(policies, map[string]any{"partition": row.Partition, "retentionDays": row.RetentionDays, "revision": row.Revision})
		}
		return auditedRead(ctx, q, p, name, 0, map[string]any{"policies": policies})
	case "memory.ingestion.failures":
		limit := valueInt(in["limit"])
		if limit == 0 {
			limit = 30
		}
		rows, err := q.MemoryListDeadEvents(ctx, dbgen.MemoryListDeadEventsParams{TenantID: p.TenantID, MaxRows: int32(limit)})
		if err != nil {
			return contracts.Output{}, err
		}
		failures := make([]map[string]any, 0, len(rows))
		for _, row := range rows {
			failures = append(failures, map[string]any{"sourceEventId": row.SourceEventID, "eventId": row.EventID, "attempts": row.Attempts, "errorCode": row.ErrorCode, "deadAt": row.DeadAt.Time.UTC().Format(time.RFC3339Nano)})
		}
		return auditedRead(ctx, q, p, name, 0, map[string]any{"failures": failures})
	case "memory.item.get":
		row, err := q.MemoryGetPersonal(ctx, dbgen.MemoryGetPersonalParams{TenantID: p.TenantID, OwnerID: p.Account.ID, ScopeID: scope, ID: valueInt(in["id"])})
		if errors.Is(err, pgx.ErrNoRows) {
			return contracts.Output{}, contracts.Problem("not_found")
		}
		if err != nil {
			return contracts.Output{}, err
		}
		return auditedRead(ctx, q, p, name, row.ID, itemProjection(row.ID, row.Partition, row.MemoryType, row.Title, row.Summary, row.Status, row.VerificationStatus, row.Version, row.Sensitivity, row.ValidFrom, row.ValidTo))
	case "memory.item.list":
		limit := valueInt(in["limit"])
		if limit == 0 {
			limit = 30
		}
		partition, _ := in["partition"].(string)
		status, from, before, err := listFilters(in)
		if err != nil {
			return contracts.Output{}, err
		}
		rows, err := q.MemoryListPersonal(ctx, dbgen.MemoryListPersonalParams{TenantID: p.TenantID, OwnerID: p.Account.ID, OwnerFilter: valueInt(in["ownerId"]), ScopeID: scope, PartitionFilter: partition, StatusFilter: status, CreatedFrom: from, CreatedBefore: before, BeforeID: valueInt(in["beforeId"]), MaxRows: int32(limit)})
		if err != nil {
			return contracts.Output{}, err
		}
		items := make([]map[string]any, 0, len(rows))
		for _, r := range rows {
			items = append(items, itemProjection(r.ID, r.Partition, r.MemoryType, r.Title, r.Summary, r.Status, r.VerificationStatus, r.Version, r.Sensitivity, r.ValidFrom, r.ValidTo))
		}
		return auditedRead(ctx, q, p, name, 0, map[string]any{"items": items})
	case "memory.item.sources":
		id := valueInt(in["id"])
		_, err := q.MemoryGetPersonal(ctx, dbgen.MemoryGetPersonalParams{TenantID: p.TenantID, OwnerID: p.Account.ID, ScopeID: scope, ID: id})
		if errors.Is(err, pgx.ErrNoRows) {
			return contracts.Output{}, contracts.Problem("not_found")
		}
		if err != nil {
			return contracts.Output{}, err
		}
		rows, err := q.MemoryListOwnSources(ctx, dbgen.MemoryListOwnSourcesParams{TenantID: p.TenantID, OwnerID: p.Account.ID, ScopeID: scope, MemoryID: id})
		if err != nil {
			return contracts.Output{}, err
		}
		sources := make([]map[string]any, 0, len(rows))
		for _, row := range rows {
			sources = append(sources, map[string]any{"eventId": row.EventID, "eventType": row.EventType, "occurredAt": row.OccurredAt.Time.UTC().Format(time.RFC3339Nano), "correlationId": row.CorrelationID})
		}
		return auditedRead(ctx, q, p, name, id, map[string]any{"sources": sources})
	case "memory.event.list":
		limit := valueInt(in["limit"])
		if limit == 0 {
			limit = 30
		}
		eventType, _ := in["eventType"].(string)
		correlation, _ := in["correlationId"].(string)
		eventID, _ := in["eventId"].(string)
		resourceType, _ := in["resourceType"].(string)
		resourceID, _ := in["resourceId"].(string)
		var from, before pgtype.Timestamptz
		for key, target := range map[string]*pgtype.Timestamptz{"occurredFrom": &from, "occurredBefore": &before} {
			if raw, ok := in[key].(string); ok {
				parsed, parseErr := time.Parse(time.RFC3339Nano, raw)
				if parseErr != nil {
					return contracts.Output{}, contracts.Problem("invalid")
				}
				*target = pgtype.Timestamptz{Time: parsed.UTC(), Valid: true}
			}
		}
		if from.Valid && before.Valid && !from.Time.Before(before.Time) {
			return contracts.Output{}, contracts.Problem("invalid")
		}
		rows, err := q.MemoryListOwnEvents(ctx, dbgen.MemoryListOwnEventsParams{TenantID: p.TenantID, ActorID: scope, AccountID: p.Account.ID, EventType: eventType, EventID: eventID, ResourceType: resourceType, ResourceID: resourceID, CorrelationID: correlation, OccurredFrom: from, OccurredBefore: before, BeforeID: valueInt(in["beforeId"]), MaxRows: int32(limit)})
		if err != nil {
			return contracts.Output{}, err
		}
		events := make([]map[string]any, 0, len(rows))
		for _, row := range rows {
			payload := map[string]any{}
			if err := json.Unmarshal(row.Payload, &payload); err != nil {
				return contracts.Output{}, contracts.Problem("internal")
			}
			events = append(events, map[string]any{"id": row.ID, "eventId": row.EventID, "eventType": row.EventType, "occurredAt": row.OccurredAt.Time.UTC().Format(time.RFC3339Nano), "sourceType": row.SourceType, "sourceId": row.SourceID, "resourceType": row.ResourceType, "resourceId": row.ResourceID, "actionName": row.ActionName, "result": row.Result, "traceId": row.TraceID, "correlationId": row.CorrelationID, "causationId": row.CausationID, "sessionId": row.SessionID, "taskId": row.TaskID, "workflowInstanceId": row.WorkflowInstanceID, "candidatePartitions": row.CandidatePartitions, "payload": payload})
		}
		return auditedRead(ctx, q, p, name, 0, map[string]any{"events": events})
	default:
		return contracts.Output{}, contracts.Problem("not_found")
	}
}

func auditedRead(ctx context.Context, q *dbgen.Queries, p *identity.Principal, action string, id int64, data any) (contracts.Output, error) {
	meta, ok := gateway.CurrentInvocation(ctx)
	if !ok {
		return contracts.Output{}, contracts.Problem("forbidden")
	}
	if err := q.MemoryRecordAccess(ctx, dbgen.MemoryRecordAccessParams{TenantID: p.TenantID, ActorID: p.Account.ID, Action: action, ObjectID: id, CorrelationID: meta.CorrelationID}); err != nil {
		return contracts.Output{}, err
	}
	return contracts.Output{Data: data}, nil
}

func (m *Module) write(ctx context.Context, tx pgx.Tx, name string, in contracts.Input) (contracts.Output, error) {
	if !manualInvocation(ctx) {
		return contracts.Output{}, contracts.Problem("forbidden")
	}
	if name == "memory.activity.collect" {
		return m.collectActivity(ctx, tx, in)
	}
	if name == "memory.ingestion.replay" {
		return m.replay(ctx, tx, in)
	}
	if name == "memory.ingestion.replay_range" {
		return m.replayRange(ctx, tx, in)
	}
	if name == "memory.policy.set" {
		return m.setPolicy(ctx, tx, in)
	}
	p, err := identity.Current(ctx)
	if err != nil {
		return contracts.Output{}, contracts.Problem("unauthenticated")
	}
	if !p.Can("memory:write") {
		return contracts.Output{}, contracts.Problem("forbidden")
	}
	q, scope := dbgen.New(tx), strconv.FormatInt(p.Account.ID, 10)
	id, version, status := valueInt(in["id"]), valueInt(in["version"]), "active"
	if name == "memory.item.create" {
		partition, _ := in["partition"].(string)
		kind, _ := in["memoryType"].(string)
		title, _ := in["title"].(string)
		summary, _ := in["summary"].(string)
		from, to, err := validity(in)
		if err != nil {
			return contracts.Output{}, err
		}
		row, err := q.MemoryCreatePersonal(ctx, dbgen.MemoryCreatePersonalParams{TenantID: p.TenantID, Partition: partition, MemoryType: kind, ScopeID: scope, OwnerID: p.Account.ID, Title: title, Summary: summary, ValidFrom: from, ValidTo: to})
		if err != nil {
			return contracts.Output{}, err
		}
		id, version = row.ID, row.Version
		if eventID, ok := in["sourceEventId"].(string); ok {
			count, err := q.MemoryAttachOwnEvent(ctx, dbgen.MemoryAttachOwnEventParams{TenantID: p.TenantID, OwnerID: p.Account.ID, ScopeID: scope, MemoryID: id, EventID: eventID})
			if err != nil {
				return contracts.Output{}, err
			}
			if count != 1 {
				return contracts.Output{}, contracts.Problem("not_found")
			}
		}
	} else {
		old, err := q.MemoryLockPersonal(ctx, dbgen.MemoryLockPersonalParams{TenantID: p.TenantID, OwnerID: p.Account.ID, ScopeID: scope, ID: id})
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
		switch name {
		case "memory.item.correct":
			if old.Status != "active" {
				return contracts.Output{}, contracts.Problem("conflict")
			}
			corrections, err := q.MemoryRecordCorrection(ctx, dbgen.MemoryRecordCorrectionParams{TenantID: p.TenantID, MemoryID: id, ActorID: p.Account.ID, PreviousVersion: version})
			if err != nil {
				return contracts.Output{}, err
			}
			if corrections != 1 {
				return contracts.Output{}, contracts.Problem("conflict")
			}
			title, _ := in["title"].(string)
			summary, _ := in["summary"].(string)
			affected, err = q.MemoryCorrectPersonal(ctx, dbgen.MemoryCorrectPersonalParams{TenantID: p.TenantID, OwnerID: p.Account.ID, ScopeID: scope, ID: id, Version: version, Title: title, Summary: summary})
		case "memory.item.archive":
			status = "archived"
			affected, err = q.MemoryArchivePersonal(ctx, dbgen.MemoryArchivePersonalParams{TenantID: p.TenantID, OwnerID: p.Account.ID, ScopeID: scope, ID: id, Version: version})
		case "memory.item.delete":
			status = "deleted"
			affected, err = q.MemoryDeletePersonal(ctx, dbgen.MemoryDeletePersonalParams{TenantID: p.TenantID, OwnerID: p.Account.ID, ScopeID: scope, ID: id, Version: version})
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
	}
	data := map[string]any{"id": id, "version": version, "status": status}
	return contracts.Output{Data: data, Summary: fmt.Sprintf("memory=%d", id), Events: []contracts.Event{{Type: name + ".completed", AggregateType: "memory_item", AggregateID: strconv.FormatInt(id, 10), Payload: map[string]any{"id": id, "version": version, "status": status}}}}, nil
}

func manualInvocation(ctx context.Context) bool {
	meta, ok := gateway.CurrentInvocation(ctx)
	return ok && (meta.Origin == contracts.OriginHumanUI || meta.Origin == contracts.OriginAPI || meta.Origin == contracts.OriginApproval)
}
