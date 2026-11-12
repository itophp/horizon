-- name: DrivePersonalEnsure :one
INSERT INTO drive_spaces(tenant_id,kind,name,owner_id)
VALUES(sqlc.arg(tenant_id),'personal','Personal',sqlc.arg(owner_id))
ON CONFLICT (tenant_id,owner_id) WHERE kind='personal'
DO UPDATE SET owner_id=EXCLUDED.owner_id
RETURNING id;

-- name: DriveSpaceWriteLock :one
SELECT s.id FROM drive_spaces s
WHERE s.tenant_id=sqlc.arg(tenant_id) AND s.id=sqlc.arg(space_id)
 AND ((s.kind='personal' AND s.owner_id=sqlc.arg(actor_id)) OR
      (s.kind='shared' AND EXISTS (SELECT 1 FROM drive_members m WHERE m.tenant_id=s.tenant_id AND m.space_id=s.id
       AND m.role IN ('admin','editor') AND (m.account_id=sqlc.arg(actor_id) OR m.department_id=sqlc.arg(department_id)))))
FOR UPDATE OF s;

-- name: DriveFolderCreate :one
INSERT INTO drive_folders(tenant_id,space_id,parent_id,name,created_by)
SELECT s.tenant_id,s.id,sqlc.narg(parent_id),sqlc.arg(name),sqlc.arg(actor_id)
FROM drive_spaces s
WHERE s.tenant_id=sqlc.arg(tenant_id) AND s.id=sqlc.arg(space_id)
 AND ((s.kind='personal' AND s.owner_id=sqlc.arg(actor_id)) OR
      (s.kind='shared' AND EXISTS (SELECT 1 FROM drive_members m WHERE m.tenant_id=s.tenant_id AND m.space_id=s.id
       AND m.role IN ('admin','editor') AND (m.account_id=sqlc.arg(actor_id) OR m.department_id=sqlc.arg(department_id)))))
 AND (sqlc.narg(parent_id)::bigint IS NULL OR EXISTS
      (SELECT 1 FROM drive_folders p WHERE p.tenant_id=s.tenant_id AND p.space_id=s.id AND p.id=sqlc.narg(parent_id) AND p.deleted_at IS NULL))
RETURNING id;

-- name: DriveFolderList :many
SELECT f.id,f.name,COALESCE(f.parent_id,0)::bigint AS parent_id
FROM drive_folders f JOIN drive_spaces s ON s.tenant_id=f.tenant_id AND s.id=f.space_id
WHERE f.tenant_id=sqlc.arg(tenant_id) AND f.space_id=sqlc.arg(space_id) AND f.deleted_at IS NULL
 AND f.parent_id IS NOT DISTINCT FROM sqlc.narg(parent_id)::bigint
 AND ((s.kind='personal' AND s.owner_id=sqlc.arg(actor_id)) OR
      (s.kind='shared' AND EXISTS (SELECT 1 FROM drive_members m WHERE m.tenant_id=s.tenant_id AND m.space_id=s.id
       AND (m.account_id=sqlc.arg(actor_id) OR m.department_id=sqlc.arg(department_id)))))
ORDER BY f.name, f.id;

-- name: DriveFolderPath :many
WITH RECURSIVE ancestors AS (
 SELECT f.id,f.name,f.parent_id,ARRAY[f.id] AS visited,0 AS depth
 FROM drive_folders f JOIN drive_spaces s ON s.tenant_id=f.tenant_id AND s.id=f.space_id
 WHERE f.tenant_id=sqlc.arg(tenant_id) AND f.space_id=sqlc.arg(space_id) AND f.id=sqlc.arg(folder_id) AND f.deleted_at IS NULL
  AND ((s.kind='personal' AND s.owner_id=sqlc.arg(actor_id)) OR
   (s.kind='shared' AND EXISTS (SELECT 1 FROM drive_members m WHERE m.tenant_id=s.tenant_id AND m.space_id=s.id
    AND (m.account_id=sqlc.arg(actor_id) OR m.department_id=sqlc.arg(department_id)))))
 UNION ALL
 SELECT p.id,p.name,p.parent_id,a.visited||p.id,a.depth+1 FROM drive_folders p JOIN ancestors a ON p.id=a.parent_id
 WHERE p.tenant_id=sqlc.arg(tenant_id) AND p.space_id=sqlc.arg(space_id) AND p.deleted_at IS NULL AND NOT p.id=ANY(a.visited)
)
SELECT id,name,COALESCE(parent_id,0)::bigint AS parent_id FROM ancestors ORDER BY depth DESC;

-- name: DriveFolderRename :execrows
UPDATE drive_folders f SET name=sqlc.arg(name),updated_at=now()
WHERE f.tenant_id=sqlc.arg(tenant_id) AND f.space_id=sqlc.arg(space_id) AND f.id=sqlc.arg(id) AND f.deleted_at IS NULL
 AND EXISTS (SELECT 1 FROM drive_spaces s WHERE s.tenant_id=f.tenant_id AND s.id=f.space_id
  AND ((s.kind='personal' AND s.owner_id=sqlc.arg(actor_id)) OR (s.kind='shared' AND EXISTS
   (SELECT 1 FROM drive_members m WHERE m.tenant_id=s.tenant_id AND m.space_id=s.id AND m.role IN ('admin','editor')
    AND (m.account_id=sqlc.arg(actor_id) OR m.department_id=sqlc.arg(department_id))))));

