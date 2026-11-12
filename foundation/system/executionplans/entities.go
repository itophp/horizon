package executionplans

import "horizon/foundation/kernel/capability/contracts"

func Entities() []contracts.Entity {
	return []contracts.Entity{execution_planEntity(), execution_bindingEntity(), execution_binding_changeEntity(), execution_recordEntity()}
}
