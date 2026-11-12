-- name: RuntimeRegisterSnapshot :execrows
INSERT INTO runtime_deployment_snapshots(digest,plugin_id,plugin_version,application_id,base_version,definition,contracts_digest)
VALUES($1,$2,$3,$4,$5,$6,$7) ON CONFLICT(digest) DO NOTHING;

-- name: RuntimeDefinitionSnapshot :one
SELECT digest,plugin_id,plugin_version,application_id,base_version,definition,contracts_digest FROM runtime_deployment_snapshots WHERE digest=$1;

-- name: RuntimeInitializeHead :execrows
INSERT INTO runtime_application_heads(tenant_id,application_id,plugin_id,base_digest,route_root)
VALUES($1,$2,$3,$4,$5) ON CONFLICT DO NOTHING;

-- name: RuntimeDefinitionHead :one
SELECT application_id,plugin_id,published_version,proposal_version,base_digest,revision,next_configuration,compatibility
FROM runtime_application_heads WHERE tenant_id=$1 AND application_id=$2;

-- name: RuntimeDefinitionHeadLock :one
SELECT application_id,plugin_id,published_version,proposal_version,base_digest,revision,next_configuration,compatibility
FROM runtime_application_heads WHERE tenant_id=$1 AND application_id=$2 FOR UPDATE;

-- name: RuntimeDefinitionInsert :exec
INSERT INTO runtime_application_versions(tenant_id,application_id,version,parent_version,base_digest,definition,digest,source,created_by,agent_run_id,correlation_id)
VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11);

-- name: RuntimeDefinitionGet :one
SELECT version,parent_version,base_digest,definition,digest,source,created_by,agent_run_id,correlation_id,created_at
FROM runtime_application_versions WHERE tenant_id=$1 AND application_id=$2 AND version=$3;

-- name: RuntimeDefinitionHistory :many
SELECT version,parent_version,digest,source,created_by,created_at FROM runtime_application_versions
WHERE tenant_id=$1 AND application_id=$2 ORDER BY created_at DESC,version DESC LIMIT 100;

-- name: RuntimePublishedDefinitions :many
SELECT h.application_id,h.plugin_id,h.revision,h.compatibility,v.version,v.definition,v.digest
FROM runtime_application_heads h JOIN runtime_application_versions v
ON v.tenant_id=h.tenant_id AND v.application_id=h.application_id AND v.version=h.published_version
WHERE h.tenant_id=$1 ORDER BY h.application_id;

-- name: RuntimeDefinitionPropose :execrows
UPDATE runtime_application_heads SET proposal_version=$4,next_configuration=next_configuration+1,revision=revision+1
WHERE tenant_id=$1 AND application_id=$2 AND revision=$3;

-- name: RuntimeDefinitionPublish :execrows
UPDATE runtime_application_heads SET published_version=$4,proposal_version='',revision=revision+1,compatibility='compatible',
route_root=(SELECT v.definition->>'route' FROM runtime_application_versions v WHERE v.tenant_id=$1 AND v.application_id=$2 AND v.version=$4),
base_digest=(SELECT v.base_digest FROM runtime_application_versions v WHERE v.tenant_id=$1 AND v.application_id=$2 AND v.version=$4)
WHERE tenant_id=$1 AND application_id=$2 AND revision=$3;

-- name: RuntimeDefinitionDiscard :execrows
UPDATE runtime_application_heads SET proposal_version='',revision=revision+1
WHERE tenant_id=$1 AND application_id=$2 AND revision=$3;

-- name: RuntimeDefinitionChange :exec
INSERT INTO runtime_application_changes(tenant_id,application_id,action,version,digest,diagnostic_code,account_id,correlation_id)
VALUES($1,$2,$3,$4,$5,$6,$7,$8);

-- name: RuntimeDefinitionTenants :many
SELECT DISTINCT tenant_id FROM runtime_installations ORDER BY tenant_id;

-- name: RuntimeDefinitionInitializePublished :execrows
UPDATE runtime_application_heads SET published_version=$3
WHERE tenant_id=$1 AND application_id=$2 AND published_version='';

-- name: RuntimeDefinitionSetCompatibility :execrows
UPDATE runtime_application_heads SET compatibility=$3,revision=revision+1
WHERE tenant_id=$1 AND application_id=$2 AND compatibility<>$3;

-- name: RuntimeDesignDefinitions :many
SELECT h.application_id,h.revision,v.version,v.definition,v.digest
FROM runtime_application_heads h JOIN runtime_application_versions v ON v.tenant_id=h.tenant_id AND v.application_id=h.application_id
AND v.version=CASE WHEN h.proposal_version<>'' THEN h.proposal_version ELSE h.published_version END
WHERE h.tenant_id=$1 ORDER BY h.application_id;
