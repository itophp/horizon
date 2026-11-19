-- name: MailMailboxCreate :one
INSERT INTO mail_mailboxes(tenant_id,owner_account_id,name,email_address,default_send)
VALUES($1,$2,$3,$4,$5) RETURNING id;

-- name: MailTenants :many
SELECT DISTINCT tenant_id FROM mail_mailboxes ORDER BY tenant_id;

-- name: MailMailboxUpdate :execrows
UPDATE mail_mailboxes SET name=$1,email_address=$2,default_send=$3,updated_at=now(),revision=revision+1
WHERE tenant_id=$4 AND id=$5 AND owner_account_id=$6 AND revision=$7;

-- name: MailMailboxStatusUpdate :execrows
UPDATE mail_mailboxes SET status=$1,updated_at=now(),revision=revision+1
WHERE tenant_id=$2 AND id=$3 AND owner_account_id=$4 AND status=ANY($5::text[]);

-- name: MailMailboxDefaultClear :execrows
UPDATE mail_mailboxes SET default_send=false,updated_at=now(),revision=revision+1
WHERE tenant_id=$1 AND owner_account_id=$2 AND default_send;

-- name: MailMailboxList :many
SELECT id,name,email_address,default_send,status,revision,created_at,updated_at
FROM mail_mailboxes WHERE tenant_id=$1 AND owner_account_id=$2
ORDER BY default_send DESC,id DESC LIMIT 100;

-- name: MailMailboxGet :one
SELECT id,name,email_address,default_send,status,revision,created_at,updated_at
FROM mail_mailboxes WHERE tenant_id=$1 AND id=$2 AND owner_account_id=$3;

-- name: MailCredentialCreate :one
INSERT INTO mail_credentials(tenant_id,mailbox_id,purpose,ciphertext,fingerprint)
VALUES($1,$2,$3,$4,$5) RETURNING id,version,fingerprint,rotated_at;

-- name: MailCredentialReplace :execrows
UPDATE mail_credentials SET ciphertext=$1,fingerprint=$2,version=version+1,rotated_at=now()
WHERE tenant_id=$3 AND mailbox_id=$4 AND purpose=$5;

-- name: MailChannelCreate :execrows
INSERT INTO mail_channels(tenant_id,mailbox_id,direction,protocol,host,port,tls_mode,username,credential_id)
VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9)
ON CONFLICT (tenant_id,mailbox_id,direction) DO UPDATE SET
 protocol=EXCLUDED.protocol,host=EXCLUDED.host,port=EXCLUDED.port,tls_mode=EXCLUDED.tls_mode,
 username=EXCLUDED.username,credential_id=EXCLUDED.credential_id,verification_status='unknown',
 last_error_code='',last_verified_at=NULL,updated_at=now();

-- name: MailChannelsList :many
SELECT c.id,c.direction,c.protocol,c.host,c.port,c.tls_mode,c.username,c.verification_status,
 c.last_error_code,c.last_verified_at,c.next_sync_at,
 cr.id AS credential_id,cr.ciphertext,cr.version,cr.fingerprint
FROM mail_channels c JOIN mail_credentials cr ON cr.tenant_id=c.tenant_id AND cr.id=c.credential_id
JOIN mail_mailboxes m ON m.tenant_id=c.tenant_id AND m.id=c.mailbox_id
WHERE c.tenant_id=$1 AND c.mailbox_id=$2 AND m.owner_account_id=$3 ORDER BY c.direction;

-- name: MailChannelSendGet :one
SELECT m.id AS mailbox_id,m.email_address,m.owner_account_id,c.host,c.port,c.tls_mode,c.username,
 cr.ciphertext,cr.version
FROM mail_mailboxes m JOIN mail_channels c ON c.tenant_id=m.tenant_id AND c.mailbox_id=m.id AND c.direction='send'
JOIN mail_credentials cr ON cr.tenant_id=c.tenant_id AND cr.id=c.credential_id
WHERE m.tenant_id=$1 AND m.id=$2 AND m.owner_account_id=$3 AND m.status='active';

-- name: MailChannelVerificationUpdate :execrows
UPDATE mail_channels SET verification_status=$1,last_error_code=$2,last_verified_at=now(),updated_at=now()
WHERE tenant_id=$3 AND mailbox_id=$4 AND direction=$5;

-- name: MailReceiveDue :many
SELECT m.id AS mailbox_id,m.tenant_id,m.owner_account_id,m.email_address,c.id AS channel_id,
 a.organization_id AS owner_organization_id,a.department_id AS owner_department_id,
 c.host,c.port,c.username,cr.ciphertext,cr.version
