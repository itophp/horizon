package applicationruntime

import "baseline/services/api/kernel/capability/contracts"

func RuntimeRunDependencyEntity() contracts.Entity {
	return contracts.Entity{Name: "runtime_run_dependency", Domain: "runtime", Table: "runtime_run_dependencies", Key: "runId", Attributes: []contracts.Attribute{{Name: "tenantId", Kind: contracts.AttributeSystem, Type: contracts.TypeString, Required: true}, {Name: "runId", Kind: contracts.AttributeSystem, Type: contracts.TypeInteger, Required: true}, {Name: "pluginId", Kind: contracts.AttributeReference, Type: contracts.TypeString, Required: true}}, Relations: []contracts.Relation{{Name: "installation", Target: "runtime_installation", Kind: "one", Via: "plugin_id", Required: true}}}
}
