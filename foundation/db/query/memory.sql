-- name: MemoryDiscoverEvents :execrows
INSERT INTO memory_event_inbox (source_event_id, tenant_id, event_id)
SELECT e.id, e.tenant_id, e.event_id FROM cap_events e
WHERE NOT EXISTS (SELECT 1 FROM memory_event_inbox i WHERE i.source_event_id = e.id)
ORDER BY e.id LIMIT sqlc.arg(batch_size)::int
ON CONFLICT DO NOTHING;

-- name: MemoryClaimEvent :one
SELECT i.source_event_id, i.tenant_id, i.event_id, i.attempts,
       e.event_type, e.aggregate_type, e.aggregate_id, e.invocation_id,
       e.account_id, e.department_id, e.correlation_id, e.causation_id,
       e.trigger_depth, e.created_at
FROM memory_event_inbox i JOIN cap_events e ON e.id = i.source_event_id AND e.tenant_id = i.tenant_id AND e.event_id = i.event_id
WHERE i.completed_at IS NULL AND i.dead_at IS NULL AND i.available_at <= now()
ORDER BY i.source_event_id FOR UPDATE OF i SKIP LOCKED LIMIT 1;

-- name: MemoryInsertEvent :execrows
INSERT INTO memory_events (tenant_id, organization_id, event_id, event_type, event_version, occurred_at,
 actor_id, actor_type, department_id, source_type, source_id, source_module, resource_type, resource_id,
 action_name, result, invocation_id, session_id, task_id, workflow_instance_id, trace_id, correlation_id, causation_id, trigger_depth, sensitivity, candidate_partitions, payload)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22, $23, $24, $25, $26, $27)
ON CONFLICT (tenant_id, event_id) DO NOTHING;

-- name: MemoryCompleteEvent :exec
UPDATE memory_event_inbox SET completed_at = now(), error_code = ''
WHERE source_event_id = $1 AND tenant_id = $2 AND completed_at IS NULL AND dead_at IS NULL;

-- name: MemoryCreatePersonal :one
INSERT INTO memory_items (tenant_id, partition, memory_type, scope_type, scope_id, owner_id, title, summary, sensitivity, valid_from, valid_to)
VALUES (sqlc.arg(tenant_id), sqlc.arg(partition), sqlc.arg(memory_type), 'user', sqlc.arg(scope_id), sqlc.arg(owner_id), sqlc.arg(title), sqlc.arg(summary), 'internal', COALESCE(sqlc.narg(valid_from)::timestamptz, now()), sqlc.narg(valid_to)::timestamptz)
RETURNING id, version;

-- name: MemoryCreateGroup :one
INSERT INTO memory_items (tenant_id,partition,memory_type,scope_type,scope_id,owner_id,title,summary,sensitivity,valid_from,valid_to)
SELECT sqlc.arg(tenant_id),sqlc.arg(partition),sqlc.arg(memory_type),sqlc.arg(scope_type),sqlc.arg(scope_id),sqlc.arg(owner_id),sqlc.arg(title),sqlc.arg(summary),'internal',COALESCE(sqlc.narg(valid_from)::timestamptz,now()),sqlc.narg(valid_to)::timestamptz
WHERE memory_group_authorized(sqlc.arg(tenant_id)::text,sqlc.arg(scope_type)::text,sqlc.arg(scope_id)::text,
 sqlc.arg(read_all)::boolean,sqlc.arg(read_orgs)::bigint[],sqlc.arg(read_depts)::bigint[],sqlc.arg(write_all)::boolean,sqlc.arg(write_orgs)::bigint[],sqlc.arg(write_depts)::bigint[],sqlc.arg(actor_id)::bigint)
RETURNING id,version;

-- These locks keep resource deletion or reparenting from racing a multi-row
-- memory write and its source/correction/relation cleanup.
-- name: MemoryLockGroupDepartment :one
SELECT d.id FROM iam_departments d WHERE d.tenant_id=sqlc.arg(tenant_id) AND d.id::text=sqlc.arg(scope_id)::text
 AND memory_group_authorized(d.tenant_id,'department',d.id::text,sqlc.arg(read_all)::boolean,sqlc.arg(read_orgs)::bigint[],sqlc.arg(read_depts)::bigint[],sqlc.arg(write_all)::boolean,sqlc.arg(write_orgs)::bigint[],sqlc.arg(write_depts)::bigint[],sqlc.arg(actor_id)::bigint) FOR SHARE OF d;

-- name: MemoryLockGroupOrganization :one
SELECT o.id FROM iam_organizations o WHERE o.tenant_id=sqlc.arg(tenant_id) AND o.id::text=sqlc.arg(scope_id)::text
 AND memory_group_authorized(o.tenant_id,'organization',o.id::text,sqlc.arg(read_all)::boolean,sqlc.arg(read_orgs)::bigint[],sqlc.arg(read_depts)::bigint[],sqlc.arg(write_all)::boolean,sqlc.arg(write_orgs)::bigint[],sqlc.arg(write_depts)::bigint[],sqlc.arg(actor_id)::bigint) FOR SHARE OF o;

-- name: MemoryLockGroupEntity :one
SELECT e.id FROM example_records e WHERE e.tenant_id=sqlc.arg(tenant_id) AND ('example_record:'||e.id::text)=sqlc.arg(scope_id)::text
 AND memory_group_authorized(e.tenant_id,'business_entity','example_record:'||e.id::text,sqlc.arg(read_all)::boolean,sqlc.arg(read_orgs)::bigint[],sqlc.arg(read_depts)::bigint[],sqlc.arg(write_all)::boolean,sqlc.arg(write_orgs)::bigint[],sqlc.arg(write_depts)::bigint[],sqlc.arg(actor_id)::bigint) FOR SHARE OF e;

