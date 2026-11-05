package memory

import (
	"context"
	"encoding/json"
	"regexp"
	"strconv"
	"time"

	"baseline/services/api/internal/repository/dbgen"
	"baseline/services/api/kernel/capability/contracts"
	"baseline/services/api/kernel/capability/gateway"
	canonical "baseline/services/api/kernel/memory"
	"baseline/services/api/system/identity"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

var activityID = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)

// Page identifiers are intentionally not URLs: URL paths and query parameters
// can contain names, secrets, or business identifiers.
var activityPages = map[string]bool{
	"analytics": true, "ai": true, "articles": true, "drive": true,
	"knowledge": true, "projects": true, "orchestration": true,
	"accounts": true, "organizations": true, "departments": true,
	"roles": true, "settings": true, "preferences": true,
	"dictionaries": true, "attachments": true, "notifications": true,
	"permissions": true, "temporary-access": true, "capabilities": true,
	"mcp": true, "webmcp": true,
}

func (m *Module) collectActivity(ctx context.Context, tx pgx.Tx, in contracts.Input) (contracts.Output, error) {
	p, err := identity.Current(ctx)
	if err != nil {
		return contracts.Output{}, contracts.Problem("unauthenticated")
	}
	if !p.Can("memory:collect") {
		return contracts.Output{}, contracts.Problem("forbidden")
	}
	batch, ok := in["events"].([]any)
	if !ok || len(batch) == 0 || len(batch) > 25 {
		return contracts.Output{}, contracts.Problem("invalid")
	}
	now := time.Now().UTC()
	meta, ok := gateway.CurrentInvocation(ctx)
	if !ok {
		return contracts.Output{}, contracts.Problem("forbidden")
	}
	q := dbgen.New(tx)
	accepted := int64(0)
	for _, raw := range batch {
		entry, ok := raw.(map[string]any)
		if !ok {
			return contracts.Output{}, contracts.Problem("invalid")
		}
		id, _ := entry["id"].(string)
		kind, _ := entry["kind"].(string)
		page, _ := entry["page"].(string)
		action, _ := entry["action"].(string)
		at, _ := entry["occurredAt"].(string)
		occurred, parseErr := time.Parse(time.RFC3339Nano, at)
		if !activityID.MatchString(id) || !activityPages[page] || parseErr != nil || occurred.Before(now.Add(-24*time.Hour)) || occurred.After(now.Add(5*time.Minute)) {
			return contracts.Output{}, contracts.Problem("invalid")
		}
		eventType := "activity.interaction.recorded"
		if kind == "page_view" && action == "view" {
			eventType = "activity.page.viewed"
		} else if kind != "interaction" || action == "view" {
			return contracts.Output{}, contracts.Problem("invalid")
		}
		e := canonical.Envelope{EventID: "web:" + strconv.FormatInt(p.Account.ID, 10) + ":" + id, EventType: eventType, TenantID: p.TenantID,
			Actor: canonical.Actor{ID: strconv.FormatInt(p.Account.ID, 10), Type: "user"}, DepartmentID: p.Account.DepartmentID,
			Source: canonical.Source{Type: "web", ID: page}, ActionName: action, OccurredAt: occurred, Sensitivity: "internal", Payload: map[string]string{"action": action},
			InvocationID: meta.InvocationID, TraceID: meta.CorrelationID, CorrelationID: meta.CorrelationID, CausationID: meta.InvocationID, Depth: meta.Depth}
		e, err = (canonical.Policy{eventType: {"action": {"view", "open", "submit", "success", "failure"}}}).Normalize(e, now)
		if err != nil {
			return contracts.Output{}, contracts.Problem("invalid")
		}
		payload, err := json.Marshal(e.Payload)
		if err != nil {
			return contracts.Output{}, err
		}
		n, err := q.MemoryInsertEvent(ctx, dbgen.MemoryInsertEventParams{TenantID: e.TenantID, EventID: e.EventID, EventType: e.EventType, EventVersion: e.EventVersion,
			OccurredAt: pgtype.Timestamptz{Time: e.OccurredAt, Valid: true}, ActorID: e.Actor.ID, ActorType: e.Actor.Type, DepartmentID: e.DepartmentID,
			SourceType: e.Source.Type, SourceID: e.Source.ID, ActionName: e.ActionName, InvocationID: e.InvocationID, TraceID: e.TraceID, CorrelationID: e.CorrelationID, CausationID: e.CausationID, TriggerDepth: int32(e.Depth), Sensitivity: e.Sensitivity, CandidatePartitions: e.CandidatePartitions, Payload: payload})
		if err != nil {
			return contracts.Output{}, err
		}
		accepted += n
	}
	return contracts.Output{Data: map[string]any{"accepted": accepted}, Events: []contracts.Event{{Type: "memory.activity.collected", AggregateType: "memory_event", AggregateID: strconv.FormatInt(p.Account.ID, 10), Payload: map[string]any{"count": accepted}}}}, nil
}
