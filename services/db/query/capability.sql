-- name: CapCreateInvocation :one
INSERT INTO cap_invocations (tenant_id, account_id, department_id, invocation_id, capability, capability_version, origin, idempotency_key, status, input, input_digest, correlation_id, causation_id, trigger_depth, trace_id)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
RETURNING id, created_at;

-- name: CapFindIdempotent :one
SELECT id, invocation_id, capability, capability_version, status, result, error_code, approval_id, trace_id, created_at, finished_at, account_id, input_digest
FROM cap_invocations
WHERE tenant_id = $1 AND capability = $2 AND idempotency_key = $3 AND idempotency_key <> '';

-- name: CapFinishInvocation :exec
UPDATE cap_invocations
SET status = sqlc.arg(status), result = sqlc.arg(result), error_code = sqlc.arg(error_code),
    duration_ms = sqlc.arg(duration_ms), finished_at = now()
WHERE tenant_id = $1 AND invocation_id = $2;

-- name: CapSetInvocationStatus :exec
UPDATE cap_invocations SET status = sqlc.arg(status), approval_id = sqlc.arg(approval_id)
WHERE tenant_id = $1 AND invocation_id = $2;

-- name: CapGetInvocation :one
SELECT id, invocation_id, capability, capability_version, origin, status, input, input_digest,
       result, error_code, approval_id, correlation_id, causation_id, trigger_depth,
       duration_ms, trace_id, created_at, finished_at
FROM cap_invocations
WHERE tenant_id = $1 AND account_id = $2 AND invocation_id = $3;

-- name: CapListInvocations :many
SELECT id, invocation_id, capability, capability_version, origin, status, input, input_digest,
       result, error_code, approval_id, correlation_id, causation_id, trigger_depth,
       duration_ms, trace_id, created_at, finished_at
FROM cap_invocations
WHERE tenant_id = $1 AND account_id = $2
ORDER BY id DESC
LIMIT sqlc.arg(max_rows)::int;

-- name: CapAppendOutbox :exec
INSERT INTO cap_outbox (tenant_id, event_id, event_type, aggregate_type, aggregate_id, payload, invocation_id, account_id, department_id, correlation_id, causation_id, trigger_depth)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
ON CONFLICT (tenant_id, event_id) DO NOTHING;

-- name: CapClaimOutbox :many
WITH claimed AS (
  SELECT id FROM cap_outbox
  WHERE status = 'pending' AND available_at <= now()
  ORDER BY id
  LIMIT sqlc.arg(max_rows)::int
  FOR UPDATE SKIP LOCKED
)
UPDATE cap_outbox o SET attempts = o.attempts + 1
FROM claimed c WHERE o.id = c.id
RETURNING o.id, o.tenant_id, o.event_id, o.event_type, o.aggregate_type, o.aggregate_id, o.payload,
          o.invocation_id, o.account_id, o.department_id, o.correlation_id, o.causation_id, o.trigger_depth, o.attempts;

-- name: CapMarkOutboxPublished :exec
UPDATE cap_outbox SET status = 'published', published_at = now() WHERE id = $1;

-- name: CapMarkOutboxFailed :exec
UPDATE cap_outbox SET status = 'failed' WHERE id = $1;

-- name: CapRescheduleOutbox :exec
UPDATE cap_outbox SET available_at = now() + make_interval(secs => sqlc.arg(delay_seconds)::int)
WHERE id = $1 AND status = 'pending';

-- name: CapAppendEvent :exec
INSERT INTO cap_events (tenant_id, event_id, event_type, aggregate_type, aggregate_id, payload, invocation_id, account_id, department_id, correlation_id, causation_id, trigger_depth)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
ON CONFLICT (tenant_id, event_id) DO NOTHING;

-- name: CapListEvents :many
SELECT e.id, e.event_id, e.event_type, e.aggregate_type, e.aggregate_id, e.payload, e.invocation_id,
       e.account_id, e.department_id, e.correlation_id, e.causation_id, e.trigger_depth, e.created_at,
       COALESCE((SELECT x.status FROM ai_trigger_executions x
                 WHERE x.tenant_id = e.tenant_id AND x.event_id = e.event_id
                 ORDER BY x.id DESC LIMIT 1), '')::text AS ai_status,
       COALESCE((SELECT x.policy_id FROM ai_trigger_executions x
                 WHERE x.tenant_id = e.tenant_id AND x.event_id = e.event_id
                 ORDER BY x.id DESC LIMIT 1), '')::text AS ai_policy,
       COALESCE((SELECT x.run_id FROM ai_trigger_executions x
                 WHERE x.tenant_id = e.tenant_id AND x.event_id = e.event_id
                 ORDER BY x.id DESC LIMIT 1), 0)::bigint AS ai_run_id
FROM cap_events e
WHERE e.tenant_id = $1 AND (e.account_id = $2 OR sqlc.arg(reviewer)::bool)
ORDER BY e.id DESC
LIMIT sqlc.arg(max_rows)::int;

-- name: CapCreateApproval :one
INSERT INTO cap_approvals (tenant_id, account_id, department_id, capability, capability_version, invocation_id, risk, approval, status, arguments, arguments_digest, expires_at, action_key)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, 'pending', $9, $10, now() + make_interval(mins => sqlc.arg(expires_minutes)::int), sqlc.arg(action_key))
ON CONFLICT (tenant_id, capability, account_id, arguments_digest, action_key) WHERE status = 'pending'
DO UPDATE SET expires_at = cap_approvals.expires_at
RETURNING id;

