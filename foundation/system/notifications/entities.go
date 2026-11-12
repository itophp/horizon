package notifications

import "horizon/foundation/kernel/capability/contracts"

// Entities lists the notification objects: what is sent, and what each recipient
// receives.
func Entities() []contracts.Entity {
	return []contracts.Entity{announcementEntity(), notificationEntity(), deliveryEntity()}
}
