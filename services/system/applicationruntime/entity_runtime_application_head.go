package applicationruntime

import "baseline/services/api/kernel/capability/contracts"

func RuntimeApplicationHeadEntity() contracts.Entity {
	return contracts.Entity{Name: "runtime_application_head", Domain: "runtime", Table: "runtime_application_heads", Key: "applicationId", Attributes: []contracts.Attribute{
		{Name: "routeRoot", Kind: contracts.AttributeSystem, Type: contracts.TypeString, Required: true},
		{Name: "tenantId", Kind: contracts.AttributeSystem, Type: contracts.TypeString, Required: true},
		{Name: "applicationId", Kind: contracts.AttributeSystem, Type: contracts.TypeString, Required: true},
		{Name: "pluginId", Kind: contracts.AttributeReference, Type: contracts.TypeString, Required: true},
		{Name: "publishedVersion", Kind: contracts.AttributeSystem, Type: contracts.TypeString, Required: true},
		{Name: "proposalVersion", Kind: contracts.AttributeSystem, Type: contracts.TypeString, Required: true},
		{Name: "baseDigest", Kind: contracts.AttributeReference, Type: contracts.TypeString, Required: true},
		{Name: "revision", Kind: contracts.AttributeSystem, Type: contracts.TypeInteger, Required: true},
		{Name: "nextConfiguration", Kind: contracts.AttributeSystem, Type: contracts.TypeInteger, Required: true},
		{Name: "compatibility", Kind: contracts.AttributeSystem, Type: contracts.TypeEnum, Required: true, Enum: []string{"compatible", "incompatible", "pending_rebase"}},
	}, Relations: []contracts.Relation{{Name: "installation", Target: "runtime_installation", Kind: "one", Via: "plugin_id", Required: true}, {Name: "baseline", Target: "runtime_deployment_snapshot", Kind: "one", Via: "base_digest", Required: true}}}
}
