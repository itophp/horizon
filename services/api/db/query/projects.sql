-- Project collaboration queries. Every statement filters by tenant_id and by
-- project membership: no member row means the project is invisible, so a caller
-- that knows a project id still reads nothing. Row level rules (only the owner
-- manages members, plan and task owners must be members) are enforced in SQL so
-- a race cannot slip between a check and a write.

-- name: ProjectList :many
SELECT p.id, p.code, p.name, p.status, p.priority, p.due_date, p.owner_id,
 a.full_name AS owner_name, p.created_at, p.updated_at,
 (SELECT count(*) FROM project_plans pl WHERE pl.tenant_id=p.tenant_id AND pl.project_id=p.id) AS plan_count,
 (SELECT count(*) FROM project_tasks t WHERE t.tenant_id=p.tenant_id AND t.project_id=p.id) AS task_count,
 (SELECT count(*) FROM project_tasks t WHERE t.tenant_id=p.tenant_id AND t.project_id=p.id AND t.status='done') AS task_done_count,
 (SELECT count(*) FROM project_tasks t WHERE t.tenant_id=p.tenant_id AND t.project_id=p.id
    AND t.status IN ('todo','in_progress') AND t.due_date < current_date) AS task_overdue_count
FROM projects p
LEFT JOIN iam_accounts a ON a.tenant_id=p.tenant_id AND a.id=p.owner_id
WHERE p.tenant_id=sqlc.arg(tenant_id)
 AND EXISTS (SELECT 1 FROM project_members m WHERE m.tenant_id=p.tenant_id AND m.project_id=p.id AND m.account_id=sqlc.arg(account_id))
 AND (sqlc.arg(status_filter)::text = '' OR p.status = sqlc.arg(status_filter))
 AND (sqlc.arg(priority_filter)::text = '' OR p.priority = sqlc.arg(priority_filter))
 AND (sqlc.arg(keyword)::text = '' OR p.name ILIKE '%'||sqlc.arg(keyword)||'%' OR p.code ILIKE '%'||sqlc.arg(keyword)||'%')
ORDER BY p.id DESC LIMIT sqlc.arg(max_rows);

-- name: ProjectGet :one
SELECT p.id, p.code, p.name, p.status, p.priority, p.due_date, p.owner_id,
 a.full_name AS owner_name, p.created_at, p.updated_at,
 (SELECT count(*) FROM project_plans pl WHERE pl.tenant_id=p.tenant_id AND pl.project_id=p.id) AS plan_count,
 (SELECT count(*) FROM project_tasks t WHERE t.tenant_id=p.tenant_id AND t.project_id=p.id) AS task_count,
 (SELECT count(*) FROM project_tasks t WHERE t.tenant_id=p.tenant_id AND t.project_id=p.id AND t.status='done') AS task_done_count,
 (SELECT count(*) FROM project_tasks t WHERE t.tenant_id=p.tenant_id AND t.project_id=p.id
    AND t.status IN ('todo','in_progress') AND t.due_date < current_date) AS task_overdue_count
FROM projects p
LEFT JOIN iam_accounts a ON a.tenant_id=p.tenant_id AND a.id=p.owner_id
WHERE p.tenant_id=sqlc.arg(tenant_id) AND p.id=sqlc.arg(id)
 AND EXISTS (SELECT 1 FROM project_members m WHERE m.tenant_id=p.tenant_id AND m.project_id=p.id AND m.account_id=sqlc.arg(account_id));

-- name: ProjectCreate :one
INSERT INTO projects(tenant_id,code,name,priority,due_date,owner_id,created_by)
VALUES(sqlc.arg(tenant_id),sqlc.arg(code),sqlc.arg(name),sqlc.arg(priority),sqlc.arg(due_date),sqlc.arg(owner_id),sqlc.arg(created_by))
RETURNING id;

