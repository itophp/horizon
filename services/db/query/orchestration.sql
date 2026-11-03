-- name: OrchPutCheckpoint :exec
INSERT INTO orch_checkpoints(tenant_id, run_id, checkpoint_key, payload)
SELECT $1, r.id, $4, $5 FROM ai_runs r
WHERE r.tenant_id = $1 AND r.account_id = $2 AND r.id = $3
ON CONFLICT (tenant_id, run_id, checkpoint_key) DO UPDATE SET payload = EXCLUDED.payload, updated_at = now();

-- name: OrchGetCheckpoint :one
SELECT c.payload FROM orch_checkpoints c JOIN ai_runs r ON r.tenant_id = c.tenant_id AND r.id = c.run_id
WHERE c.tenant_id = $1 AND r.account_id = $2 AND c.run_id = $3 AND c.checkpoint_key = $4;

-- name: OrchCandidates :many
SELECT r.tenant_id, r.account_id, r.department_id, r.id
FROM ai_runs r JOIN orch_checkpoints c ON c.tenant_id = r.tenant_id AND c.run_id = r.id AND c.checkpoint_key = 'manifest'
WHERE r.agent = 'orchestration' AND r.status IN ('running', 'awaiting_approval') AND c.lease_until < now()
ORDER BY c.lease_until, r.id LIMIT 16;

-- name: OrchClaim :one
UPDATE orch_checkpoints c SET lease_until = now() + interval '90 seconds', lease_token = $4
FROM ai_runs r WHERE c.tenant_id = $1 AND c.run_id = $2 AND c.checkpoint_key = 'manifest'
AND c.lease_until < now() AND r.tenant_id = c.tenant_id AND r.id = c.run_id AND r.account_id = $3
AND r.status IN ('running','awaiting_approval') RETURNING c.run_id;

-- name: OrchRelease :exec
UPDATE orch_checkpoints SET lease_until = now() + interval '1 second', lease_token = ''
WHERE tenant_id = $1 AND run_id = $2 AND checkpoint_key = 'manifest' AND lease_token = $3;

-- name: OrchSetStatus :execrows
UPDATE ai_runs SET status = $4, error_code = $5,
finished_at = CASE WHEN $4 IN ('completed','failed','cancelled') THEN now() ELSE NULL END
WHERE tenant_id = $1 AND account_id = $2 AND id = $3 AND agent = 'orchestration'
AND status IN ('running','awaiting_approval');

-- name: OrchFindStart :one
SELECT c.run_id FROM orch_checkpoints c JOIN ai_runs r ON r.tenant_id = c.tenant_id AND r.id = c.run_id
WHERE c.tenant_id = $1 AND r.account_id = $2 AND c.checkpoint_key = $3;

-- name: OrchStartLock :exec
SELECT pg_advisory_xact_lock(hashtextextended(sqlc.arg(lock_key)::text, 0));

-- name: OrchCancelPendingApprovals :exec
UPDATE cap_approvals a SET status = 'rejected', decided_at = now(), execution_code = 'cancelled'
FROM cap_invocations i WHERE a.tenant_id = $1 AND a.account_id = $2 AND a.status = 'pending'
AND i.tenant_id = a.tenant_id AND i.invocation_id = a.invocation_id
AND i.idempotency_key LIKE ('orch:' || sqlc.arg(run_id)::bigint::text || ':%');

-- name: OrchRunForInvocation :one
SELECT r.status FROM ai_runs r JOIN cap_invocations i
ON i.tenant_id = r.tenant_id AND i.account_id = r.account_id
AND i.idempotency_key LIKE ('orch:' || r.id::text || ':%')
WHERE r.tenant_id = $1 AND r.account_id = $2 AND i.invocation_id = $3 AND r.agent = 'orchestration';

-- name: OrchPendingApprovals :one
SELECT count(*)::bigint FROM cap_approvals a JOIN cap_invocations i
ON i.tenant_id = a.tenant_id AND i.invocation_id = a.invocation_id
WHERE a.tenant_id = $1 AND a.account_id = $2 AND a.status = 'pending'
AND i.idempotency_key LIKE ('orch:' || sqlc.arg(run_id)::bigint::text || ':%');
