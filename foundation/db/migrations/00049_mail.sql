-- +goose Up
-- Personal external mailboxes. Messages and delivery records belong to the IAM
-- account that owns the mailbox; no organizational data scope can widen them.
CREATE TABLE mail_mailboxes (
 id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
 tenant_id text NOT NULL,
 owner_account_id bigint NOT NULL,
 name text NOT NULL CHECK (length(name) BETWEEN 1 AND 120),
 email_address text NOT NULL CHECK (length(email_address) BETWEEN 3 AND 254),
 default_send boolean NOT NULL DEFAULT false,
 status text NOT NULL DEFAULT 'unverified' CHECK (status IN ('unverified','active','disabled')),
 revision bigint NOT NULL DEFAULT 1 CHECK (revision > 0),
 created_at timestamptz NOT NULL DEFAULT now(),
 updated_at timestamptz NOT NULL DEFAULT now(),
 UNIQUE (tenant_id, id),
 CONSTRAINT mail_mailboxes_owner_fkey FOREIGN KEY (tenant_id, owner_account_id)
   REFERENCES gov_identity_refs(tenant_id, account_id)
);
CREATE UNIQUE INDEX mail_mailboxes_address ON mail_mailboxes(tenant_id, owner_account_id, lower(email_address));
CREATE UNIQUE INDEX mail_mailboxes_default_send ON mail_mailboxes (tenant_id, owner_account_id)
 WHERE default_send;
CREATE INDEX mail_mailboxes_owner ON mail_mailboxes (tenant_id, owner_account_id, id DESC);

-- Credentials are sealed outside PostgreSQL and are bound by additional
-- authenticated data to tenant, mailbox, purpose and version. Only the
-- fingerprint and timestamps are projected by capabilities.
CREATE TABLE mail_credentials (
 id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
 tenant_id text NOT NULL,
 mailbox_id bigint NOT NULL,
 purpose text NOT NULL CHECK (purpose IN ('smtp','pop3')),
 ciphertext text NOT NULL CHECK (length(ciphertext) BETWEEN 12 AND 20000),
 fingerprint text NOT NULL CHECK (length(fingerprint) = 16),
 version integer NOT NULL DEFAULT 1 CHECK (version > 0),
 created_at timestamptz NOT NULL DEFAULT now(),
 rotated_at timestamptz NOT NULL DEFAULT now(),
 last_used_at timestamptz,
 UNIQUE (tenant_id, id),
 UNIQUE (tenant_id, mailbox_id, purpose),
 FOREIGN KEY (tenant_id, mailbox_id) REFERENCES mail_mailboxes(tenant_id, id) ON DELETE CASCADE
);

-- v1 deliberately has fixed protocols. Keeping endpoint data in rows rather
-- than a JSON blob lets PostgreSQL enforce direction, protocol and ports while
-- later Graph/EWS adapters can add protocol values without changing ownership.
CREATE TABLE mail_channels (
 id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
 tenant_id text NOT NULL,
 mailbox_id bigint NOT NULL,
 direction text NOT NULL CHECK (direction IN ('send','receive')),
 protocol text NOT NULL CHECK (protocol IN ('smtp','pop3')),
 host text NOT NULL CHECK (length(host) BETWEEN 1 AND 253),
 port integer NOT NULL CHECK (port BETWEEN 1 AND 65535),
 tls_mode text NOT NULL CHECK (tls_mode IN ('starttls','tls')),
 username text NOT NULL CHECK (length(username) BETWEEN 0 AND 200),
 credential_id bigint NOT NULL,
 verification_status text NOT NULL DEFAULT 'unknown' CHECK (verification_status IN ('unknown','ok','failed')),
 last_error_code text NOT NULL DEFAULT '',
 last_verified_at timestamptz,
 next_sync_at timestamptz,
 lease_until timestamptz,
 lease_owner text NOT NULL DEFAULT '',
 created_at timestamptz NOT NULL DEFAULT now(),
 updated_at timestamptz NOT NULL DEFAULT now(),
 UNIQUE (tenant_id, id),
 UNIQUE (tenant_id, mailbox_id, direction),
 FOREIGN KEY (tenant_id, mailbox_id) REFERENCES mail_mailboxes(tenant_id, id) ON DELETE CASCADE,
 FOREIGN KEY (tenant_id, credential_id) REFERENCES mail_credentials(tenant_id, id),
 CHECK ((direction='send' AND protocol='smtp') OR (direction='receive' AND protocol='pop3' AND tls_mode='tls'))
);
CREATE INDEX mail_channels_due ON mail_channels(tenant_id, next_sync_at, id) WHERE direction='receive';