-- name: ProjectUpdate :execrows
UPDATE projects p SET name=sqlc.arg(name), priority=sqlc.arg(priority), due_date=sqlc.arg(due_date), updated_at=now()
WHERE p.tenant_id=sqlc.arg(tenant_id) AND p.id=sqlc.arg(id) AND p.status<>'archived'
 AND EXISTS (SELECT 1 FROM project_members m WHERE m.tenant_id=p.tenant_id AND m.project_id=p.id AND m.account_id=sqlc.arg(actor_id));

-- name: ProjectStatusChange :execrows
-- Archiving is an owner action; pausing and completing are member actions. The
-- expected source state keeps concurrent transitions from both succeeding.
UPDATE projects p SET status=sqlc.arg(to_status), updated_at=now()
WHERE p.tenant_id=sqlc.arg(tenant_id) AND p.id=sqlc.arg(id)
 AND p.status=sqlc.arg(from_status) AND p.status<>'archived'
 AND EXISTS (SELECT 1 FROM project_members m WHERE m.tenant_id=p.tenant_id AND m.project_id=p.id AND m.account_id=sqlc.arg(actor_id))
 AND (NOT sqlc.arg(require_owner)::boolean OR p.owner_id=sqlc.arg(actor_id));

-- name: ProjectOwnerSet :execrows
UPDATE projects p SET owner_id=sqlc.arg(new_owner_id), updated_at=now()
WHERE p.tenant_id=sqlc.arg(tenant_id) AND p.id=sqlc.arg(id) AND p.status<>'archived'
 AND p.owner_id=sqlc.arg(actor_id)
 AND EXISTS (SELECT 1 FROM project_members m WHERE m.tenant_id=p.tenant_id AND m.project_id=p.id AND m.account_id=sqlc.arg(new_owner_id));

-- name: ProjectMemberList :many
SELECT m.account_id, a.full_name AS name, a.email, m.joined_at, (p.owner_id = m.account_id) AS is_owner
FROM project_members m
JOIN projects p ON p.tenant_id=m.tenant_id AND p.id=m.project_id
JOIN iam_accounts a ON a.tenant_id=m.tenant_id AND a.id=m.account_id
WHERE m.tenant_id=sqlc.arg(tenant_id) AND m.project_id=sqlc.arg(project_id)
 AND EXISTS (SELECT 1 FROM project_members v WHERE v.tenant_id=m.tenant_id AND v.project_id=m.project_id AND v.account_id=sqlc.arg(account_id))
ORDER BY a.full_name LIMIT sqlc.arg(max_rows);

-- name: ProjectMemberAdd :execrows
INSERT INTO project_members(tenant_id,project_id,account_id)
SELECT sqlc.arg(tenant_id),sqlc.arg(project_id),sqlc.arg(account_id) WHERE EXISTS (
  SELECT 1 FROM projects p WHERE p.tenant_id=sqlc.arg(tenant_id) AND p.id=sqlc.arg(project_id)
    AND p.status<>'archived' AND p.owner_id=sqlc.arg(actor_id))
ON CONFLICT DO NOTHING;

-- name: ProjectMemberRemove :execrows
DELETE FROM project_members m
USING projects p
WHERE m.tenant_id=sqlc.arg(tenant_id) AND m.project_id=sqlc.arg(project_id) AND m.account_id=sqlc.arg(account_id)
 AND p.tenant_id=m.tenant_id AND p.id=m.project_id AND p.status<>'archived'
 AND p.owner_id=sqlc.arg(actor_id) AND p.owner_id<>m.account_id;

-- name: ProjectMemberExists :one
SELECT EXISTS (
  SELECT 1 FROM project_members m WHERE m.tenant_id=sqlc.arg(tenant_id) AND m.project_id=sqlc.arg(project_id) AND m.account_id=sqlc.arg(account_id)
) AS member;

-- name: ProjectOwnerGet :one
SELECT owner_id, status FROM projects WHERE tenant_id=sqlc.arg(tenant_id) AND id=sqlc.arg(id);

-- name: ProjectMemberCount :one
SELECT count(*) FROM project_members WHERE tenant_id=sqlc.arg(tenant_id) AND project_id=sqlc.arg(project_id);

