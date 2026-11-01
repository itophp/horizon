package settings

import "baseline/services/api/internal/platform/capability/contracts"

// Entities lists the settings objects: the published values, the drafts that
// stage changes, the schedules that publish them, and the versions they produce.
func Entities() []contracts.Entity {
	return []contracts.Entity{
		settingsEntity(),
		settingsDraftEntity(),
		settingsScheduleEntity(),
		settingsVersionEntity(),
	}
}
