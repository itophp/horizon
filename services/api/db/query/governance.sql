-- name: GovEnsureScope :exec
INSERT INTO gov_scopes(tenant_id,scope_key,kind,entity_id,organization_id,department_id,account_id) VALUES($1,$2,$3,$4,$5,$6,$7) ON CONFLICT DO NOTHING;

-- name: GovGetScope :one
SELECT kind,entity_id FROM gov_scopes WHERE tenant_id=$1 AND scope_key=$2;

-- name: GovEnsureRevision :exec
INSERT INTO gov_revisions(tenant_id,resource_key,scope_key) VALUES($1,$2,$3) ON CONFLICT DO NOTHING;

-- name: GovRevision :one
SELECT revision FROM gov_revisions WHERE tenant_id=$1 AND resource_key=$2;

-- name: GovLockRevision :one
SELECT revision FROM gov_revisions WHERE tenant_id=$1 AND resource_key=$2 FOR UPDATE;

-- name: GovBumpRevision :one
UPDATE gov_revisions SET revision=revision+1 WHERE tenant_id=$1 AND resource_key=$2 AND revision=$3 RETURNING revision;

-- name: GovValues :many
SELECT scope_key,config_key,value_json FROM gov_values WHERE tenant_id=$1 AND scope_key=ANY($2::text[]) ORDER BY scope_key,config_key;

-- name: GovSetValue :exec
INSERT INTO gov_values(tenant_id,scope_key,config_key,value_json) VALUES($1,$2,$3,$4) ON CONFLICT(tenant_id,scope_key,config_key) DO UPDATE SET value_json=EXCLUDED.value_json;

-- name: GovResetValue :exec
DELETE FROM gov_values WHERE tenant_id=$1 AND scope_key=$2 AND config_key=$3;

-- name: GovDraft :one
SELECT id,domain,resource_key,scope_key,entity_id,base_revision,payload,reason,sensitive,status,actor_id,created_at FROM gov_drafts WHERE scope_key='workspace:0' AND tenant_id=$1 AND id=$2 AND ($3::boolean OR scope_key=ANY($4::text[]));

-- name: GovDrafts :many
SELECT id,domain,resource_key,scope_key,entity_id,base_revision,payload,reason,sensitive,status,actor_id,created_at FROM gov_drafts WHERE tenant_id=$1 AND resource_key=$2 AND status='draft' ORDER BY id DESC LIMIT 100;

-- name: GovCreateDraft :one
INSERT INTO gov_drafts(tenant_id,domain,resource_key,scope_key,entity_id,base_revision,payload,reason,sensitive,actor_id) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10) RETURNING id;

-- name: GovPublishDraft :one
UPDATE gov_drafts SET status='published' WHERE tenant_id=$1 AND id=$2 AND status='draft' RETURNING id;

-- name: GovVersions :many
SELECT id,domain,resource_key,scope_key,entity_id,revision,payload,reason,actor_id,created_at FROM gov_versions WHERE tenant_id=$1 AND resource_key=$2 ORDER BY revision DESC LIMIT 100;

-- name: GovVersion :one
SELECT id,domain,resource_key,scope_key,entity_id,revision,payload,reason,actor_id,created_at FROM gov_versions WHERE scope_key='workspace:0' AND tenant_id=$1 AND id=$2 AND ($3::boolean OR scope_key=ANY($4::text[]));

-- name: GovAddVersion :one
INSERT INTO gov_versions(tenant_id,domain,resource_key,scope_key,entity_id,revision,payload,reason,actor_id) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9) RETURNING id;

-- name: GovAudit :exec
INSERT INTO gov_audit(tenant_id,scope_key,actor_id,action,resource_key,revision,detail) VALUES($1,$2,$3,$4,$5,$6,$7);

-- name: GovCreateProof :exec
INSERT INTO gov_reauth(token_hash,tenant_id,session_hash,department_id,operation,resource_id,revision,expires_at) VALUES($1,$2,$3,$4,$5,$6,$7,now()+interval '5 minutes');

-- name: GovConsumeProof :one
DELETE FROM gov_reauth WHERE token_hash=$1 AND tenant_id=$2 AND session_hash=$3 AND department_id=$4 AND operation=$5 AND resource_id=$6 AND revision=$7 AND expires_at>now() RETURNING token_hash;

