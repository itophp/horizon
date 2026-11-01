package notifications

import "baseline/services/api/internal/platform/capability/contracts"

// notificationEntity mirrors gov_notifications: the recipient's copy of a message.
// read_at carries the read state; there is no status column, so no state machine.
func notificationEntity() contracts.Entity {
	return contracts.Entity{
		Name:        "notification",
		Domain:      "notifications",
		Table:       "gov_notifications",
		Description: "One inbox message for one recipient, with an optional read timestamp.",
		Key:         "id",
		Attributes: []contracts.Attribute{
			{Name: "id", Kind: contracts.AttributeSystem, Type: contracts.TypeInteger, Required: true},
			{Name: "eventId", Kind: contracts.AttributeSystem, Type: contracts.TypeInteger, Nullable: true},
			{Name: "recipientId", Kind: contracts.AttributeReference, Type: contracts.TypeReference, Required: true},
			{Name: "departmentId", Kind: contracts.AttributeReference, Type: contracts.TypeReference, Required: true},
			{Name: "payload", Kind: contracts.AttributeSystem, Type: contracts.TypeJSON, Shape: "object", Required: true},
			{Name: "readAt", Kind: contracts.AttributeSystem, Type: contracts.TypeTimestamp, Nullable: true},
			{Name: "announcementId", Kind: contracts.AttributeReference, Type: contracts.TypeReference, Nullable: true},
		},
		Relations: []contracts.Relation{
			{Name: "recipient", Target: "account", Kind: "one", Via: "recipient_id", Required: true},
			{Name: "announcement", Target: "announcement", Kind: "one", Via: "announcement_id", Required: false},
		},
	}
}
