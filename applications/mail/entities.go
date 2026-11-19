package mail

import "horizon/foundation/kernel/capability/contracts"

// Entities describes the personal external-mail tables for the schema checker.
func Entities() []contracts.Entity {
	return []contracts.Entity{
		mailboxEntity(),
		credentialEntity(),
		channelEntity(),
		messageEntity(),
		messageRecipientEntity(),
		messageAttachmentEntity(),
		outboundDeliveryEntity(),
		inboxSyncEntity(),
	}
}
