-- +goose Up

-- A conversation is one entry of the owner's task list. Its runs already carry
-- the turns, but the title shown in the list is derived from the first question
-- and cannot express an owner rename, nor remember that the task was archived.
-- A row is therefore only written when the owner renames or archives, so every
-- existing conversation keeps its derived title and its active state.
CREATE TABLE ai_conversations (
 id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
 tenant_id text NOT NULL,
 account_id bigint NOT NULL,
 session_id text NOT NULL CHECK (length(session_id) BETWEEN 1 AND 80),
 title text NOT NULL DEFAULT '' CHECK (length(title) <= 200),
 archived_at timestamptz,
 created_at timestamptz NOT NULL DEFAULT now(),
 updated_at timestamptz NOT NULL DEFAULT now(),
 UNIQUE (tenant_id, id),
 -- One row per owner session: the upsert of a rename or an archive must never
 -- fork into two rows that disagree about the title.
 UNIQUE (tenant_id, account_id, session_id)
);
CREATE INDEX ai_conversations_owner ON ai_conversations (tenant_id, account_id, updated_at DESC);

-- +goose Down
DROP TABLE IF EXISTS ai_conversations;
