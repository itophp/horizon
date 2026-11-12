package memory

import (
	"context"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"horizon/foundation/kernel/capability/contracts"
	"horizon/foundation/kernel/capability/gateway"
	"horizon/foundation/kernel/capability/registry"
	"horizon/foundation/system/identity"
	"strings"
)

type Module struct {
	pool     *pgxpool.Pool
	gateway  *gateway.Gateway
	identity *identity.Service
}

func New(pool *pgxpool.Pool) *Module                 { return &Module{pool: pool, identity: identity.New(pool, false)} }
func (m *Module) AttachGateway(g *gateway.Gateway)   { m.gateway = g }
func (m *Module) AttachIdentity(s *identity.Service) { m.identity = s }

func (m *Module) Entries() []registry.Entry {
	entries := []registry.Entry{}
	all := append(definitions(), projectDefinitions()...)
	all = append(all, groupDefinitions()...)
	all = append(all, entityDefinitions()...)
	for _, d := range all {
		name := d.Name
		if strings.HasPrefix(name, "memory.group.") || strings.HasPrefix(name, "memory.entity.data.") {
			if d.ReadOnly {
				entries = append(entries, registry.Entry{Definition: d, Handler: contracts.Handler{Execute: func(ctx context.Context, in contracts.Input) (contracts.Output, error) {
					return m.readGroup(ctx, name, in)
				}}})
			} else {
				entries = append(entries, registry.Entry{Definition: d, Handler: contracts.Handler{ExecuteTx: func(ctx context.Context, tx pgx.Tx, in contracts.Input) (contracts.Output, error) {
					return m.writeGroup(ctx, tx, name, in)
				}}})
			}
			continue
		}
		if d.Layer == contracts.LayerOrchestration {
			entries = append(entries, registry.Entry{Definition: d, Handler: contracts.Handler{Execute: func(ctx context.Context, in contracts.Input) (contracts.Output, error) {
				if strings.HasPrefix(name, "memory.entity.") {
					return m.entityFlow(ctx, name, in)
				}
				return m.projectFlow(ctx, name, in)
			}}})
			continue
		}
		if strings.HasPrefix(name, "memory.project.data.") {
			if d.ReadOnly {
				entries = append(entries, registry.Entry{Definition: d, Handler: contracts.Handler{Execute: func(ctx context.Context, in contracts.Input) (contracts.Output, error) {
					return m.readProjectData(ctx, name, in)
				}}})
			} else {
				entries = append(entries, registry.Entry{Definition: d, Handler: contracts.Handler{ExecuteTx: func(ctx context.Context, tx pgx.Tx, in contracts.Input) (contracts.Output, error) {
					return m.writeProjectData(ctx, tx, name, in)
				}}})
			}
			continue
		}
		if d.ReadOnly {
			entries = append(entries, registry.Entry{Definition: d, Handler: contracts.Handler{Execute: func(ctx context.Context, in contracts.Input) (contracts.Output, error) { return m.read(ctx, name, in) }}})
		} else {
			entries = append(entries, registry.Entry{Definition: d, Handler: contracts.Handler{ExecuteTx: func(ctx context.Context, tx pgx.Tx, in contracts.Input) (contracts.Output, error) {
				return m.write(ctx, tx, name, in)
			}}})
		}
	}
	return entries
}

