-- +goose Up

-- User owned automation tasks: a time or interval driven instruction that
-- launches one workspace agent run per occurrence under the owner's current
-- authority. The instruction is the agent question, so it is bounded by the
-- same 4000 character limit the AI run record already enforces; nothing is
-- truncated between the form, the capability and the stored instruction.
CREATE TABLE automation_tasks (
 id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
 tenant_id text NOT NULL,
 owner_id bigint NOT NULL,
 department_id bigint NOT NULL DEFAULT 0,
 name text NOT NULL CHECK (length(name) BETWEEN 1 AND 120),
 description text NOT NULL CHECK (length(description) BETWEEN 1 AND 4000),
 schedule_kind text NOT NULL CHECK (schedule_kind IN ('once','hourly','daily','weekly','monthly')),
 run_at timestamptz NOT NULL,
 timezone text NOT NULL CHECK (length(timezone) BETWEEN 1 AND 100),
 enabled boolean NOT NULL DEFAULT true,
 next_run_at timestamptz,
 last_run_at timestamptz,
 failure_count int NOT NULL DEFAULT 0 CHECK (failure_count >= 0),
 created_at timestamptz NOT NULL DEFAULT now(),
 updated_at timestamptz NOT NULL DEFAULT now(),
 UNIQUE (tenant_id, id),
 CONSTRAINT automation_tasks_owner_fkey FOREIGN KEY (tenant_id, owner_id)
   REFERENCES gov_identity_refs(tenant_id, account_id)
);
CREATE INDEX automation_tasks_owner ON automation_tasks (tenant_id, owner_id, id DESC);
CREATE INDEX automation_tasks_due ON automation_tasks (next_run_at, id) WHERE enabled AND next_run_at IS NOT NULL;

-- One row per occurrence. The unique key makes at-least-once scheduling, worker
-- restarts and retried capability calls idempotent. ai_run_id joins the
-- occurrence to the recorded agent task; the answer is never copied here.
CREATE TABLE automation_runs (
 id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
 tenant_id text NOT NULL,
 task_id bigint NOT NULL,
 occurrence_key text NOT NULL CHECK (length(occurrence_key) BETWEEN 1 AND 120),
 scheduled_for timestamptz NOT NULL,
 trigger_kind text NOT NULL DEFAULT 'schedule' CHECK (trigger_kind IN ('schedule','manual')),
 status text NOT NULL CHECK (status IN ('queued','running','awaiting_approval','completed','failed','skipped','cancelled')),
 error_code text NOT NULL DEFAULT '',
 ai_run_id bigint,
 created_at timestamptz NOT NULL DEFAULT now(),
 started_at timestamptz,
 finished_at timestamptz,
 UNIQUE (tenant_id, task_id, occurrence_key),
 UNIQUE (tenant_id, id),
 FOREIGN KEY (tenant_id, task_id) REFERENCES automation_tasks (tenant_id, id) ON DELETE CASCADE
);
CREATE INDEX automation_runs_task ON automation_runs (tenant_id, task_id, id DESC);
CREATE INDEX automation_runs_active ON automation_runs (tenant_id, task_id) WHERE status IN ('queued','running','awaiting_approval');
CREATE INDEX automation_runs_queue ON automation_runs (id) WHERE status = 'queued';

-- Deployment provisioning policy: a built-in application is explicit, never
-- inferred from a missing installation row. Existing tenants get the enabled
-- state; RuntimeProvisionTenant applies the template to new tenants.
INSERT INTO runtime_installation_templates(plugin_id,version,application_id,enabled)
VALUES ('builtin:automation','1.0.0','automation',true)
ON CONFLICT (plugin_id) DO NOTHING;
INSERT INTO runtime_installations(tenant_id,plugin_id,version,status)
SELECT DISTINCT tenant_id,'builtin:automation','1.0.0','enabled' FROM iam_accounts
ON CONFLICT DO NOTHING;
INSERT INTO runtime_application_states(tenant_id,application_id,plugin_id)
SELECT tenant_id,'automation','builtin:automation' FROM runtime_installations
WHERE plugin_id='builtin:automation'
ON CONFLICT DO NOTHING;

-- +goose Down
-- Only the provisioning template rolls back: runtime definitions are immutable
-- once published, so an applied installation cannot be deleted without breaking
-- the heads/versions the runtime already recorded. Re-running Up reuses the
-- existing installation rows via ON CONFLICT.
DELETE FROM runtime_installation_templates WHERE plugin_id='builtin:automation';
DROP TABLE IF EXISTS automation_runs;
DROP TABLE IF EXISTS automation_tasks;
