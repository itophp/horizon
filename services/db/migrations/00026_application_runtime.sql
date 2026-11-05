-- +goose Up
CREATE TABLE runtime_installation_templates (
 plugin_id text PRIMARY KEY, version text NOT NULL, application_id text NOT NULL UNIQUE,
 enabled boolean NOT NULL DEFAULT false
);
-- Explicit deployment provisioning policy; never inferred from a missing installation.
INSERT INTO runtime_installation_templates VALUES
 ('builtin:articles','1.0.0','articles',true), ('com.horizon.projects','1.0.0','projects',true);
CREATE TABLE runtime_installations (
 tenant_id text NOT NULL, plugin_id text NOT NULL, version text NOT NULL,
 status text NOT NULL CHECK(status IN ('installed','enabled','disabled','uninstalled')),
 revision bigint NOT NULL DEFAULT 1, digest text NOT NULL DEFAULT '',
 previous_version text NOT NULL DEFAULT '', updated_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY(tenant_id,plugin_id)
);
CREATE TABLE runtime_application_states (
 tenant_id text NOT NULL, application_id text NOT NULL, plugin_id text NOT NULL,
 enabled boolean NOT NULL DEFAULT true, revision bigint NOT NULL DEFAULT 1,
 PRIMARY KEY(tenant_id,application_id),
 FOREIGN KEY(tenant_id,plugin_id) REFERENCES runtime_installations(tenant_id,plugin_id)
);
CREATE TABLE runtime_installation_history (
 id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY, tenant_id text NOT NULL,
 plugin_id text NOT NULL, account_id bigint NOT NULL, action text NOT NULL,
 version text NOT NULL, revision bigint NOT NULL, created_at timestamptz NOT NULL DEFAULT now(),
 FOREIGN KEY(tenant_id,plugin_id) REFERENCES runtime_installations(tenant_id,plugin_id)
);
INSERT INTO runtime_installations(tenant_id,plugin_id,version,status)
 SELECT DISTINCT a.tenant_id,t.plugin_id,t.version,CASE WHEN t.enabled THEN 'enabled' ELSE 'installed' END
 FROM iam_accounts a CROSS JOIN runtime_installation_templates t;
INSERT INTO runtime_application_states(tenant_id,application_id,plugin_id)
 SELECT i.tenant_id,t.application_id,i.plugin_id FROM runtime_installations i
 JOIN runtime_installation_templates t ON t.plugin_id=i.plugin_id;
CREATE TABLE runtime_invocation_dependencies (
 tenant_id text NOT NULL, invocation_id text NOT NULL, plugin_id text NOT NULL,
 PRIMARY KEY(tenant_id,invocation_id,plugin_id),
 FOREIGN KEY(tenant_id,invocation_id) REFERENCES cap_invocations(tenant_id,invocation_id),
 FOREIGN KEY(tenant_id,plugin_id) REFERENCES runtime_installations(tenant_id,plugin_id)
);
CREATE TABLE runtime_run_dependencies (
 tenant_id text NOT NULL, run_id bigint NOT NULL, plugin_id text NOT NULL,
 PRIMARY KEY(tenant_id,run_id,plugin_id),
 FOREIGN KEY(tenant_id,run_id) REFERENCES ai_runs(tenant_id,id),
 FOREIGN KEY(tenant_id,plugin_id) REFERENCES runtime_installations(tenant_id,plugin_id)
);
CREATE TABLE ai_run_events (
 tenant_id text NOT NULL, run_id bigint NOT NULL, sequence bigint NOT NULL,
 event_type text NOT NULL, payload jsonb NOT NULL DEFAULT '{}', created_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY(tenant_id,run_id,sequence),
 FOREIGN KEY(tenant_id,run_id) REFERENCES ai_runs(tenant_id,id)
);
-- +goose StatementBegin
CREATE FUNCTION runtime_capture_ai_event() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE kind text; seq bigint;
BEGIN
 IF TG_OP='UPDATE' AND NEW.status=OLD.status THEN RETURN NEW; END IF;
 kind:=CASE NEW.status WHEN 'running' THEN 'RUN_STARTED' WHEN 'completed' THEN 'RUN_FINISHED'
 WHEN 'failed' THEN 'RUN_ERROR' WHEN 'cancelled' THEN 'RUN_ERROR' ELSE 'STATE_SNAPSHOT' END;
 PERFORM pg_advisory_xact_lock(hashtextextended(NEW.tenant_id||':run-event:'||NEW.id::text,0));
 SELECT COALESCE(MAX(sequence),0)+1 INTO seq FROM ai_run_events WHERE tenant_id=NEW.tenant_id AND run_id=NEW.id;
 INSERT INTO ai_run_events(tenant_id,run_id,sequence,event_type,payload)
 VALUES(NEW.tenant_id,NEW.id,seq,kind,jsonb_build_object('status',NEW.status,'code',NEW.error_code));
 RETURN NEW;
