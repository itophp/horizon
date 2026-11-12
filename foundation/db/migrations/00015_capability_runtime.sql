-- +goose Up

-- One row per business capability invocation. This is the audit spine: who
-- called, through which origin, with which version, what happened and which
-- business events it produced. Input is the redacted projection; the digest
-- binds idempotent replays to the exact payload.
CREATE TABLE cap_invocations (
 id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
 tenant_id text NOT NULL,
 account_id bigint NOT NULL,
 department_id bigint NOT NULL DEFAULT 0,
 invocation_id text NOT NULL,
 capability text NOT NULL,
 capability_version text NOT NULL,
 origin text NOT NULL CHECK (origin IN ('human-ui','agent','workflow','api','event','approval')),
 idempotency_key text NOT NULL DEFAULT '',
 status text NOT NULL CHECK (status IN ('requested','awaiting_approval','executing','succeeded','failed','cancelled')),
 input jsonb NOT NULL DEFAULT '{}'::jsonb,
 input_digest text NOT NULL,
 result jsonb,
 error_code text NOT NULL DEFAULT '',
 approval_id bigint NOT NULL DEFAULT 0,
 correlation_id text NOT NULL DEFAULT '',
 causation_id text NOT NULL DEFAULT '',
 trigger_depth int NOT NULL DEFAULT 0 CHECK (trigger_depth >= 0),
 duration_ms int NOT NULL DEFAULT 0 CHECK (duration_ms >= 0),
 trace_id text NOT NULL,
 created_at timestamptz NOT NULL DEFAULT now(),
 finished_at timestamptz,
 UNIQUE (tenant_id, invocation_id)
);
-- A retried button click or agent retry with the same key must not execute twice.
CREATE UNIQUE INDEX cap_invocations_idempotency ON cap_invocations (tenant_id, capability, idempotency_key) WHERE idempotency_key <> '';
CREATE INDEX cap_invocations_owner ON cap_invocations (tenant_id, account_id, id DESC);
CREATE INDEX cap_invocations_correlation ON cap_invocations (tenant_id, correlation_id);

-- Transactional outbox. Business executors that accept the gateway transaction
-- write here in the same commit as their data, so an event can never be lost
-- after the business change is visible.
CREATE TABLE cap_outbox (
 id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
 tenant_id text NOT NULL,
 event_id text NOT NULL,
 event_type text NOT NULL,
 aggregate_type text NOT NULL,
 aggregate_id text NOT NULL,
 payload jsonb NOT NULL DEFAULT '{}'::jsonb,
 invocation_id text NOT NULL DEFAULT '',
 account_id bigint NOT NULL DEFAULT 0,
 department_id bigint NOT NULL DEFAULT 0,
 correlation_id text NOT NULL DEFAULT '',
 causation_id text NOT NULL DEFAULT '',
 trigger_depth int NOT NULL DEFAULT 0 CHECK (trigger_depth >= 0),
 status text NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','published','failed')),
 attempts int NOT NULL DEFAULT 0 CHECK (attempts >= 0),
 available_at timestamptz NOT NULL DEFAULT now(),
 published_at timestamptz,
 created_at timestamptz NOT NULL DEFAULT now(),
 UNIQUE (tenant_id, event_id)
);
CREATE INDEX cap_outbox_pending ON cap_outbox (status, available_at, id);

-- Durable business event log. Consumers deduplicate on (tenant_id, event_id),
-- so at-least-once delivery is safe.
CREATE TABLE cap_events (
 id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
 tenant_id text NOT NULL,
 event_id text NOT NULL,
 event_type text NOT NULL,
 aggregate_type text NOT NULL,
 aggregate_id text NOT NULL,
 payload jsonb NOT NULL DEFAULT '{}'::jsonb,
 invocation_id text NOT NULL DEFAULT '',
 account_id bigint NOT NULL DEFAULT 0,
 department_id bigint NOT NULL DEFAULT 0,
 correlation_id text NOT NULL DEFAULT '',
 causation_id text NOT NULL DEFAULT '',
 trigger_depth int NOT NULL DEFAULT 0 CHECK (trigger_depth >= 0),
 created_at timestamptz NOT NULL DEFAULT now(),
 UNIQUE (tenant_id, event_id)
);
CREATE INDEX cap_events_recent ON cap_events (tenant_id, id DESC);

-- Human confirmation for capabilities that policy marks as approval-bound. The
-- reviewed payload is kept for execution; the API only projects allowlisted
-- fields.
CREATE TABLE cap_approvals (
 id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
 tenant_id text NOT NULL,
 account_id bigint NOT NULL,
 department_id bigint NOT NULL DEFAULT 0,
 capability text NOT NULL,
 capability_version text NOT NULL,
 invocation_id text NOT NULL DEFAULT '',
 risk text NOT NULL,
 approval text NOT NULL CHECK (approval IN ('confirm','four-eyes')),
 arguments jsonb NOT NULL,
 arguments_digest text NOT NULL,
 status text NOT NULL CHECK (status IN ('pending','approved','rejected','expired','executed','failed')),
 requested_at timestamptz NOT NULL DEFAULT now(),
 expires_at timestamptz NOT NULL,
 decided_by bigint,
 decided_at timestamptz,
 decision_note text NOT NULL DEFAULT '',
 execution_code text NOT NULL DEFAULT '',
 UNIQUE (tenant_id, id)
);
CREATE UNIQUE INDEX cap_approvals_pending ON cap_approvals (tenant_id, capability, account_id, arguments_digest) WHERE status = 'pending';
CREATE INDEX cap_approvals_owner ON cap_approvals (tenant_id, account_id, status, id DESC);

