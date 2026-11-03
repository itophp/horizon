-- +goose Up
CREATE TABLE mcp_tokens (
 id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
 tenant_id text NOT NULL,
 account_id bigint NOT NULL,
 department_id bigint NOT NULL,
 name text NOT NULL CHECK (length(name) BETWEEN 1 AND 100),
 token_hash text NOT NULL UNIQUE CHECK (length(token_hash) = 64),
 token_prefix text NOT NULL,
 created_at timestamptz NOT NULL DEFAULT now(),
 expires_at timestamptz NOT NULL,
 revoked_at timestamptz,
 last_used_at timestamptz,
 UNIQUE (tenant_id, id),
 FOREIGN KEY (tenant_id, account_id) REFERENCES iam_accounts(tenant_id, id) ON DELETE CASCADE,
 FOREIGN KEY (tenant_id, department_id) REFERENCES iam_departments(tenant_id, id),
 CHECK (expires_at > created_at AND expires_at <= created_at + interval '90 days')
);
CREATE INDEX mcp_tokens_owner ON mcp_tokens(tenant_id, account_id, id);
CREATE TABLE mcp_token_tools (
 token_id bigint NOT NULL REFERENCES mcp_tokens(id) ON DELETE CASCADE,
 tool text NOT NULL,
 PRIMARY KEY (token_id, tool)
);
CREATE TABLE mcp_call_audits (
 id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
 tenant_id text NOT NULL,
 account_id bigint NOT NULL,
 token_id bigint NOT NULL,
 department_id bigint NOT NULL,
 tool text NOT NULL,
 result_code text NOT NULL,
 duration_ms bigint NOT NULL CHECK (duration_ms >= 0),
 request_id text NOT NULL,
 created_at timestamptz NOT NULL DEFAULT now(),
 FOREIGN KEY (tenant_id, token_id) REFERENCES mcp_tokens(tenant_id, id) ON DELETE CASCADE,
 FOREIGN KEY (tenant_id, account_id) REFERENCES iam_accounts(tenant_id, id) ON DELETE CASCADE,
 FOREIGN KEY (tenant_id, department_id) REFERENCES iam_departments(tenant_id, id)
);
CREATE INDEX mcp_call_audits_owner ON mcp_call_audits(tenant_id, account_id, id DESC);
-- +goose Down
DROP TABLE mcp_call_audits, mcp_token_tools, mcp_tokens;