func definitions() []contracts.Definition {
	id := contracts.Int(1, 1<<53)
	version := contracts.Int(1, 1<<53)
	partition := &contracts.Schema{Type: "string", Enum: []string{"agent", "activity", "exception", "business", "knowledge", "execution"}}
	item := itemSchema()
	result := mutationSchema()
	base := func(name, title, permission string, in, out *contracts.Schema, read bool) contracts.Definition {
		d := contracts.Definition{Layer: contracts.LayerAtomic, Name: name, Version: "1.0.0", Title: title, Description: title + " within the caller's authorized personal scope.", Domain: "memory", Entity: "memory_item", Permission: permission, Risk: contracts.RiskLow, Approval: contracts.ApprovalAuto, ReadOnly: read, Executor: "go:" + name, Input: in, Output: out}
		d.Tags = []string{"manual-only"}
		if read {
			d.TransientResult = true
		} else {
			d.Risk = contracts.RiskMedium
			d.Idempotency = contracts.Idempotency{Required: true, Scope: "tenant+operation+key"}
			d.SuccessEvents = []string{name + ".completed"}
		}
		return d
	}
	create := base("memory.item.create", "Create personal memory", "memory:write", contracts.Object(map[string]*contracts.Schema{"partition": partition, "memoryType": contracts.Str(1, 100), "title": contracts.Str(1, 200), "summary": contracts.Str(0, 4000), "sourceEventId": contracts.Str(1, 256)}, "partition", "memoryType", "title", "summary"), result, false)
	addValiditySchema(create.Input)
	create.Entities = []string{"memory_item", "memory_source"}
	correct := base("memory.item.correct", "Correct personal memory", "memory:write", contracts.Object(map[string]*contracts.Schema{"id": id, "version": version, "title": contracts.Str(1, 200), "summary": contracts.Str(0, 4000)}, "id", "version", "title", "summary"), result, false)
	correct.Entities = []string{"memory_item", "memory_correction"}
	correct.Loggable = []string{"id", "version"}
	archive := base("memory.item.archive", "Archive personal memory", "memory:write", contracts.Object(map[string]*contracts.Schema{"id": id, "version": version}, "id", "version"), result, false)
	archive.Loggable = []string{"id", "version"}
	remove := base("memory.item.delete", "Delete personal memory", "memory:write", contracts.Object(map[string]*contracts.Schema{"id": id, "version": version}, "id", "version"), result, false)
	remove.Entities = []string{"memory_item", "memory_source", "memory_relation", "memory_correction"}
	remove.Loggable = []string{"id", "version"}
	get := base("memory.item.get", "Read personal memory", "memory:read", contracts.Object(map[string]*contracts.Schema{"id": id}, "id"), item, true)
	get.Loggable = []string{"id"}
	get.Entities = []string{"memory_item", "memory_access_audit"}
	list := base("memory.item.list", "List personal memories", "memory:read", contracts.Object(map[string]*contracts.Schema{"partition": partition, "beforeId": contracts.Int(0, 1<<53), "limit": contracts.Int(1, 100)}), contracts.Object(map[string]*contracts.Schema{"items": {Type: "array", Items: item}}, "items"), true)
	addListFilterSchema(list.Input)
	list.Loggable = []string{"partition", "beforeId", "limit"}
	list.Entities = []string{"memory_item", "memory_access_audit"}
	source := contracts.Object(map[string]*contracts.Schema{"eventId": contracts.Str(1, 256), "eventType": contracts.Str(1, 256), "occurredAt": contracts.Str(1, 64), "correlationId": {Type: "string"}}, "eventId", "eventType", "occurredAt", "correlationId")
	sources := base("memory.item.sources", "List personal memory sources", "memory:read", contracts.Object(map[string]*contracts.Schema{"id": id}, "id"), contracts.Object(map[string]*contracts.Schema{"sources": {Type: "array", Items: source}}, "sources"), true)
	sources.Loggable = []string{"id"}
	sources.Entities = []string{"memory_item", "memory_access_audit"}
	event := contracts.Object(map[string]*contracts.Schema{
		"id": contracts.Str(1, 64), "kind": {Type: "string", Enum: []string{"page_view", "interaction"}},
		"page": contracts.Str(1, 64), "action": {Type: "string", Enum: []string{"view", "open", "submit", "success", "failure"}},
		"occurredAt": contracts.Str(1, 64),
	}, "id", "kind", "page", "action", "occurredAt")
	maxItems := 25
	collect := contracts.Definition{Layer: contracts.LayerAtomic, Name: "memory.activity.collect", Version: "1.0.0", Title: "Collect activity observations", Description: "Collect a bounded batch of allowlisted activity metadata for the current user.", Domain: "memory", Entity: "memory_event", Permission: "memory:collect", Risk: contracts.RiskMedium, Approval: contracts.ApprovalAuto, Tags: []string{"manual-only"}, Executor: "go:memory.activity.collect", Idempotency: contracts.Idempotency{Required: true, Scope: "tenant+operation+key"}, SuccessEvents: []string{"memory.activity.collected"}, Input: contracts.Object(map[string]*contracts.Schema{"events": {Type: "array", Items: event, MaxItems: &maxItems}}, "events"), Output: contracts.Object(map[string]*contracts.Schema{"accepted": contracts.Int(0, 25)}, "accepted")}
	eventResult := contracts.Object(map[string]*contracts.Schema{
		"id": id, "eventId": contracts.Str(1, 256), "eventType": contracts.Str(1, 256), "occurredAt": contracts.Str(1, 64),
		"sourceType": {Type: "string"}, "sourceId": {Type: "string"}, "resourceType": {Type: "string"}, "resourceId": {Type: "string"},
		"actionName": {Type: "string"}, "result": {Type: "string"}, "traceId": {Type: "string"}, "correlationId": {Type: "string"}, "causationId": {Type: "string"},
		"sessionId": {Type: "string"}, "taskId": {Type: "string"}, "workflowInstanceId": {Type: "string"},
		"candidatePartitions": {Type: "array", Items: &contracts.Schema{Type: "string"}},
		"payload":             {Type: "object", AdditionalProperties: true},
	}, "id", "eventId", "eventType", "occurredAt", "sourceType", "sourceId", "resourceType", "resourceId", "actionName", "result", "traceId", "correlationId", "causationId", "sessionId", "taskId", "workflowInstanceId", "candidatePartitions", "payload")
	events := contracts.Definition{Layer: contracts.LayerAtomic, Name: "memory.event.list", Version: "1.0.0", Title: "List personal observations", Description: "Read authorized observations for the current user.", Domain: "memory", Entity: "memory_event", Permission: "memory:read", Risk: contracts.RiskLow, Approval: contracts.ApprovalAuto, ReadOnly: true, TransientResult: true, Tags: []string{"manual-only"}, Executor: "go:memory.event.list", Input: contracts.Object(map[string]*contracts.Schema{"eventType": contracts.Str(1, 256), "eventId": contracts.Str(1, 256), "resourceType": contracts.Str(1, 256), "resourceId": contracts.Str(1, 256), "correlationId": contracts.Str(1, 256), "occurredFrom": contracts.Str(1, 64), "occurredBefore": contracts.Str(1, 64), "beforeId": contracts.Int(0, 1<<53), "limit": contracts.Int(1, 100)}), Output: contracts.Object(map[string]*contracts.Schema{"events": {Type: "array", Items: eventResult}}, "events"), Loggable: []string{"beforeId", "limit"}}
	events.Entities = []string{"memory_event", "memory_access_audit"}
	failure := contracts.Object(map[string]*contracts.Schema{"sourceEventId": id, "eventId": contracts.Str(1, 256), "attempts": contracts.Int(0, 1000), "errorCode": contracts.Str(0, 64), "deadAt": contracts.Str(1, 64)}, "sourceEventId", "eventId", "attempts", "errorCode", "deadAt")
	failures := contracts.Definition{Layer: contracts.LayerAtomic, Name: "memory.ingestion.failures", Version: "1.0.0", Title: "Inspect failed memory ingestion", Description: "List ingestion failures for the current tenant.", Domain: "memory", Entity: "memory_event_inbox", Entities: []string{"memory_event_inbox", "memory_access_audit"}, Permission: "memory:replay", Risk: contracts.RiskLow, Approval: contracts.ApprovalAuto, ReadOnly: true, Tags: []string{"manual-only"}, Executor: "go:memory.ingestion.failures", Input: contracts.Object(map[string]*contracts.Schema{"limit": contracts.Int(1, 100)}), Output: contracts.Object(map[string]*contracts.Schema{"failures": {Type: "array", Items: failure}}, "failures"), Loggable: []string{"limit"}}
	replay := contracts.Definition{Layer: contracts.LayerAtomic, Name: "memory.ingestion.replay", Version: "1.0.0", Title: "Replay memory ingestion", Description: "Requeue a failed or completed source event within the current tenant.", Domain: "memory", Entity: "memory_event_inbox", Permission: "memory:replay", Risk: contracts.RiskHigh, Approval: contracts.ApprovalConfirm, Tags: []string{"manual-only"}, Executor: "go:memory.ingestion.replay", Idempotency: contracts.Idempotency{Required: true, Scope: "tenant+operation+key"}, SuccessEvents: []string{"memory.ingestion.requeued"}, Input: contracts.Object(map[string]*contracts.Schema{"eventId": contracts.Str(1, 256)}, "eventId"), Output: contracts.Object(map[string]*contracts.Schema{"queued": {Type: "boolean"}}, "queued")}
	replayRange := contracts.Definition{Layer: contracts.LayerAtomic, Name: "memory.ingestion.replay_range", Version: "1.0.0", Title: "Replay a bounded ingestion range", Description: "Requeue up to 100 completed or failed source events in an inclusive source ID range within the current tenant.", Domain: "memory", Entity: "memory_event_inbox", Permission: "memory:replay", Risk: contracts.RiskHigh, Approval: contracts.ApprovalConfirm, Tags: []string{"manual-only"}, Executor: "go:memory.ingestion.replay_range", Idempotency: contracts.Idempotency{Required: true, Scope: "tenant+operation+key"}, SuccessEvents: []string{"memory.ingestion.range_requeued"}, Input: contracts.Object(map[string]*contracts.Schema{"firstSourceId": id, "lastSourceId": id, "limit": contracts.Int(1, 100)}, "firstSourceId", "lastSourceId", "limit"), Output: contracts.Object(map[string]*contracts.Schema{"queued": contracts.Int(0, 100), "lastQueuedSourceId": contracts.Int(0, 1<<53)}, "queued", "lastQueuedSourceId"), Loggable: []string{"firstSourceId", "lastSourceId", "limit"}}
	policy := contracts.Object(map[string]*contracts.Schema{"partition": partition, "retentionDays": contracts.Int(0, 36500), "revision": contracts.Int(1, 1<<53)}, "partition", "retentionDays", "revision")
	policyList := contracts.Definition{Layer: contracts.LayerAtomic, Name: "memory.policy.list", Version: "1.0.0", Title: "List memory retention policies", Description: "List configured policies for the current tenant; no automatic cleanup is performed.", Domain: "memory", Entity: "memory_partition_policy", Entities: []string{"memory_partition_policy", "memory_access_audit"}, Permission: "memory:manage", Risk: contracts.RiskLow, Approval: contracts.ApprovalAuto, ReadOnly: true, Tags: []string{"manual-only"}, Executor: "go:memory.policy.list", Input: contracts.Object(nil), Output: contracts.Object(map[string]*contracts.Schema{"policies": {Type: "array", Items: policy}}, "policies")}
	policySet := contracts.Definition{Layer: contracts.LayerAtomic, Name: "memory.policy.set", Version: "1.0.0", Title: "Set a memory retention policy", Description: "Record a tenant partition retention policy; no automatic cleanup is performed.", Domain: "memory", Entity: "memory_partition_policy", Permission: "memory:manage", Risk: contracts.RiskHigh, Approval: contracts.ApprovalConfirm, Tags: []string{"manual-only"}, Executor: "go:memory.policy.set", Idempotency: contracts.Idempotency{Required: true, Scope: "tenant+operation+key"}, SuccessEvents: []string{"memory.policy.updated"}, Input: contracts.Object(map[string]*contracts.Schema{"partition": partition, "retentionDays": contracts.Int(0, 36500), "expectedRevision": contracts.Int(0, 1<<53)}, "partition", "retentionDays", "expectedRevision"), Output: policy, Loggable: []string{"partition", "retentionDays", "expectedRevision"}}
	return []contracts.Definition{create, correct, archive, remove, get, list, sources, collect, events, failures, replay, replayRange, policyList, policySet}
}

