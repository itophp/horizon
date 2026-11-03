-- name: MCPCreateToken :one
INSERT INTO mcp_tokens (tenant_id, account_id, department_id, name, token_hash, token_prefix, expires_at)
SELECT $1, $2, $3, $4, $5, $6, COALESCE(sqlc.narg(preserved_expiry)::timestamptz, now()+make_interval(days=>sqlc.arg(valid_days)::int))
WHERE EXISTS(SELECT 1 FROM iam_accounts a WHERE a.tenant_id=$1 AND a.id=$2 AND a.status='active'
AND (a.department_id=$3 OR EXISTS(SELECT 1 FROM iam_department_memberships m WHERE m.tenant_id=a.tenant_id AND m.account_id=a.id AND m.department_id=$3)))
RETURNING id, tenant_id, account_id, department_id, name, token_prefix, created_at, expires_at, revoked_at, last_used_at;

-- name: MCPAddTokenTool :exec
INSERT INTO mcp_token_tools (token_id, tool) VALUES ($1, $2);

-- name: MCPListTokens :many
SELECT id, department_id, name, token_prefix, created_at, expires_at, revoked_at, last_used_at, COALESCE(revoked_by,0)::bigint AS revoked_by
FROM mcp_tokens WHERE tenant_id = $1 AND account_id = $2 ORDER BY (revoked_at IS NULL AND expires_at>now()) DESC, id DESC LIMIT 100;

-- name: MCPTokenTools :many
SELECT tt.tool FROM mcp_token_tools tt JOIN mcp_tokens t ON t.id = tt.token_id
WHERE t.tenant_id = $1 AND t.account_id = $2 AND t.id = $3 ORDER BY tt.tool;

-- name: MCPRevokeToken :one
UPDATE mcp_tokens SET revoked_at = now(), revoked_by=account_id
WHERE tenant_id = $1 AND account_id = $2 AND id = $3 AND revoked_at IS NULL
RETURNING id;

-- name: MCPAuthenticateToken :one
SELECT id, tenant_id, account_id, department_id, token_hash
FROM mcp_tokens WHERE token_hash = $1 AND revoked_at IS NULL AND expires_at > now();

-- name: MCPTouchToken :exec
UPDATE mcp_tokens SET last_used_at = now() WHERE id = $1 AND revoked_at IS NULL AND expires_at > now();

-- name: MCPInsertAudit :exec
INSERT INTO mcp_call_audits (tenant_id, account_id, token_id, department_id, tool, result_code, duration_ms, request_id)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8);

-- name: MCPListAudits :many
SELECT id, token_id, department_id, tool, result_code, duration_ms, request_id, created_at
FROM mcp_call_audits WHERE tenant_id = $1 AND account_id = $2 ORDER BY id DESC LIMIT 100;

-- name: MCPTokenForRotation :one
SELECT id, department_id, name, expires_at FROM mcp_tokens
WHERE tenant_id = $1 AND account_id = $2 AND id = $3 AND revoked_at IS NULL AND expires_at > now()
FOR UPDATE;

-- name: MCPLockOwner :one
SELECT id,password_hash FROM iam_accounts WHERE tenant_id=$1 AND id=$2 AND status='active' FOR UPDATE;

-- name: MCPActiveCount :one
SELECT count(*) FROM mcp_tokens WHERE tenant_id=$1 AND account_id=$2 AND revoked_at IS NULL AND expires_at>now();

-- name: MCPRotationDepartment :one
SELECT department_id FROM mcp_tokens WHERE tenant_id=$1 AND account_id=$2 AND id=$3 AND revoked_at IS NULL AND expires_at>now();

-- name: MCPPruneAuditBatch :exec
DELETE FROM mcp_call_audits WHERE id IN (
 SELECT id FROM mcp_call_audits WHERE created_at<now()-interval '90 days' ORDER BY id LIMIT 1000
);

-- name: MCPPruneTokenBatch :exec
DELETE FROM mcp_tokens WHERE id IN (
 SELECT id FROM mcp_tokens WHERE expires_at<now()-interval '90 days' OR revoked_at<now()-interval '90 days' ORDER BY id LIMIT 1000
);

-- name: MCPAdminRevokeToken :one
UPDATE mcp_tokens SET revoked_at=now(),revoked_by=sqlc.arg(actor_id)
WHERE tenant_id=sqlc.arg(tenant_id) AND id=sqlc.arg(id) AND revoked_at IS NULL RETURNING id;