-- name: AccountSearch :many
-- Member management needs to find people without granting accounts:read. Only
-- the minimal projection is returned.
SELECT id, full_name AS name, email FROM iam_accounts
WHERE tenant_id=sqlc.arg(tenant_id)
 AND (full_name ILIKE '%'||sqlc.arg(keyword)||'%' OR email ILIKE '%'||sqlc.arg(keyword)||'%')
ORDER BY full_name LIMIT sqlc.arg(max_rows);

-- name: ProjectPlanList :many
SELECT pl.id, pl.project_id, pl.plan_type, pl.title, pl.status, pl.priority, pl.due_date, pl.owner_id,
 o.full_name AS owner_name, pl.created_at, pl.updated_at
FROM project_plans pl
LEFT JOIN iam_accounts o ON o.tenant_id=pl.tenant_id AND o.id=pl.owner_id
WHERE pl.tenant_id=sqlc.arg(tenant_id) AND pl.project_id=sqlc.arg(project_id)
 AND EXISTS (SELECT 1 FROM project_members m WHERE m.tenant_id=pl.tenant_id AND m.project_id=pl.project_id AND m.account_id=sqlc.arg(account_id))
ORDER BY pl.id DESC LIMIT sqlc.arg(max_rows);

-- name: ProjectPlanGet :one
SELECT pl.id, pl.project_id, pl.plan_type, pl.title, pl.content, pl.status, pl.priority, pl.due_date, pl.owner_id,
 o.full_name AS owner_name, pl.created_by, pl.created_at, pl.updated_at
FROM project_plans pl
LEFT JOIN iam_accounts o ON o.tenant_id=pl.tenant_id AND o.id=pl.owner_id
WHERE pl.tenant_id=sqlc.arg(tenant_id) AND pl.id=sqlc.arg(id)
 AND EXISTS (SELECT 1 FROM project_members m WHERE m.tenant_id=pl.tenant_id AND m.project_id=pl.project_id AND m.account_id=sqlc.arg(account_id));

-- name: ProjectPlanCreate :one
INSERT INTO project_plans(tenant_id,project_id,plan_type,title,content,priority,due_date,owner_id,created_by)
SELECT sqlc.arg(tenant_id),sqlc.arg(project_id),sqlc.arg(plan_type),sqlc.arg(title),sqlc.arg(content),
       sqlc.arg(priority),sqlc.arg(due_date),sqlc.arg(owner_id),sqlc.arg(created_by)
WHERE EXISTS (
  SELECT 1 FROM project_members m JOIN projects p ON p.tenant_id=m.tenant_id AND p.id=m.project_id
  WHERE m.tenant_id=sqlc.arg(tenant_id) AND m.project_id=sqlc.arg(project_id)
    AND m.account_id=sqlc.arg(created_by) AND p.status<>'archived')
 AND EXISTS (
  SELECT 1 FROM project_members o
  WHERE o.tenant_id=sqlc.arg(tenant_id) AND o.project_id=sqlc.arg(project_id) AND o.account_id=sqlc.arg(owner_id))
RETURNING id;

-- name: ProjectPlanUpdate :execrows
UPDATE project_plans pl SET plan_type=sqlc.arg(plan_type), title=sqlc.arg(title), content=sqlc.arg(content),
 priority=sqlc.arg(priority), due_date=sqlc.arg(due_date), owner_id=sqlc.arg(owner_id), updated_at=now()
WHERE pl.tenant_id=sqlc.arg(tenant_id) AND pl.id=sqlc.arg(id)
 AND EXISTS (SELECT 1 FROM project_members m JOIN projects p ON p.tenant_id=m.tenant_id AND p.id=m.project_id
   WHERE m.tenant_id=pl.tenant_id AND m.project_id=pl.project_id
     AND m.account_id=sqlc.arg(actor_id) AND p.status<>'archived')
 AND EXISTS (SELECT 1 FROM project_members o
   WHERE o.tenant_id=pl.tenant_id AND o.project_id=pl.project_id AND o.account_id=sqlc.arg(owner_id));