func addListFilterSchema(input *contracts.Schema) {
	input.Properties["ownerId"] = contracts.Int(1, 1<<53)
	input.Properties["status"] = &contracts.Schema{Type: "string", Enum: []string{"active", "archived"}}
	input.Properties["createdFrom"] = contracts.Str(1, 64)
	input.Properties["createdBefore"] = contracts.Str(1, 64)
}

func addValiditySchema(input *contracts.Schema) {
	input.Properties["validFrom"] = contracts.Str(1, 64)
	input.Properties["validTo"] = contracts.Str(1, 64)
}

func itemSchema() *contracts.Schema {
	return contracts.Object(map[string]*contracts.Schema{"id": contracts.Int(1, 1<<53), "partition": &contracts.Schema{Type: "string", Enum: []string{"agent", "activity", "exception", "business", "knowledge", "execution"}}, "memoryType": contracts.Str(1, 100), "title": contracts.Str(1, 200), "summary": contracts.Str(0, 4000), "status": contracts.Str(1, 20), "verificationStatus": contracts.Str(1, 20), "sensitivity": contracts.Str(1, 20), "validFrom": contracts.Str(1, 64), "validTo": contracts.Str(0, 64), "version": contracts.Int(1, 1<<53)}, "id", "partition", "memoryType", "title", "summary", "status", "verificationStatus", "sensitivity", "validFrom", "validTo", "version")
}
func mutationSchema() *contracts.Schema {
	return contracts.Object(map[string]*contracts.Schema{"id": contracts.Int(1, 1<<53), "version": contracts.Int(1, 1<<53), "status": contracts.Str(1, 20)}, "id", "version", "status")
}
