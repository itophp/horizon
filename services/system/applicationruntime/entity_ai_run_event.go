package applicationruntime

import "baseline/services/api/kernel/capability/contracts"

func AiRunEventEntity() contracts.Entity {
	return contracts.Entity{Name: "ai_run_event", Domain: "runtime", Table: "ai_run_events", Key: "runId", Attributes: []contracts.Attribute{
		{Name: "tenantId", Kind: contracts.AttributeSystem, Type: contracts.TypeString, Required: true},
		{Name: "runId", Kind: contracts.AttributeSystem, Type: contracts.TypeInteger, Required: true},
		{Name: "sequence", Kind: contracts.AttributeSystem, Type: contracts.TypeInteger, Required: true},
		{Name: "eventType", Kind: contracts.AttributeSystem, Type: contracts.TypeString, Required: true},
		{Name: "payload", Kind: contracts.AttributeSystem, Type: contracts.TypeJSON, Required: true, Shape: "object", Sensitive: true},
		{Name: "createdAt", Kind: contracts.AttributeSystem, Type: contracts.TypeTimestamp, Required: true},
	}}
}
