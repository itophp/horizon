package applicationruntime

import "horizon/foundation/kernel/capability/contracts"

func RuntimeDeploymentSnapshotEntity() contracts.Entity {
	return contracts.Entity{Name: "runtime_deployment_snapshot", Domain: "runtime", Table: "runtime_deployment_snapshots", Key: "digest", Attributes: []contracts.Attribute{
		{Name: "digest", Kind: contracts.AttributeSystem, Type: contracts.TypeString, Required: true},
		{Name: "pluginId", Kind: contracts.AttributeSystem, Type: contracts.TypeString, Required: true},
		{Name: "pluginVersion", Kind: contracts.AttributeSystem, Type: contracts.TypeString, Required: true},
		{Name: "applicationId", Kind: contracts.AttributeSystem, Type: contracts.TypeString, Required: true},
		{Name: "baseVersion", Kind: contracts.AttributeSystem, Type: contracts.TypeString, Required: true},
		{Name: "definition", Kind: contracts.AttributeSystem, Type: contracts.TypeJSON, Required: true, Shape: "object"},
		{Name: "contractsDigest", Kind: contracts.AttributeSystem, Type: contracts.TypeString, Required: true},
		{Name: "createdAt", Kind: contracts.AttributeSystem, Type: contracts.TypeTimestamp, Required: true},
	}}
}
