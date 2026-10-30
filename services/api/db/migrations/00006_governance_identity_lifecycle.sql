-- +goose Up
CREATE TABLE gov_identity_refs (
 tenant_id text NOT NULL,account_id bigint NOT NULL,
 PRIMARY KEY(tenant_id,account_id)
);
INSERT INTO gov_identity_refs(tenant_id,account_id) SELECT tenant_id,id FROM iam_accounts;
-- +goose StatementBegin
CREATE FUNCTION gov_identity_remember() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 INSERT INTO gov_identity_refs(tenant_id,account_id) VALUES(NEW.tenant_id,NEW.id) ON CONFLICT DO NOTHING;
 RETURN NEW;
END $$;
-- +goose StatementEnd
CREATE TRIGGER gov_identity_remember_trigger AFTER INSERT ON iam_accounts FOR EACH ROW EXECUTE FUNCTION gov_identity_remember();
-- Personal preferences follow their account's lifetime. Shared audit actor and
-- upload owner IDs are historical identifiers; account deletion must not destroy
-- shared versions/files or make every account with preferences undeletable.
ALTER TABLE gov_scopes DROP CONSTRAINT gov_scopes_tenant_id_account_id_fkey;
ALTER TABLE gov_scopes ADD CONSTRAINT gov_scopes_tenant_id_account_id_fkey FOREIGN KEY(tenant_id,account_id) REFERENCES iam_accounts(tenant_id,id) ON DELETE CASCADE;
ALTER TABLE gov_drafts DROP CONSTRAINT gov_drafts_tenant_id_actor_id_fkey;
ALTER TABLE gov_versions DROP CONSTRAINT gov_versions_tenant_id_actor_id_fkey;
ALTER TABLE gov_audit DROP CONSTRAINT gov_audit_tenant_id_actor_id_fkey;
ALTER TABLE gov_files DROP CONSTRAINT gov_files_tenant_id_owner_id_fkey;
ALTER TABLE gov_drafts ADD CONSTRAINT gov_drafts_tenant_id_actor_id_fkey FOREIGN KEY(tenant_id,actor_id) REFERENCES gov_identity_refs(tenant_id,account_id);
ALTER TABLE gov_versions ADD CONSTRAINT gov_versions_tenant_id_actor_id_fkey FOREIGN KEY(tenant_id,actor_id) REFERENCES gov_identity_refs(tenant_id,account_id);
ALTER TABLE gov_audit ADD CONSTRAINT gov_audit_tenant_id_actor_id_fkey FOREIGN KEY(tenant_id,actor_id) REFERENCES gov_identity_refs(tenant_id,account_id);
ALTER TABLE gov_files ADD CONSTRAINT gov_files_tenant_id_owner_id_fkey FOREIGN KEY(tenant_id,owner_id) REFERENCES gov_identity_refs(tenant_id,account_id);

ALTER TABLE gov_revisions DROP CONSTRAINT gov_revisions_tenant_id_scope_key_fkey;
ALTER TABLE gov_revisions ADD CONSTRAINT gov_revisions_tenant_id_scope_key_fkey FOREIGN KEY(tenant_id,scope_key) REFERENCES gov_scopes(tenant_id,scope_key) ON DELETE CASCADE;
ALTER TABLE gov_values DROP CONSTRAINT gov_values_tenant_id_scope_key_fkey;
ALTER TABLE gov_values ADD CONSTRAINT gov_values_tenant_id_scope_key_fkey FOREIGN KEY(tenant_id,scope_key) REFERENCES gov_scopes(tenant_id,scope_key) ON DELETE CASCADE;
ALTER TABLE gov_drafts DROP CONSTRAINT gov_drafts_tenant_id_scope_key_fkey;
ALTER TABLE gov_drafts ADD CONSTRAINT gov_drafts_tenant_id_scope_key_fkey FOREIGN KEY(tenant_id,scope_key) REFERENCES gov_scopes(tenant_id,scope_key) ON DELETE CASCADE;
ALTER TABLE gov_versions DROP CONSTRAINT gov_versions_tenant_id_scope_key_fkey;
ALTER TABLE gov_versions ADD CONSTRAINT gov_versions_tenant_id_scope_key_fkey FOREIGN KEY(tenant_id,scope_key) REFERENCES gov_scopes(tenant_id,scope_key) ON DELETE CASCADE;
ALTER TABLE gov_audit DROP CONSTRAINT gov_audit_tenant_id_scope_key_fkey;
ALTER TABLE gov_audit ADD CONSTRAINT gov_audit_tenant_id_scope_key_fkey FOREIGN KEY(tenant_id,scope_key) REFERENCES gov_scopes(tenant_id,scope_key) ON DELETE CASCADE;

