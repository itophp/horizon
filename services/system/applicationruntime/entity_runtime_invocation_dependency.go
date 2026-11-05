package applicationruntime

import "baseline/services/api/kernel/capability/contracts"

func RuntimeInvocationDependencyEntity() contracts.Entity {
	return contracts.Entity{Name: "runtime_invocation_dependency", Domain: "runtime", Table: "runtime_invocation_dependencies", Key: "invocationId", Attributes: []contracts.Attribute{{Name: "tenantId", Kind: contracts.AttributeSystem, Type: contracts.TypeString, Required: true}, {Name: "invocationId", Kind: contracts.AttributeSystem, Type: contracts.TypeString, Required: true}, {Name: "pluginId", Kind: contracts.AttributeReference, Type: contracts.TypeString, Required: true}}, Relations: []contracts.Relation{{Name: "installation", Target: "runtime_installation", Kind: "one", Via: "plugin_id", Required: true}}}
}
