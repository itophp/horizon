-- name: AIConfiguration :one
SELECT revision, values_json
FROM ai_configuration
WHERE tenant_id = $1;

-- name: AIConfigurationSave :one
WITH updated AS (
 UPDATE ai_configuration AS config
 SET values_json = sqlc.arg(values_json), revision = revision + 1
 WHERE config.tenant_id = sqlc.arg(tenant_id) AND config.revision = sqlc.arg(base_revision)
 RETURNING revision
), inserted AS (
 INSERT INTO ai_configuration (tenant_id, revision, values_json)
 SELECT sqlc.arg(tenant_id)::text, 1, sqlc.arg(values_json)::jsonb
 WHERE sqlc.arg(base_revision)::bigint = 0
 ON CONFLICT (tenant_id) DO NOTHING
 RETURNING revision
)
SELECT revision FROM updated UNION ALL SELECT revision FROM inserted;