-- name: GovDepartmentPath :many
WITH RECURSIVE path AS (SELECT d.id,d.parent_id,d.organization_id,0 AS depth FROM iam_departments d WHERE d.tenant_id=$1 AND d.id=$2 UNION ALL SELECT d.id,d.parent_id,d.organization_id,p.depth+1 FROM iam_departments d JOIN path p ON p.parent_id=d.id WHERE d.tenant_id=$1 AND p.depth<100) SELECT id,organization_id,depth FROM path ORDER BY depth DESC;

-- name: GovSessionTiming :one
SELECT created_at,expires_at,last_seen_at FROM iam_sessions WHERE token_hash=$1;

-- name: GovTouchSession :exec
UPDATE iam_sessions SET last_seen_at=now() WHERE token_hash=$1 AND last_seen_at<now()-interval '1 minute';

-- name: GovAccountPassword :one
SELECT password_hash FROM iam_accounts WHERE tenant_id=$1 AND id=$2 AND status='active';

-- name: GovDictionaryList :many
SELECT id,scope_key,code,spec,revision FROM gov_dictionaries WHERE scope_key='workspace:0' AND tenant_id=$1 AND scope_key=ANY($2::text[]) AND (scope_key=sqlc.arg(view_scope) OR (revision>0 AND (sqlc.arg(all_data)::boolean OR spec->>'Visibility'='shared'))) ORDER BY code LIMIT 1000;

-- name: GovDictionary :one
SELECT id,scope_key,code,spec,revision FROM gov_dictionaries WHERE scope_key='workspace:0' AND tenant_id=$1 AND id=$2 AND scope_key=ANY($3::text[]) AND (scope_key=sqlc.arg(view_scope) OR (revision>0 AND (sqlc.arg(all_data)::boolean OR spec->>'Visibility'='shared')));

-- name: GovDictionaryCode :one
SELECT id,scope_key,code,spec,revision FROM gov_dictionaries WHERE tenant_id=$1 AND code=$2 AND scope_key=ANY($3::text[]);

-- name: GovCreateDictionary :one
INSERT INTO gov_dictionaries(tenant_id,scope_key,code,spec) VALUES($1,$2,$3,$4) RETURNING id;

-- name: GovUpdateDictionary :exec
UPDATE gov_dictionaries SET spec=$4,revision=$5 WHERE tenant_id=$1 AND id=$2 AND scope_key=$3;

-- name: GovDeleteDictionary :one
DELETE FROM gov_dictionaries WHERE gov_dictionaries.tenant_id=$1 AND gov_dictionaries.id=$2 AND gov_dictionaries.scope_key=$3 AND gov_dictionaries.revision=0 AND NOT EXISTS (SELECT 1 FROM gov_dictionary_items i WHERE i.tenant_id=gov_dictionaries.tenant_id AND i.dictionary_id=gov_dictionaries.id) RETURNING gov_dictionaries.id;

-- name: GovDictionaryItems :many
SELECT id,scope_key,code,parent_code,spec FROM gov_dictionary_items WHERE tenant_id=$1 AND dictionary_id=$2 AND scope_key=ANY($3::text[]) ORDER BY id;

-- name: GovUpsertItem :one
INSERT INTO gov_dictionary_items(tenant_id,dictionary_id,scope_key,code,parent_code,spec) VALUES($1,$2,$3,$4,$5,$6) ON CONFLICT(tenant_id,dictionary_id,code) DO UPDATE SET parent_code=EXCLUDED.parent_code,spec=EXCLUDED.spec WHERE gov_dictionary_items.scope_key=EXCLUDED.scope_key RETURNING id;

-- name: GovItemOverrides :many
SELECT item_id,scope_key,spec FROM gov_dictionary_overrides WHERE tenant_id=$1 AND dictionary_id=$2 AND scope_key=ANY($3::text[]) ORDER BY scope_key,item_id;

-- name: GovSetItemOverride :exec
INSERT INTO gov_dictionary_overrides(tenant_id,dictionary_id,item_id,scope_key,spec) VALUES($1,$2,$3,$4,$5) ON CONFLICT(tenant_id,dictionary_id,item_id,scope_key) DO UPDATE SET spec=EXCLUDED.spec;

-- name: GovResetItemOverride :exec
DELETE FROM gov_dictionary_overrides WHERE tenant_id=$1 AND dictionary_id=$2 AND item_id=$3 AND scope_key=$4;