-- name: CapGetApproval :one
SELECT id, tenant_id, account_id, department_id, capability, capability_version, invocation_id, risk, approval,
       arguments, arguments_digest, status, requested_at, expires_at, decided_by, decided_at,
       decision_note, execution_code, action_key
FROM cap_approvals
WHERE tenant_id = $1 AND id = $2;

-- name: CapListApprovals :many
SELECT id, tenant_id, account_id, department_id, capability, capability_version, invocation_id, risk, approval,
       arguments, arguments_digest, status, requested_at, expires_at, decided_by, decided_at,
       decision_note, execution_code, action_key
FROM cap_approvals
WHERE tenant_id = $1 AND status = 'pending' AND (account_id = $2 OR sqlc.arg(reviewer)::bool)
ORDER BY id DESC
LIMIT 100;

-- name: CapDecideApproval :execrows
UPDATE cap_approvals
SET status = sqlc.arg(status), decided_by = sqlc.arg(decided_by), decided_at = now(), decision_note = sqlc.arg(decision_note)
WHERE tenant_id = $1 AND id = $2 AND status = 'pending' AND expires_at > now();

-- name: CapCompleteApproval :exec
UPDATE cap_approvals SET status = sqlc.arg(status), execution_code = sqlc.arg(execution_code)
WHERE tenant_id = $1 AND id = $2;

-- name: CapExpireApprovals :exec
UPDATE cap_approvals SET status = 'expired'
WHERE tenant_id = $1 AND status = 'pending' AND expires_at <= now();

-- name: AITriggerPoliciesForEvent :many
SELECT id, tenant_id, policy_id, version, enabled, event_type, agent, mode, instruction, max_depth
FROM ai_trigger_policies
WHERE event_type = $2 AND enabled AND (tenant_id = $1 OR tenant_id = '*')
ORDER BY (tenant_id = $1) DESC, version DESC;

-- name: AITriggerExecutionCreate :one
INSERT INTO ai_trigger_executions (tenant_id, policy_id, policy_version, event_id, status)
VALUES ($1, $2, $3, $4, 'queued')
ON CONFLICT (tenant_id, policy_id, event_id) DO NOTHING
RETURNING id;

-- name: AITriggerExecutionStart :exec
UPDATE ai_trigger_executions SET status = 'running', started_at = now(), run_id = sqlc.arg(run_id)
WHERE tenant_id = $1 AND id = $2;

-- name: AITriggerExecutionFinish :exec
UPDATE ai_trigger_executions SET status = sqlc.arg(status), error_code = sqlc.arg(error_code), finished_at = now(), run_id = sqlc.arg(run_id)
WHERE tenant_id = $1 AND id = $2;

-- name: AITriggerExecutionList :many
SELECT id, tenant_id, policy_id, policy_version, event_id, run_id, status, error_code, created_at, started_at, finished_at
FROM ai_trigger_executions
WHERE tenant_id = $1
ORDER BY id DESC
LIMIT sqlc.arg(max_rows)::int;

-- name: ExampleRecordCreate :one
INSERT INTO example_records (tenant_id, title, created_by) VALUES ($1, $2, $3)
RETURNING id, tenant_id, title, status, created_by, created_at, updated_at;

-- name: ExampleRecordGet :one
SELECT id, tenant_id, title, status, created_by, created_at, updated_at
FROM example_records WHERE tenant_id = $1 AND id = $2 AND created_by = $3;

-- name: ExampleRecordList :many
SELECT id, tenant_id, title, status, created_by, created_at, updated_at
FROM example_records WHERE tenant_id = $1
ORDER BY id DESC
LIMIT sqlc.arg(max_rows)::int;

-- name: ExampleRecordArchive :one
UPDATE example_records SET status = 'archived', updated_at = now()
WHERE tenant_id = $1 AND id = $2 AND status <> 'archived'
RETURNING id, tenant_id, title, status, created_by, created_at, updated_at;

-- name: CapInvocationTrace :one
SELECT trace_id, correlation_id, causation_id, trigger_depth, capability, capability_version
FROM cap_invocations
WHERE tenant_id = $1 AND invocation_id = $2;

-- name: CapResumeInvocation :exec
UPDATE cap_invocations SET status = 'executing' WHERE tenant_id = $1 AND invocation_id = $2 AND status = 'awaiting_approval';

-- name: CapListInvocationsByCorrelation :many
SELECT id, invocation_id, capability, capability_version, origin, status, input, input_digest,
       result, error_code, approval_id, correlation_id, causation_id, trigger_depth,
       duration_ms, trace_id, created_at, finished_at
FROM cap_invocations
WHERE tenant_id = $1 AND correlation_id = $2
ORDER BY id ASC
LIMIT 100;

-- name: CapPendingApprovalFor :one
SELECT a.id, a.invocation_id, i.trace_id
FROM cap_approvals a
JOIN cap_invocations i ON i.tenant_id = a.tenant_id AND i.invocation_id = a.invocation_id
WHERE a.tenant_id = $1 AND a.capability = $2 AND a.account_id = $3 AND a.arguments_digest = $4 AND a.action_key = $5 AND a.status = 'pending'
LIMIT 1;
