-- +goose Up
-- Infrastructure checkpoints contain encrypted Eino state and node receipts.
-- No business payload is exposed through the run summary or audit projection.
CREATE TABLE orch_checkpoints (
 tenant_id text NOT NULL,
 run_id bigint NOT NULL,
 checkpoint_key text NOT NULL,
 payload text NOT NULL,
 lease_until timestamptz NOT NULL DEFAULT '-infinity',
 lease_token text NOT NULL DEFAULT '',
 updated_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY (tenant_id, run_id, checkpoint_key),
 FOREIGN KEY (tenant_id, run_id) REFERENCES ai_runs(tenant_id, id) ON DELETE CASCADE,
 CHECK (length(payload) <= 8388608)
);
CREATE INDEX orch_checkpoint_queue ON orch_checkpoints(lease_until) WHERE checkpoint_key = 'manifest';

ALTER TABLE cap_invocations DROP CONSTRAINT cap_invocations_origin_check;
ALTER TABLE cap_invocations ADD CONSTRAINT cap_invocations_origin_check CHECK (origin IN ('human-ui','agent','workflow','api','event','approval','capability'));
ALTER TABLE cap_approvals ADD COLUMN action_key text NOT NULL DEFAULT '';
DROP INDEX cap_approvals_pending;
CREATE UNIQUE INDEX cap_approvals_pending ON cap_approvals(tenant_id, capability, account_id, arguments_digest, action_key) WHERE status = 'pending';

-- +goose Down
DROP TABLE orch_checkpoints;
DROP INDEX cap_approvals_pending;
-- Refuse rollback while independently scoped pending approvals remain.
ALTER TABLE cap_approvals DROP COLUMN action_key;
CREATE UNIQUE INDEX cap_approvals_pending ON cap_approvals(tenant_id, capability, account_id, arguments_digest) WHERE status = 'pending';
ALTER TABLE cap_invocations DROP CONSTRAINT cap_invocations_origin_check;
UPDATE cap_invocations SET origin = 'workflow' WHERE origin = 'capability';
ALTER TABLE cap_invocations ADD CONSTRAINT cap_invocations_origin_check CHECK (origin IN ('human-ui','agent','workflow','api','event','approval'));
