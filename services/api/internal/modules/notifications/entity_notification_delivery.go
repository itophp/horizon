package notifications

import "baseline/services/api/internal/platform/capability/contracts"

func deliveryEntity() contracts.Entity {
	return contracts.Entity{
		Name:        "notification_delivery",
		Domain:      "notifications",
		Table:       "gov_deliveries",
		Description: "One channel attempt for one recipient of an announcement.",
		Key:         "id",
		Attributes: []contracts.Attribute{
			{Name: "id", Kind: contracts.AttributeSystem, Type: contracts.TypeInteger, Required: true},
			{Name: "messageId", Kind: contracts.AttributeReference, Type: contracts.TypeReference, Required: true,
				Description: "Announcement message this delivery belongs to."},
			{Name: "channel", Kind: contracts.AttributeBusiness, Type: contracts.TypeString, Required: true,
				Description: "in-app or email; validated by the sender, not by a CHECK."},
			{Name: "status", Kind: contracts.AttributeState, Type: contracts.TypeEnum, Required: true,
				Enum: []string{"pending", "processing", "sent", "failed", "skipped"}},
			{Name: "attempts", Kind: contracts.AttributeSystem, Type: contracts.TypeInteger, Required: true,
				Description: "Claim count; the relay stops after five."},
			{Name: "errorCode", Kind: contracts.AttributeSystem, Type: contracts.TypeString, Required: true,
				Sensitive: true, Description: "Stable failure code; never the upstream message."},
			{Name: "nextAttemptAt", Kind: contracts.AttributeSystem, Type: contracts.TypeTimestamp, Required: true},
			{Name: "leaseUntil", Kind: contracts.AttributeSystem, Type: contracts.TypeTimestamp, Nullable: true,
				Description: "Lease held by the worker that claimed the row."},
		},
		Relations: []contracts.Relation{
			{Name: "message", Target: "notification", Kind: "one", Via: "message_id", Required: true},
		},
		States: []contracts.State{
			{Name: "pending"},
			{Name: "processing"},
			{Name: "sent", Terminal: true},
			{Name: "failed", Terminal: true},
			{Name: "skipped", Terminal: true},
		},
		Transitions: []contracts.Transition{
			{From: []string{"pending"}, To: "processing"},
			{From: []string{"processing"}, To: "sent"},
			{From: []string{"processing"}, To: "failed"},
			{From: []string{"pending"}, To: "skipped"},
		},
	}
}
