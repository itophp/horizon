-- name: AICreateRun :one
INSERT INTO ai_runs (tenant_id, account_id, department_id, session_id, agent, model, instruction_version, question, trace_id, context, origin, correlation_id, causation_id, trigger_depth, trigger_policy, event_id)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)
RETURNING id, created_at;

-- name: AIFinishRun :exec
UPDATE ai_runs
SET status = sqlc.arg(status), answer = sqlc.arg(answer), error_code = sqlc.arg(error_code),
    tool_calls = sqlc.arg(tool_calls), prompt_tokens = sqlc.arg(prompt_tokens),
    completion_tokens = sqlc.arg(completion_tokens), duration_ms = sqlc.arg(duration_ms),
    finished_at = now()
WHERE tenant_id = $1 AND id = $2;

-- name: AISetRunStatus :exec
UPDATE ai_runs SET status = sqlc.arg(status) WHERE tenant_id = $1 AND id = $2;

-- name: AIGetRun :one
SELECT id, account_id, department_id, session_id, agent, model, instruction_version, question,
       answer, status, error_code, tool_calls, prompt_tokens, completion_tokens, duration_ms,
       trace_id, context, origin, correlation_id, causation_id, trigger_depth, trigger_policy, event_id,
       created_at, finished_at
FROM ai_runs
WHERE tenant_id = $1 AND account_id = $2 AND id = $3;

-- name: AIListRuns :many
SELECT id, account_id, department_id, session_id, agent, model, instruction_version, question,
       answer, status, error_code, tool_calls, prompt_tokens, completion_tokens, duration_ms,
       trace_id, context, origin, correlation_id, causation_id, trigger_depth, trigger_policy, event_id,
       created_at, finished_at
FROM ai_runs
WHERE tenant_id = $1 AND account_id = $2
ORDER BY id DESC
LIMIT sqlc.arg(max_rows)::int;

-- name: AIEvaluation :one
SELECT count(*)::bigint AS total,
       count(*) FILTER (WHERE status = 'completed')::bigint AS completed,
       count(*) FILTER (WHERE status = 'failed')::bigint AS failed,
       COALESCE(sum(prompt_tokens), 0)::bigint AS prompt_tokens,
       COALESCE(sum(completion_tokens), 0)::bigint AS completion_tokens,
       COALESCE(avg(duration_ms), 0)::bigint AS average_duration_ms
FROM ai_runs
WHERE tenant_id = $1;

-- One row per conversation: the workbench groups tasks by the session they
-- belong to, so a task list can be reopened and continued. A renamed title wins
-- over the derived first question; an archived session stays in the result so
-- the owner can still find and restore it.
-- name: AIListConversations :many
SELECT r.session_id,
       COALESCE(NULLIF(c.title, ''), (array_agg(r.question ORDER BY r.id))[1])::text AS title,
       max(r.created_at)::timestamptz AS updated_at,
       count(*)::int AS message_count,
       (c.archived_at IS NOT NULL)::boolean AS archived
FROM ai_runs r
LEFT JOIN ai_conversations c
       ON c.tenant_id = r.tenant_id
      AND c.account_id = r.account_id
      AND c.session_id = r.session_id
WHERE r.tenant_id = $1 AND r.account_id = $2 AND r.session_id <> ''
GROUP BY r.session_id, c.title, c.archived_at
ORDER BY max(r.created_at) DESC
LIMIT sqlc.arg(max_rows)::int;

-- One conversation of the caller, used to confirm ownership before a rename or
-- an archive and to return the refreshed list entry.
-- name: AIGetConversation :one
SELECT r.session_id,
       COALESCE(NULLIF(c.title, ''), (array_agg(r.question ORDER BY r.id))[1])::text AS title,
       max(r.created_at)::timestamptz AS updated_at,
       count(*)::int AS message_count,
       (c.archived_at IS NOT NULL)::boolean AS archived
FROM ai_runs r
LEFT JOIN ai_conversations c
       ON c.tenant_id = r.tenant_id
      AND c.account_id = r.account_id
      AND c.session_id = r.session_id
WHERE r.tenant_id = $1 AND r.account_id = $2 AND r.session_id = $3
GROUP BY r.session_id, c.title, c.archived_at;

-- name: AIRenameConversation :exec
INSERT INTO ai_conversations (tenant_id, account_id, session_id, title)
VALUES ($1, $2, $3, sqlc.arg(title))
ON CONFLICT (tenant_id, account_id, session_id)
DO UPDATE SET title = EXCLUDED.title, updated_at = now();

-- name: AISetConversationArchived :exec
INSERT INTO ai_conversations (tenant_id, account_id, session_id, archived_at)
VALUES ($1, $2, $3, CASE WHEN sqlc.arg(archived)::boolean THEN now() ELSE NULL END)
ON CONFLICT (tenant_id, account_id, session_id)
DO UPDATE SET archived_at = EXCLUDED.archived_at, updated_at = now();

-- name: AIListSessionRuns :many
SELECT id, account_id, department_id, session_id, agent, model, instruction_version, question,
       answer, status, error_code, tool_calls, prompt_tokens, completion_tokens, duration_ms,
       trace_id, context, origin, correlation_id, causation_id, trigger_depth, trigger_policy, event_id,
       created_at, finished_at
FROM ai_runs
WHERE tenant_id = $1 AND account_id = $2 AND session_id = $3
ORDER BY id ASC
LIMIT sqlc.arg(max_rows)::int;