FROM mail_channels c JOIN mail_mailboxes m ON m.tenant_id=c.tenant_id AND m.id=c.mailbox_id
JOIN iam_accounts a ON a.tenant_id=m.tenant_id AND a.id=m.owner_account_id
JOIN mail_credentials cr ON cr.tenant_id=c.tenant_id AND cr.id=c.credential_id
WHERE c.tenant_id=$1 AND c.id=$2 AND c.direction='receive' AND c.protocol='pop3'
 AND m.status='active' AND (c.next_sync_at IS NULL OR c.next_sync_at<=now())
 AND (c.lease_until IS NULL OR c.lease_until<now());

-- name: MailReceiveDueIDs :many
SELECT c.id FROM mail_channels c JOIN mail_mailboxes m ON m.tenant_id=c.tenant_id AND m.id=c.mailbox_id
WHERE c.tenant_id=$1 AND c.direction='receive' AND c.protocol='pop3' AND m.status='active'
 AND (c.next_sync_at IS NULL OR c.next_sync_at<=now())
 AND (c.lease_until IS NULL OR c.lease_until<now())
ORDER BY c.next_sync_at NULLS FIRST,c.id LIMIT 20;

-- name: MailReceiveLease :execrows
UPDATE mail_channels SET lease_owner=$1,lease_until=now()+interval '5 minutes',updated_at=now()
WHERE tenant_id=$2 AND id=$3 AND direction='receive'
 AND (lease_until IS NULL OR lease_until<now());

-- name: MailReceiveRelease :execrows
UPDATE mail_channels SET next_sync_at=$1,lease_until=NULL,lease_owner='',updated_at=now()
WHERE tenant_id=$2 AND id=$3 AND direction='receive';

-- name: MailReceiveFail :execrows
UPDATE mail_channels SET last_error_code=$1,next_sync_at=$2,verification_status='failed',lease_until=NULL,
 lease_owner='',updated_at=now()
WHERE tenant_id=$3 AND id=$4 AND direction='receive';

-- name: MailMessageCreate :one
INSERT INTO mail_messages(tenant_id,mailbox_id,owner_account_id,direction,folder,provider_uid,
 rfc_message_id,subject,from_address,reply_to,body_text,body_html,snippet,occurred_at)
VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)
ON CONFLICT (tenant_id,mailbox_id,direction,provider_uid)
 DO UPDATE SET updated_at=mail_messages.updated_at
RETURNING id,created_at,(xmax = 0) AS inserted;

-- name: MailMessageList :many
SELECT id,mailbox_id,direction,folder,subject,from_address,snippet,occurred_at,read_at,created_at
FROM mail_messages WHERE tenant_id=$1 AND owner_account_id=$2 AND folder=ANY($3::text[])
 AND ($4::bigint=0 OR mailbox_id=$4)
ORDER BY occurred_at DESC LIMIT $5 OFFSET $6;

-- name: MailMessageCount :one
SELECT count(*)::bigint FROM mail_messages
WHERE tenant_id=$1 AND owner_account_id=$2 AND folder=ANY($3::text[])
 AND ($4::bigint=0 OR mailbox_id=$4);

-- name: MailMessageGet :one
SELECT id,mailbox_id,direction,folder,provider_uid,rfc_message_id,subject,from_address,reply_to,
 body_text,body_html,snippet,occurred_at,read_at,created_at
FROM mail_messages WHERE tenant_id=$1 AND id=$2 AND owner_account_id=$3;

-- name: MailMessageUIDExists :one
SELECT EXISTS(SELECT 1 FROM mail_messages
 WHERE tenant_id=$1 AND mailbox_id=$2 AND direction='inbound' AND provider_uid=$3) AS exists;

-- name: MailMessageMarkRead :execrows
UPDATE mail_messages SET read_at=now(),updated_at=now()
WHERE tenant_id=$1 AND id=$2 AND owner_account_id=$3 AND read_at IS NULL;

-- name: MailMessageRecipientsAdd :exec
INSERT INTO mail_message_recipients(tenant_id,message_id,kind,address,display_name) VALUES($1,$2,$3,$4,$5);

-- name: MailMessageRecipientsList :many
SELECT kind,address,display_name FROM mail_message_recipients
WHERE tenant_id=$1 AND message_id=$2 ORDER BY kind,address;

-- name: MailMessageAttachmentAdd :execrows
INSERT INTO mail_message_attachments(tenant_id,message_id,file_id,content_id,filename,mime_type,size,disposition)
VALUES($1,$2,$3,$4,$5,$6,$7,$8)
ON CONFLICT (tenant_id,message_id,file_id) DO NOTHING;