END $$;
-- +goose StatementEnd
CREATE TRIGGER runtime_ai_event AFTER INSERT OR UPDATE OF status ON ai_runs FOR EACH ROW EXECUTE FUNCTION runtime_capture_ai_event();
-- +goose StatementBegin
CREATE FUNCTION runtime_capture_tool_event() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE task record; kind text; seq bigint;
BEGIN
 IF TG_OP='UPDATE' AND NEW.status=OLD.status THEN RETURN NEW; END IF;
 IF NEW.status='executing' THEN kind:='TOOL_CALL_START';
 ELSIF NEW.status IN ('succeeded','failed','cancelled') THEN kind:='TOOL_CALL_END';
 ELSIF NEW.status='awaiting_approval' THEN kind:='STATE_SNAPSHOT'; ELSE RETURN NEW; END IF;
 FOR task IN SELECT id FROM ai_runs WHERE tenant_id=NEW.tenant_id AND account_id=NEW.account_id
 AND correlation_id=NEW.correlation_id AND status IN ('running','awaiting_approval') ORDER BY id LOOP
  PERFORM pg_advisory_xact_lock(hashtextextended(NEW.tenant_id||':run-event:'||task.id::text,0));
  SELECT COALESCE(MAX(sequence),0)+1 INTO seq FROM ai_run_events WHERE tenant_id=NEW.tenant_id AND run_id=task.id;
  INSERT INTO ai_run_events(tenant_id,run_id,sequence,event_type,payload)
  VALUES(NEW.tenant_id,task.id,seq,kind,jsonb_build_object('status',NEW.status,'toolCallId',NEW.invocation_id,'toolCallName',NEW.capability));
 END LOOP;
 RETURN NEW;
END $$;
-- +goose StatementEnd
CREATE TRIGGER runtime_tool_event AFTER INSERT OR UPDATE OF status ON cap_invocations FOR EACH ROW EXECUTE FUNCTION runtime_capture_tool_event();
-- +goose StatementBegin
CREATE FUNCTION runtime_project_scope_changed() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF TG_OP='DELETE' THEN
 UPDATE runtime_application_states SET revision=revision+1 WHERE tenant_id=OLD.tenant_id AND application_id='projects';
 ELSE
 UPDATE runtime_application_states SET revision=revision+1 WHERE tenant_id=NEW.tenant_id AND application_id='projects';
 END IF;
 RETURN NULL;
END $$;
-- +goose StatementEnd
CREATE TRIGGER runtime_project_scope AFTER INSERT OR DELETE ON project_members FOR EACH ROW EXECUTE FUNCTION runtime_project_scope_changed();
-- +goose Down
DROP TRIGGER runtime_project_scope ON project_members;
DROP FUNCTION runtime_project_scope_changed();
DROP TRIGGER runtime_tool_event ON cap_invocations;
DROP FUNCTION runtime_capture_tool_event();
DROP TABLE runtime_run_dependencies;
DROP TABLE runtime_invocation_dependencies;
DROP TRIGGER runtime_ai_event ON ai_runs;
DROP FUNCTION runtime_capture_ai_event();
DROP TABLE ai_run_events;
DROP TABLE runtime_installation_history;
DROP TABLE runtime_application_states;
DROP TABLE runtime_installations;
DROP TABLE runtime_installation_templates;