-- name: DriveFolderMove :execrows
WITH RECURSIVE descendants(id) AS (
 SELECT f.id FROM drive_folders f WHERE f.tenant_id=sqlc.arg(tenant_id) AND f.space_id=sqlc.arg(space_id) AND f.id=sqlc.arg(id) AND f.deleted_at IS NULL
 UNION
 SELECT c.id FROM drive_folders c JOIN descendants d ON c.parent_id=d.id
 WHERE c.tenant_id=sqlc.arg(tenant_id) AND c.space_id=sqlc.arg(space_id) AND c.deleted_at IS NULL
)
UPDATE drive_folders f SET parent_id=sqlc.narg(parent_id),updated_at=now()
WHERE f.tenant_id=sqlc.arg(tenant_id) AND f.space_id=sqlc.arg(space_id) AND f.id=sqlc.arg(id) AND f.deleted_at IS NULL
 AND (sqlc.narg(parent_id)::bigint IS NULL OR (EXISTS
  (SELECT 1 FROM drive_folders p WHERE p.tenant_id=f.tenant_id AND p.space_id=f.space_id AND p.id=sqlc.narg(parent_id) AND p.deleted_at IS NULL)
  AND NOT EXISTS (SELECT 1 FROM descendants d WHERE d.id=sqlc.narg(parent_id))))
 AND EXISTS (SELECT 1 FROM drive_spaces s WHERE s.tenant_id=f.tenant_id AND s.id=f.space_id
  AND ((s.kind='personal' AND s.owner_id=sqlc.arg(actor_id)) OR (s.kind='shared' AND EXISTS
   (SELECT 1 FROM drive_members m WHERE m.tenant_id=s.tenant_id AND m.space_id=s.id AND m.role IN ('admin','editor')
    AND (m.account_id=sqlc.arg(actor_id) OR m.department_id=sqlc.arg(department_id))))));

-- name: DriveFolderTrashBatchCreate :one
INSERT INTO drive_trash_batches(tenant_id,space_id,deleted_by)
SELECT s.tenant_id,s.id,sqlc.arg(actor_id)
FROM drive_spaces s JOIN drive_folders f ON f.tenant_id=s.tenant_id AND f.space_id=s.id
WHERE s.tenant_id=sqlc.arg(tenant_id) AND s.id=sqlc.arg(space_id) AND f.id=sqlc.arg(folder_id) AND f.deleted_at IS NULL
 AND ((s.kind='personal' AND s.owner_id=sqlc.arg(actor_id)) OR
  (s.kind='shared' AND EXISTS (SELECT 1 FROM drive_members m WHERE m.tenant_id=s.tenant_id AND m.space_id=s.id
   AND m.role IN ('admin','editor') AND (m.account_id=sqlc.arg(actor_id) OR m.department_id=sqlc.arg(department_id)))))
RETURNING id;

-- name: DriveFolderTrashMark :execrows
WITH RECURSIVE tree(id) AS (
 SELECT f.id FROM drive_folders f WHERE f.tenant_id=sqlc.arg(tenant_id) AND f.space_id=sqlc.arg(space_id)
  AND f.id=sqlc.arg(folder_id) AND f.deleted_at IS NULL
 UNION
 SELECT c.id FROM drive_folders c JOIN tree p ON p.id=c.parent_id
 WHERE c.tenant_id=sqlc.arg(tenant_id) AND c.space_id=sqlc.arg(space_id) AND c.deleted_at IS NULL
)
UPDATE drive_folders f SET deleted_at=now(),trash_batch_id=sqlc.arg(batch_id),trash_root=(f.id=sqlc.arg(folder_id)),updated_at=now()
WHERE f.tenant_id=sqlc.arg(tenant_id) AND f.space_id=sqlc.arg(space_id) AND f.id IN (SELECT id FROM tree);

-- name: DriveFolderTrashMarkFiles :execrows
UPDATE drive_files f SET deleted_at=now(),trash_batch_id=sqlc.arg(batch_id),trash_root=false,updated_at=now()
WHERE f.tenant_id=sqlc.arg(tenant_id) AND f.space_id=sqlc.arg(space_id) AND f.deleted_at IS NULL
 AND EXISTS (SELECT 1 FROM drive_folders d WHERE d.tenant_id=f.tenant_id AND d.space_id=f.space_id
  AND d.id=f.folder_id AND d.trash_batch_id=sqlc.arg(batch_id));

-- name: DriveTrashList :many
SELECT b.id,f.id AS folder_id,f.name,COALESCE(f.parent_id,0)::bigint AS parent_id,b.deleted_at,b.expires_at
FROM drive_trash_batches b JOIN drive_folders f ON f.tenant_id=b.tenant_id AND f.space_id=b.space_id AND f.trash_batch_id=b.id AND f.trash_root
JOIN drive_spaces s ON s.tenant_id=b.tenant_id AND s.id=b.space_id
WHERE b.tenant_id=sqlc.arg(tenant_id) AND b.space_id=sqlc.arg(space_id) AND b.expires_at>now() AND b.purging_at IS NULL
 AND ((s.kind='personal' AND s.owner_id=sqlc.arg(actor_id)) OR
  (s.kind='shared' AND EXISTS (SELECT 1 FROM drive_members m WHERE m.tenant_id=s.tenant_id AND m.space_id=s.id
   AND m.role IN ('admin','editor') AND (m.account_id=sqlc.arg(actor_id) OR m.department_id=sqlc.arg(department_id)))))
ORDER BY b.id DESC;