-- name: MemoryListGroup :many
SELECT m.id,m.partition,m.memory_type,m.title,m.summary,m.status,m.verification_status,m.version,m.sensitivity,m.valid_from,m.valid_to FROM memory_items m
WHERE m.tenant_id=sqlc.arg(tenant_id) AND m.scope_type=sqlc.arg(scope_type) AND m.scope_id=sqlc.arg(scope_id)
 AND m.status=sqlc.arg(status_filter) AND (m.status<>'active' OR (m.valid_from<=now() AND (m.valid_to IS NULL OR m.valid_to>now())))
 AND (sqlc.narg(created_from)::timestamptz IS NULL OR m.created_at>=sqlc.narg(created_from))
 AND (sqlc.narg(created_before)::timestamptz IS NULL OR m.created_at<sqlc.narg(created_before))
 AND (sqlc.arg(partition_filter)::text='' OR m.partition=sqlc.arg(partition_filter)) AND (sqlc.arg(before_id)::bigint=0 OR m.id<sqlc.arg(before_id))
 AND (sqlc.arg(owner_filter)::bigint=0 OR m.owner_id=sqlc.arg(owner_filter))
 AND memory_group_authorized(m.tenant_id,m.scope_type,m.scope_id,sqlc.arg(read_all)::boolean,sqlc.arg(read_orgs)::bigint[],sqlc.arg(read_depts)::bigint[],sqlc.arg(write_all)::boolean,sqlc.arg(write_orgs)::bigint[],sqlc.arg(write_depts)::bigint[],sqlc.arg(actor_id)::bigint)
ORDER BY m.id DESC LIMIT sqlc.arg(max_rows)::int;

-- name: MemoryGetGroup :one
SELECT m.id,m.partition,m.memory_type,m.title,m.summary,m.status,m.verification_status,m.version,m.sensitivity,m.valid_from,m.valid_to FROM memory_items m
WHERE m.tenant_id=sqlc.arg(tenant_id) AND m.scope_type=sqlc.arg(scope_type) AND m.scope_id=sqlc.arg(scope_id) AND m.id=sqlc.arg(id)
 AND m.status='active' AND m.valid_from<=now() AND (m.valid_to IS NULL OR m.valid_to>now())
 AND memory_group_authorized(m.tenant_id,m.scope_type,m.scope_id,sqlc.arg(read_all)::boolean,sqlc.arg(read_orgs)::bigint[],sqlc.arg(read_depts)::bigint[],sqlc.arg(write_all)::boolean,sqlc.arg(write_orgs)::bigint[],sqlc.arg(write_depts)::bigint[],sqlc.arg(actor_id)::bigint);

-- name: MemoryLockGroup :one
SELECT m.id,m.version,m.status FROM memory_items m WHERE m.tenant_id=sqlc.arg(tenant_id) AND m.scope_type=sqlc.arg(scope_type) AND m.scope_id=sqlc.arg(scope_id) AND m.id=sqlc.arg(id)
 AND m.status IN ('active','archived') AND memory_group_authorized(m.tenant_id,m.scope_type,m.scope_id,sqlc.arg(read_all)::boolean,sqlc.arg(read_orgs)::bigint[],sqlc.arg(read_depts)::bigint[],sqlc.arg(write_all)::boolean,sqlc.arg(write_orgs)::bigint[],sqlc.arg(write_depts)::bigint[],sqlc.arg(actor_id)::bigint) FOR UPDATE OF m;

-- name: MemoryCorrectGroup :execrows
UPDATE memory_items m SET title=sqlc.arg(title),summary=sqlc.arg(summary),version=m.version+1,updated_at=now()
WHERE m.tenant_id=sqlc.arg(tenant_id) AND m.scope_type=sqlc.arg(scope_type) AND m.scope_id=sqlc.arg(scope_id) AND m.id=sqlc.arg(id) AND m.status='active' AND m.version=sqlc.arg(version)
 AND memory_group_authorized(m.tenant_id,m.scope_type,m.scope_id,sqlc.arg(read_all)::boolean,sqlc.arg(read_orgs)::bigint[],sqlc.arg(read_depts)::bigint[],sqlc.arg(write_all)::boolean,sqlc.arg(write_orgs)::bigint[],sqlc.arg(write_depts)::bigint[],sqlc.arg(actor_id)::bigint);

-- name: MemoryArchiveGroup :execrows
UPDATE memory_items m SET status='archived',version=m.version+1,updated_at=now()
WHERE m.tenant_id=sqlc.arg(tenant_id) AND m.scope_type=sqlc.arg(scope_type) AND m.scope_id=sqlc.arg(scope_id) AND m.id=sqlc.arg(id) AND m.status='active' AND m.version=sqlc.arg(version)
 AND memory_group_authorized(m.tenant_id,m.scope_type,m.scope_id,sqlc.arg(read_all)::boolean,sqlc.arg(read_orgs)::bigint[],sqlc.arg(read_depts)::bigint[],sqlc.arg(write_all)::boolean,sqlc.arg(write_orgs)::bigint[],sqlc.arg(write_depts)::bigint[],sqlc.arg(actor_id)::bigint);

-- name: MemoryDeleteGroup :execrows
UPDATE memory_items m SET status='deleted',title='',summary='',version=m.version+1,updated_at=now()
WHERE m.tenant_id=sqlc.arg(tenant_id) AND m.scope_type=sqlc.arg(scope_type) AND m.scope_id=sqlc.arg(scope_id) AND m.id=sqlc.arg(id) AND m.status IN ('active','archived') AND m.version=sqlc.arg(version)
 AND memory_group_authorized(m.tenant_id,m.scope_type,m.scope_id,sqlc.arg(read_all)::boolean,sqlc.arg(read_orgs)::bigint[],sqlc.arg(read_depts)::bigint[],sqlc.arg(write_all)::boolean,sqlc.arg(write_orgs)::bigint[],sqlc.arg(write_depts)::bigint[],sqlc.arg(actor_id)::bigint);

