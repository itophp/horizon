-- Automation queries. Every statement is tenant scoped and owner scoped: an
-- automation task is private to the account that created it, and a run is only
-- visible through the same owner check as the task it belongs to. The worker
-- reads due tasks and queued runs with FOR UPDATE SKIP LOCKED so several
-- instances cannot execute the same occurrence.

-- name: AutomationTaskCreate :one
INSERT INTO automation_tasks(tenant_id,owner_id,department_id,name,description,schedule_kind,run_at,timezone,next_run_at,schedule_day)
VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,sqlc.narg(schedule_day)) RETURNING id;

-- name: AutomationTaskUpdate :execrows
UPDATE automation_tasks
SET name=$4,description=$5,schedule_kind=$6,run_at=$7,timezone=$8,next_run_at=sqlc.narg(next_run_at),schedule_day=sqlc.narg(schedule_day),updated_at=now()
WHERE tenant_id=$1 AND owner_id=$2 AND id=$3;

-- name: AutomationTaskSetEnabled :execrows
UPDATE automation_tasks
SET enabled=sqlc.arg(enabled),next_run_at=sqlc.narg(next_run_at),updated_at=now()
WHERE tenant_id=$1 AND owner_id=$2 AND id=$3;

-- name: AutomationTaskDelete :execrows
DELETE FROM automation_tasks WHERE tenant_id=$1 AND owner_id=$2 AND id=$3;

-- name: AutomationTaskGet :one
SELECT id,name,description,schedule_kind,run_at,timezone,schedule_day,enabled,next_run_at,last_run_at,failure_count,created_at,updated_at
FROM automation_tasks
WHERE tenant_id=$1 AND owner_id=$2 AND id=$3;

-- name: AutomationTaskList :many
SELECT id,name,description,schedule_kind,run_at,timezone,schedule_day,enabled,next_run_at,last_run_at,failure_count,created_at,updated_at
FROM automation_tasks
WHERE tenant_id=$1 AND owner_id=$2
AND (sqlc.arg(keyword)::text = '' OR strpos(lower(name),lower(sqlc.arg(keyword))) > 0)
ORDER BY id DESC LIMIT sqlc.arg(page_limit) OFFSET sqlc.arg(page_offset);

-- name: AutomationTaskCount :one
SELECT count(*) FROM automation_tasks
WHERE tenant_id=$1 AND owner_id=$2
AND (sqlc.arg(keyword)::text = '' OR strpos(lower(name),lower(sqlc.arg(keyword))) > 0);

-- name: AutomationDueTasks :many
SELECT id,tenant_id,owner_id,department_id,name,description,schedule_kind,run_at,timezone,schedule_day,next_run_at
FROM automation_tasks
WHERE enabled AND next_run_at IS NOT NULL AND next_run_at <= now()
ORDER BY next_run_at,id LIMIT sqlc.arg(max_rows) FOR UPDATE SKIP LOCKED;

-- name: AutomationTaskAdvance :execrows
UPDATE automation_tasks
SET next_run_at=sqlc.narg(next_run_at),enabled=sqlc.arg(enabled),updated_at=now()
WHERE tenant_id=$1 AND id=$2 AND next_run_at=sqlc.arg(expected)::timestamptz;

-- name: AutomationTaskRecordRun :exec
UPDATE automation_tasks
SET last_run_at=now(),
 failure_count=CASE WHEN sqlc.arg(failed)::boolean THEN failure_count+1 ELSE 0 END,
 updated_at=now()
WHERE tenant_id=$1 AND id=$2;

-- name: AutomationRunQueue :one
INSERT INTO automation_runs(tenant_id,task_id,occurrence_key,scheduled_for,trigger_kind,status,error_code)
VALUES($1,$2,$3,$4,$5,$6,$7)
ON CONFLICT (tenant_id,task_id,occurrence_key) DO NOTHING
RETURNING id;

-- name: AutomationRunByKey :one
SELECT id FROM automation_runs
WHERE tenant_id=$1 AND task_id=$2 AND occurrence_key=$3;

-- name: AutomationRunActive :one
SELECT EXISTS(
 SELECT 1 FROM automation_runs
 WHERE tenant_id=$1 AND task_id=$2 AND status IN ('queued','running','awaiting_approval')
) AS active;

-- name: AutomationRunList :many
SELECT r.id,r.task_id,r.occurrence_key,r.scheduled_for,r.trigger_kind,r.status,r.error_code,r.ai_run_id,r.created_at,r.started_at,r.finished_at
FROM automation_runs r JOIN automation_tasks t ON t.tenant_id=r.tenant_id AND t.id=r.task_id
WHERE r.tenant_id=$1 AND t.owner_id=$2
AND (sqlc.arg(task_id)::bigint = 0 OR r.task_id = sqlc.arg(task_id))
ORDER BY r.id DESC LIMIT sqlc.arg(page_limit) OFFSET sqlc.arg(page_offset);

-- name: AutomationRunCount :one
SELECT count(*)
FROM automation_runs r JOIN automation_tasks t ON t.tenant_id=r.tenant_id AND t.id=r.task_id
WHERE r.tenant_id=$1 AND t.owner_id=$2
AND (sqlc.arg(task_id)::bigint = 0 OR r.task_id = sqlc.arg(task_id));

-- name: AutomationRunGet :one
SELECT r.id,r.task_id,r.occurrence_key,r.scheduled_for,r.trigger_kind,r.status,r.error_code,r.ai_run_id,r.created_at,r.started_at,r.finished_at,
 COALESCE(a.answer,'') AS answer, COALESCE(a.tool_calls,0) AS tool_calls, COALESCE(a.status,'') AS ai_status
FROM automation_runs r JOIN automation_tasks t ON t.tenant_id=r.tenant_id AND t.id=r.task_id
LEFT JOIN ai_runs a ON a.tenant_id=r.tenant_id AND a.id=r.ai_run_id
WHERE r.tenant_id=$1 AND t.owner_id=$2 AND r.id=$3;

-- name: AutomationRunsClaim :many
UPDATE automation_runs AS r SET status='running',started_at=now()
FROM (SELECT id FROM automation_runs WHERE status='queued' ORDER BY id LIMIT sqlc.arg(max_rows) FOR UPDATE SKIP LOCKED) AS queued
WHERE r.id=queued.id
RETURNING r.id,r.tenant_id,r.task_id,r.occurrence_key,r.scheduled_for,r.trigger_kind,
 (SELECT owner_id FROM automation_tasks t WHERE t.tenant_id=r.tenant_id AND t.id=r.task_id) AS owner_id,
 (SELECT department_id FROM automation_tasks t WHERE t.tenant_id=r.tenant_id AND t.id=r.task_id) AS department_id,
 (SELECT name FROM automation_tasks t WHERE t.tenant_id=r.tenant_id AND t.id=r.task_id) AS task_name,
 (SELECT description FROM automation_tasks t WHERE t.tenant_id=r.tenant_id AND t.id=r.task_id) AS task_description;

-- name: AutomationRunFinish :execrows
UPDATE automation_runs
SET status=$3,error_code=$4,ai_run_id=sqlc.narg(ai_run_id),finished_at=now()
WHERE tenant_id=$1 AND id=$2 AND status='running';

-- name: AutomationRunInterrupted :execrows
UPDATE automation_runs
SET status='failed',error_code='interrupted',finished_at=now()
WHERE status='running' AND started_at IS NOT NULL AND started_at < now()-interval '15 minutes';