-- name: DriveTrashRootPrepare :one
UPDATE drive_folders f SET parent_id=sqlc.narg(parent_id),name=sqlc.arg(name),updated_at=now()
FROM drive_trash_batches b,drive_spaces s
WHERE f.tenant_id=sqlc.arg(tenant_id) AND f.space_id=sqlc.arg(space_id) AND f.trash_batch_id=sqlc.arg(batch_id) AND f.trash_root
 AND b.tenant_id=f.tenant_id AND b.space_id=f.space_id AND b.id=f.trash_batch_id AND b.expires_at>now() AND b.purging_at IS NULL
 AND s.tenant_id=f.tenant_id AND s.id=f.space_id
 AND (sqlc.narg(parent_id)::bigint IS NULL OR EXISTS (SELECT 1 FROM drive_folders p WHERE p.tenant_id=f.tenant_id
   AND p.space_id=f.space_id AND p.id=sqlc.narg(parent_id) AND p.deleted_at IS NULL))
 AND ((s.kind='personal' AND s.owner_id=sqlc.arg(actor_id)) OR
  (s.kind='shared' AND EXISTS (SELECT 1 FROM drive_members m WHERE m.tenant_id=s.tenant_id AND m.space_id=s.id
   AND m.role IN ('admin','editor') AND (m.account_id=sqlc.arg(actor_id) OR m.department_id=sqlc.arg(department_id)))))
RETURNING f.id;

-- name: DriveTrashRestoreFolders :execrows
UPDATE drive_folders SET deleted_at=NULL,trash_batch_id=NULL,trash_root=false,updated_at=now()
WHERE tenant_id=sqlc.arg(tenant_id) AND space_id=sqlc.arg(space_id) AND trash_batch_id=sqlc.arg(batch_id);

-- name: DriveTrashRestoreFiles :execrows
UPDATE drive_files SET deleted_at=NULL,trash_batch_id=NULL,trash_root=false,updated_at=now()
WHERE tenant_id=sqlc.arg(tenant_id) AND space_id=sqlc.arg(space_id) AND trash_batch_id=sqlc.arg(batch_id);

-- name: DriveTrashBatchDelete :execrows
DELETE FROM drive_trash_batches WHERE tenant_id=sqlc.arg(tenant_id) AND space_id=sqlc.arg(space_id) AND id=sqlc.arg(batch_id);

-- name: DriveSharedCreate :one
INSERT INTO drive_spaces(tenant_id,kind,name,owner_id)
VALUES(sqlc.arg(tenant_id),'shared',sqlc.arg(name),sqlc.arg(owner_id))
RETURNING id;

-- name: DriveCreatorGrant :exec
INSERT INTO drive_members(tenant_id,space_id,account_id,role)
VALUES(sqlc.arg(tenant_id),sqlc.arg(space_id),sqlc.arg(account_id),'admin');

-- name: DriveSpaceList :many
SELECT s.id, s.kind, s.name, s.owner_id,
 CASE WHEN s.kind='personal' THEN 'admin'
      WHEN bool_or(m.role='admin') THEN 'admin'
      WHEN bool_or(m.role='editor') THEN 'editor' ELSE 'viewer' END AS role
FROM drive_spaces s
LEFT JOIN drive_members m ON m.tenant_id=s.tenant_id AND m.space_id=s.id
 AND (m.account_id=sqlc.arg(account_id) OR m.department_id=sqlc.arg(department_id))
WHERE s.tenant_id=sqlc.arg(tenant_id)
 AND ((s.kind='personal' AND s.owner_id=sqlc.arg(account_id)) OR (s.kind='shared' AND m.id IS NOT NULL))
GROUP BY s.id, s.kind, s.name, s.owner_id
ORDER BY s.id DESC;

-- name: DriveSharedAdminLock :one
-- Serialize membership changes for a space before evaluating the actor's
-- membership. The department is the authenticated session's active one.
SELECT s.id FROM drive_spaces s
WHERE s.tenant_id=sqlc.arg(tenant_id) AND s.id=sqlc.arg(space_id) AND s.kind='shared'
 AND EXISTS (SELECT 1 FROM drive_members m WHERE m.tenant_id=s.tenant_id AND m.space_id=s.id
  AND m.role='admin' AND (m.account_id=sqlc.arg(actor_id) OR m.department_id=sqlc.arg(department_id)))
FOR UPDATE OF s;

-- name: DriveSharedAdminAllowed :one
-- This separate statement takes a fresh READ COMMITTED snapshot after the
-- space lock has been acquired, so a revoked administrator cannot use a stale
-- pre-lock membership check.
SELECT EXISTS (SELECT 1 FROM drive_members m
 WHERE m.tenant_id=sqlc.arg(tenant_id) AND m.space_id=sqlc.arg(space_id) AND m.role='admin'
 AND (m.account_id=sqlc.arg(actor_id) OR m.department_id=sqlc.arg(department_id))) AS allowed;

-- name: DriveMemberList :many
SELECT m.id, m.account_id, m.department_id, m.role, m.created_at
FROM drive_members m
WHERE m.tenant_id=sqlc.arg(tenant_id) AND m.space_id=sqlc.arg(space_id)
 AND EXISTS (SELECT 1 FROM drive_members a JOIN drive_spaces s ON s.tenant_id=a.tenant_id AND s.id=a.space_id
   WHERE a.tenant_id=m.tenant_id AND a.space_id=m.space_id AND s.kind='shared' AND a.role='admin'
   AND (a.account_id=sqlc.arg(actor_id) OR a.department_id=sqlc.arg(department_id)))
ORDER BY m.id;