-- One normalized message row per inbound or outbound message. Provider UIDs
-- make POP3 at-least-once retrieval idempotent within one mailbox.
CREATE TABLE mail_messages (
 id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
 tenant_id text NOT NULL,
 mailbox_id bigint NOT NULL,
 owner_account_id bigint NOT NULL,
 direction text NOT NULL CHECK (direction IN ('inbound','outbound')),
 folder text NOT NULL CHECK (folder IN ('inbox','sent')),
 provider_uid text NOT NULL CHECK (length(provider_uid) BETWEEN 1 AND 255),
 rfc_message_id text NOT NULL DEFAULT '' CHECK (length(rfc_message_id) <= 998),
 subject text NOT NULL DEFAULT '' CHECK (length(subject) <= 998),
 from_address text NOT NULL DEFAULT '' CHECK (length(from_address) <= 320),
 reply_to text NOT NULL DEFAULT '' CHECK (length(reply_to) <= 320),
 body_text text NOT NULL CHECK (length(body_text) <= 26214400),
 body_html text NOT NULL DEFAULT '' CHECK (length(body_html) <= 26214400),
 snippet text NOT NULL DEFAULT '' CHECK (length(snippet) BETWEEN 0 AND 500),
 occurred_at timestamptz NOT NULL DEFAULT now(),
 read_at timestamptz,
 created_at timestamptz NOT NULL DEFAULT now(),
 updated_at timestamptz NOT NULL DEFAULT now(),
 UNIQUE (tenant_id, id),
 UNIQUE (tenant_id, mailbox_id, direction, provider_uid),
 FOREIGN KEY (tenant_id, mailbox_id) REFERENCES mail_mailboxes(tenant_id, id) ON DELETE CASCADE,
 FOREIGN KEY (tenant_id, owner_account_id) REFERENCES gov_identity_refs(tenant_id, account_id)
);
CREATE INDEX mail_messages_owner ON mail_messages(tenant_id, owner_account_id, folder, occurred_at DESC, id DESC);
CREATE INDEX mail_messages_mailbox ON mail_messages(tenant_id, mailbox_id, direction, occurred_at DESC);

CREATE TABLE mail_message_recipients (
 id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
 tenant_id text NOT NULL,
 message_id bigint NOT NULL,
 kind text NOT NULL CHECK (kind IN ('to','cc','bcc')),
 address text NOT NULL CHECK (length(address) BETWEEN 3 AND 320),
 display_name text NOT NULL DEFAULT '' CHECK (length(display_name) <= 200),
 created_at timestamptz NOT NULL DEFAULT now(),
 UNIQUE (tenant_id, id),
 UNIQUE (tenant_id, message_id, kind, address),
 FOREIGN KEY (tenant_id, message_id) REFERENCES mail_messages(tenant_id, id) ON DELETE CASCADE
);
CREATE INDEX mail_message_recipients_message ON mail_message_recipients(tenant_id, message_id, kind);

-- Attachment bytes are intentionally not stored here. The row records the
-- MIME part and binds a managed private file; downloads always join through
-- the owner's message binding rather than exposing a global file ID.
CREATE TABLE mail_message_attachments (
 id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
 tenant_id text NOT NULL,
 message_id bigint NOT NULL,
 file_id bigint NOT NULL,
 content_id text NOT NULL DEFAULT '' CHECK (length(content_id) <= 255),
 filename text NOT NULL CHECK (length(filename) BETWEEN 1 AND 255),
 mime_type text NOT NULL CHECK (length(mime_type) BETWEEN 3 AND 255),
 size bigint NOT NULL CHECK (size > 0),
 disposition text NOT NULL DEFAULT 'attachment' CHECK (disposition IN ('attachment','inline')),
 created_at timestamptz NOT NULL DEFAULT now(),
 UNIQUE (tenant_id, id),
 UNIQUE (tenant_id, message_id, file_id),
 FOREIGN KEY (tenant_id, message_id) REFERENCES mail_messages(tenant_id, id) ON DELETE CASCADE,
 FOREIGN KEY (tenant_id, file_id) REFERENCES gov_files(tenant_id, id)
);
CREATE INDEX mail_message_attachments_message ON mail_message_attachments(tenant_id, message_id);

