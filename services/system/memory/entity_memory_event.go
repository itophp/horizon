package memory

import "baseline/services/api/kernel/capability/contracts"

func memoryEventEntity() contracts.Entity {
	return contracts.Entity{Name: "memory_event", Domain: "memory", Table: "memory_events", Key: "id", Description: "Sanitized, tenant-scoped canonical observation.", Attributes: []contracts.Attribute{
		{Name: "id", Kind: contracts.AttributeSystem, Type: contracts.TypeInteger, Required: true},
		{Name: "tenantId", Kind: contracts.AttributeSystem, Type: contracts.TypeString, Required: true},
		{Name: "organizationId", Kind: contracts.AttributeSystem, Type: contracts.TypeString, Required: true},
		{Name: "eventId", Kind: contracts.AttributeSystem, Type: contracts.TypeString, Required: true},
		{Name: "eventType", Kind: contracts.AttributeSystem, Type: contracts.TypeString, Required: true},
		{Name: "eventVersion", Kind: contracts.AttributeSystem, Type: contracts.TypeString, Required: true},
		{Name: "occurredAt", Kind: contracts.AttributeSystem, Type: contracts.TypeTimestamp, Required: true},
		{Name: "ingestedAt", Kind: contracts.AttributeSystem, Type: contracts.TypeTimestamp, Required: true},
		{Name: "actorId", Kind: contracts.AttributeSystem, Type: contracts.TypeString, Required: true},
		{Name: "actorType", Kind: contracts.AttributeSystem, Type: contracts.TypeString, Required: true},
		{Name: "departmentId", Kind: contracts.AttributeSystem, Type: contracts.TypeInteger, Required: true},
		{Name: "sourceType", Kind: contracts.AttributeSystem, Type: contracts.TypeString, Required: true},
		{Name: "sourceId", Kind: contracts.AttributeSystem, Type: contracts.TypeString, Required: true},
		{Name: "sourceModule", Kind: contracts.AttributeSystem, Type: contracts.TypeString, Required: true},
		{Name: "resourceType", Kind: contracts.AttributeSystem, Type: contracts.TypeString, Required: true},
		{Name: "resourceId", Kind: contracts.AttributeSystem, Type: contracts.TypeString, Required: true},
		{Name: "actionName", Kind: contracts.AttributeSystem, Type: contracts.TypeString, Required: true},
		{Name: "result", Kind: contracts.AttributeSystem, Type: contracts.TypeString, Required: true},
		{Name: "invocationId", Kind: contracts.AttributeSystem, Type: contracts.TypeString, Required: true},
		{Name: "sessionId", Kind: contracts.AttributeSystem, Type: contracts.TypeString, Required: true},
		{Name: "taskId", Kind: contracts.AttributeSystem, Type: contracts.TypeString, Required: true},
		{Name: "workflowInstanceId", Kind: contracts.AttributeSystem, Type: contracts.TypeString, Required: true},
		{Name: "traceId", Kind: contracts.AttributeSystem, Type: contracts.TypeString, Required: true},
		{Name: "correlationId", Kind: contracts.AttributeSystem, Type: contracts.TypeString, Required: true},
		{Name: "causationId", Kind: contracts.AttributeSystem, Type: contracts.TypeString, Required: true},
		{Name: "triggerDepth", Kind: contracts.AttributeSystem, Type: contracts.TypeInteger, Required: true},
		{Name: "sensitivity", Kind: contracts.AttributeSystem, Type: contracts.TypeString, Required: true},
		{Name: "candidatePartitions", Kind: contracts.AttributeSystem, Type: contracts.TypeUnknown, SQLType: "text[]", Required: true},
		{Name: "payload", Kind: contracts.AttributeSystem, Type: contracts.TypeJSON, Shape: "object", Required: true, Sensitive: true},
	}}
}