-- name: DriveMemberAddAccount :one
INSERT INTO drive_members(tenant_id,space_id,account_id,role)
SELECT sqlc.arg(tenant_id),sqlc.arg(space_id),a.id,sqlc.arg(role)
FROM iam_accounts a
WHERE a.tenant_id=sqlc.arg(tenant_id) AND a.id=sqlc.arg(account_id) AND a.status='active'
RETURNING id;

-- name: DriveMemberAddDepartment :one
INSERT INTO drive_members(tenant_id,space_id,department_id,role)
SELECT sqlc.arg(tenant_id),sqlc.arg(space_id),d.id,sqlc.arg(role)
FROM iam_departments d
WHERE d.tenant_id=sqlc.arg(tenant_id) AND d.id=sqlc.arg(department_id) AND d.status='active'
RETURNING id;

-- name: DriveMemberChangeRole :execrows
UPDATE drive_members SET role=sqlc.arg(role)
WHERE tenant_id=sqlc.arg(tenant_id) AND space_id=sqlc.arg(space_id) AND id=sqlc.arg(member_id);

-- name: DriveMemberRemove :execrows
DELETE FROM drive_members
WHERE tenant_id=sqlc.arg(tenant_id) AND space_id=sqlc.arg(space_id) AND id=sqlc.arg(member_id);

-- name: DriveAdminCandidates :many
-- Permission is evaluated by the identity service for each active candidate;
-- membership alone is not proof that a candidate may use the cloud drive.
SELECT a.id AS account_id,a.department_id FROM iam_accounts a
JOIN iam_departments d ON d.tenant_id=a.tenant_id AND d.id=a.department_id AND d.status='active'
JOIN drive_members m ON m.tenant_id=a.tenant_id AND m.space_id=sqlc.arg(space_id) AND m.role='admin'
 AND (m.account_id=a.id OR m.department_id=a.department_id)
WHERE a.tenant_id=sqlc.arg(tenant_id) AND a.status='active'
UNION
SELECT a.id AS account_id,dm.department_id FROM iam_accounts a
JOIN iam_department_memberships dm ON dm.tenant_id=a.tenant_id AND dm.account_id=a.id
JOIN iam_departments d ON d.tenant_id=dm.tenant_id AND d.id=dm.department_id AND d.status='active'
JOIN drive_members m ON m.tenant_id=a.tenant_id AND m.space_id=sqlc.arg(space_id) AND m.role='admin'
 AND (m.account_id=a.id OR m.department_id=dm.department_id)
WHERE a.tenant_id=sqlc.arg(tenant_id) AND a.status='active';

-- name: DriveUploadCreate :one
INSERT INTO drive_uploads(tenant_id,space_id,folder_id,owner_id,storage_profile_id,object_key,name,size)
SELECT s.tenant_id,s.id,sqlc.narg(folder_id),sqlc.arg(actor_id),sqlc.arg(storage_profile_id),sqlc.arg(object_key),sqlc.arg(name),sqlc.arg(size)
FROM drive_spaces s
WHERE s.tenant_id=sqlc.arg(tenant_id) AND s.id=sqlc.arg(space_id)
 AND ((s.kind='personal' AND s.owner_id=sqlc.arg(actor_id)) OR
  (s.kind='shared' AND EXISTS (SELECT 1 FROM drive_members m WHERE m.tenant_id=s.tenant_id AND m.space_id=s.id
   AND m.role IN ('admin','editor') AND (m.account_id=sqlc.arg(actor_id) OR m.department_id=sqlc.arg(department_id)))))
 AND (sqlc.narg(folder_id)::bigint IS NULL OR EXISTS (SELECT 1 FROM drive_folders f WHERE f.tenant_id=s.tenant_id
  AND f.space_id=s.id AND f.id=sqlc.narg(folder_id) AND f.deleted_at IS NULL))
RETURNING id;

-- name: DriveSpaceReservedBytes :one
SELECT (COALESCE((SELECT sum(f.size) FROM drive_files d JOIN gov_files f ON f.tenant_id=d.tenant_id AND f.id=d.file_id
 WHERE d.tenant_id=sqlc.arg(tenant_id) AND d.space_id=sqlc.arg(space_id)),0)
 + COALESCE((SELECT sum(u.size) FROM drive_uploads u WHERE u.tenant_id=sqlc.arg(tenant_id) AND u.space_id=sqlc.arg(space_id)
   AND u.state='open' AND u.expires_at>now()),0))::bigint AS reserved_bytes;

-- name: DriveSpaceUsage :one
SELECT COALESCE((SELECT sum(f.size) FROM drive_files d JOIN gov_files f ON f.tenant_id=d.tenant_id AND f.id=d.file_id
 WHERE d.tenant_id=s.tenant_id AND d.space_id=s.id),0)::bigint AS used_bytes,
 COALESCE((SELECT sum(u.size) FROM drive_uploads u WHERE u.tenant_id=s.tenant_id AND u.space_id=s.id
 AND u.state='open' AND u.expires_at>now()),0)::bigint AS pending_bytes
FROM drive_spaces s WHERE s.tenant_id=sqlc.arg(tenant_id) AND s.id=sqlc.arg(space_id)
 AND ((s.kind='personal' AND s.owner_id=sqlc.arg(actor_id)) OR
  (s.kind='shared' AND EXISTS (SELECT 1 FROM drive_members m WHERE m.tenant_id=s.tenant_id AND m.space_id=s.id
   AND (m.account_id=sqlc.arg(actor_id) OR m.department_id=sqlc.arg(department_id)))));

