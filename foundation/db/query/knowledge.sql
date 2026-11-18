-- name: KBCreateDocument :one
INSERT INTO kb_documents (tenant_id, title, source, created_by, chunks, content)
VALUES ($1, $2, $3, $4, $5, $6) RETURNING id;

-- name: KBListDocuments :many
SELECT d.id, d.title, d.source, d.created_at, d.chunks::bigint AS chunks
FROM kb_documents d
WHERE d.tenant_id = $1 ORDER BY d.id DESC LIMIT 200;

-- name: KBQueryDocuments :many
SELECT d.id, d.title, d.source, d.created_at, d.chunks::bigint AS chunks
FROM kb_documents d
WHERE d.tenant_id = sqlc.arg(tenant_id)
  AND (sqlc.arg(before_id)::bigint = 0 OR d.id < sqlc.arg(before_id))
  AND (sqlc.arg(keyword)::text = ''
       OR strpos(lower(d.title), lower(sqlc.arg(keyword))) > 0
       OR strpos(lower(d.content), lower(sqlc.arg(keyword))) > 0)
ORDER BY d.id DESC
LIMIT sqlc.arg(row_limit)::int;

-- name: KBReadDocument :one
SELECT d.id, d.title, d.source, d.created_at, d.chunks::bigint AS chunks, d.content, d.version
FROM kb_documents d
WHERE d.tenant_id = $1 AND d.id = $2;

-- name: KBLockDocument :one
SELECT d.id, d.title, d.content, d.version
FROM kb_documents d
WHERE d.tenant_id = $1 AND d.id = $2
FOR UPDATE;

-- name: KBUpdateDocument :exec
UPDATE kb_documents SET title = $3, content = $4, chunks = $5, version = $6
WHERE tenant_id = $1 AND id = $2;

-- name: KBAllDocumentIDs :many
SELECT d.id
FROM kb_documents d
WHERE d.tenant_id = $1 ORDER BY d.id ASC;

-- name: KBUpdateChunks :exec
UPDATE kb_documents SET chunks = $3 WHERE tenant_id = $1 AND id = $2;

-- name: KBDeleteDocument :exec
DELETE FROM kb_documents WHERE tenant_id = $1 AND id = $2;

-- name: KBCurrentDocumentVersions :many
SELECT id, version FROM kb_documents
WHERE tenant_id = sqlc.arg(tenant_id) AND id = ANY(sqlc.arg(document_ids)::bigint[]);