-- name: MemoryGroupCorrection :execrows
INSERT INTO memory_corrections (tenant_id,memory_id,actor_id,previous_version,previous_title,previous_summary)
SELECT m.tenant_id,m.id,sqlc.arg(actor_id),m.version,m.title,m.summary FROM memory_items m
WHERE m.tenant_id=sqlc.arg(tenant_id) AND m.scope_type=sqlc.arg(scope_type) AND m.scope_id=sqlc.arg(scope_id) AND m.id=sqlc.arg(id) AND m.version=sqlc.arg(version)
 AND memory_group_authorized(m.tenant_id,m.scope_type,m.scope_id,sqlc.arg(read_all)::boolean,sqlc.arg(read_orgs)::bigint[],sqlc.arg(read_depts)::bigint[],sqlc.arg(write_all)::boolean,sqlc.arg(write_orgs)::bigint[],sqlc.arg(write_depts)::bigint[],sqlc.arg(actor_id)::bigint);

-- name: MemoryGroupDeleteSources :exec
DELETE FROM memory_sources s USING memory_items m WHERE s.tenant_id=m.tenant_id AND s.memory_id=m.id AND m.tenant_id=sqlc.arg(tenant_id) AND m.scope_type=sqlc.arg(scope_type) AND m.scope_id=sqlc.arg(scope_id) AND m.id=sqlc.arg(id)
 AND memory_group_authorized(m.tenant_id,m.scope_type,m.scope_id,sqlc.arg(read_all)::boolean,sqlc.arg(read_orgs)::bigint[],sqlc.arg(read_depts)::bigint[],sqlc.arg(write_all)::boolean,sqlc.arg(write_orgs)::bigint[],sqlc.arg(write_depts)::bigint[],sqlc.arg(actor_id)::bigint);

-- name: MemoryGroupDeleteCorrections :exec
DELETE FROM memory_corrections c USING memory_items m WHERE c.tenant_id=m.tenant_id AND c.memory_id=m.id AND m.tenant_id=sqlc.arg(tenant_id) AND m.scope_type=sqlc.arg(scope_type) AND m.scope_id=sqlc.arg(scope_id) AND m.id=sqlc.arg(id)
 AND memory_group_authorized(m.tenant_id,m.scope_type,m.scope_id,sqlc.arg(read_all)::boolean,sqlc.arg(read_orgs)::bigint[],sqlc.arg(read_depts)::bigint[],sqlc.arg(write_all)::boolean,sqlc.arg(write_orgs)::bigint[],sqlc.arg(write_depts)::bigint[],sqlc.arg(actor_id)::bigint);

-- name: MemoryGroupDeleteRelations :exec
DELETE FROM memory_relations r USING memory_items m WHERE r.tenant_id=m.tenant_id AND (r.source_memory_id=m.id OR r.target_memory_id=m.id)
 AND m.tenant_id=sqlc.arg(tenant_id) AND m.scope_type=sqlc.arg(scope_type) AND m.scope_id=sqlc.arg(scope_id) AND m.id=sqlc.arg(id)
 AND memory_group_authorized(m.tenant_id,m.scope_type,m.scope_id,sqlc.arg(read_all)::boolean,sqlc.arg(read_orgs)::bigint[],sqlc.arg(read_depts)::bigint[],sqlc.arg(write_all)::boolean,sqlc.arg(write_orgs)::bigint[],sqlc.arg(write_depts)::bigint[],sqlc.arg(actor_id)::bigint);

-- name: MemoryGroupAttachEvent :execrows
INSERT INTO memory_sources (tenant_id,memory_id,event_id)
SELECT m.tenant_id,m.id,e.event_id FROM memory_items m JOIN memory_events e ON e.tenant_id=m.tenant_id AND e.event_id=sqlc.arg(event_id)
WHERE m.tenant_id=sqlc.arg(tenant_id) AND m.scope_type=sqlc.arg(scope_type) AND m.scope_id=sqlc.arg(scope_id) AND m.id=sqlc.arg(id)
 AND ((e.resource_type=m.scope_type AND e.resource_id=m.scope_id) OR (m.scope_type='business_entity' AND e.resource_type='example_record' AND ('example_record:'||e.resource_id)=m.scope_id))
 AND memory_group_authorized(m.tenant_id,m.scope_type,m.scope_id,sqlc.arg(read_all)::boolean,sqlc.arg(read_orgs)::bigint[],sqlc.arg(read_depts)::bigint[],sqlc.arg(write_all)::boolean,sqlc.arg(write_orgs)::bigint[],sqlc.arg(write_depts)::bigint[],sqlc.arg(actor_id)::bigint)
ON CONFLICT DO NOTHING;

-- name: MemoryGroupSources :many
SELECT e.event_id,e.event_type,e.occurred_at,e.correlation_id FROM memory_sources s
JOIN memory_items m ON m.tenant_id=s.tenant_id AND m.id=s.memory_id JOIN memory_events e ON e.tenant_id=s.tenant_id AND e.event_id=s.event_id
WHERE m.tenant_id=sqlc.arg(tenant_id) AND m.scope_type=sqlc.arg(scope_type) AND m.scope_id=sqlc.arg(scope_id) AND m.id=sqlc.arg(id)
 AND m.status='active' AND m.valid_from<=now() AND (m.valid_to IS NULL OR m.valid_to>now())
 AND ((e.resource_type=m.scope_type AND e.resource_id=m.scope_id) OR (m.scope_type='business_entity' AND e.resource_type='example_record' AND ('example_record:'||e.resource_id)=m.scope_id))
 AND memory_group_authorized(m.tenant_id,m.scope_type,m.scope_id,sqlc.arg(read_all)::boolean,sqlc.arg(read_orgs)::bigint[],sqlc.arg(read_depts)::bigint[],sqlc.arg(write_all)::boolean,sqlc.arg(write_orgs)::bigint[],sqlc.arg(write_depts)::bigint[],sqlc.arg(actor_id)::bigint)
ORDER BY e.id DESC LIMIT 100;

-- The project scope is passed only by the internal data capability after its
-- registered parent has authorized project.get through the Gateway.
-- name: MemoryCreateProject :one
INSERT INTO memory_items (tenant_id, partition, memory_type, scope_type, scope_id, owner_id, title, summary, sensitivity, valid_from, valid_to)
SELECT sqlc.arg(tenant_id), sqlc.arg(partition), sqlc.arg(memory_type), 'project', sqlc.arg(scope_id), sqlc.arg(owner_id), sqlc.arg(title), sqlc.arg(summary), 'internal', COALESCE(sqlc.narg(valid_from)::timestamptz, now()), sqlc.narg(valid_to)::timestamptz
WHERE EXISTS (SELECT 1 FROM project_members p WHERE p.tenant_id=sqlc.arg(tenant_id) AND p.project_id::text=sqlc.arg(scope_id) AND p.account_id=sqlc.arg(owner_id))
RETURNING id, version;