-- name: DriveUploadChunkReserve :exec
INSERT INTO drive_upload_chunks(tenant_id,upload_id,chunk_index,storage_key,expected_size)
VALUES(sqlc.arg(tenant_id),sqlc.arg(upload_id),sqlc.arg(chunk_index),sqlc.arg(storage_key),sqlc.arg(expected_size));

-- name: DriveUploadStatus :one
SELECT u.id,u.space_id,COALESCE(u.folder_id,0)::bigint AS folder_id,u.name,u.size,u.state,u.expires_at,
 COALESCE((SELECT d.id FROM drive_files d WHERE d.tenant_id=u.tenant_id AND d.file_id=u.file_id),0)::bigint AS completed_file_id
FROM drive_uploads u JOIN drive_spaces s ON s.tenant_id=u.tenant_id AND s.id=u.space_id
WHERE u.tenant_id=sqlc.arg(tenant_id) AND u.id=sqlc.arg(upload_id) AND u.owner_id=sqlc.arg(actor_id)
 AND ((s.kind='personal' AND s.owner_id=sqlc.arg(actor_id)) OR
  (s.kind='shared' AND EXISTS (SELECT 1 FROM drive_members m WHERE m.tenant_id=s.tenant_id AND m.space_id=s.id
   AND m.role IN ('admin','editor') AND (m.account_id=sqlc.arg(actor_id) OR m.department_id=sqlc.arg(department_id)))));

-- name: DriveUploadCompletedChunks :many
SELECT c.chunk_index,c.sha256 FROM drive_upload_chunks c JOIN drive_uploads u ON u.tenant_id=c.tenant_id AND u.id=c.upload_id
JOIN drive_spaces s ON s.tenant_id=u.tenant_id AND s.id=u.space_id
WHERE c.tenant_id=sqlc.arg(tenant_id) AND c.upload_id=sqlc.arg(upload_id) AND u.owner_id=sqlc.arg(actor_id) AND c.uploaded_at IS NOT NULL
 AND ((s.kind='personal' AND s.owner_id=sqlc.arg(actor_id)) OR
  (s.kind='shared' AND EXISTS (SELECT 1 FROM drive_members m WHERE m.tenant_id=s.tenant_id AND m.space_id=s.id
   AND m.role IN ('admin','editor') AND (m.account_id=sqlc.arg(actor_id) OR m.department_id=sqlc.arg(department_id)))))
ORDER BY c.chunk_index;

-- name: DriveUploadChunkLock :one
SELECT u.storage_profile_id,c.storage_key,c.expected_size,c.sha256,c.uploaded_at
FROM drive_uploads u JOIN drive_spaces s ON s.tenant_id=u.tenant_id AND s.id=u.space_id
JOIN drive_upload_chunks c ON c.tenant_id=u.tenant_id AND c.upload_id=u.id
WHERE u.tenant_id=sqlc.arg(tenant_id) AND u.id=sqlc.arg(upload_id) AND u.space_id=sqlc.arg(space_id)
 AND u.owner_id=sqlc.arg(actor_id) AND u.state='open' AND u.expires_at>now() AND c.chunk_index=sqlc.arg(chunk_index)
 AND (u.folder_id IS NULL OR EXISTS (SELECT 1 FROM drive_folders f WHERE f.tenant_id=u.tenant_id AND f.space_id=u.space_id AND f.id=u.folder_id AND f.deleted_at IS NULL))
 AND ((s.kind='personal' AND s.owner_id=sqlc.arg(actor_id)) OR
  (s.kind='shared' AND EXISTS (SELECT 1 FROM drive_members m WHERE m.tenant_id=s.tenant_id AND m.space_id=s.id
   AND m.role IN ('admin','editor') AND (m.account_id=sqlc.arg(actor_id) OR m.department_id=sqlc.arg(department_id)))))
FOR UPDATE OF u,c;

-- name: DriveUploadChunkMark :execrows
UPDATE drive_upload_chunks SET sha256=sqlc.arg(sha256),uploaded_at=now()
WHERE tenant_id=sqlc.arg(tenant_id) AND upload_id=sqlc.arg(upload_id) AND chunk_index=sqlc.arg(chunk_index)
 AND (sha256 IS NULL OR sha256=sqlc.arg(sha256));

-- name: DriveUploadCompleteLock :one
SELECT u.storage_profile_id,u.object_key,u.folder_id,u.name,u.size
FROM drive_uploads u JOIN drive_spaces s ON s.tenant_id=u.tenant_id AND s.id=u.space_id
WHERE u.tenant_id=sqlc.arg(tenant_id) AND u.id=sqlc.arg(upload_id) AND u.space_id=sqlc.arg(space_id)
 AND u.owner_id=sqlc.arg(actor_id) AND u.state='open' AND u.expires_at>now()
 AND (u.folder_id IS NULL OR EXISTS (SELECT 1 FROM drive_folders f WHERE f.tenant_id=u.tenant_id AND f.space_id=u.space_id AND f.id=u.folder_id AND f.deleted_at IS NULL))
 AND ((s.kind='personal' AND s.owner_id=sqlc.arg(actor_id)) OR
  (s.kind='shared' AND EXISTS (SELECT 1 FROM drive_members m WHERE m.tenant_id=s.tenant_id AND m.space_id=s.id
   AND m.role IN ('admin','editor') AND (m.account_id=sqlc.arg(actor_id) OR m.department_id=sqlc.arg(department_id)))))
FOR UPDATE OF u;

