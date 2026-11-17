-- +goose Up
WITH linked AS (
    SELECT a.tenant_id, a.id AS ai_id, a.account_id,
           'automation:' || r.id::text AS session_id, t.name
    FROM automation_runs r
    JOIN automation_tasks t ON t.tenant_id = r.tenant_id AND t.id = r.task_id
    JOIN ai_runs a ON a.tenant_id = r.tenant_id AND a.id = r.ai_run_id
        AND a.account_id = t.owner_id AND a.origin = 'automation'
    WHERE a.session_id = ''
), attached AS (
    UPDATE ai_runs a SET session_id = linked.session_id
    FROM linked
    WHERE a.tenant_id = linked.tenant_id AND a.id = linked.ai_id AND a.session_id = ''
    RETURNING a.tenant_id, a.account_id, a.session_id, linked.name
)
INSERT INTO ai_conversations (tenant_id, account_id, session_id, title)
SELECT tenant_id, account_id, session_id, left(name, 200) FROM attached
ON CONFLICT (tenant_id, account_id, session_id) DO NOTHING;

-- +goose Down
-- Keep conversation links because users may have added replies or renamed them.
SELECT 1;
