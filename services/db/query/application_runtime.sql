-- name: RuntimeProvisionTenant :exec
WITH installations AS (
 INSERT INTO runtime_installations(tenant_id,plugin_id,version,status)
 SELECT sqlc.arg(tenant_id)::text,plugin_id,version,CASE WHEN enabled THEN 'enabled' ELSE 'installed' END
 FROM runtime_installation_templates ON CONFLICT DO NOTHING RETURNING tenant_id,plugin_id
)
INSERT INTO runtime_application_states(tenant_id,application_id,plugin_id)
 SELECT i.tenant_id,t.application_id,i.plugin_id FROM installations i
 JOIN runtime_installation_templates t ON t.plugin_id=i.plugin_id ON CONFLICT DO NOTHING;

-- name: RuntimeInstallation :one
SELECT plugin_id,version,status,revision,digest,previous_version,updated_at
FROM runtime_installations WHERE tenant_id=$1 AND plugin_id=$2;

-- name: RuntimeInstallationLock :one
SELECT plugin_id,version,status,revision,digest,previous_version,updated_at
FROM runtime_installations WHERE tenant_id=$1 AND plugin_id=$2 FOR UPDATE;

-- name: RuntimeInstallationReadLock :one
SELECT plugin_id,version,status,revision,digest,previous_version,updated_at
FROM runtime_installations WHERE tenant_id=$1 AND plugin_id=$2 FOR SHARE;

-- name: RuntimeInstallations :many
SELECT plugin_id,version,status,revision,digest,previous_version,updated_at
FROM runtime_installations WHERE tenant_id=$1 ORDER BY plugin_id;

-- name: RuntimeInstall :exec
INSERT INTO runtime_installations(tenant_id,plugin_id,version,status,digest)
VALUES($1,$2,$3,'installed',$4);

-- name: RuntimeChangeInstallation :execrows
UPDATE runtime_installations SET version=$4,status=$5,digest=$6,previous_version=$7,revision=revision+1,updated_at=now()
WHERE tenant_id=$1 AND plugin_id=$2 AND revision=$3;

-- name: RuntimeSetApplication :exec
INSERT INTO runtime_application_states(tenant_id,application_id,plugin_id,enabled)
VALUES($1,$2,$3,$4) ON CONFLICT(tenant_id,application_id)
DO UPDATE SET enabled=excluded.enabled, revision=runtime_application_states.revision+1;

-- name: RuntimeApplication :one
SELECT enabled,revision FROM runtime_application_states WHERE tenant_id=$1 AND application_id=$2;

-- name: RuntimeRecordHistory :exec
INSERT INTO runtime_installation_history(tenant_id,plugin_id,account_id,action,version,revision)
VALUES($1,$2,$3,$4,$5,$6);

-- name: RuntimeHistory :many
SELECT id,plugin_id,account_id,action,version,revision,created_at
FROM runtime_installation_history WHERE tenant_id=$1 AND plugin_id=$2 ORDER BY id DESC LIMIT 100;

-- name: RuntimeActiveReferences :one
SELECT (
 SELECT count(*) FROM cap_approvals a WHERE a.tenant_id=$1 AND a.status='pending'
 AND (a.capability = ANY(sqlc.arg(capabilities)::text[]) OR EXISTS(
 SELECT 1 FROM runtime_invocation_dependencies dep WHERE dep.tenant_id=a.tenant_id AND dep.invocation_id=a.invocation_id AND dep.plugin_id=sqlc.arg(plugin_id)::text
 ) OR (a.capability='orchestration.workflow.run' AND NOT EXISTS(SELECT 1 FROM runtime_invocation_dependencies dep WHERE dep.tenant_id=a.tenant_id AND dep.invocation_id=a.invocation_id)))
) + (
 SELECT count(*) FROM ai_runs r JOIN runtime_run_dependencies d ON d.tenant_id=r.tenant_id AND d.run_id=r.id
 WHERE r.tenant_id=$1 AND r.status IN ('running','awaiting_approval') AND d.plugin_id=sqlc.arg(plugin_id)::text
) AS count;

-- name: AIRunAppendEvent :one
INSERT INTO ai_run_events(tenant_id,run_id,sequence,event_type,payload)
SELECT $1,$2,COALESCE(MAX(sequence),0)+1,$3,$4 FROM ai_run_events WHERE tenant_id=$1 AND run_id=$2
RETURNING sequence;

-- name: AIRunEvents :many
SELECT e.sequence,e.event_type,e.payload,e.created_at FROM ai_run_events e
JOIN ai_runs r ON r.tenant_id=e.tenant_id AND r.id=e.run_id
WHERE e.tenant_id=$1 AND e.run_id=$2 AND r.account_id=$3 AND r.department_id=$4
AND e.sequence > $5 ORDER BY e.sequence LIMIT 200;

-- name: RuntimeTenantLock :exec
SELECT pg_advisory_xact_lock(hashtextextended(sqlc.arg(lock_key)::text,0));

-- name: RuntimeBindRun :exec
INSERT INTO runtime_run_dependencies(tenant_id,run_id,plugin_id) VALUES($1,$2,$3) ON CONFLICT DO NOTHING;

-- name: RuntimeBindInvocation :exec
INSERT INTO runtime_invocation_dependencies(tenant_id,invocation_id,plugin_id) VALUES($1,$2,$3) ON CONFLICT DO NOTHING;

-- name: RuntimeScopeStamp :many
SELECT i.plugin_id,i.version,i.status,i.revision,a.application_id,a.enabled,a.revision AS application_revision
FROM runtime_installations i JOIN runtime_application_states a ON a.tenant_id=i.tenant_id AND a.plugin_id=i.plugin_id
WHERE i.tenant_id=$1 ORDER BY i.plugin_id,a.application_id;

-- name: RuntimeApplications :many
SELECT application_id,plugin_id,enabled,revision FROM runtime_application_states WHERE tenant_id=$1 ORDER BY application_id;