-- name: DriveUploadChunksForAssembly :many
SELECT c.chunk_index,c.storage_key,c.expected_size,c.sha256,c.uploaded_at FROM drive_upload_chunks c
WHERE c.tenant_id=sqlc.arg(tenant_id) AND c.upload_id=sqlc.arg(upload_id)
ORDER BY c.chunk_index;

-- name: DriveFileCreate :one
INSERT INTO drive_files(tenant_id,space_id,folder_id,file_id,name,created_by)
VALUES(sqlc.arg(tenant_id),sqlc.arg(space_id),sqlc.narg(folder_id),sqlc.arg(file_id),sqlc.arg(name),sqlc.arg(actor_id))
RETURNING id;

-- name: DriveUploadComplete :execrows
UPDATE drive_uploads SET state='completed',file_id=sqlc.arg(file_id)
WHERE tenant_id=sqlc.arg(tenant_id) AND id=sqlc.arg(upload_id) AND space_id=sqlc.arg(space_id) AND state='open' AND expires_at>now();

-- name: DriveUploadCancel :execrows
UPDATE drive_uploads u SET state='cancelled'
FROM drive_spaces s
WHERE u.tenant_id=sqlc.arg(tenant_id) AND u.id=sqlc.arg(upload_id) AND u.space_id=sqlc.arg(space_id)
 AND u.owner_id=sqlc.arg(actor_id) AND u.state='open' AND s.tenant_id=u.tenant_id AND s.id=u.space_id
 AND ((s.kind='personal' AND s.owner_id=sqlc.arg(actor_id)) OR
  (s.kind='shared' AND EXISTS (SELECT 1 FROM drive_members m WHERE m.tenant_id=s.tenant_id AND m.space_id=s.id
   AND m.role IN ('admin','editor') AND (m.account_id=sqlc.arg(actor_id) OR m.department_id=sqlc.arg(department_id)))));

-- name: DriveFileList :many
SELECT d.id,d.name,d.folder_id,f.size,f.created_at,f.sha256
FROM drive_files d JOIN drive_spaces s ON s.tenant_id=d.tenant_id AND s.id=d.space_id
JOIN gov_files f ON f.tenant_id=d.tenant_id AND f.id=d.file_id AND f.purpose='cloud_drive' AND f.deleted_at IS NULL
WHERE d.tenant_id=sqlc.arg(tenant_id) AND d.space_id=sqlc.arg(space_id) AND d.deleted_at IS NULL
 AND d.folder_id IS NOT DISTINCT FROM sqlc.narg(folder_id)::bigint
 AND ((s.kind='personal' AND s.owner_id=sqlc.arg(actor_id)) OR
  (s.kind='shared' AND EXISTS (SELECT 1 FROM drive_members m WHERE m.tenant_id=s.tenant_id AND m.space_id=s.id
   AND (m.account_id=sqlc.arg(actor_id) OR m.department_id=sqlc.arg(department_id)))))
ORDER BY d.name,d.id;

-- name: DriveFileRead :one
SELECT d.id,d.name,f.size,f.sha256,f.storage_profile_id,f.storage_key
FROM drive_files d JOIN drive_spaces s ON s.tenant_id=d.tenant_id AND s.id=d.space_id
JOIN gov_files f ON f.tenant_id=d.tenant_id AND f.id=d.file_id AND f.purpose='cloud_drive' AND f.deleted_at IS NULL
WHERE d.tenant_id=sqlc.arg(tenant_id) AND d.space_id=sqlc.arg(space_id) AND d.id=sqlc.arg(file_id) AND d.deleted_at IS NULL
 AND ((s.kind='personal' AND s.owner_id=sqlc.arg(actor_id)) OR
  (s.kind='shared' AND EXISTS (SELECT 1 FROM drive_members m WHERE m.tenant_id=s.tenant_id AND m.space_id=s.id
   AND (m.account_id=sqlc.arg(actor_id) OR m.department_id=sqlc.arg(department_id)))));

-- name: DriveFileRename :execrows
UPDATE drive_files f SET name=sqlc.arg(name),updated_at=now()
WHERE f.tenant_id=sqlc.arg(tenant_id) AND f.space_id=sqlc.arg(space_id) AND f.id=sqlc.arg(file_id) AND f.deleted_at IS NULL
 AND EXISTS (SELECT 1 FROM drive_spaces s WHERE s.tenant_id=f.tenant_id AND s.id=f.space_id
  AND ((s.kind='personal' AND s.owner_id=sqlc.arg(actor_id)) OR (s.kind='shared' AND EXISTS
   (SELECT 1 FROM drive_members m WHERE m.tenant_id=s.tenant_id AND m.space_id=s.id AND m.role IN ('admin','editor')
    AND (m.account_id=sqlc.arg(actor_id) OR m.department_id=sqlc.arg(department_id))))));

-- name: DriveFileMove :execrows
UPDATE drive_files f SET folder_id=sqlc.narg(folder_id),updated_at=now()
WHERE f.tenant_id=sqlc.arg(tenant_id) AND f.space_id=sqlc.arg(space_id) AND f.id=sqlc.arg(file_id) AND f.deleted_at IS NULL
 AND (sqlc.narg(folder_id)::bigint IS NULL OR EXISTS (SELECT 1 FROM drive_folders d
  WHERE d.tenant_id=f.tenant_id AND d.space_id=f.space_id AND d.id=sqlc.narg(folder_id) AND d.deleted_at IS NULL))
 AND EXISTS (SELECT 1 FROM drive_spaces s WHERE s.tenant_id=f.tenant_id AND s.id=f.space_id
  AND ((s.kind='personal' AND s.owner_id=sqlc.arg(actor_id)) OR (s.kind='shared' AND EXISTS
   (SELECT 1 FROM drive_members m WHERE m.tenant_id=s.tenant_id AND m.space_id=s.id AND m.role IN ('admin','editor')
    AND (m.account_id=sqlc.arg(actor_id) OR m.department_id=sqlc.arg(department_id))))));

