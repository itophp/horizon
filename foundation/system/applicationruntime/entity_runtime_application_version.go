package applicationruntime

import "horizon/foundation/kernel/capability/contracts"

func RuntimeApplicationVersionEntity() contracts.Entity {
	return contracts.Entity{Name: "runtime_application_version", Domain: "runtime", Table: "runtime_application_versions", Key: "version", Attributes: []contracts.Attribute{
		{Name: "tenantId", Kind: contracts.AttributeSystem, Type: contracts.TypeString, Required: true},
		{Name: "applicationId", Kind: contracts.AttributeReference, Type: contracts.TypeString, Required: true},
		{Name: "version", Kind: contracts.AttributeSystem, Type: contracts.TypeString, Required: true},
		{Name: "parentVersion", Kind: contracts.AttributeSystem, Type: contracts.TypeString, Required: true},
		{Name: "baseDigest", Kind: contracts.AttributeReference, Type: contracts.TypeString, Required: true},
		{Name: "definition", Kind: contracts.AttributeSystem, Type: contracts.TypeJSON, Required: true, Shape: "object"},
		{Name: "digest", Kind: contracts.AttributeSystem, Type: contracts.TypeString, Required: true},
		{Name: "source", Kind: contracts.AttributeSystem, Type: contracts.TypeEnum, Required: true, Enum: []string{"deployment", "human", "agent", "rollback", "rebase"}},
		{Name: "createdBy", Kind: contracts.AttributeSystem, Type: contracts.TypeInteger, Required: true},
		{Name: "agentRunId", Kind: contracts.AttributeSystem, Type: contracts.TypeInteger, Required: false},
		{Name: "correlationId", Kind: contracts.AttributeSystem, Type: contracts.TypeString, Required: true},
		{Name: "createdAt", Kind: contracts.AttributeSystem, Type: contracts.TypeTimestamp, Required: true},
	}, Relations: []contracts.Relation{{Name: "application", Target: "runtime_application_head", Kind: "one", Via: "application_id", Required: true}, {Name: "baseline", Target: "runtime_deployment_snapshot", Kind: "one", Via: "base_digest", Required: true}}}
}
