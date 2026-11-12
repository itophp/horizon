package applicationruntime

import "horizon/foundation/kernel/capability/contracts"

func Entities() []contracts.Entity {
	return []contracts.Entity{RuntimeDeploymentSnapshotEntity(), RuntimeApplicationHeadEntity(), RuntimeApplicationVersionEntity(), RuntimeApplicationChangeEntity(), RuntimeInstallationEntity(), RuntimeApplicationStateEntity(), RuntimeInstallationHistoryEntity(), RuntimeInstallationTemplateEntity(), AiRunEventEntity(), RuntimeRunDependencyEntity(), RuntimeInvocationDependencyEntity()}
}