-- name: MailMessageAttachmentsList :many
SELECT id,file_id,content_id,filename,mime_type,size,disposition
FROM mail_message_attachments WHERE tenant_id=$1 AND message_id=$2 ORDER BY id;

-- name: MailAttachmentFile :one
SELECT id,owner_id,organization_id,department_id,storage_profile_id,storage_key,name,mime,size,purpose,sha256,created_at
FROM gov_files WHERE tenant_id=$1 AND id=$2 AND owner_id=$3 AND purpose='mail_attachment' AND deleted_at IS NULL;

-- name: MailDeliveryCreate :one
INSERT INTO mail_outbound_deliveries(tenant_id,mailbox_id,message_id,submitted_by)
VALUES($1,$2,$3,$4) RETURNING id,status;

-- name: MailDeliveryList :many
SELECT d.id,d.mailbox_id,d.message_id,d.status,d.attempts,d.next_attempt_at,d.error_code,d.sent_at,d.created_at,
 m.subject,m.from_address
FROM mail_outbound_deliveries d JOIN mail_messages m ON m.tenant_id=d.tenant_id AND m.id=d.message_id
WHERE d.tenant_id=$1 AND d.submitted_by=$2 ORDER BY d.created_at DESC LIMIT $3;

-- name: MailDeliveryRetry :execrows
UPDATE mail_outbound_deliveries SET status='queued',error_code='',next_attempt_at=now(),updated_at=now()
WHERE tenant_id=$1 AND id=$2 AND submitted_by=$3 AND status IN ('failed','ambiguous') AND attempts < $4;

-- name: MailDeliveryCancel :execrows
UPDATE mail_outbound_deliveries SET status='cancelled',updated_at=now()
WHERE tenant_id=$1 AND id=$2 AND submitted_by=$3 AND status='queued';

-- name: MailOutboundDueIDs :many
SELECT d.id FROM mail_outbound_deliveries d
JOIN mail_mailboxes m ON m.tenant_id=d.tenant_id AND m.id=d.mailbox_id
WHERE d.tenant_id=$1 AND d.status='queued' AND d.next_attempt_at<=now() AND m.status='active'
ORDER BY d.next_attempt_at,d.id LIMIT $2;

-- name: MailOutboundGet :one
SELECT d.id AS delivery_id,d.tenant_id,d.mailbox_id,d.message_id,d.attempts,
 m.owner_account_id,m.provider_uid,m.rfc_message_id,m.subject,m.from_address,m.reply_to,m.body_text,m.body_html,
 ch.host,ch.port,ch.tls_mode,ch.username,cr.ciphertext,cr.version
FROM mail_outbound_deliveries d JOIN mail_messages m ON m.tenant_id=d.tenant_id AND m.id=d.message_id
JOIN mail_channels ch ON ch.tenant_id=d.tenant_id AND ch.mailbox_id=d.mailbox_id AND ch.direction='send'
JOIN mail_credentials cr ON cr.tenant_id=ch.tenant_id AND cr.id=ch.credential_id
WHERE d.tenant_id=$1 AND d.id=$2 AND d.status IN ('queued','failed','ambiguous');

-- name: MailOutboundRecipients :many
SELECT kind,address FROM mail_message_recipients
WHERE tenant_id=$1 AND message_id=$2 AND kind=ANY($3::text[]);

-- name: MailOutboundStart :execrows
UPDATE mail_outbound_deliveries SET lease_owner=$1,status='sending',attempts=attempts+1,
 lease_until=now()+interval '5 minutes',error_code='',updated_at=now()
WHERE tenant_id=$2 AND id=$3 AND status IN ('queued','failed','ambiguous');

-- name: MailOutboundFinishSent :execrows
UPDATE mail_outbound_deliveries SET status='sent',error_code='',sent_at=now(),lease_until=NULL,lease_owner='',
 next_attempt_at=now(),updated_at=now() WHERE tenant_id=$1 AND id=$2 AND status='sending';

-- name: MailOutboundFinishRetryable :execrows
UPDATE mail_outbound_deliveries SET status=$1,error_code=$2,next_attempt_at=$3,lease_until=NULL,
 lease_owner='',updated_at=now() WHERE tenant_id=$4 AND id=$5 AND status='sending';

-- name: MailInboxSyncCreate :one
INSERT INTO mail_inbox_syncs(tenant_id,mailbox_id) VALUES($1,$2) RETURNING id;

-- name: MailInboxSyncFinish :execrows
UPDATE mail_inbox_syncs SET status=$1,fetched_count=$2,imported_count=$3,duplicate_count=$4,
 error_code=$5,finished_at=now() WHERE tenant_id=$6 AND id=$7 AND mailbox_id=$8 AND status='running';