-- name: MemoryLockProjectMembership :one
SELECT p.project_id FROM project_members p
WHERE p.tenant_id=sqlc.arg(tenant_id) AND p.project_id::text=sqlc.arg(scope_id)::text AND p.account_id=sqlc.arg(account_id)
FOR SHARE OF p;

-- name: MemoryListProject :many
SELECT m.id, m.partition, m.memory_type, m.title, m.summary, m.status, m.verification_status, m.version, m.sensitivity, m.valid_from, m.valid_to
FROM memory_items m WHERE m.tenant_id=sqlc.arg(tenant_id) AND m.scope_type='project' AND m.scope_id=sqlc.arg(scope_id)
 AND EXISTS (SELECT 1 FROM project_members p WHERE p.tenant_id=sqlc.arg(tenant_id) AND p.project_id::text=sqlc.arg(scope_id) AND p.account_id=sqlc.arg(account_id))
 AND m.status=sqlc.arg(status_filter) AND (m.status<>'active' OR (m.valid_from<=now() AND (m.valid_to IS NULL OR m.valid_to>now())))
 AND (sqlc.narg(created_from)::timestamptz IS NULL OR m.created_at>=sqlc.narg(created_from))
 AND (sqlc.narg(created_before)::timestamptz IS NULL OR m.created_at<sqlc.narg(created_before))
 AND (sqlc.arg(partition_filter)::text='' OR m.partition=sqlc.arg(partition_filter))
 AND (sqlc.arg(owner_filter)::bigint=0 OR m.owner_id=sqlc.arg(owner_filter))
 AND (sqlc.arg(before_id)::bigint=0 OR m.id<sqlc.arg(before_id))
ORDER BY m.id DESC LIMIT sqlc.arg(max_rows)::int;

-- name: MemoryGetProject :one
SELECT m.id, m.partition, m.memory_type, m.title, m.summary, m.status, m.verification_status, m.version, m.sensitivity, m.valid_from, m.valid_to
FROM memory_items m WHERE m.tenant_id=sqlc.arg(tenant_id) AND m.scope_type='project' AND m.scope_id=sqlc.arg(scope_id)
 AND m.id=sqlc.arg(id) AND m.status='active' AND m.valid_from<=now() AND (m.valid_to IS NULL OR m.valid_to>now())
 AND EXISTS (SELECT 1 FROM project_members p WHERE p.tenant_id=m.tenant_id AND p.project_id::text=m.scope_id AND p.account_id=sqlc.arg(account_id));

-- name: MemoryLockProject :one
SELECT m.id, m.title, m.summary, m.version, m.status FROM memory_items m
WHERE m.tenant_id=sqlc.arg(tenant_id) AND m.scope_type='project' AND m.scope_id=sqlc.arg(scope_id) AND m.id=sqlc.arg(id)
 AND m.status IN ('active','archived') AND EXISTS
 (SELECT 1 FROM project_members p WHERE p.tenant_id=m.tenant_id AND p.project_id::text=m.scope_id AND p.account_id=sqlc.arg(account_id)) FOR UPDATE OF m;

-- name: MemoryCorrectProject :execrows
UPDATE memory_items m SET title=sqlc.arg(title), summary=sqlc.arg(summary), version=m.version+1, updated_at=now()
WHERE m.tenant_id=sqlc.arg(tenant_id) AND m.scope_type='project' AND m.scope_id=sqlc.arg(scope_id) AND m.id=sqlc.arg(id)
 AND m.status='active' AND m.version=sqlc.arg(version) AND EXISTS
 (SELECT 1 FROM project_members p WHERE p.tenant_id=m.tenant_id AND p.project_id::text=m.scope_id AND p.account_id=sqlc.arg(account_id));

-- name: MemoryArchiveProject :execrows
UPDATE memory_items m SET status='archived', version=m.version+1, updated_at=now()
WHERE m.tenant_id=sqlc.arg(tenant_id) AND m.scope_type='project' AND m.scope_id=sqlc.arg(scope_id) AND m.id=sqlc.arg(id)
 AND m.status='active' AND m.version=sqlc.arg(version) AND EXISTS
 (SELECT 1 FROM project_members p WHERE p.tenant_id=m.tenant_id AND p.project_id::text=m.scope_id AND p.account_id=sqlc.arg(account_id));

-- name: MemoryDeleteProject :execrows
UPDATE memory_items m SET status='deleted', title='', summary='', version=m.version+1, updated_at=now()
WHERE m.tenant_id=sqlc.arg(tenant_id) AND m.scope_type='project' AND m.scope_id=sqlc.arg(scope_id) AND m.id=sqlc.arg(id)
 AND m.status IN ('active','archived') AND m.version=sqlc.arg(version) AND EXISTS
 (SELECT 1 FROM project_members p WHERE p.tenant_id=m.tenant_id AND p.project_id::text=m.scope_id AND p.account_id=sqlc.arg(account_id));

-- name: MemoryAttachProjectEvent :execrows
INSERT INTO memory_sources (tenant_id, memory_id, event_id)
SELECT m.tenant_id, m.id, e.event_id FROM memory_items m JOIN memory_events e ON e.tenant_id=m.tenant_id AND e.event_id=sqlc.arg(event_id)
WHERE m.tenant_id=sqlc.arg(tenant_id) AND m.id=sqlc.arg(memory_id) AND m.scope_type='project' AND m.scope_id=sqlc.arg(scope_id)
 AND e.resource_type='project' AND e.resource_id=m.scope_id AND EXISTS
 (SELECT 1 FROM project_members p WHERE p.tenant_id=m.tenant_id AND p.project_id::text=m.scope_id AND p.account_id=sqlc.arg(account_id))
ON CONFLICT DO NOTHING;

