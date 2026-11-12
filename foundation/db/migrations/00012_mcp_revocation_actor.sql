-- +goose Up
-- Historical/system revocations remain NULL rather than inventing an actor.
ALTER TABLE mcp_tokens ADD COLUMN revoked_by bigint;
ALTER TABLE mcp_tokens ADD FOREIGN KEY(tenant_id,revoked_by) REFERENCES gov_identity_refs(tenant_id,account_id);
-- +goose Down
ALTER TABLE mcp_tokens DROP COLUMN revoked_by;