-- name: GovDictionaryReferences :one
SELECT count(*) FROM gov_dictionary_references WHERE tenant_id=$1 AND dictionary_id=$2;

-- name: GovRegisterDictionaryReference :exec
INSERT INTO gov_dictionary_references(tenant_id,dictionary_id,item_id,resource_type,resource_id) VALUES($1,$2,$3,$4,$5) ON CONFLICT DO NOTHING;

-- name: GovRemoveDictionaryReferences :exec
DELETE FROM gov_dictionary_references WHERE tenant_id=$1 AND resource_type=$2 AND resource_id=$3 AND dictionary_id=$4;

-- name: GovStorageProfile :one
SELECT driver,config FROM gov_storage_profiles WHERE tenant_id=$1 AND id=$2;

-- name: GovEnsureStorageProfile :one
INSERT INTO gov_storage_profiles(tenant_id,driver,config_hash,config) VALUES($1,$2,$3,$4) ON CONFLICT(tenant_id,config_hash) DO UPDATE SET config_hash=EXCLUDED.config_hash RETURNING id;

-- name: GovFiles :many
SELECT f.id,f.owner_id,f.organization_id,f.department_id,f.storage_profile_id,f.storage_key,f.name,f.mime,f.size,f.purpose,f.sha256,f.created_at FROM gov_files f WHERE f.tenant_id=sqlc.arg(tenant_id) AND (sqlc.arg(all_data)::boolean OR f.owner_id=sqlc.arg(own_id)::bigint OR f.organization_id=ANY(sqlc.arg(org_ids)::bigint[]) OR f.department_id=ANY(sqlc.arg(dept_ids)::bigint[])) AND f.deleted_at IS NULL ORDER BY f.id DESC LIMIT 1000;

-- name: GovFile :one
SELECT f.id,f.owner_id,f.organization_id,f.department_id,f.storage_profile_id,f.storage_key,f.name,f.mime,f.size,f.purpose,f.sha256,f.created_at FROM gov_files f WHERE f.tenant_id=sqlc.arg(tenant_id) AND (sqlc.arg(all_data)::boolean OR f.owner_id=sqlc.arg(own_id)::bigint OR f.organization_id=ANY(sqlc.arg(org_ids)::bigint[]) OR f.department_id=ANY(sqlc.arg(dept_ids)::bigint[])) AND f.id=sqlc.arg(id) AND f.deleted_at IS NULL;

-- name: GovCreateFile :one
INSERT INTO gov_files(tenant_id,owner_id,organization_id,department_id,storage_profile_id,storage_key,name,mime,size,purpose,sha256) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11) RETURNING id;

-- name: GovDeleteFile :one
UPDATE gov_files f SET deleted_at=now() WHERE f.tenant_id=sqlc.arg(tenant_id) AND (sqlc.arg(all_data)::boolean OR f.owner_id=sqlc.arg(own_id)::bigint OR f.organization_id=ANY(sqlc.arg(org_ids)::bigint[]) OR f.department_id=ANY(sqlc.arg(dept_ids)::bigint[])) AND f.id=sqlc.arg(id) AND f.deleted_at IS NULL AND NOT EXISTS(SELECT 1 FROM gov_file_bindings b WHERE b.tenant_id=f.tenant_id AND b.file_id=f.id) AND NOT EXISTS(SELECT 1 FROM gov_announcement_files b WHERE b.tenant_id=f.tenant_id AND b.file_id=f.id) RETURNING f.id;

-- name: GovBindFile :exec
INSERT INTO gov_file_bindings(tenant_id,file_id,scope_key,revision) VALUES($1,$2,$3,$4) ON CONFLICT DO NOTHING;

-- name: GovPendingFileDeletes :many
SELECT tenant_id,id,storage_profile_id,storage_key FROM gov_files WHERE deleted_at IS NOT NULL AND purged_at IS NULL ORDER BY id LIMIT 100;

-- name: GovPurgedFile :exec
UPDATE gov_files SET purged_at=now() WHERE tenant_id=$1 AND id=$2 AND deleted_at IS NOT NULL;

-- name: GovClaimEvent :one
UPDATE gov_events SET lease_until=now()+interval '1 minute',attempts=attempts+1 WHERE id=(SELECT id FROM gov_events WHERE NOT processed AND (lease_until IS NULL OR lease_until<now()) AND attempts<10 ORDER BY id FOR UPDATE SKIP LOCKED LIMIT 1) RETURNING id,tenant_id,request_id,event;