-- name: MemoryListProjectSources :many
SELECT e.event_id, e.event_type, e.occurred_at, e.correlation_id
FROM memory_sources s JOIN memory_items m ON m.tenant_id=s.tenant_id AND m.id=s.memory_id
JOIN memory_events e ON e.tenant_id=s.tenant_id AND e.event_id=s.event_id
WHERE m.tenant_id=sqlc.arg(tenant_id) AND m.id=sqlc.arg(memory_id) AND m.scope_type='project' AND m.scope_id=sqlc.arg(scope_id)
 AND m.status='active' AND m.valid_from<=now() AND (m.valid_to IS NULL OR m.valid_to>now())
 AND e.resource_type='project' AND e.resource_id=m.scope_id AND EXISTS
 (SELECT 1 FROM project_members p WHERE p.tenant_id=m.tenant_id AND p.project_id::text=m.scope_id AND p.account_id=sqlc.arg(account_id))
ORDER BY e.id DESC LIMIT 100;

-- name: MemoryGetPersonal :one
SELECT id, partition, memory_type, title, summary, status, verification_status, version, sensitivity, valid_from, valid_to, created_at, updated_at
FROM memory_items WHERE tenant_id = sqlc.arg(tenant_id) AND owner_id = sqlc.arg(owner_id)
 AND scope_type = 'user' AND scope_id = sqlc.arg(scope_id) AND id = sqlc.arg(id)
 AND status = 'active' AND valid_from <= now() AND (valid_to IS NULL OR valid_to > now());

-- name: MemoryListPersonal :many
SELECT id, partition, memory_type, title, summary, status, verification_status, version, sensitivity, valid_from, valid_to, created_at, updated_at
FROM memory_items WHERE tenant_id = sqlc.arg(tenant_id) AND owner_id = sqlc.arg(owner_id)
 AND scope_type = 'user' AND scope_id = sqlc.arg(scope_id) AND status = sqlc.arg(status_filter)
 AND (status <> 'active' OR (valid_from <= now() AND (valid_to IS NULL OR valid_to > now())))
 AND (sqlc.narg(created_from)::timestamptz IS NULL OR created_at>=sqlc.narg(created_from))
 AND (sqlc.narg(created_before)::timestamptz IS NULL OR created_at<sqlc.narg(created_before))
 AND (sqlc.arg(partition_filter)::text = '' OR partition = sqlc.arg(partition_filter))
 AND (sqlc.arg(owner_filter)::bigint=0 OR owner_id=sqlc.arg(owner_filter))
 AND (sqlc.arg(before_id)::bigint = 0 OR id < sqlc.arg(before_id))
ORDER BY id DESC LIMIT sqlc.arg(max_rows)::int;

-- name: MemoryLockPersonal :one
SELECT id, title, summary, version, status FROM memory_items
WHERE tenant_id = sqlc.arg(tenant_id) AND owner_id = sqlc.arg(owner_id)
 AND scope_type = 'user' AND scope_id = sqlc.arg(scope_id) AND id = sqlc.arg(id)
 AND status IN ('active', 'archived') FOR UPDATE;

-- name: MemoryRecordCorrection :execrows
INSERT INTO memory_corrections (tenant_id, memory_id, actor_id, previous_version, previous_title, previous_summary)
SELECT m.tenant_id, m.id, sqlc.arg(actor_id), m.version, m.title, m.summary FROM memory_items m
WHERE m.tenant_id=sqlc.arg(tenant_id) AND m.id=sqlc.arg(memory_id) AND m.version=sqlc.arg(previous_version)
 AND ((m.scope_type='user' AND m.owner_id=sqlc.arg(actor_id) AND m.scope_id=sqlc.arg(actor_id)::bigint::text)
 OR (m.scope_type='project' AND EXISTS (SELECT 1 FROM project_members p WHERE p.tenant_id=m.tenant_id AND p.project_id::text=m.scope_id AND p.account_id=sqlc.arg(actor_id))));

-- name: MemoryCorrectPersonal :execrows
UPDATE memory_items SET title = sqlc.arg(title), summary = sqlc.arg(summary), version = version + 1, updated_at = now()
WHERE tenant_id = sqlc.arg(tenant_id) AND owner_id = sqlc.arg(owner_id) AND scope_type = 'user'
 AND scope_id = sqlc.arg(scope_id) AND id = sqlc.arg(id) AND status = 'active' AND version = sqlc.arg(version);

-- name: MemoryArchivePersonal :execrows
UPDATE memory_items SET status = 'archived', version = version + 1, updated_at = now()
WHERE tenant_id = sqlc.arg(tenant_id) AND owner_id = sqlc.arg(owner_id) AND scope_type = 'user'
 AND scope_id = sqlc.arg(scope_id) AND id = sqlc.arg(id) AND status = 'active' AND version = sqlc.arg(version);

-- name: MemoryDeletePersonal :execrows
UPDATE memory_items SET status = 'deleted', title = '', summary = '', version = version + 1, updated_at = now()
WHERE tenant_id = sqlc.arg(tenant_id) AND owner_id = sqlc.arg(owner_id) AND scope_type = 'user'
 AND scope_id = sqlc.arg(scope_id) AND id = sqlc.arg(id) AND status IN ('active', 'archived') AND version = sqlc.arg(version);

-- name: MemoryDeleteSources :exec
DELETE FROM memory_sources s USING memory_items m WHERE s.tenant_id=m.tenant_id AND s.memory_id=m.id
 AND m.tenant_id=sqlc.arg(tenant_id) AND m.id=sqlc.arg(memory_id)
 AND ((m.scope_type='user' AND m.owner_id=sqlc.arg(actor_id) AND m.scope_id=sqlc.arg(actor_id)::bigint::text)
 OR (m.scope_type='project' AND EXISTS (SELECT 1 FROM project_members p WHERE p.tenant_id=m.tenant_id AND p.project_id::text=m.scope_id AND p.account_id=sqlc.arg(actor_id))));

