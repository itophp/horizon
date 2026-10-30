-- name: ListRecords :many
SELECT id, label FROM technical_demo_records WHERE tenant_id = sqlc.arg(tenant_id)
 AND (sqlc.arg(scope_kind)::text = 'ALL'
 OR (sqlc.arg(scope_kind)::text = 'OWN' AND owner_id = sqlc.arg(user_id))
 OR (sqlc.arg(scope_kind)::text = 'ORGANIZATION' AND organization_id = ANY(sqlc.arg(organization_ids)::text[]))
 OR (sqlc.arg(scope_kind)::text = 'DEPARTMENT' AND department_id = ANY(sqlc.arg(department_ids)::text[]))
 OR (sqlc.arg(scope_kind)::text = 'CUSTOM' AND id = ANY(sqlc.arg(resource_ids)::bigint[]))) ORDER BY id;
-- name: GetRecord :one
SELECT id, label FROM technical_demo_records WHERE id = sqlc.arg(id) AND tenant_id = sqlc.arg(tenant_id)
 AND (sqlc.arg(scope_kind)::text = 'ALL'
 OR (sqlc.arg(scope_kind)::text = 'OWN' AND owner_id = sqlc.arg(user_id))
 OR (sqlc.arg(scope_kind)::text = 'ORGANIZATION' AND organization_id = ANY(sqlc.arg(organization_ids)::text[]))
 OR (sqlc.arg(scope_kind)::text = 'DEPARTMENT' AND department_id = ANY(sqlc.arg(department_ids)::text[]))
 OR (sqlc.arg(scope_kind)::text = 'CUSTOM' AND id = ANY(sqlc.arg(resource_ids)::bigint[])));
-- name: UpdateRecord :one
UPDATE technical_demo_records SET label = sqlc.arg(label) WHERE id = sqlc.arg(id) AND tenant_id = sqlc.arg(tenant_id)
 AND (sqlc.arg(scope_kind)::text = 'ALL'
 OR (sqlc.arg(scope_kind)::text = 'OWN' AND owner_id = sqlc.arg(user_id))
 OR (sqlc.arg(scope_kind)::text = 'ORGANIZATION' AND organization_id = ANY(sqlc.arg(organization_ids)::text[]))
 OR (sqlc.arg(scope_kind)::text = 'DEPARTMENT' AND department_id = ANY(sqlc.arg(department_ids)::text[]))
 OR (sqlc.arg(scope_kind)::text = 'CUSTOM' AND id = ANY(sqlc.arg(resource_ids)::bigint[]))) RETURNING id, label;
-- name: DeleteRecord :one
DELETE FROM technical_demo_records WHERE id = sqlc.arg(id) AND tenant_id = sqlc.arg(tenant_id)
 AND (sqlc.arg(scope_kind)::text = 'ALL'
 OR (sqlc.arg(scope_kind)::text = 'OWN' AND owner_id = sqlc.arg(user_id))
 OR (sqlc.arg(scope_kind)::text = 'ORGANIZATION' AND organization_id = ANY(sqlc.arg(organization_ids)::text[]))
 OR (sqlc.arg(scope_kind)::text = 'DEPARTMENT' AND department_id = ANY(sqlc.arg(department_ids)::text[]))
 OR (sqlc.arg(scope_kind)::text = 'CUSTOM' AND id = ANY(sqlc.arg(resource_ids)::bigint[]))) RETURNING id;
