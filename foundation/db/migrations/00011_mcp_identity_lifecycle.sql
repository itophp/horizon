-- +goose Up
-- Defer membership checks until the enclosing identity transaction has replaced
-- memberships. Unchanged memberships are preserved; removed and later re-added
-- memberships must not resurrect an old credential.
-- +goose StatementBegin
CREATE FUNCTION mcp_membership_removed() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 UPDATE mcp_tokens t SET revoked_at=now()
 WHERE t.tenant_id=OLD.tenant_id AND t.account_id=OLD.account_id
 AND t.department_id=OLD.department_id AND t.revoked_at IS NULL
 AND NOT EXISTS(SELECT 1 FROM iam_accounts a WHERE a.tenant_id=t.tenant_id AND a.id=t.account_id AND a.department_id=t.department_id)
 AND NOT EXISTS(SELECT 1 FROM iam_department_memberships m WHERE m.tenant_id=t.tenant_id AND m.account_id=t.account_id AND m.department_id=t.department_id);
 RETURN NULL;
END $$;
-- +goose StatementEnd
CREATE CONSTRAINT TRIGGER mcp_membership_removed_trigger AFTER DELETE ON iam_department_memberships DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION mcp_membership_removed();
-- +goose StatementBegin
CREATE FUNCTION mcp_account_changed() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF NEW.password_hash IS DISTINCT FROM OLD.password_hash OR NEW.status='inactive' THEN
 UPDATE mcp_tokens SET revoked_at=now() WHERE tenant_id=NEW.tenant_id AND account_id=NEW.id AND revoked_at IS NULL;
 ELSE
 UPDATE mcp_tokens t SET revoked_at=now() WHERE t.tenant_id=NEW.tenant_id AND t.account_id=NEW.id AND t.revoked_at IS NULL
 AND NOT EXISTS(SELECT 1 FROM iam_accounts a WHERE a.tenant_id=t.tenant_id AND a.id=t.account_id AND a.department_id=t.department_id)
 AND NOT EXISTS(SELECT 1 FROM iam_department_memberships m WHERE m.tenant_id=t.tenant_id AND m.account_id=t.account_id AND m.department_id=t.department_id);
 END IF;
 RETURN NULL;
END $$;
-- +goose StatementEnd
CREATE CONSTRAINT TRIGGER mcp_account_changed_trigger AFTER UPDATE ON iam_accounts DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION mcp_account_changed();
-- +goose Down
DROP TRIGGER mcp_account_changed_trigger ON iam_accounts;
DROP FUNCTION mcp_account_changed();
DROP TRIGGER mcp_membership_removed_trigger ON iam_department_memberships;
DROP FUNCTION mcp_membership_removed();
