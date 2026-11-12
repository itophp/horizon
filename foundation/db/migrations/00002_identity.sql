-- +goose Up
CREATE TABLE iam_organizations (
 id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY, tenant_id text NOT NULL,
 name text NOT NULL CHECK(char_length(name) BETWEEN 1 AND 100), code text NOT NULL,
 kind text NOT NULL CHECK(kind IN ('enterprise','company')), parent_id bigint,
 status text NOT NULL CHECK(status IN ('active','inactive')), description text NOT NULL DEFAULT '',
 created_at timestamptz NOT NULL DEFAULT now(), UNIQUE(tenant_id,id), UNIQUE(tenant_id,code),
 FOREIGN KEY(tenant_id,parent_id) REFERENCES iam_organizations(tenant_id,id),
 CHECK((kind='enterprise' AND parent_id IS NULL) OR (kind='company' AND parent_id IS NOT NULL))
);
CREATE TABLE iam_departments (
 id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY, tenant_id text NOT NULL,
 organization_id bigint NOT NULL, parent_id bigint, name text NOT NULL CHECK(char_length(name) BETWEEN 1 AND 100),
 code text NOT NULL, manager text NOT NULL DEFAULT '', status text NOT NULL CHECK(status IN ('active','inactive')),
 description text NOT NULL DEFAULT '', created_at timestamptz NOT NULL DEFAULT now(), UNIQUE(tenant_id,id), UNIQUE(tenant_id,id,organization_id), UNIQUE(tenant_id,organization_id,code),
 FOREIGN KEY(tenant_id,organization_id) REFERENCES iam_organizations(tenant_id,id),
 FOREIGN KEY(tenant_id,parent_id,organization_id) REFERENCES iam_departments(tenant_id,id,organization_id)
);
CREATE TABLE iam_roles (
 id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY, tenant_id text NOT NULL,
 name text NOT NULL CHECK(char_length(name) BETWEEN 1 AND 100), code text NOT NULL,
 description text NOT NULL DEFAULT '', status text NOT NULL CHECK(status IN ('active','inactive')),
 grants jsonb NOT NULL DEFAULT '[]' CHECK(jsonb_typeof(grants)='array'),
 created_at timestamptz NOT NULL DEFAULT now(), UNIQUE(tenant_id,id), UNIQUE(tenant_id,code)
);
CREATE TABLE iam_accounts (
 id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY, tenant_id text NOT NULL,
 username text NOT NULL UNIQUE, full_name text NOT NULL, email text NOT NULL DEFAULT '',
 password_hash text NOT NULL, status text NOT NULL CHECK(status IN ('active','inactive')),
 organization_id bigint NOT NULL, department_id bigint NOT NULL,
 overrides jsonb NOT NULL DEFAULT '[]' CHECK(jsonb_typeof(overrides)='array'), is_admin boolean NOT NULL DEFAULT false,
 created_at timestamptz NOT NULL DEFAULT now(), last_login timestamptz,
 UNIQUE(tenant_id,id), FOREIGN KEY(tenant_id,organization_id) REFERENCES iam_organizations(tenant_id,id),
 FOREIGN KEY(tenant_id,department_id,organization_id) REFERENCES iam_departments(tenant_id,id,organization_id)
);
CREATE INDEX iam_accounts_scope ON iam_accounts(tenant_id,organization_id,department_id);
CREATE TABLE iam_account_roles (
 tenant_id text NOT NULL, account_id bigint NOT NULL, role_id bigint NOT NULL,
 PRIMARY KEY(tenant_id,account_id,role_id),
 FOREIGN KEY(tenant_id,account_id) REFERENCES iam_accounts(tenant_id,id) ON DELETE CASCADE,
 FOREIGN KEY(tenant_id,role_id) REFERENCES iam_roles(tenant_id,id) ON DELETE RESTRICT
);
CREATE TABLE iam_sessions (
 token_hash text PRIMARY KEY, tenant_id text NOT NULL, account_id bigint NOT NULL, csrf_token text NOT NULL,
 expires_at timestamptz NOT NULL, created_at timestamptz NOT NULL DEFAULT now(),
 FOREIGN KEY(tenant_id,account_id) REFERENCES iam_accounts(tenant_id,id) ON DELETE CASCADE
);
CREATE INDEX iam_sessions_account ON iam_sessions(tenant_id,account_id);
CREATE TABLE iam_login_attempts (attempt_key text PRIMARY KEY, failures integer NOT NULL DEFAULT 0, locked_until timestamptz NOT NULL DEFAULT now());
-- +goose Down
DROP TABLE iam_login_attempts, iam_sessions, iam_account_roles, iam_accounts, iam_roles, iam_departments, iam_organizations;
