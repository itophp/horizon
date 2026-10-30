-- name: GovCreateSettingsSchedule :one
INSERT INTO gov_settings_schedules(tenant_id,draft_id,actor_id,department_id,base_revision,draft_hash,sensitive_confirmed,scheduled_at)
SELECT $1,$2,$3,$4,$5,$6,sqlc.arg(sensitive_confirmed)::boolean,sqlc.arg(scheduled_at)::timestamptz WHERE sqlc.arg(scheduled_at)::timestamptz>clock_timestamp()+interval '1 minute' AND sqlc.arg(scheduled_at)::timestamptz<=clock_timestamp()+interval '8760 hours' RETURNING id;

-- name: GovSettingsSchedule :one
SELECT s.id,s.draft_id,s.actor_id,s.department_id,s.base_revision,s.scheduled_at,s.status,s.error_code,s.version_id,s.created_at,s.finished_at,d.reason
FROM gov_settings_schedules s JOIN gov_drafts d ON d.tenant_id=s.tenant_id AND d.id=s.draft_id WHERE s.tenant_id=$1 AND s.id=$2;

-- name: GovSettingsSchedules :many
SELECT s.id,s.draft_id,s.actor_id,s.department_id,s.base_revision,s.scheduled_at,s.status,s.error_code,s.version_id,s.created_at,s.finished_at,d.reason
FROM gov_settings_schedules s JOIN gov_drafts d ON d.tenant_id=s.tenant_id AND d.id=s.draft_id WHERE s.tenant_id=$1 ORDER BY (s.status='pending') DESC,s.id DESC LIMIT 200;

-- name: GovHasPendingSettingsSchedule :one
SELECT EXISTS(SELECT 1 FROM gov_settings_schedules WHERE tenant_id=$1 AND draft_id=$2 AND status='pending');

-- name: GovCancelSettingsSchedule :one
UPDATE gov_settings_schedules SET status='cancelled',finished_at=clock_timestamp() WHERE tenant_id=$1 AND id=$2 AND status='pending' RETURNING id;

-- name: GovClaimSettingsSchedule :one
SELECT id,tenant_id,draft_id,actor_id,department_id,base_revision,draft_hash,sensitive_confirmed FROM gov_settings_schedules WHERE status='pending' AND scheduled_at<=clock_timestamp() ORDER BY scheduled_at,id LIMIT 1 FOR UPDATE SKIP LOCKED;

-- name: GovFinishSettingsSchedule :exec
UPDATE gov_settings_schedules SET status=$3,error_code=$4,version_id=$5,finished_at=clock_timestamp() WHERE tenant_id=$1 AND id=$2 AND status='pending';
