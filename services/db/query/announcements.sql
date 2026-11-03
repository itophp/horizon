-- name: GovNotificationAudience :many
SELECT kind,target_id,name,count(DISTINCT account_id)::integer AS recipients FROM gov_notification_targets
WHERE tenant_id=sqlc.arg(tenant_id)
GROUP BY kind,target_id,name
HAVING CASE WHEN kind='account' THEN bool_or(sqlc.arg(all_data)::boolean OR account_id=sqlc.arg(own_id)::bigint OR organization_id=ANY(sqlc.arg(org_ids)::bigint[]) OR department_id=ANY(sqlc.arg(dept_ids)::bigint[]))
ELSE bool_and(sqlc.arg(all_data)::boolean OR account_id=sqlc.arg(own_id)::bigint OR organization_id=ANY(sqlc.arg(org_ids)::bigint[]) OR department_id=ANY(sqlc.arg(dept_ids)::bigint[])) END
ORDER BY kind,name,target_id LIMIT 20000;

-- name: GovAnnouncementRecipients :many
SELECT DISTINCT ON(account_id) account_id,department_id FROM gov_notification_targets
WHERE tenant_id=sqlc.arg(tenant_id) AND kind||':'||target_id::text=ANY(sqlc.arg(target_keys)::text[])
AND (sqlc.arg(all_data)::boolean OR account_id=sqlc.arg(own_id)::bigint OR organization_id=ANY(sqlc.arg(org_ids)::bigint[]) OR department_id=ANY(sqlc.arg(dept_ids)::bigint[]))
ORDER BY account_id,department_id LIMIT 10001;

-- name: GovCreateAnnouncement :one
INSERT INTO gov_announcements(tenant_id,sender_id,department_id,client_key,payload_hash,title,markdown,targets,email,recipient_count) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
ON CONFLICT(tenant_id,sender_id,client_key) DO UPDATE SET client_key=EXCLUDED.client_key
RETURNING id,payload_hash,title,markdown,recipient_count,created_at,department_id,(xmax=0)::boolean AS inserted;

-- name: GovSentAnnouncements :many
SELECT id,title,markdown,recipient_count,created_at FROM gov_announcements WHERE tenant_id=$1 AND sender_id=$2 AND department_id=$3 ORDER BY id DESC LIMIT 500;

-- name: GovBindAnnouncementFile :one
INSERT INTO gov_announcement_files(tenant_id,announcement_id,file_id) SELECT f.tenant_id,$2,f.id FROM gov_files f WHERE f.tenant_id=$1 AND f.id=$3 AND f.owner_id=$4 AND f.purpose='attachment' AND f.deleted_at IS NULL ON CONFLICT(tenant_id,announcement_id,file_id) DO UPDATE SET file_id=EXCLUDED.file_id RETURNING file_id;

-- name: GovAddAnnouncementMessage :one
INSERT INTO gov_notifications(tenant_id,announcement_id,recipient_id,department_id,payload) VALUES($1,$2,$3,$4,$5)
ON CONFLICT(tenant_id,announcement_id,recipient_id) DO UPDATE SET announcement_id=EXCLUDED.announcement_id RETURNING id;

-- name: GovNotificationFile :one
SELECT f.id,f.owner_id,f.organization_id,f.department_id,f.storage_profile_id,f.storage_key,f.name,f.mime,f.size,f.purpose,f.sha256,f.created_at
FROM gov_notifications n JOIN gov_announcement_files b ON b.tenant_id=n.tenant_id AND b.announcement_id=n.announcement_id
JOIN gov_files f ON f.tenant_id=b.tenant_id AND f.id=b.file_id
JOIN gov_notification_members m ON m.tenant_id=n.tenant_id AND m.account_id=n.recipient_id AND m.department_id=n.department_id
WHERE n.tenant_id=$1 AND n.id=$2 AND n.recipient_id=$3 AND f.id=$4 AND f.deleted_at IS NULL;

-- name: GovAnnouncementFiles :many
SELECT f.id,f.name,f.mime FROM gov_announcement_files b JOIN gov_files f ON f.tenant_id=b.tenant_id AND f.id=b.file_id WHERE b.tenant_id=$1 AND b.announcement_id=$2 AND f.deleted_at IS NULL;

-- name: GovAddInAppDelivery :exec
INSERT INTO gov_deliveries(tenant_id,message_id,channel,status,error_code) VALUES($1,$2,'in_app',$3,$4) ON CONFLICT DO NOTHING;

-- name: GovLockAttachment :exec
SELECT pg_advisory_xact_lock(hashtextextended(sqlc.arg(tenant_id)::text||':file:'||sqlc.arg(file_id)::bigint::text,0));

-- name: GovInboxMessage :one
SELECT n.id,n.payload,n.read_at,n.created_at FROM gov_notifications n WHERE n.tenant_id=$1 AND n.recipient_id=$2 AND n.id=$3
AND (n.department_id=$4 OR (n.announcement_id IS NOT NULL AND EXISTS(SELECT 1 FROM gov_notification_members m WHERE m.tenant_id=n.tenant_id AND m.account_id=n.recipient_id AND m.department_id=n.department_id)));