-- name: ProjectPlanStatusChange :execrows
UPDATE project_plans pl SET status=sqlc.arg(to_status), updated_at=now()
WHERE pl.tenant_id=sqlc.arg(tenant_id) AND pl.id=sqlc.arg(id)
 AND pl.status=sqlc.arg(from_status) AND pl.status<>'cancelled'
 AND EXISTS (SELECT 1 FROM project_members m JOIN projects p ON p.tenant_id=m.tenant_id AND p.id=m.project_id
   WHERE m.tenant_id=pl.tenant_id AND m.project_id=pl.project_id
     AND m.account_id=sqlc.arg(actor_id) AND p.status<>'archived');

-- name: ProjectTaskList :many
SELECT t.id, t.project_id, t.plan_id, t.title, t.status, t.priority, t.assignee_id, t.due_date, t.completed_at,
 pl.title AS plan_title, s.full_name AS assignee_name
FROM project_tasks t
LEFT JOIN project_plans pl ON pl.tenant_id=t.tenant_id AND pl.id=t.plan_id
LEFT JOIN iam_accounts s ON s.tenant_id=t.tenant_id AND s.id=t.assignee_id
WHERE t.tenant_id=sqlc.arg(tenant_id) AND t.project_id=sqlc.arg(project_id)
 AND EXISTS (SELECT 1 FROM project_members m WHERE m.tenant_id=t.tenant_id AND m.project_id=t.project_id AND m.account_id=sqlc.arg(account_id))
 AND (sqlc.arg(plan_filter)::bigint = 0 OR t.plan_id = sqlc.arg(plan_filter))
 AND (sqlc.arg(assignee_filter)::bigint = 0 OR t.assignee_id = sqlc.arg(assignee_filter))
 AND (sqlc.arg(status_filter)::text = '' OR t.status = sqlc.arg(status_filter))
ORDER BY t.id DESC LIMIT sqlc.arg(max_rows);

-- name: ProjectTaskGet :one
SELECT t.id, t.project_id, t.plan_id, t.title, t.description, t.status, t.priority, t.assignee_id,
 t.due_date, t.completed_at, t.created_by, t.created_at, t.updated_at, pl.title AS plan_title,
 s.full_name AS assignee_name
FROM project_tasks t
LEFT JOIN project_plans pl ON pl.tenant_id=t.tenant_id AND pl.id=t.plan_id
LEFT JOIN iam_accounts s ON s.tenant_id=t.tenant_id AND s.id=t.assignee_id
WHERE t.tenant_id=sqlc.arg(tenant_id) AND t.id=sqlc.arg(id)
 AND EXISTS (SELECT 1 FROM project_members m WHERE m.tenant_id=t.tenant_id AND m.project_id=t.project_id AND m.account_id=sqlc.arg(account_id));

-- name: ProjectTaskCreate :one
INSERT INTO project_tasks(tenant_id,project_id,plan_id,title,description,priority,assignee_id,due_date,created_by)
SELECT sqlc.arg(tenant_id),sqlc.arg(project_id),sqlc.arg(plan_id),sqlc.arg(title),sqlc.arg(description),
       sqlc.arg(priority),sqlc.arg(assignee_id),sqlc.arg(due_date),sqlc.arg(created_by)
WHERE EXISTS (
  SELECT 1 FROM project_members m JOIN projects p ON p.tenant_id=m.tenant_id AND p.id=m.project_id
  WHERE m.tenant_id=sqlc.arg(tenant_id) AND m.project_id=sqlc.arg(project_id)
    AND m.account_id=sqlc.arg(created_by) AND p.status<>'archived')
 AND (sqlc.arg(plan_id)::bigint IS NULL OR EXISTS (
  SELECT 1 FROM project_plans pl WHERE pl.tenant_id=sqlc.arg(tenant_id) AND pl.id=sqlc.arg(plan_id) AND pl.project_id=sqlc.arg(project_id)))
 AND (sqlc.arg(assignee_id)::bigint IS NULL OR EXISTS (
  SELECT 1 FROM project_members a WHERE a.tenant_id=sqlc.arg(tenant_id) AND a.project_id=sqlc.arg(project_id) AND a.account_id=sqlc.arg(assignee_id)))
