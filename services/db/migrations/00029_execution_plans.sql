-- +goose Up
CREATE TABLE cap_execution_plans (
 tenant_id text NOT NULL, plan_id text NOT NULL, version bigint NOT NULL CHECK(version > 0),
 capability text NOT NULL, capability_version text NOT NULL, name text NOT NULL,
 mode text NOT NULL CHECK(mode IN ('direct','workflow','graph')),
 implementation_ref text NOT NULL, contract_hash text NOT NULL,
 status text NOT NULL CHECK(status IN ('draft','validated','published','archived')),
 created_by bigint NOT NULL, created_at timestamptz NOT NULL DEFAULT now(),
 published_by bigint NOT NULL DEFAULT 0, published_at timestamptz,
 PRIMARY KEY(tenant_id,plan_id,version),
 UNIQUE(tenant_id,capability,capability_version,plan_id,version)
);
CREATE TABLE cap_execution_bindings (
 tenant_id text NOT NULL, capability text NOT NULL, capability_version text NOT NULL,
 plan_tenant_id text NOT NULL, plan_id text NOT NULL, plan_version bigint NOT NULL,
 revision bigint NOT NULL CHECK(revision > 0), updated_by bigint NOT NULL,
 updated_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY(tenant_id,capability,capability_version),
 CHECK(plan_tenant_id = '' OR plan_tenant_id = tenant_id),
 FOREIGN KEY(plan_tenant_id,capability,capability_version,plan_id,plan_version)
 REFERENCES cap_execution_plans(tenant_id,capability,capability_version,plan_id,version)
);
CREATE TABLE cap_execution_binding_changes (
 id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY, tenant_id text NOT NULL,
 capability text NOT NULL, capability_version text NOT NULL, plan_id text NOT NULL,
 plan_version bigint NOT NULL, revision bigint NOT NULL, action text NOT NULL,
 reason text NOT NULL, account_id bigint NOT NULL, created_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE cap_execution_records (
 tenant_id text NOT NULL, invocation_id text NOT NULL, account_id bigint NOT NULL,
 binding jsonb NOT NULL CHECK(jsonb_typeof(binding)='object'),
 PRIMARY KEY(tenant_id,invocation_id),
 FOREIGN KEY(tenant_id,invocation_id) REFERENCES cap_invocations(tenant_id,invocation_id)
);
-- Existing pending approvals and historical invocations retain the native plan.
INSERT INTO cap_execution_records(tenant_id,invocation_id,account_id,binding)
SELECT tenant_id,invocation_id,account_id,jsonb_build_object(
 'bindingId',capability||'@'||capability_version||'.default.platform','scopeType','platform','revision',0,
 'plan',jsonb_build_object('planId',capability||'@'||capability_version||'.default',
 'capabilityId',capability,'capabilityVersion',capability_version,'version',1,
 'mode','direct','implementationRef',capability||'@'||capability_version,'status','published'))
FROM cap_invocations;
-- Definitions are immutable even before publication: editing creates a version.
-- +goose StatementBegin
CREATE FUNCTION cap_execution_plan_guard() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF TG_OP='DELETE' OR OLD.status IN ('published','archived') OR
 (NEW.tenant_id,NEW.plan_id,NEW.version,NEW.capability,NEW.capability_version,NEW.name,NEW.mode,NEW.implementation_ref,NEW.contract_hash,NEW.created_by,NEW.created_at)
 IS DISTINCT FROM
 (OLD.tenant_id,OLD.plan_id,OLD.version,OLD.capability,OLD.capability_version,OLD.name,OLD.mode,OLD.implementation_ref,OLD.contract_hash,OLD.created_by,OLD.created_at)
 THEN RAISE EXCEPTION 'immutable execution plan' USING ERRCODE='23514'; END IF;
 RETURN NEW;
END $$;
-- +goose StatementEnd
CREATE TRIGGER cap_execution_plan_immutable BEFORE UPDATE OR DELETE ON cap_execution_plans FOR EACH ROW EXECUTE FUNCTION cap_execution_plan_guard();
-- +goose Down
DROP TABLE cap_execution_records;
DROP TABLE cap_execution_binding_changes;
DROP TABLE cap_execution_bindings;
DROP TABLE cap_execution_plans;
DROP FUNCTION cap_execution_plan_guard();
