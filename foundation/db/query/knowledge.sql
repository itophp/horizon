-- name: KBCreateDocument :one
INSERT INTO kb_documents (tenant_id, title, source, created_by, chunks, content)
VALUES ($1, $2, $3, $4, $5, $6) RETURNING id;

-- name: KBListDocuments :many
SELECT d.id, d.title, d.source, d.created_at, d.chunks::bigint AS chunks
FROM kb_documents d
WHERE d.tenant_id = $1 ORDER BY d.id DESC LIMIT 200;

-- name: KBAllDocuments :many
SELECT d.id, d.title, d.source, d.content
FROM kb_documents d
WHERE d.tenant_id = $1 ORDER BY d.id ASC;

-- name: KBUpdateChunks :exec
UPDATE kb_documents SET chunks = $3 WHERE tenant_id = $1 AND id = $2;

-- name: KBDeleteDocument :exec
DELETE FROM kb_documents WHERE tenant_id = $1 AND id = $2;
