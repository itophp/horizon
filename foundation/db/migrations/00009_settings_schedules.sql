-- +goose Up
CREATE TABLE gov_settings_schedules (
 id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,tenant_id text NOT NULL,draft_id bigint NOT NULL,actor_id bigint NOT NULL,department_id bigint NOT NULL,
 base_revision bigint NOT NULL CHECK(base_revision>=0),draft_hash text NOT NULL,sensitive_confirmed boolean NOT NULL,scheduled_at timestamptz NOT NULL,
 status text NOT NULL DEFAULT 'pending' CHECK(status IN('pending','published','cancelled','failed')),error_code text NOT NULL DEFAULT '',version_id bigint,
 created_at timestamptz NOT NULL DEFAULT now(),finished_at timestamptz,
 UNIQUE(tenant_id,id),FOREIGN KEY(tenant_id,draft_id) REFERENCES gov_drafts(tenant_id,id),
 FOREIGN KEY(tenant_id,actor_id) REFERENCES gov_identity_refs(tenant_id,account_id),FOREIGN KEY(tenant_id,department_id) REFERENCES iam_departments(tenant_id,id),
 FOREIGN KEY(tenant_id,version_id) REFERENCES gov_versions(tenant_id,id),
 CHECK((status='published')=(version_id IS NOT NULL)),CHECK((status='pending')=(finished_at IS NULL))
);
CREATE UNIQUE INDEX gov_one_pending_settings_schedule ON gov_settings_schedules(tenant_id,draft_id) WHERE status='pending';
CREATE INDEX gov_settings_schedules_due ON gov_settings_schedules(scheduled_at,id) WHERE status='pending';
-- +goose Down
DROP TABLE gov_settings_schedules;