-- +goose Down
ALTER TABLE gov_drafts DROP CONSTRAINT gov_drafts_tenant_id_actor_id_fkey;
ALTER TABLE gov_versions DROP CONSTRAINT gov_versions_tenant_id_actor_id_fkey;
ALTER TABLE gov_audit DROP CONSTRAINT gov_audit_tenant_id_actor_id_fkey;
ALTER TABLE gov_files DROP CONSTRAINT gov_files_tenant_id_owner_id_fkey;
-- Reinstating live-account FKs deliberately fails if historical actors/owners
-- have already been deleted; never erase retained audit/files to force rollback.
ALTER TABLE gov_audit DROP CONSTRAINT gov_audit_tenant_id_scope_key_fkey;
ALTER TABLE gov_audit ADD CONSTRAINT gov_audit_tenant_id_scope_key_fkey FOREIGN KEY(tenant_id,scope_key) REFERENCES gov_scopes(tenant_id,scope_key);
ALTER TABLE gov_versions DROP CONSTRAINT gov_versions_tenant_id_scope_key_fkey;
ALTER TABLE gov_versions ADD CONSTRAINT gov_versions_tenant_id_scope_key_fkey FOREIGN KEY(tenant_id,scope_key) REFERENCES gov_scopes(tenant_id,scope_key);
ALTER TABLE gov_drafts DROP CONSTRAINT gov_drafts_tenant_id_scope_key_fkey;
ALTER TABLE gov_drafts ADD CONSTRAINT gov_drafts_tenant_id_scope_key_fkey FOREIGN KEY(tenant_id,scope_key) REFERENCES gov_scopes(tenant_id,scope_key);
ALTER TABLE gov_values DROP CONSTRAINT gov_values_tenant_id_scope_key_fkey;
ALTER TABLE gov_values ADD CONSTRAINT gov_values_tenant_id_scope_key_fkey FOREIGN KEY(tenant_id,scope_key) REFERENCES gov_scopes(tenant_id,scope_key);
ALTER TABLE gov_revisions DROP CONSTRAINT gov_revisions_tenant_id_scope_key_fkey;
ALTER TABLE gov_revisions ADD CONSTRAINT gov_revisions_tenant_id_scope_key_fkey FOREIGN KEY(tenant_id,scope_key) REFERENCES gov_scopes(tenant_id,scope_key);
ALTER TABLE gov_scopes DROP CONSTRAINT gov_scopes_tenant_id_account_id_fkey;
ALTER TABLE gov_scopes ADD CONSTRAINT gov_scopes_tenant_id_account_id_fkey FOREIGN KEY(tenant_id,account_id) REFERENCES iam_accounts(tenant_id,id);
ALTER TABLE gov_drafts ADD CONSTRAINT gov_drafts_tenant_id_actor_id_fkey FOREIGN KEY(tenant_id,actor_id) REFERENCES iam_accounts(tenant_id,id);
ALTER TABLE gov_versions ADD CONSTRAINT gov_versions_tenant_id_actor_id_fkey FOREIGN KEY(tenant_id,actor_id) REFERENCES iam_accounts(tenant_id,id);
ALTER TABLE gov_audit ADD CONSTRAINT gov_audit_tenant_id_actor_id_fkey FOREIGN KEY(tenant_id,actor_id) REFERENCES iam_accounts(tenant_id,id);
ALTER TABLE gov_files ADD CONSTRAINT gov_files_tenant_id_owner_id_fkey FOREIGN KEY(tenant_id,owner_id) REFERENCES iam_accounts(tenant_id,id);

DROP TRIGGER gov_identity_remember_trigger ON iam_accounts;
DROP FUNCTION gov_identity_remember();
DROP TABLE gov_identity_refs;