-- The queue is the transactional boundary for submission. External providers
-- are at-least-once; ambiguous preserves a timeout after an unknown outcome.
CREATE TABLE mail_outbound_deliveries (
 id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
 tenant_id text NOT NULL,
 mailbox_id bigint NOT NULL,
 message_id bigint NOT NULL,
 status text NOT NULL DEFAULT 'queued' CHECK (status IN ('queued','sending','sent','failed','ambiguous','cancelled')),
 attempts integer NOT NULL DEFAULT 0 CHECK (attempts >= 0),
 next_attempt_at timestamptz NOT NULL DEFAULT now(),
 lease_until timestamptz,
 lease_owner text NOT NULL DEFAULT '',
 error_code text NOT NULL DEFAULT '',
 submitted_by bigint NOT NULL,
 sent_at timestamptz,
 created_at timestamptz NOT NULL DEFAULT now(),
 updated_at timestamptz NOT NULL DEFAULT now(),
 UNIQUE (tenant_id, id),
 UNIQUE (tenant_id, message_id),
 FOREIGN KEY (tenant_id, mailbox_id) REFERENCES mail_mailboxes(tenant_id, id) ON DELETE CASCADE,
 FOREIGN KEY (tenant_id, message_id) REFERENCES mail_messages(tenant_id, id) ON DELETE CASCADE,
 FOREIGN KEY (tenant_id, submitted_by) REFERENCES gov_identity_refs(tenant_id, account_id)
);
CREATE INDEX mail_outbound_due ON mail_outbound_deliveries(tenant_id, next_attempt_at, id) WHERE status='queued';
CREATE INDEX mail_outbound_owner ON mail_outbound_deliveries(tenant_id, submitted_by, created_at DESC);

CREATE TABLE mail_inbox_syncs (
 id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
 tenant_id text NOT NULL,
 mailbox_id bigint NOT NULL,
 status text NOT NULL DEFAULT 'running' CHECK (status IN ('running','completed','failed')),
 fetched_count integer NOT NULL DEFAULT 0 CHECK (fetched_count >= 0),
 imported_count integer NOT NULL DEFAULT 0 CHECK (imported_count >= 0),
 duplicate_count integer NOT NULL DEFAULT 0 CHECK (duplicate_count >= 0),
 error_code text NOT NULL DEFAULT '',
 started_at timestamptz NOT NULL DEFAULT now(),
 finished_at timestamptz,
 UNIQUE (tenant_id, id),
 FOREIGN KEY (tenant_id, mailbox_id) REFERENCES mail_mailboxes(tenant_id, id) ON DELETE CASCADE
);
CREATE INDEX mail_inbox_syncs_mailbox ON mail_inbox_syncs(tenant_id, mailbox_id, started_at DESC);

-- Allow private mail attachments while retaining the managed-file purpose
-- constraint. Existing logo/attachment values are unchanged.
ALTER TABLE gov_files DROP CONSTRAINT gov_files_purpose_check;
ALTER TABLE gov_files ADD CONSTRAINT gov_files_purpose_check
 CHECK (purpose IN ('logo','attachment','cloud_drive','mail_attachment'));

INSERT INTO runtime_installation_templates(plugin_id,version,application_id,enabled)
VALUES ('builtin:mail','1.0.0','mail',true)
ON CONFLICT (plugin_id) DO NOTHING;
INSERT INTO runtime_installations(tenant_id,plugin_id,version,status)
SELECT DISTINCT tenant_id,'builtin:mail','1.0.0','enabled' FROM iam_accounts
ON CONFLICT DO NOTHING;
INSERT INTO runtime_application_states(tenant_id,application_id,plugin_id)
SELECT tenant_id,'mail','builtin:mail' FROM runtime_installations
WHERE plugin_id='builtin:mail'
ON CONFLICT DO NOTHING;

-- +goose Down
DELETE FROM runtime_installation_templates WHERE plugin_id='builtin:mail';
DROP TABLE IF EXISTS mail_inbox_syncs;
DROP TABLE IF EXISTS mail_outbound_deliveries;
DROP TABLE IF EXISTS mail_message_attachments;
DROP TABLE IF EXISTS mail_message_recipients;
DROP TABLE IF EXISTS mail_messages;
DROP TABLE IF EXISTS mail_channels;
DROP TABLE IF EXISTS mail_credentials;
DROP TABLE IF EXISTS mail_mailboxes;
ALTER TABLE gov_files DROP CONSTRAINT gov_files_purpose_check;
ALTER TABLE gov_files ADD CONSTRAINT gov_files_purpose_check
 CHECK (purpose IN ('logo','attachment'));