-- AI trigger policy: which business event wakes which agent, at which depth and
-- with which instruction. Versioned; running tasks bind to the version used.
CREATE TABLE ai_trigger_policies (
 id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
 tenant_id text NOT NULL DEFAULT '*',
 policy_id text NOT NULL,
 version int NOT NULL DEFAULT 1 CHECK (version > 0),
 enabled boolean NOT NULL DEFAULT true,
 event_type text NOT NULL,
 agent text NOT NULL,
 mode text NOT NULL DEFAULT 'async' CHECK (mode IN ('async','event')),
 instruction text NOT NULL DEFAULT '',
 max_depth int NOT NULL DEFAULT 3 CHECK (max_depth BETWEEN 1 AND 10),
 created_at timestamptz NOT NULL DEFAULT now(),
 updated_at timestamptz NOT NULL DEFAULT now(),
 UNIQUE (tenant_id, policy_id, version)
);

-- One execution per policy and event. The unique key is the deduplication
-- guarantee for at-least-once delivery.
CREATE TABLE ai_trigger_executions (
 id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
 tenant_id text NOT NULL,
 policy_id text NOT NULL,
 policy_version int NOT NULL,
 event_id text NOT NULL,
 run_id bigint,
 status text NOT NULL CHECK (status IN ('queued','running','completed','failed','skipped')),
 error_code text NOT NULL DEFAULT '',
 created_at timestamptz NOT NULL DEFAULT now(),
 started_at timestamptz,
 finished_at timestamptz,
 UNIQUE (tenant_id, policy_id, event_id)
);
CREATE INDEX ai_trigger_executions_recent ON ai_trigger_executions (tenant_id, id DESC);

-- Example business entity used to validate the framework without coupling to
-- MES/ERP. Its status machine is enforced by the database, not by the model.
CREATE TABLE example_records (
 id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
 tenant_id text NOT NULL,
 title text NOT NULL CHECK (length(title) BETWEEN 1 AND 200),
 status text NOT NULL DEFAULT 'draft' CHECK (status IN ('draft','active','archived')),
 created_by bigint NOT NULL,
 created_at timestamptz NOT NULL DEFAULT now(),
 updated_at timestamptz NOT NULL DEFAULT now(),
 UNIQUE (tenant_id, id)
);
CREATE INDEX example_records_owner ON example_records (tenant_id, id DESC);

-- AI tasks now record their origin and causal chain: an event-triggered task
-- must be distinguishable from a user conversation, and loop prevention needs
-- correlation, causation and depth on every hop.
ALTER TABLE ai_runs ADD COLUMN origin text NOT NULL DEFAULT 'human-ui';
ALTER TABLE ai_runs ADD COLUMN correlation_id text NOT NULL DEFAULT '';
ALTER TABLE ai_runs ADD COLUMN causation_id text NOT NULL DEFAULT '';
ALTER TABLE ai_runs ADD COLUMN trigger_depth int NOT NULL DEFAULT 0;
ALTER TABLE ai_runs ADD COLUMN trigger_policy text NOT NULL DEFAULT '';
ALTER TABLE ai_runs ADD COLUMN event_id text NOT NULL DEFAULT '';

-- Agent tool approvals are now gateway approvals; the old AI-only queue is
-- replaced instead of being kept as a second, divergent mechanism. Tool calls are
-- audited as capability invocations, so the duplicated step table is removed.
DROP TABLE IF EXISTS ai_approvals;
DROP TABLE IF EXISTS ai_run_steps;

-- A global policy so the event → AI loop works out of the box; tenants can add
-- their own versioned policy for the same event.
INSERT INTO ai_trigger_policies (tenant_id, policy_id, version, enabled, event_type, agent, mode, instruction, max_depth)
VALUES ('*', 'example-record-review', 1, true, 'example.record.created', 'workspace_agent', 'async',
 'A new example record was created. Review it: summarise what was created, check whether the title is ambiguous, and recommend one concrete next step. You may call read-only capabilities; do not modify business state.', 3);

-- +goose Down
ALTER TABLE ai_runs DROP COLUMN IF EXISTS event_id;
ALTER TABLE ai_runs DROP COLUMN IF EXISTS trigger_policy;
ALTER TABLE ai_runs DROP COLUMN IF EXISTS trigger_depth;
ALTER TABLE ai_runs DROP COLUMN IF EXISTS causation_id;
ALTER TABLE ai_runs DROP COLUMN IF EXISTS correlation_id;
ALTER TABLE ai_runs DROP COLUMN IF EXISTS origin;
DROP TABLE IF EXISTS example_records;
DROP TABLE IF EXISTS ai_trigger_executions;
DROP TABLE IF EXISTS ai_trigger_policies;
DROP TABLE IF EXISTS cap_approvals;
DROP TABLE IF EXISTS cap_events;
DROP TABLE IF EXISTS cap_outbox;
DROP TABLE IF EXISTS cap_invocations;
CREATE TABLE IF NOT EXISTS ai_run_steps (
 id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
 tenant_id text NOT NULL,
 run_id bigint NOT NULL,
 seq int NOT NULL,
 kind text NOT NULL,
 capability text NOT NULL DEFAULT '',
 capability_version text NOT NULL DEFAULT '',
 risk text NOT NULL DEFAULT '',
 status text NOT NULL,
 arguments jsonb NOT NULL DEFAULT '{}'::jsonb,
 arguments_digest text NOT NULL DEFAULT '',
 result_code text NOT NULL DEFAULT '',
 summary text NOT NULL DEFAULT '',
 duration_ms int NOT NULL DEFAULT 0,
 created_at timestamptz NOT NULL DEFAULT now()
);
