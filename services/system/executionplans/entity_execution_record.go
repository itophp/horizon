package executionplans

import "baseline/services/api/kernel/capability/contracts"

func execution_recordEntity() contracts.Entity {
	return contracts.Entity{Name: "execution_record", Domain: "executionplans", Table: "cap_execution_records", Key: "invocationId", Attributes: []contracts.Attribute{
		{Name: "runId", Kind: contracts.AttributeReference, Type: contracts.TypeInteger, Required: false},
		{Name: "invocationId", Kind: contracts.AttributeSystem, Type: contracts.TypeString, Required: true},
		{Name: "accountId", Kind: contracts.AttributeSystem, Type: contracts.TypeInteger, Required: true},
		{Name: "binding", Kind: contracts.AttributeSystem, Type: contracts.TypeJSON, Required: true, Shape: "object"},
	}}
}