-- name: MemoryDeleteCorrections :exec
DELETE FROM memory_corrections c USING memory_items m WHERE c.tenant_id=m.tenant_id AND c.memory_id=m.id
 AND m.tenant_id=sqlc.arg(tenant_id) AND m.id=sqlc.arg(memory_id)
 AND ((m.scope_type='user' AND m.owner_id=sqlc.arg(actor_id) AND m.scope_id=sqlc.arg(actor_id)::bigint::text)
 OR (m.scope_type='project' AND EXISTS (SELECT 1 FROM project_members p WHERE p.tenant_id=m.tenant_id AND p.project_id::text=m.scope_id AND p.account_id=sqlc.arg(actor_id))));

-- name: MemoryDeleteRelations :exec
DELETE FROM memory_relations r USING memory_items m WHERE r.tenant_id=m.tenant_id AND (r.source_memory_id=m.id OR r.target_memory_id=m.id)
 AND m.tenant_id=sqlc.arg(tenant_id) AND m.id=sqlc.arg(memory_id)
 AND ((m.scope_type='user' AND m.owner_id=sqlc.arg(actor_id) AND m.scope_id=sqlc.arg(actor_id)::bigint::text)
 OR (m.scope_type='project' AND EXISTS (SELECT 1 FROM project_members p WHERE p.tenant_id=m.tenant_id AND p.project_id::text=m.scope_id AND p.account_id=sqlc.arg(actor_id))));

-- name: MemoryRecordAccess :exec
INSERT INTO memory_access_audit (tenant_id, actor_id, action, object_id, correlation_id)
VALUES ($1, $2, $3, $4, $5);

-- name: MemoryListPolicies :many
SELECT partition, retention_days, revision, updated_by, updated_at FROM memory_partition_policies
WHERE tenant_id=sqlc.arg(tenant_id) ORDER BY partition;

-- name: MemorySetPolicy :one
INSERT INTO memory_partition_policies (tenant_id, partition, retention_days, updated_by)
SELECT sqlc.arg(tenant_id), sqlc.arg(partition), sqlc.arg(retention_days), sqlc.arg(updated_by)
WHERE sqlc.arg(expected_revision)::bigint=0 OR EXISTS
 (SELECT 1 FROM memory_partition_policies p WHERE p.tenant_id=sqlc.arg(tenant_id) AND p.partition=sqlc.arg(partition))
ON CONFLICT (tenant_id, partition) DO UPDATE SET retention_days=EXCLUDED.retention_days, updated_by=EXCLUDED.updated_by,
 revision=memory_partition_policies.revision+1, updated_at=now()
WHERE memory_partition_policies.revision=sqlc.arg(expected_revision)::bigint
RETURNING revision;

-- name: MemoryAttachOwnEvent :execrows
INSERT INTO memory_sources (tenant_id, memory_id, event_id)
SELECT m.tenant_id, m.id, e.event_id FROM memory_items m
JOIN memory_events e ON e.tenant_id = m.tenant_id AND e.event_id = sqlc.arg(event_id)
WHERE m.tenant_id = sqlc.arg(tenant_id) AND m.id = sqlc.arg(memory_id)
 AND m.owner_id = sqlc.arg(owner_id) AND m.scope_type = 'user' AND m.scope_id = sqlc.arg(scope_id)
 AND e.actor_type = 'user' AND e.actor_id = sqlc.arg(scope_id)
 AND ((e.resource_type = '' AND e.resource_id = '') OR (e.resource_type = 'ai_run' AND EXISTS
   (SELECT 1 FROM ai_runs r WHERE r.tenant_id=e.tenant_id AND r.id::text=e.resource_id AND r.account_id=sqlc.arg(owner_id))))
ON CONFLICT DO NOTHING;

-- name: MemoryListOwnSources :many
SELECT e.event_id, e.event_type, e.occurred_at, e.resource_type, e.resource_id, e.correlation_id
FROM memory_sources s JOIN memory_items m ON m.tenant_id = s.tenant_id AND m.id = s.memory_id
JOIN memory_events e ON e.tenant_id = s.tenant_id AND e.event_id = s.event_id
WHERE m.tenant_id = sqlc.arg(tenant_id) AND m.id = sqlc.arg(memory_id) AND m.owner_id = sqlc.arg(owner_id)
 AND m.scope_type = 'user' AND m.scope_id = sqlc.arg(scope_id) AND m.status = 'active'
 AND m.valid_from <= now() AND (m.valid_to IS NULL OR m.valid_to > now())
 AND e.actor_type = 'user' AND e.actor_id = sqlc.arg(scope_id)
 AND ((e.resource_type = '' AND e.resource_id = '') OR (e.resource_type = 'ai_run' AND EXISTS
   (SELECT 1 FROM ai_runs r WHERE r.tenant_id=e.tenant_id AND r.id::text=e.resource_id AND r.account_id=sqlc.arg(owner_id))))
ORDER BY e.id DESC LIMIT 100;

-- name: MemoryListOwnEvents :many
SELECT e.id, e.event_id, e.event_type, e.occurred_at, e.source_type, e.source_id,
 e.resource_type, e.resource_id, e.action_name, e.result, e.session_id, e.task_id, e.workflow_instance_id, e.trace_id, e.correlation_id, e.causation_id, e.candidate_partitions, e.payload
FROM memory_events e
WHERE e.tenant_id=sqlc.arg(tenant_id) AND e.actor_type='user' AND e.actor_id=sqlc.arg(actor_id)
 AND ((e.resource_type='' AND e.resource_id='') OR (e.resource_type='ai_run' AND EXISTS
   (SELECT 1 FROM ai_runs r WHERE r.tenant_id=e.tenant_id AND r.id::text=e.resource_id AND r.account_id=sqlc.arg(account_id))))
 AND (sqlc.arg(event_type)::text='' OR e.event_type=sqlc.arg(event_type))
 AND (sqlc.arg(event_id)::text='' OR e.event_id=sqlc.arg(event_id))
 AND (sqlc.arg(resource_type)::text='' OR e.resource_type=sqlc.arg(resource_type))
 AND (sqlc.arg(resource_id)::text='' OR e.resource_id=sqlc.arg(resource_id))
 AND (sqlc.arg(correlation_id)::text='' OR e.correlation_id=sqlc.arg(correlation_id))
 AND (sqlc.narg(occurred_from)::timestamptz IS NULL OR e.occurred_at>=sqlc.narg(occurred_from))
 AND (sqlc.narg(occurred_before)::timestamptz IS NULL OR e.occurred_at<sqlc.narg(occurred_before))
 AND (sqlc.arg(before_id)::bigint=0 OR e.id<sqlc.arg(before_id))
