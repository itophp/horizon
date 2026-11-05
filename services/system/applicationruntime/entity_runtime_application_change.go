package applicationruntime

import "baseline/services/api/kernel/capability/contracts"

func RuntimeApplicationChangeEntity() contracts.Entity {
	return contracts.Entity{Name: "runtime_application_change", Domain: "runtime", Table: "runtime_application_changes", Key: "id", Attributes: []contracts.Attribute{
		{Name: "id", Kind: contracts.AttributeSystem, Type: contracts.TypeInteger, Required: true},
		{Name: "tenantId", Kind: contracts.AttributeSystem, Type: contracts.TypeString, Required: true},
		{Name: "applicationId", Kind: contracts.AttributeReference, Type: contracts.TypeString, Required: true},
		{Name: "action", Kind: contracts.AttributeSystem, Type: contracts.TypeString, Required: true},
		{Name: "version", Kind: contracts.AttributeSystem, Type: contracts.TypeString, Required: true},
		{Name: "digest", Kind: contracts.AttributeSystem, Type: contracts.TypeString, Required: true},
		{Name: "diagnosticCode", Kind: contracts.AttributeSystem, Type: contracts.TypeString, Required: true},
		{Name: "accountId", Kind: contracts.AttributeSystem, Type: contracts.TypeInteger, Required: true},
		{Name: "correlationId", Kind: contracts.AttributeSystem, Type: contracts.TypeString, Required: true},
		{Name: "createdAt", Kind: contracts.AttributeSystem, Type: contracts.TypeTimestamp, Required: true},
	}, Relations: []contracts.Relation{{Name: "application", Target: "runtime_application_head", Kind: "one", Via: "application_id", Required: true}}}
}
