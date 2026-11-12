-- +goose Up
CREATE TABLE runtime_deployment_snapshots (
 digest text PRIMARY KEY, plugin_id text NOT NULL, plugin_version text NOT NULL,
 application_id text NOT NULL, base_version text NOT NULL,
 definition jsonb NOT NULL CHECK(jsonb_typeof(definition)='object'),
 contracts_digest text NOT NULL, created_at timestamptz NOT NULL DEFAULT now(),
 CHECK(octet_length(definition::text)<=1048576)
);
CREATE UNIQUE INDEX runtime_deployment_identity ON runtime_deployment_snapshots(plugin_id,plugin_version,application_id);
CREATE TABLE runtime_application_heads (
 tenant_id text NOT NULL, application_id text NOT NULL, plugin_id text NOT NULL,
 published_version text NOT NULL DEFAULT '', proposal_version text NOT NULL DEFAULT '',
 route_root text NOT NULL DEFAULT '',
 base_digest text NOT NULL REFERENCES runtime_deployment_snapshots(digest),
 revision bigint NOT NULL DEFAULT 1, next_configuration bigint NOT NULL DEFAULT 1,
 compatibility text NOT NULL DEFAULT 'compatible' CHECK(compatibility IN ('compatible','incompatible','pending_rebase')),
 PRIMARY KEY(tenant_id,application_id),
 FOREIGN KEY(tenant_id,plugin_id) REFERENCES runtime_installations(tenant_id,plugin_id)
);
CREATE UNIQUE INDEX runtime_application_route_root ON runtime_application_heads(tenant_id,route_root) WHERE route_root<>'';
CREATE TABLE runtime_application_versions (
 tenant_id text NOT NULL, application_id text NOT NULL, version text NOT NULL,
 parent_version text NOT NULL DEFAULT '', base_digest text NOT NULL REFERENCES runtime_deployment_snapshots(digest),
 definition jsonb NOT NULL CHECK(jsonb_typeof(definition)='object'), digest text NOT NULL,
 source text NOT NULL CHECK(source IN ('deployment','human','agent','rollback','rebase')),
 created_by bigint NOT NULL, agent_run_id bigint, correlation_id text NOT NULL DEFAULT '',
 created_at timestamptz NOT NULL DEFAULT now(), PRIMARY KEY(tenant_id,application_id,version),
 FOREIGN KEY(tenant_id,application_id) REFERENCES runtime_application_heads(tenant_id,application_id),
 FOREIGN KEY(tenant_id,agent_run_id) REFERENCES ai_runs(tenant_id,id),
 CHECK(version ~ '^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$'),
 CHECK(octet_length(definition::text)<=1048576)
);
CREATE TABLE runtime_application_changes (
 id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY, tenant_id text NOT NULL,
 application_id text NOT NULL, action text NOT NULL, version text NOT NULL DEFAULT '',
 digest text NOT NULL DEFAULT '', diagnostic_code text NOT NULL DEFAULT '',
 account_id bigint NOT NULL, correlation_id text NOT NULL DEFAULT '', created_at timestamptz NOT NULL DEFAULT now(),
 FOREIGN KEY(tenant_id,application_id) REFERENCES runtime_application_heads(tenant_id,application_id)
);
CREATE INDEX runtime_application_changes_history ON runtime_application_changes(tenant_id,application_id,id DESC);
-- +goose StatementBegin
CREATE FUNCTION runtime_immutable_definition() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN RAISE EXCEPTION 'immutable runtime definition' USING ERRCODE='23514'; END $$;
-- +goose StatementEnd
CREATE TRIGGER runtime_snapshot_immutable BEFORE UPDATE OR DELETE ON runtime_deployment_snapshots FOR EACH ROW EXECUTE FUNCTION runtime_immutable_definition();
CREATE TRIGGER runtime_version_immutable BEFORE UPDATE OR DELETE ON runtime_application_versions FOR EACH ROW EXECUTE FUNCTION runtime_immutable_definition();
-- Published and proposal pointers are checked at commit so initialization can
-- insert the head and its immutable versions in the same transaction.
-- +goose StatementBegin
CREATE FUNCTION runtime_definition_pointer_check() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF NEW.published_version<>'' AND NOT EXISTS(
  SELECT 1 FROM runtime_application_versions WHERE tenant_id=NEW.tenant_id AND application_id=NEW.application_id AND version=NEW.published_version
 ) THEN RAISE EXCEPTION 'invalid runtime published pointer' USING ERRCODE='23503'; END IF;
 IF NEW.proposal_version<>'' AND NOT EXISTS(
  SELECT 1 FROM runtime_application_versions WHERE tenant_id=NEW.tenant_id AND application_id=NEW.application_id AND version=NEW.proposal_version
 ) THEN RAISE EXCEPTION 'invalid runtime proposal pointer' USING ERRCODE='23503'; END IF;
 RETURN NEW;
END $$;
-- +goose StatementEnd
CREATE CONSTRAINT TRIGGER runtime_definition_pointers AFTER INSERT OR UPDATE ON runtime_application_heads DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION runtime_definition_pointer_check();
-- +goose Down
DROP TRIGGER runtime_definition_pointers ON runtime_application_heads;
DROP FUNCTION runtime_definition_pointer_check();
DROP TRIGGER runtime_version_immutable ON runtime_application_versions;
DROP TRIGGER runtime_snapshot_immutable ON runtime_deployment_snapshots;
DROP FUNCTION runtime_immutable_definition();
DROP TABLE runtime_application_changes;
DROP TABLE runtime_application_versions;
DROP TABLE runtime_application_heads;
DROP TABLE runtime_deployment_snapshots;