ORDER BY e.id DESC LIMIT sqlc.arg(max_rows)::int;

-- name: MemoryFailEvent :exec
UPDATE memory_event_inbox SET attempts = attempts + 1,
 dead_at = CASE WHEN attempts + 1 >= sqlc.arg(max_attempts)::int THEN now() ELSE NULL END,
 available_at = now() + make_interval(secs => sqlc.arg(delay_seconds)::int),
 error_code = sqlc.arg(error_code)
WHERE source_event_id = $1 AND tenant_id = $2 AND completed_at IS NULL AND dead_at IS NULL;

-- name: MemoryListDeadEvents :many
SELECT source_event_id, event_id, attempts, error_code, dead_at FROM memory_event_inbox
WHERE tenant_id=sqlc.arg(tenant_id) AND dead_at IS NOT NULL
ORDER BY dead_at DESC, source_event_id DESC LIMIT sqlc.arg(max_rows)::int;

-- name: MemoryIngestionStats :one
SELECT count(*) FILTER (WHERE completed_at IS NULL AND dead_at IS NULL)::bigint AS pending,
       count(*) FILTER (WHERE dead_at IS NOT NULL)::bigint AS dead,
       count(*) FILTER (WHERE completed_at IS NOT NULL)::bigint AS completed
FROM memory_event_inbox;

-- name: MemoryReplayEvent :execrows
UPDATE memory_event_inbox SET attempts=0, available_at=now(), completed_at=NULL, dead_at=NULL, error_code=''
WHERE tenant_id=sqlc.arg(tenant_id) AND event_id=sqlc.arg(event_id)
 AND (dead_at IS NOT NULL OR completed_at IS NOT NULL);

-- name: MemoryReplayRange :many
WITH selected AS (
 SELECT source_event_id FROM memory_event_inbox
 WHERE tenant_id=sqlc.arg(tenant_id) AND source_event_id>=sqlc.arg(first_id)::bigint
   AND source_event_id<=sqlc.arg(last_id)::bigint AND (dead_at IS NOT NULL OR completed_at IS NOT NULL)
 ORDER BY source_event_id LIMIT sqlc.arg(max_rows)::int FOR UPDATE
)
UPDATE memory_event_inbox i SET attempts=0, available_at=now(), completed_at=NULL, dead_at=NULL, error_code=''
FROM selected s WHERE i.tenant_id=sqlc.arg(tenant_id) AND i.source_event_id=s.source_event_id
RETURNING i.source_event_id;

-- Durable execution records are consumed by set reconciliation, not an ID
-- watermark: rows may commit out of order and may become terminal later.
-- Only stable metadata is projected; inputs, outputs and model text stay in
-- their original stores and are never copied into memory events.
-- name: MemoryProjectInvocationStarts :execrows
INSERT INTO memory_events (tenant_id, event_id, event_type, event_version, occurred_at, actor_id, actor_type,
 department_id, source_type, source_id, invocation_id, trace_id, correlation_id, causation_id, trigger_depth, candidate_partitions, payload)
SELECT i.tenant_id, 'inv:' || i.invocation_id || ':started', 'capability.invocation.started', '1.0', i.created_at,
 CASE WHEN i.account_id > 0 THEN i.account_id::text ELSE 'unknown' END, CASE WHEN i.account_id > 0 THEN 'user' ELSE 'unknown' END, i.department_id, 'capability', i.capability, i.invocation_id, i.trace_id,
 i.correlation_id, i.causation_id, i.trigger_depth, ARRAY['activity','execution']::text[], jsonb_build_object('origin',i.origin,'capabilityVersion',i.capability_version)
FROM cap_invocations i WHERE NOT EXISTS (SELECT 1 FROM memory_events m WHERE m.tenant_id=i.tenant_id AND m.event_id='inv:' || i.invocation_id || ':started')
ORDER BY i.id LIMIT sqlc.arg(batch_size)::int ON CONFLICT (tenant_id, event_id) DO NOTHING;

-- name: MemoryProjectInvocationResults :execrows
INSERT INTO memory_events (tenant_id, event_id, event_type, event_version, occurred_at, actor_id, actor_type,
 department_id, source_type, source_id, result, invocation_id, trace_id, correlation_id, causation_id, trigger_depth, candidate_partitions, payload)
SELECT i.tenant_id, 'inv:' || i.invocation_id || ':' || i.status,
 CASE WHEN i.status='succeeded' THEN 'capability.invocation.completed' WHEN i.status='cancelled' THEN 'capability.invocation.cancelled' ELSE 'capability.invocation.failed' END,
 '1.0', i.finished_at, CASE WHEN i.account_id > 0 THEN i.account_id::text ELSE 'unknown' END, CASE WHEN i.account_id > 0 THEN 'user' ELSE 'unknown' END, i.department_id, 'capability', i.capability, i.status,
 i.invocation_id, i.trace_id, i.correlation_id, i.causation_id, i.trigger_depth, ARRAY['activity','execution']::text[], jsonb_build_object('origin',i.origin,'capabilityVersion',i.capability_version)
FROM cap_invocations i WHERE i.finished_at IS NOT NULL AND i.status IN ('succeeded','failed','cancelled')
 AND NOT EXISTS (SELECT 1 FROM memory_events m WHERE m.tenant_id=i.tenant_id AND m.event_id='inv:' || i.invocation_id || ':' || i.status)
ORDER BY i.id LIMIT sqlc.arg(batch_size)::int ON CONFLICT (tenant_id, event_id) DO NOTHING;

-- name: MemoryProjectRunStarts :execrows
INSERT INTO memory_events (tenant_id, event_id, event_type, event_version, occurred_at, actor_id, actor_type,
 department_id, source_type, resource_type, resource_id, task_id, workflow_instance_id, trace_id, correlation_id, causation_id, trigger_depth, candidate_partitions, payload)
