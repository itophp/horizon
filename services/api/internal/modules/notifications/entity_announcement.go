package notifications

import "baseline/services/api/internal/platform/capability/contracts"

// announcementEntity mirrors gov_announcements: the sender's side of one broadcast.
func announcementEntity() contracts.Entity {
	return contracts.Entity{
		Name:        "announcement",
		Domain:      "notifications",
		Table:       "gov_announcements",
		Description: "One announcement sent to a set of audiences, with its delivery summary.",
		Key:         "id",
		Attributes: []contracts.Attribute{
			{Name: "id", Kind: contracts.AttributeSystem, Type: contracts.TypeInteger, Required: true},
			{Name: "senderId", Kind: contracts.AttributeReference, Type: contracts.TypeReference, Required: true},
			{Name: "departmentId", Kind: contracts.AttributeReference, Type: contracts.TypeReference, Required: true},
			{Name: "clientKey", Kind: contracts.AttributeSystem, Type: contracts.TypeString, Required: true,
				Description: "Idempotency key supplied by the sender."},
			{Name: "payloadHash", Kind: contracts.AttributeSystem, Type: contracts.TypeString, Required: true},
			{Name: "title", Kind: contracts.AttributeBusiness, Type: contracts.TypeString, Required: true},
			{Name: "markdown", Kind: contracts.AttributeBusiness, Type: contracts.TypeString, Required: true},
			{Name: "targets", Kind: contracts.AttributeSystem, Type: contracts.TypeJSON, Shape: "array", Required: true,
				Description: "Audience selectors frozen at send time."},
			{Name: "email", Kind: contracts.AttributeSystem, Type: contracts.TypeBoolean, Required: true},
			{Name: "recipientCount", Kind: contracts.AttributeSystem, Type: contracts.TypeInteger, Required: true},
		},
		Relations: []contracts.Relation{
			{Name: "sender", Target: "account", Kind: "one", Via: "sender_id", Required: true},
			{Name: "department", Target: "department", Kind: "one", Via: "department_id", Required: true},
		},
	}
}
