-- +goose Up
-- Primary membership stays in iam_accounts / iam_account_roles; this table extends it without data loss.
CREATE TABLE iam_department_memberships (
 tenant_id text NOT NULL, account_id bigint NOT NULL, organization_id bigint NOT NULL, department_id bigint NOT NULL,
 overrides jsonb NOT NULL DEFAULT '[]' CHECK(jsonb_typeof(overrides)='array'),
 PRIMARY KEY(tenant_id,account_id,department_id),
 FOREIGN KEY(tenant_id,account_id) REFERENCES iam_accounts(tenant_id,id) ON DELETE CASCADE,
 FOREIGN KEY(tenant_id,department_id,organization_id) REFERENCES iam_departments(tenant_id,id,organization_id)
);
CREATE INDEX iam_memberships_scope ON iam_department_memberships(tenant_id,organization_id,department_id);
CREATE TABLE iam_department_membership_roles (
 tenant_id text NOT NULL, account_id bigint NOT NULL, department_id bigint NOT NULL, role_id bigint NOT NULL,
 PRIMARY KEY(tenant_id,account_id,department_id,role_id),
 FOREIGN KEY(tenant_id,account_id,department_id) REFERENCES iam_department_memberships(tenant_id,account_id,department_id) ON DELETE CASCADE,
 FOREIGN KEY(tenant_id,role_id) REFERENCES iam_roles(tenant_id,id) ON DELETE RESTRICT
);
ALTER TABLE iam_sessions ADD COLUMN active_department_id bigint;
ALTER TABLE iam_sessions ADD FOREIGN KEY(tenant_id,active_department_id) REFERENCES iam_departments(tenant_id,id);
ALTER TABLE iam_temporary_requests ADD COLUMN department_id bigint;
UPDATE iam_temporary_requests r SET department_id=a.department_id FROM iam_accounts a WHERE a.tenant_id=r.tenant_id AND a.id=r.applicant_id;
ALTER TABLE iam_temporary_requests ALTER COLUMN department_id SET NOT NULL;
ALTER TABLE iam_temporary_requests ADD FOREIGN KEY(tenant_id,department_id) REFERENCES iam_departments(tenant_id,id);
-- +goose Down
ALTER TABLE iam_temporary_requests DROP COLUMN department_id;
ALTER TABLE iam_sessions DROP COLUMN active_department_id;
DROP TABLE iam_department_membership_roles,iam_department_memberships;
