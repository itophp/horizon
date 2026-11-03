-- +goose Up

-- Agent runs are the durable trace of one goal driven execution: who asked,
-- under which department, with which model and instruction version, and how it
-- ended. Only identifiers, the question and a redacted page context are stored.
CREATE TABLE ai_runs (
 id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
 tenant_id text NOT NULL,
 account_id bigint NOT NULL,
 department_id bigint NOT NULL DEFAULT 0,
 session_id text NOT NULL DEFAULT '',
 agent text NOT NULL,
 model text NOT NULL,
 instruction_version text NOT NULL,
 question text NOT NULL CHECK (length(question) BETWEEN 1 AND 4000),
 answer text NOT NULL DEFAULT '',
 status text NOT NULL CHECK (status IN ('running','completed','awaiting_approval','failed','cancelled')),
 error_code text NOT NULL DEFAULT '',
 tool_calls int NOT NULL DEFAULT 0 CHECK (tool_calls >= 0),
 prompt_tokens int NOT NULL DEFAULT 0 CHECK (prompt_tokens >= 0),
 completion_tokens int NOT NULL DEFAULT 0 CHECK (completion_tokens >= 0),
 duration_ms int NOT NULL DEFAULT 0 CHECK (duration_ms >= 0),
 trace_id text NOT NULL,
 context jsonb NOT NULL DEFAULT '{}'::jsonb,
 created_at timestamptz NOT NULL DEFAULT now(),
 finished_at timestamptz,
 UNIQUE (tenant_id, id)
);
CREATE INDEX ai_runs_owner ON ai_runs (tenant_id, account_id, id DESC);

-- One step per capability call. Arguments are projected through the capability
-- loggable whitelist before they reach this table; a digest binds approvals to
-- the exact reviewed payload.
CREATE TABLE ai_run_steps (
 id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
 tenant_id text NOT NULL,
 run_id bigint NOT NULL,
 seq int NOT NULL CHECK (seq > 0),
 kind text NOT NULL CHECK (kind IN ('tool','approval','answer','error')),
 capability text NOT NULL DEFAULT '',
 capability_version text NOT NULL DEFAULT '',
 risk text NOT NULL DEFAULT '',
 status text NOT NULL CHECK (status IN ('succeeded','failed','denied','pending_approval')),
 arguments jsonb NOT NULL DEFAULT '{}'::jsonb,
 arguments_digest text NOT NULL DEFAULT '',
 result_code text NOT NULL DEFAULT '',
 summary text NOT NULL DEFAULT '',
 duration_ms int NOT NULL DEFAULT 0 CHECK (duration_ms >= 0),
 created_at timestamptz NOT NULL DEFAULT now(),
 UNIQUE (run_id, seq),
 UNIQUE (tenant_id, id),
 FOREIGN KEY (tenant_id, run_id) REFERENCES ai_runs (tenant_id, id) ON DELETE CASCADE
);
CREATE INDEX ai_run_steps_run ON ai_run_steps (tenant_id, run_id, seq);

-- Human confirmation for high risk capabilities. Execution still happens under
-- the requester's own authority, re-resolved from the database at decision time.
CREATE TABLE ai_approvals (
 id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
 tenant_id text NOT NULL,
 run_id bigint NOT NULL,
 step_id bigint,
 account_id bigint NOT NULL,
 department_id bigint NOT NULL DEFAULT 0,
 capability text NOT NULL,
 capability_version text NOT NULL,
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
 UNIQUE (tenant_id, id),
FOREIGN KEY (tenant_id, run_id) REFERENCES ai_runs (tenant_id, id) ON DELETE CASCADE,
FOREIGN KEY (tenant_id, step_id) REFERENCES ai_run_steps (tenant_id, id) ON DELETE SET NULL
);
-- One open confirmation per capability and payload: a repeated request reuses the
-- pending card instead of stacking duplicates.
CREATE UNIQUE INDEX ai_approvals_pending ON ai_approvals (tenant_id, run_id, capability, arguments_digest) WHERE status = 'pending';
CREATE INDEX ai_approvals_owner ON ai_approvals (tenant_id, account_id, status, id DESC);
CREATE INDEX ai_approvals_review ON ai_approvals (tenant_id, status, id DESC);

-- +goose Down
DROP TABLE IF EXISTS ai_approvals;
DROP TABLE IF EXISTS ai_run_steps;
DROP TABLE IF EXISTS ai_runs;
