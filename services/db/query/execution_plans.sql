-- name: ExecutionSeedDirect :exec
INSERT INTO cap_execution_plans(tenant_id,plan_id,version,capability,capability_version,name,mode,implementation_ref,contract_hash,status,created_by,published_by,published_at)
VALUES('', $1,1,$2,$3,$4,'direct',$5,$6,'published',0,0,now()) ON CONFLICT DO NOTHING;

-- name: ExecutionGetPlan :one
SELECT tenant_id,plan_id,version,capability,capability_version,name,mode,implementation_ref,contract_hash,status,created_by,created_at,published_by,published_at,workflow_hash
FROM cap_execution_plans WHERE tenant_id=$1 AND plan_id=$2 AND version=$3;

-- name: ExecutionListPlans :many
SELECT tenant_id,plan_id,version,capability,capability_version,name,mode,implementation_ref,contract_hash,status,created_by,created_at,published_by,published_at,workflow_hash
FROM cap_execution_plans WHERE (tenant_id=$1 OR tenant_id='') AND capability=$2 AND capability_version=$3
ORDER BY (tenant_id='') DESC,created_at DESC,plan_id,version DESC LIMIT 200;

-- name: ExecutionPlanLock :exec
SELECT pg_advisory_xact_lock(hashtextextended(sqlc.arg(lock_key)::text, 29));

-- name: ExecutionCreateDraft :one
INSERT INTO cap_execution_plans(tenant_id,plan_id,version,capability,capability_version,name,mode,implementation_ref,contract_hash,status,created_by,workflow_hash)
SELECT $1,$2,COALESCE(MAX(version),0)+1,$3,$4,$5,sqlc.arg(mode),$6,$7,'draft',$8,sqlc.arg(workflow_hash)
FROM cap_execution_plans WHERE tenant_id=$1 AND plan_id=$2
RETURNING version;

-- name: ExecutionValidatePlan :execrows
UPDATE cap_execution_plans SET status='validated'
WHERE tenant_id=$1 AND plan_id=$2 AND version=$3 AND status='draft' AND contract_hash=$4;

-- name: ExecutionPublishPlan :execrows
UPDATE cap_execution_plans SET status='published',published_by=$4,published_at=now()
WHERE tenant_id=$1 AND plan_id=$2 AND version=$3 AND status='validated' AND contract_hash=$5;

-- name: ExecutionGetBinding :one
SELECT plan_tenant_id,plan_id,plan_version,revision FROM cap_execution_bindings
WHERE tenant_id=$1 AND capability=$2 AND capability_version=$3;

-- name: ExecutionSetBinding :execrows
INSERT INTO cap_execution_bindings(tenant_id,capability,capability_version,plan_tenant_id,plan_id,plan_version,revision,updated_by)
SELECT $1,$2,$3,$4,$5,$6,1,$7 WHERE sqlc.arg(expected_revision)::bigint=0
ON CONFLICT(tenant_id,capability,capability_version) DO UPDATE
SET plan_tenant_id=$4,plan_id=$5,plan_version=$6,revision=cap_execution_bindings.revision+1,updated_by=$7,updated_at=now()
WHERE cap_execution_bindings.revision=sqlc.arg(expected_revision)::bigint;

-- name: ExecutionUpdateBinding :execrows
UPDATE cap_execution_bindings SET plan_tenant_id=$4,plan_id=$5,plan_version=$6,revision=revision+1,updated_by=$7,updated_at=now()
WHERE tenant_id=$1 AND capability=$2 AND capability_version=$3 AND revision=sqlc.arg(expected_revision)::bigint;

-- name: ExecutionAppendBindingChange :exec
INSERT INTO cap_execution_binding_changes(tenant_id,capability,capability_version,plan_id,plan_version,revision,action,reason,account_id)
VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9);

-- name: ExecutionBindingHistory :many
SELECT id,plan_id,plan_version,revision,action,reason,account_id,created_at FROM cap_execution_binding_changes
WHERE tenant_id=$1 AND capability=$2 AND capability_version=$3 ORDER BY id DESC LIMIT 100;

-- name: ExecutionGetRecord :one
SELECT binding FROM cap_execution_records WHERE tenant_id=$1 AND invocation_id=$2 AND account_id=$3;

-- name: ExecutionListRecords :many
SELECT r.invocation_id,r.binding,r.run_id,COALESCE(a.status,'')::text AS run_status,i.status,i.trace_id,i.causation_id,i.created_at,i.finished_at,i.error_code
FROM cap_execution_records r JOIN cap_invocations i USING(tenant_id,invocation_id)
LEFT JOIN ai_runs a ON a.tenant_id=r.tenant_id AND a.id=r.run_id AND a.account_id=r.account_id
WHERE r.tenant_id=$1 AND r.account_id=$2 AND (sqlc.arg(capability)::text='' OR (i.capability=sqlc.arg(capability)::text AND i.capability_version=sqlc.arg(capability_version)::text))
ORDER BY i.id DESC LIMIT 100;

-- name: ExecutionPlanIdentity :one
SELECT capability,capability_version FROM cap_execution_plans WHERE tenant_id=$1 AND plan_id=$2 ORDER BY version DESC LIMIT 1;

-- name: ExecutionAttachRun :execrows
UPDATE cap_execution_records SET run_id=$4 WHERE tenant_id=$1 AND account_id=$2 AND invocation_id=$3 AND run_id IS NULL;

-- name: ExecutionCheckRun :one
SELECT binding FROM cap_execution_records WHERE tenant_id=$1 AND account_id=$2 AND invocation_id=$3 AND run_id=$4;

-- name: ExecutionRootForRun :one
SELECT invocation_id FROM cap_execution_records WHERE tenant_id=$1 AND account_id=$2 AND run_id=$3;

-- name: ExecutionFinishWorkflow :execrows
UPDATE cap_invocations SET status=$4,result=$5,error_code=$6,finished_at=now()
WHERE tenant_id=$1 AND account_id=$2 AND invocation_id=$3 AND status='executing';

-- name: ExecutionCancelWorkflow :exec
UPDATE cap_invocations i SET status='cancelled',error_code='cancelled',finished_at=now()
FROM cap_execution_records r WHERE i.tenant_id=r.tenant_id AND i.invocation_id=r.invocation_id
AND r.tenant_id=$1 AND r.account_id=$2 AND r.run_id=$3 AND i.status='executing';

-- name: ExecutionRunForInvocation :one
SELECT run_id FROM cap_execution_records WHERE tenant_id=$1 AND account_id=$2 AND invocation_id=$3;

-- name: ExecutionTrace :many
SELECT e.sequence,e.event_type,COALESCE(e.payload->>'stepName','')::text AS step_name,e.created_at
FROM cap_execution_records r JOIN ai_run_events e ON e.tenant_id=r.tenant_id AND e.run_id=r.run_id
WHERE r.tenant_id=$1 AND r.account_id=$2 AND r.invocation_id=$3
AND e.event_type IN ('STEP_STARTED','STEP_FINISHED') ORDER BY e.sequence DESC LIMIT 200;