-- name: GovEventDone :exec
UPDATE gov_events SET processed=true,lease_until=NULL WHERE tenant_id=$1 AND id=$2;

-- name: GovEventRequest :one
SELECT r.applicant_id,r.department_id,r.status,r.revoked_by,a.full_name,d.name AS department_name FROM iam_temporary_requests r JOIN iam_accounts a ON a.tenant_id=r.tenant_id AND a.id=r.applicant_id JOIN iam_departments d ON d.tenant_id=r.tenant_id AND d.id=r.department_id WHERE r.tenant_id=$1 AND r.id=$2;

-- name: GovCandidateMemberships :many
SELECT a.id AS account_id,a.department_id FROM iam_accounts a WHERE a.tenant_id=$1 AND a.status='active' UNION SELECT m.account_id,m.department_id FROM iam_department_memberships m JOIN iam_accounts a ON a.tenant_id=m.tenant_id AND a.id=m.account_id WHERE m.tenant_id=$1 AND a.status='active';

-- name: GovAddNotification :one
INSERT INTO gov_notifications(tenant_id,event_id,recipient_id,department_id,payload) VALUES($1,$2,$3,$4,$5) ON CONFLICT(tenant_id,event_id,recipient_id,department_id) DO UPDATE SET event_id=EXCLUDED.event_id RETURNING id;

-- name: GovInbox :many
SELECT n.id,n.payload,n.read_at,n.created_at FROM gov_notifications n WHERE n.tenant_id=$1 AND n.recipient_id=$2 AND (n.department_id=$3 OR (n.announcement_id IS NOT NULL AND EXISTS(SELECT 1 FROM gov_notification_members m WHERE m.tenant_id=n.tenant_id AND m.account_id=n.recipient_id AND m.department_id=n.department_id))) ORDER BY n.id DESC LIMIT 500;

-- name: GovReadNotification :one
UPDATE gov_notifications n SET read_at=now() WHERE n.tenant_id=$1 AND n.id=$2 AND n.recipient_id=$3 AND (n.department_id=$4 OR (n.announcement_id IS NOT NULL AND EXISTS(SELECT 1 FROM gov_notification_members m WHERE m.tenant_id=n.tenant_id AND m.account_id=n.recipient_id AND m.department_id=n.department_id))) RETURNING n.id;

-- name: GovAddDelivery :exec
INSERT INTO gov_deliveries(tenant_id,message_id,status,error_code) VALUES($1,$2,$3,$4) ON CONFLICT DO NOTHING;

-- name: GovClaimDelivery :one
UPDATE gov_deliveries SET status='processing',lease_until=now()+interval '1 minute',attempts=attempts+1 WHERE id=(SELECT id FROM gov_deliveries WHERE ((status='pending' AND next_attempt_at<=now()) OR (status='processing' AND lease_until<now())) AND attempts<5 ORDER BY id FOR UPDATE SKIP LOCKED LIMIT 1) RETURNING id,tenant_id,message_id,attempts;

-- name: GovDeliveryMessage :one
SELECT n.recipient_id,n.department_id,n.payload,a.email FROM gov_notifications n JOIN iam_accounts a ON a.tenant_id=n.tenant_id AND a.id=n.recipient_id WHERE n.tenant_id=$1 AND n.id=$2;

-- name: GovFinishDelivery :exec
UPDATE gov_deliveries SET status=$3,error_code=$4,next_attempt_at=now()+make_interval(secs=>$5::int),lease_until=NULL WHERE tenant_id=$1 AND id=$2;

-- name: GovDeliveries :many
SELECT d.id,d.message_id,d.channel,d.status,d.attempts,d.error_code,d.created_at,d.next_attempt_at,COALESCE(n.payload->>'title','')::text AS title,a.full_name AS recipient_name FROM gov_deliveries d JOIN gov_notifications n ON n.tenant_id=d.tenant_id AND n.id=d.message_id JOIN iam_accounts a ON a.tenant_id=n.tenant_id AND a.id=n.recipient_id JOIN iam_departments dept ON dept.tenant_id=n.tenant_id AND dept.id=n.department_id WHERE d.tenant_id=sqlc.arg(tenant_id) AND (sqlc.arg(all_data)::boolean OR n.recipient_id=sqlc.arg(own_id)::bigint OR n.department_id=ANY(sqlc.arg(dept_ids)::bigint[]) OR dept.organization_id=ANY(sqlc.arg(org_ids)::bigint[])) ORDER BY d.id DESC LIMIT 500;

