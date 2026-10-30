-- +goose Up
CREATE VIEW gov_notification_members AS
WITH members AS (
SELECT a.tenant_id,a.id AS account_id,a.full_name,a.department_id,a.organization_id FROM iam_accounts a WHERE a.status='active'
UNION SELECT a.tenant_id,a.id,a.full_name,m.department_id,m.organization_id FROM iam_accounts a JOIN iam_department_memberships m ON m.tenant_id=a.tenant_id AND m.account_id=a.id WHERE a.status='active'
)
SELECT m.* FROM members m JOIN iam_departments d ON d.tenant_id=m.tenant_id AND d.id=m.department_id AND d.status='active'
JOIN iam_organizations c ON c.tenant_id=m.tenant_id AND c.id=m.organization_id AND c.status='active'
JOIN iam_organizations e ON e.tenant_id=c.tenant_id AND e.id=c.parent_id AND e.status='active';
CREATE VIEW gov_notification_targets AS
WITH RECURSIVE paths AS (
 SELECT tenant_id,id AS ancestor,id FROM iam_departments WHERE status='active'
 UNION ALL SELECT p.tenant_id,p.ancestor,d.id FROM paths p JOIN iam_departments d ON d.tenant_id=p.tenant_id AND d.parent_id=p.id AND d.status='active'
), members AS (
 SELECT m.*,d.name AS department_name,c.name AS company_name,c.parent_id AS enterprise_id,e.name AS enterprise_name
 FROM gov_notification_members m JOIN iam_departments d ON d.tenant_id=m.tenant_id AND d.id=m.department_id AND d.status='active'
 JOIN iam_organizations c ON c.tenant_id=m.tenant_id AND c.id=m.organization_id AND c.status='active'
 JOIN iam_organizations e ON e.tenant_id=c.tenant_id AND e.id=c.parent_id AND e.status='active'
)
SELECT tenant_id,'account'::text AS kind,account_id AS target_id,full_name AS name,account_id,department_id,organization_id FROM members
UNION ALL SELECT m.tenant_id,'department',p.ancestor,d.name,m.account_id,m.department_id,m.organization_id FROM members m JOIN paths p ON p.tenant_id=m.tenant_id AND p.id=m.department_id JOIN iam_departments d ON d.tenant_id=p.tenant_id AND d.id=p.ancestor
UNION ALL SELECT tenant_id,'company',organization_id,company_name,account_id,department_id,organization_id FROM members
UNION ALL SELECT tenant_id,'enterprise',enterprise_id,enterprise_name,account_id,department_id,organization_id FROM members;
CREATE TABLE gov_announcements (
 id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,tenant_id text NOT NULL,sender_id bigint NOT NULL,department_id bigint NOT NULL,client_key text NOT NULL,payload_hash text NOT NULL,
 title text NOT NULL,markdown text NOT NULL,targets jsonb NOT NULL,email boolean NOT NULL,recipient_count integer NOT NULL,created_at timestamptz NOT NULL DEFAULT now(),
 UNIQUE(tenant_id,id),UNIQUE(tenant_id,sender_id,client_key),FOREIGN KEY(tenant_id,sender_id) REFERENCES gov_identity_refs(tenant_id,account_id),FOREIGN KEY(tenant_id,department_id) REFERENCES iam_departments(tenant_id,id)
);
CREATE TABLE gov_announcement_files (
 tenant_id text NOT NULL,announcement_id bigint NOT NULL,file_id bigint NOT NULL,
 PRIMARY KEY(tenant_id,announcement_id,file_id),FOREIGN KEY(tenant_id,announcement_id) REFERENCES gov_announcements(tenant_id,id),FOREIGN KEY(tenant_id,file_id) REFERENCES gov_files(tenant_id,id)
);
ALTER TABLE gov_notifications ALTER COLUMN event_id DROP NOT NULL;
ALTER TABLE gov_notifications ADD COLUMN announcement_id bigint;
ALTER TABLE gov_notifications ADD FOREIGN KEY(tenant_id,announcement_id) REFERENCES gov_announcements(tenant_id,id);
ALTER TABLE gov_notifications ADD CHECK((event_id IS NULL)<>(announcement_id IS NULL));
CREATE UNIQUE INDEX gov_announcement_recipient ON gov_notifications(tenant_id,announcement_id,recipient_id);
-- +goose Down
DELETE FROM gov_notifications WHERE announcement_id IS NOT NULL;
ALTER TABLE gov_notifications DROP COLUMN announcement_id;
ALTER TABLE gov_notifications ALTER COLUMN event_id SET NOT NULL;
DROP TABLE gov_announcement_files,gov_announcements;
DROP VIEW gov_notification_targets,gov_notification_members;