SELECT r.tenant_id, 'run:' || r.id::text || ':started',
 CASE WHEN r.agent='orchestration' THEN 'workflow.instance.started' ELSE 'agent.task.started' END,
 '1.0', r.created_at, CASE WHEN r.account_id > 0 THEN r.account_id::text ELSE 'unknown' END, CASE WHEN r.account_id > 0 THEN 'user' ELSE 'unknown' END, r.department_id,
 CASE WHEN r.agent='orchestration' THEN 'workflow' ELSE 'agent' END,
 'ai_run', r.id::text, r.id::text, CASE WHEN r.agent='orchestration' THEN r.id::text ELSE '' END, r.trace_id, r.correlation_id, r.causation_id, r.trigger_depth, ARRAY['execution']::text[], '{}'::jsonb
FROM ai_runs r WHERE NOT EXISTS (SELECT 1 FROM memory_events m WHERE m.tenant_id=r.tenant_id AND m.event_id='run:' || r.id::text || ':started')
ORDER BY r.id LIMIT sqlc.arg(batch_size)::int ON CONFLICT (tenant_id, event_id) DO NOTHING;

-- name: MemoryProjectAgentToolStarts :execrows
INSERT INTO memory_events (tenant_id,event_id,event_type,event_version,occurred_at,actor_id,actor_type,department_id,source_type,source_id,invocation_id,trace_id,correlation_id,causation_id,trigger_depth,candidate_partitions,payload)
SELECT i.tenant_id,'agent-tool:'||i.invocation_id||':started','agent.tool.started','1.0',i.created_at,
 CASE WHEN i.account_id>0 THEN i.account_id::text ELSE 'unknown' END, CASE WHEN i.account_id>0 THEN 'user' ELSE 'unknown' END,
 i.department_id,'agent',i.capability,i.invocation_id,i.trace_id,i.correlation_id,i.causation_id,i.trigger_depth,ARRAY['execution']::text[],jsonb_build_object('capabilityVersion',i.capability_version)
FROM cap_invocations i WHERE i.origin='agent' AND NOT EXISTS
 (SELECT 1 FROM memory_events m WHERE m.tenant_id=i.tenant_id AND m.event_id='agent-tool:'||i.invocation_id||':started')
ORDER BY i.id LIMIT sqlc.arg(batch_size)::int ON CONFLICT (tenant_id,event_id) DO NOTHING;

-- name: MemoryProjectAgentToolResults :execrows
INSERT INTO memory_events (tenant_id,event_id,event_type,event_version,occurred_at,actor_id,actor_type,department_id,source_type,source_id,result,invocation_id,trace_id,correlation_id,causation_id,trigger_depth,candidate_partitions,payload)
SELECT i.tenant_id,'agent-tool:'||i.invocation_id||':'||i.status,
 CASE WHEN i.status='succeeded' THEN 'agent.tool.completed' WHEN i.status='cancelled' THEN 'agent.tool.cancelled' ELSE 'agent.tool.failed' END,
 '1.0',i.finished_at,CASE WHEN i.account_id>0 THEN i.account_id::text ELSE 'unknown' END,CASE WHEN i.account_id>0 THEN 'user' ELSE 'unknown' END,
 i.department_id,'agent',i.capability,i.status,i.invocation_id,i.trace_id,i.correlation_id,i.causation_id,i.trigger_depth,ARRAY['execution']::text[],jsonb_build_object('capabilityVersion',i.capability_version)
FROM cap_invocations i WHERE i.origin='agent' AND i.finished_at IS NOT NULL AND i.status IN ('succeeded','failed','cancelled') AND NOT EXISTS
 (SELECT 1 FROM memory_events m WHERE m.tenant_id=i.tenant_id AND m.event_id='agent-tool:'||i.invocation_id||':'||i.status)
ORDER BY i.id LIMIT sqlc.arg(batch_size)::int ON CONFLICT (tenant_id,event_id) DO NOTHING;

-- name: MemoryProjectRunResults :execrows
INSERT INTO memory_events (tenant_id, event_id, event_type, event_version, occurred_at, actor_id, actor_type,
 department_id, source_type, resource_type, resource_id, task_id, workflow_instance_id, result, trace_id, correlation_id, causation_id, trigger_depth, candidate_partitions, payload)
SELECT r.tenant_id, 'run:' || r.id::text || ':' || r.status,
 CASE WHEN r.agent='orchestration' AND r.status='completed' THEN 'workflow.instance.completed'
      WHEN r.agent='orchestration' AND r.status='cancelled' THEN 'workflow.instance.cancelled'
      WHEN r.agent='orchestration' THEN 'workflow.instance.failed'
      WHEN r.status='completed' THEN 'agent.task.completed' WHEN r.status='cancelled' THEN 'agent.task.cancelled' ELSE 'agent.task.failed' END,
 '1.0', r.finished_at, CASE WHEN r.account_id > 0 THEN r.account_id::text ELSE 'unknown' END, CASE WHEN r.account_id > 0 THEN 'user' ELSE 'unknown' END, r.department_id,
 CASE WHEN r.agent='orchestration' THEN 'workflow' ELSE 'agent' END,
 'ai_run', r.id::text, r.id::text, CASE WHEN r.agent='orchestration' THEN r.id::text ELSE '' END, r.status, r.trace_id, r.correlation_id, r.causation_id, r.trigger_depth, CASE WHEN r.status='failed' THEN ARRAY['execution','exception']::text[] ELSE ARRAY['execution']::text[] END, '{}'::jsonb
FROM ai_runs r WHERE r.finished_at IS NOT NULL AND r.status IN ('completed','failed','cancelled')
 AND NOT EXISTS (SELECT 1 FROM memory_events m WHERE m.tenant_id=r.tenant_id AND m.event_id='run:' || r.id::text || ':' || r.status)
ORDER BY r.id LIMIT sqlc.arg(batch_size)::int ON CONFLICT (tenant_id, event_id) DO NOTHING;