-- name: DriveFileTrashBatchCreate :one
INSERT INTO drive_trash_batches(tenant_id,space_id,deleted_by)
SELECT s.tenant_id,s.id,sqlc.arg(actor_id)
FROM drive_spaces s JOIN drive_files f ON f.tenant_id=s.tenant_id AND f.space_id=s.id
WHERE s.tenant_id=sqlc.arg(tenant_id) AND s.id=sqlc.arg(space_id) AND f.id=sqlc.arg(file_id) AND f.deleted_at IS NULL
 AND ((s.kind='personal' AND s.owner_id=sqlc.arg(actor_id)) OR
  (s.kind='shared' AND EXISTS (SELECT 1 FROM drive_members m WHERE m.tenant_id=s.tenant_id AND m.space_id=s.id
   AND m.role IN ('admin','editor') AND (m.account_id=sqlc.arg(actor_id) OR m.department_id=sqlc.arg(department_id)))))
RETURNING id;

-- name: DriveFileTrashMark :execrows
UPDATE drive_files SET deleted_at=now(),trash_batch_id=sqlc.arg(batch_id),trash_root=true,updated_at=now()
WHERE tenant_id=sqlc.arg(tenant_id) AND space_id=sqlc.arg(space_id) AND id=sqlc.arg(file_id) AND deleted_at IS NULL;

-- name: DriveTrashFilesList :many
SELECT b.id,f.id AS file_id,f.name,COALESCE(f.folder_id,0)::bigint AS folder_id,b.deleted_at,b.expires_at
FROM drive_trash_batches b JOIN drive_files f ON f.tenant_id=b.tenant_id AND f.space_id=b.space_id AND f.trash_batch_id=b.id AND f.trash_root
JOIN drive_spaces s ON s.tenant_id=b.tenant_id AND s.id=b.space_id
WHERE b.tenant_id=sqlc.arg(tenant_id) AND b.space_id=sqlc.arg(space_id) AND b.expires_at>now() AND b.purging_at IS NULL
 AND ((s.kind='personal' AND s.owner_id=sqlc.arg(actor_id)) OR
  (s.kind='shared' AND EXISTS (SELECT 1 FROM drive_members m WHERE m.tenant_id=s.tenant_id AND m.space_id=s.id
   AND m.role IN ('admin','editor') AND (m.account_id=sqlc.arg(actor_id) OR m.department_id=sqlc.arg(department_id)))))
ORDER BY b.id DESC;

-- name: DriveTrashFileRestore :one
UPDATE drive_files f SET folder_id=sqlc.narg(folder_id),name=sqlc.arg(name),deleted_at=NULL,trash_batch_id=NULL,trash_root=false,updated_at=now()
FROM drive_trash_batches b,drive_spaces s
WHERE f.tenant_id=sqlc.arg(tenant_id) AND f.space_id=sqlc.arg(space_id) AND f.trash_batch_id=sqlc.arg(batch_id) AND f.trash_root
 AND b.tenant_id=f.tenant_id AND b.space_id=f.space_id AND b.id=f.trash_batch_id AND b.expires_at>now() AND b.purging_at IS NULL
 AND s.tenant_id=f.tenant_id AND s.id=f.space_id
 AND (sqlc.narg(folder_id)::bigint IS NULL OR EXISTS (SELECT 1 FROM drive_folders d WHERE d.tenant_id=f.tenant_id
   AND d.space_id=f.space_id AND d.id=sqlc.narg(folder_id) AND d.deleted_at IS NULL))
 AND ((s.kind='personal' AND s.owner_id=sqlc.arg(actor_id)) OR
  (s.kind='shared' AND EXISTS (SELECT 1 FROM drive_members m WHERE m.tenant_id=s.tenant_id AND m.space_id=s.id
   AND m.role IN ('admin','editor') AND (m.account_id=sqlc.arg(actor_id) OR m.department_id=sqlc.arg(department_id)))))
RETURNING f.id;

-- name: DriveTrashPurgeRequest :execrows
UPDATE drive_trash_batches b SET purging_at=now()
FROM drive_spaces s
WHERE b.tenant_id=sqlc.arg(tenant_id) AND b.space_id=sqlc.arg(space_id) AND b.id=sqlc.arg(batch_id)
 AND b.purging_at IS NULL AND s.tenant_id=b.tenant_id AND s.id=b.space_id
 AND ((s.kind='personal' AND s.owner_id=sqlc.arg(actor_id)) OR
  (s.kind='shared' AND EXISTS (SELECT 1 FROM drive_members m WHERE m.tenant_id=s.tenant_id AND m.space_id=s.id
   AND m.role='admin' AND (m.account_id=sqlc.arg(actor_id) OR m.department_id=sqlc.arg(department_id)))));

-- name: DriveSystemSpaceLock :one
SELECT id FROM drive_spaces WHERE tenant_id=sqlc.arg(tenant_id) AND id=sqlc.arg(space_id) FOR UPDATE;

-- name: DriveTrashClaimExpired :execrows
UPDATE drive_trash_batches SET purging_at=now()
WHERE tenant_id=sqlc.arg(tenant_id) AND space_id=sqlc.arg(space_id) AND id=sqlc.arg(batch_id)
 AND purging_at IS NULL AND expires_at<=now();

