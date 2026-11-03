-- +goose Up
-- Project collaboration: projects own members, plans and tasks. Every table is
-- tenant scoped and the member table is the visibility boundary: no member row
-- means the project does not exist for that account, admins included.
CREATE TABLE projects (
 id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
 tenant_id text NOT NULL,
 code text NOT NULL CHECK (length(code) BETWEEN 1 AND 32),
 name text NOT NULL CHECK (length(name) BETWEEN 1 AND 120),
 status text NOT NULL DEFAULT 'active' CHECK (status IN ('active','paused','completed','archived')),
 priority text NOT NULL DEFAULT 'normal' CHECK (priority IN ('low','normal','high','urgent')),
 due_date date,
 owner_id bigint NOT NULL,
 created_by bigint NOT NULL,
 created_at timestamptz NOT NULL DEFAULT now(),
 updated_at timestamptz NOT NULL DEFAULT now(),
 UNIQUE (tenant_id, code),
 UNIQUE (tenant_id, id),
 CONSTRAINT projects_owner_fkey FOREIGN KEY (tenant_id, owner_id)
   REFERENCES gov_identity_refs(tenant_id, account_id)
);
CREATE INDEX projects_owner ON projects(tenant_id, owner_id, id DESC);

-- Membership has no role column on purpose: the owner is projects.owner_id, and
-- project.owner.transfer keeps the two in step inside one transaction.
CREATE TABLE project_members (
 id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
 tenant_id text NOT NULL,
 project_id bigint NOT NULL,
 account_id bigint NOT NULL,
 joined_at timestamptz NOT NULL DEFAULT now(),
 UNIQUE (tenant_id, project_id, account_id),
 CONSTRAINT project_members_project_fkey FOREIGN KEY (tenant_id, project_id)
   REFERENCES projects(tenant_id, id) ON DELETE CASCADE,
 CONSTRAINT project_members_account_fkey FOREIGN KEY (tenant_id, account_id)
   REFERENCES gov_identity_refs(tenant_id, account_id)
);
CREATE INDEX project_members_account ON project_members(tenant_id, account_id, project_id);

CREATE TABLE project_plans (
 id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
 tenant_id text NOT NULL,
 project_id bigint NOT NULL,
 plan_type text NOT NULL DEFAULT 'phase' CHECK (plan_type IN ('milestone','phase','deliverable')),
 title text NOT NULL CHECK (length(title) BETWEEN 1 AND 200),
 content text NOT NULL DEFAULT '' CHECK (length(content) BETWEEN 0 AND 20000),
 status text NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','in_progress','done','cancelled')),
 priority text NOT NULL DEFAULT 'normal' CHECK (priority IN ('low','normal','high','urgent')),
 due_date date,
 owner_id bigint NOT NULL,
 created_by bigint NOT NULL,
 created_at timestamptz NOT NULL DEFAULT now(),
 updated_at timestamptz NOT NULL DEFAULT now(),
 UNIQUE (tenant_id, id),
 CONSTRAINT project_plans_project_fkey FOREIGN KEY (tenant_id, project_id)
   REFERENCES projects(tenant_id, id) ON DELETE CASCADE,
 CONSTRAINT project_plans_owner_fkey FOREIGN KEY (tenant_id, owner_id)
   REFERENCES gov_identity_refs(tenant_id, account_id)
);
CREATE INDEX project_plans_project ON project_plans(tenant_id, project_id, id DESC);

CREATE TABLE project_tasks (
 id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
 tenant_id text NOT NULL,
 project_id bigint NOT NULL,
 plan_id bigint,
 title text NOT NULL CHECK (length(title) BETWEEN 1 AND 200),
 description text NOT NULL DEFAULT '' CHECK (length(description) BETWEEN 0 AND 20000),
 status text NOT NULL DEFAULT 'todo' CHECK (status IN ('todo','in_progress','done','cancelled')),
 priority text NOT NULL DEFAULT 'normal' CHECK (priority IN ('low','normal','high','urgent')),
 assignee_id bigint,
 due_date date,
 completed_at timestamptz,
 created_by bigint NOT NULL,
 created_at timestamptz NOT NULL DEFAULT now(),
 updated_at timestamptz NOT NULL DEFAULT now(),
 UNIQUE (tenant_id, id),
 CONSTRAINT project_tasks_project_fkey FOREIGN KEY (tenant_id, project_id)
   REFERENCES projects(tenant_id, id) ON DELETE CASCADE,
 CONSTRAINT project_tasks_plan_fkey FOREIGN KEY (tenant_id, plan_id)
   REFERENCES project_plans(tenant_id, id) ON DELETE SET NULL,
 CONSTRAINT project_tasks_assignee_fkey FOREIGN KEY (tenant_id, assignee_id)
   REFERENCES gov_identity_refs(tenant_id, account_id)
);
CREATE INDEX project_tasks_project ON project_tasks(tenant_id, project_id, status, id DESC);
CREATE INDEX project_tasks_assignee ON project_tasks(tenant_id, assignee_id, status);

-- AI output lives in its own table: a model failure must never rewrite a task,
-- and one task keeps only the newest brief.
CREATE TABLE project_task_briefs (
 id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
 tenant_id text NOT NULL,
 task_id bigint NOT NULL,
 run_id bigint NOT NULL DEFAULT 0,
 summary text NOT NULL CHECK (length(summary) BETWEEN 1 AND 4000),
 model text NOT NULL DEFAULT '',
 created_at timestamptz NOT NULL DEFAULT now(),
 UNIQUE (tenant_id, task_id),
 CONSTRAINT project_task_briefs_task_fkey FOREIGN KEY (tenant_id, task_id)
   REFERENCES project_tasks(tenant_id, id) ON DELETE CASCADE
);

-- Assignment is the AI entry point: the brief is advice only and is written
-- back through project.task.brief.record.
INSERT INTO ai_trigger_policies (tenant_id,policy_id,version,enabled,event_type,agent,mode,instruction,max_depth)
VALUES ('*','project-task-brief',1,true,'project.task.assigned','task_briefer','async',
 'A project task was just assigned. Read it with project_task_get (and project_plan_list when it has a plan), then store a short handover brief with project_task_brief_record: what the task is for, what the assignee needs first, and the main risk. Do not change any business field.',5)
ON CONFLICT DO NOTHING;

-- +goose Down
DELETE FROM ai_trigger_policies WHERE policy_id='project-task-brief';
DROP TABLE project_task_briefs;
DROP TABLE project_tasks;
DROP TABLE project_plans;
DROP TABLE project_members;
DROP TABLE projects;