-- name: GovRetryDelivery :one
UPDATE gov_deliveries d SET status='pending',attempts=0,next_attempt_at=now(),lease_until=NULL FROM gov_notifications n JOIN iam_departments dept ON dept.tenant_id=n.tenant_id AND dept.id=n.department_id WHERE n.tenant_id=d.tenant_id AND n.id=d.message_id AND d.tenant_id=sqlc.arg(tenant_id) AND (sqlc.arg(all_data)::boolean OR n.recipient_id=sqlc.arg(own_id)::bigint OR n.department_id=ANY(sqlc.arg(dept_ids)::bigint[]) OR dept.organization_id=ANY(sqlc.arg(org_ids)::bigint[])) AND d.id=sqlc.arg(id) AND d.channel='smtp' AND d.status IN('failed','skipped') RETURNING d.id;

-- name: GovAccountEmail :one
SELECT email FROM iam_accounts WHERE tenant_id=$1 AND id=$2 AND status='active';

-- name: GovAuthorizedScopes :many
SELECT s.scope_key FROM gov_scopes s LEFT JOIN iam_departments d ON d.tenant_id=s.tenant_id AND d.id=s.department_id WHERE s.tenant_id=sqlc.arg(tenant_id) AND (sqlc.arg(all_data)::boolean OR s.account_id=sqlc.arg(own_id)::bigint OR s.organization_id=ANY(sqlc.arg(org_ids)::bigint[]) OR s.department_id=ANY(sqlc.arg(dept_ids)::bigint[]) OR d.organization_id=ANY(sqlc.arg(org_ids)::bigint[]));

-- name: GovLoginFailure :exec
INSERT INTO iam_login_attempts(attempt_key,failures,locked_until) VALUES(sqlc.arg(attempt_key),1,now()) ON CONFLICT(attempt_key) DO UPDATE SET
 failures=CASE WHEN (iam_login_attempts.failures>=sqlc.arg(max_failures)::int AND iam_login_attempts.locked_until<=now()) OR iam_login_attempts.locked_until<now()-make_interval(mins=>sqlc.arg(lock_minutes)::int) THEN 1 ELSE iam_login_attempts.failures+1 END,
 locked_until=CASE WHEN (iam_login_attempts.failures>=sqlc.arg(max_failures)::int AND iam_login_attempts.locked_until<=now()) OR iam_login_attempts.locked_until<now()-make_interval(mins=>sqlc.arg(lock_minutes)::int) THEN now() WHEN iam_login_attempts.failures+1>=sqlc.arg(max_failures)::int THEN now()+make_interval(mins=>sqlc.arg(lock_minutes)::int) ELSE now() END;

-- name: GovRuntimeDictionary :one
SELECT id,scope_key,code,spec,revision FROM gov_dictionaries WHERE scope_key='workspace:0' AND tenant_id=sqlc.arg(tenant_id) AND code=sqlc.arg(code) AND scope_key=ANY(sqlc.arg(visible_scopes)::text[]) AND revision>0 AND (spec->>'Visibility'='shared' OR sqlc.arg(all_data)::boolean OR scope_key=ANY(sqlc.arg(management_scopes)::text[]));

-- name: GovOwnedDictionary :one
SELECT id,scope_key,code,spec,revision FROM gov_dictionaries WHERE scope_key='workspace:0' AND tenant_id=$1 AND id=$2 AND ($3::boolean OR scope_key=ANY($4::text[]));

-- name: GovExpireDeliveryLeases :exec
UPDATE gov_deliveries SET status='failed',error_code='lease_expired',lease_until=NULL WHERE status='processing' AND lease_until<now() AND attempts>=5;

-- name: GovClampSessions :exec
UPDATE iam_sessions SET expires_at=LEAST(expires_at,created_at+make_interval(mins=>sqlc.arg(maximum_minutes)::int)) WHERE tenant_id=sqlc.arg(tenant_id);

-- name: GovExpireIdleSessions :exec
DELETE FROM iam_sessions WHERE tenant_id=sqlc.arg(tenant_id) AND sqlc.arg(idle_minutes)::int>0 AND last_seen_at+make_interval(mins=>sqlc.arg(idle_minutes)::int)<=now();