-- name: DriveTrashPurgeLock :one
SELECT id FROM drive_trash_batches WHERE tenant_id=sqlc.arg(tenant_id) AND space_id=sqlc.arg(space_id)
 AND id=sqlc.arg(batch_id) AND purging_at IS NOT NULL FOR UPDATE;

-- name: DrivePurgeFiles :many
SELECT d.id AS drive_id,f.id,f.storage_profile_id,f.storage_key FROM drive_files d JOIN gov_files f ON f.tenant_id=d.tenant_id AND f.id=d.file_id
WHERE d.tenant_id=sqlc.arg(tenant_id) AND d.space_id=sqlc.arg(space_id) AND d.trash_batch_id=sqlc.arg(batch_id)
ORDER BY d.id LIMIT 50;

-- name: DrivePurgeStoredFile :execrows
UPDATE gov_files SET deleted_at=COALESCE(deleted_at,now()),purged_at=COALESCE(purged_at,now())
WHERE tenant_id=sqlc.arg(tenant_id) AND id=sqlc.arg(file_id) AND purpose='cloud_drive';

-- name: DrivePurgeOneFile :execrows
DELETE FROM drive_files WHERE tenant_id=sqlc.arg(tenant_id) AND space_id=sqlc.arg(space_id)
 AND trash_batch_id=sqlc.arg(batch_id) AND id=sqlc.arg(drive_id);

-- name: DrivePurgeFilesRemaining :one
SELECT EXISTS (SELECT 1 FROM drive_files WHERE tenant_id=sqlc.arg(tenant_id) AND space_id=sqlc.arg(space_id)
 AND trash_batch_id=sqlc.arg(batch_id)) AS remaining;

-- name: DrivePurgeDetachFiles :execrows
UPDATE drive_files f SET folder_id=NULL WHERE f.tenant_id=sqlc.arg(tenant_id) AND f.space_id=sqlc.arg(space_id)
 AND f.deleted_at IS NOT NULL AND f.trash_batch_id<>sqlc.arg(batch_id)
 AND EXISTS (SELECT 1 FROM drive_folders d WHERE d.tenant_id=f.tenant_id AND d.space_id=f.space_id
  AND d.id=f.folder_id AND d.trash_batch_id=sqlc.arg(batch_id));

-- name: DrivePurgeDetachFolders :execrows
UPDATE drive_folders f SET parent_id=NULL WHERE f.tenant_id=sqlc.arg(tenant_id) AND f.space_id=sqlc.arg(space_id)
 AND f.deleted_at IS NOT NULL AND f.trash_batch_id<>sqlc.arg(batch_id)
 AND EXISTS (SELECT 1 FROM drive_folders d WHERE d.tenant_id=f.tenant_id AND d.space_id=f.space_id
  AND d.id=f.parent_id AND d.trash_batch_id=sqlc.arg(batch_id));

-- name: DrivePurgeDetachUploads :execrows
UPDATE drive_uploads u SET folder_id=NULL,state=CASE WHEN state='open' THEN 'cancelled' ELSE state END
WHERE u.tenant_id=sqlc.arg(tenant_id) AND u.space_id=sqlc.arg(space_id)
 AND EXISTS (SELECT 1 FROM drive_folders d WHERE d.tenant_id=u.tenant_id AND d.space_id=u.space_id
  AND d.id=u.folder_id AND d.trash_batch_id=sqlc.arg(batch_id));

-- name: DrivePurgeFolderRows :execrows
DELETE FROM drive_folders WHERE tenant_id=sqlc.arg(tenant_id) AND space_id=sqlc.arg(space_id) AND trash_batch_id=sqlc.arg(batch_id);

-- name: DriveCleanupTenants :many
SELECT tenant_id FROM drive_trash_batches WHERE purging_at IS NOT NULL OR expires_at<=now()
UNION SELECT tenant_id FROM drive_uploads WHERE state IN ('completed','cancelled') OR (state='open' AND expires_at<=now());

-- name: DriveCleanupBatchCandidates :many
SELECT id,space_id,purging_at FROM drive_trash_batches
WHERE tenant_id=sqlc.arg(tenant_id) AND (purging_at IS NOT NULL OR expires_at<=now()) ORDER BY id LIMIT 50;

-- name: DriveCleanupUploadCandidates :many
SELECT id,space_id FROM drive_uploads WHERE tenant_id=sqlc.arg(tenant_id)
 AND (state IN ('completed','cancelled') OR (state='open' AND expires_at<=now())) ORDER BY id LIMIT 50;

-- name: DriveCleanupUploadLock :one
SELECT storage_profile_id,object_key,state FROM drive_uploads WHERE tenant_id=sqlc.arg(tenant_id) AND space_id=sqlc.arg(space_id)
 AND id=sqlc.arg(upload_id) AND (state IN ('completed','cancelled') OR (state='open' AND expires_at<=now())) FOR UPDATE;

-- name: DriveCleanupUploadKeys :many
SELECT storage_key FROM drive_upload_chunks WHERE tenant_id=sqlc.arg(tenant_id) AND upload_id=sqlc.arg(upload_id) ORDER BY chunk_index;

-- name: DriveCleanupUploadFinish :execrows
UPDATE drive_uploads SET state='cleaned' WHERE tenant_id=sqlc.arg(tenant_id) AND space_id=sqlc.arg(space_id)
 AND id=sqlc.arg(upload_id) AND (state IN ('completed','cancelled') OR (state='open' AND expires_at<=now()));
