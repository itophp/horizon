package projects

import "baseline/services/api/kernel/capability/contracts"

// Entities lists the objects this module owns. The composition root and the
// entity check read this same function.
func Entities() []contracts.Entity {
	return []contracts.Entity{
		projectEntity(),
		projectMemberEntity(),
		projectPlanEntity(),
		projectTaskEntity(),
		projectTaskBriefEntity(),
	}
}
