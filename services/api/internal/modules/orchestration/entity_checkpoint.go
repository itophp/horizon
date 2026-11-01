package orchestration

import "baseline/services/api/internal/platform/capability/contracts"

func Entities() []contracts.Entity {
	return []contracts.Entity{{Name: "orchestration_checkpoint", Domain: "orchestration", Table: "orch_checkpoints", Key: "runId", Attributes: []contracts.Attribute{
		{Name: "runId", Kind: contracts.AttributeSystem, Type: contracts.TypeInteger, Required: true},
		{Name: "checkpointKey", Kind: contracts.AttributeSystem, Type: contracts.TypeString, Required: true},
		{Name: "payload", Kind: contracts.AttributeSystem, Type: contracts.TypeString, Required: true, Sensitive: true},
		{Name: "leaseUntil", Kind: contracts.AttributeSystem, Type: contracts.TypeTimestamp, Required: true},
		{Name: "leaseToken", Kind: contracts.AttributeSystem, Type: contracts.TypeString, Required: true, Sensitive: true},
	}}}
}