RETURNING id;

-- name: ProjectTaskUpdate :execrows
UPDATE project_tasks t SET plan_id=sqlc.arg(plan_id), title=sqlc.arg(title), description=sqlc.arg(description),
 priority=sqlc.arg(priority), assignee_id=sqlc.arg(assignee_id), due_date=sqlc.arg(due_date), updated_at=now()
WHERE t.tenant_id=sqlc.arg(tenant_id) AND t.id=sqlc.arg(id)
 AND EXISTS (SELECT 1 FROM project_members m JOIN projects p ON p.tenant_id=m.tenant_id AND p.id=m.project_id
   WHERE m.tenant_id=t.tenant_id AND m.project_id=t.project_id
     AND m.account_id=sqlc.arg(actor_id) AND p.status<>'archived')
 AND (sqlc.arg(plan_id)::bigint IS NULL OR EXISTS (
  SELECT 1 FROM project_plans pl WHERE pl.tenant_id=t.tenant_id AND pl.id=sqlc.arg(plan_id) AND pl.project_id=t.project_id))
 AND (sqlc.arg(assignee_id)::bigint IS NULL OR EXISTS (
  SELECT 1 FROM project_members a WHERE a.tenant_id=t.tenant_id AND a.project_id=t.project_id AND a.account_id=sqlc.arg(assignee_id)));

-- name: ProjectTaskStatusChange :execrows
UPDATE project_tasks t
SET status=sqlc.arg(to_status),
 completed_at=CASE WHEN sqlc.arg(to_status)='done' THEN now() ELSE NULL END,
 updated_at=now()
WHERE t.tenant_id=sqlc.arg(tenant_id) AND t.id=sqlc.arg(id)
 AND t.status=sqlc.arg(from_status) AND t.status<>'cancelled'
 AND EXISTS (SELECT 1 FROM project_members m JOIN projects p ON p.tenant_id=m.tenant_id AND p.id=m.project_id
   WHERE m.tenant_id=t.tenant_id AND m.project_id=t.project_id
     AND m.account_id=sqlc.arg(actor_id) AND p.status<>'archived');

-- name: ProjectTaskAssign :execrows
UPDATE project_tasks t SET assignee_id=sqlc.arg(assignee_id), updated_at=now()
WHERE t.tenant_id=sqlc.arg(tenant_id) AND t.id=sqlc.arg(id)
 AND EXISTS (SELECT 1 FROM project_members m JOIN projects p ON p.tenant_id=m.tenant_id AND p.id=m.project_id
   WHERE m.tenant_id=t.tenant_id AND m.project_id=t.project_id
     AND m.account_id=sqlc.arg(actor_id) AND p.status<>'archived')
 AND EXISTS (SELECT 1 FROM project_members a
   WHERE a.tenant_id=t.tenant_id AND a.project_id=t.project_id AND a.account_id=sqlc.arg(assignee_id));

-- name: ProjectTaskBriefUpsert :one
INSERT INTO project_task_briefs(tenant_id,task_id,run_id,summary,model)
VALUES(sqlc.arg(tenant_id),sqlc.arg(task_id),sqlc.arg(run_id),sqlc.arg(summary),sqlc.arg(model))
ON CONFLICT (tenant_id,task_id) DO UPDATE SET run_id=EXCLUDED.run_id, summary=EXCLUDED.summary, model=EXCLUDED.model, created_at=now()
RETURNING id;

-- name: ProjectTaskBriefGet :one
SELECT id, task_id, run_id, summary, model, created_at FROM project_task_briefs
WHERE tenant_id=sqlc.arg(tenant_id) AND task_id=sqlc.arg(task_id);

-- name: ProjectArchived :one
SELECT (status='archived') AS archived FROM projects WHERE tenant_id=sqlc.arg(tenant_id) AND id=sqlc.arg(id);
