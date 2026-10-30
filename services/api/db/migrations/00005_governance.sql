-- +goose Up
ALTER TABLE iam_temporary_requests ADD CONSTRAINT iam_temporary_tenant_id_unique UNIQUE(tenant_id,id);
CREATE TABLE gov_scopes (
 tenant_id text NOT NULL, scope_key text NOT NULL, kind text NOT NULL CHECK(kind IN('workspace','enterprise','company','department','account')), entity_id bigint NOT NULL,
 organization_id bigint, department_id bigint, account_id bigint,
 PRIMARY KEY(tenant_id,scope_key),
 CHECK(scope_key=kind||':'||entity_id::text),
 FOREIGN KEY(tenant_id,organization_id) REFERENCES iam_organizations(tenant_id,id),
 FOREIGN KEY(tenant_id,department_id) REFERENCES iam_departments(tenant_id,id),
 FOREIGN KEY(tenant_id,account_id) REFERENCES iam_accounts(tenant_id,id),
 CHECK((kind='workspace' AND entity_id=0 AND organization_id IS NULL AND department_id IS NULL AND account_id IS NULL) OR (kind IN('enterprise','company') AND organization_id IS NOT NULL AND entity_id=organization_id AND department_id IS NULL AND account_id IS NULL) OR (kind='department' AND department_id IS NOT NULL AND entity_id=department_id AND organization_id IS NULL AND account_id IS NULL) OR (kind='account' AND account_id IS NOT NULL AND entity_id=account_id AND organization_id IS NULL AND department_id IS NULL))
);
CREATE TABLE gov_revisions (
 tenant_id text NOT NULL, resource_key text NOT NULL, scope_key text NOT NULL, revision bigint NOT NULL DEFAULT 0,
 PRIMARY KEY(tenant_id,resource_key), FOREIGN KEY(tenant_id,scope_key) REFERENCES gov_scopes(tenant_id,scope_key)
);
CREATE TABLE gov_values (
 tenant_id text NOT NULL, scope_key text NOT NULL, config_key text NOT NULL, value_json jsonb NOT NULL,
 PRIMARY KEY(tenant_id,scope_key,config_key), FOREIGN KEY(tenant_id,scope_key) REFERENCES gov_scopes(tenant_id,scope_key)
);
CREATE TABLE gov_drafts (
 id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,tenant_id text NOT NULL,domain text NOT NULL CHECK(domain IN('settings','dictionary','override')),
 resource_key text NOT NULL,scope_key text NOT NULL,entity_id bigint NOT NULL DEFAULT 0,base_revision bigint NOT NULL,
 payload jsonb NOT NULL,reason text NOT NULL CHECK(char_length(reason) BETWEEN 5 AND 500),sensitive boolean NOT NULL,
 status text NOT NULL DEFAULT 'draft' CHECK(status IN('draft','published')),actor_id bigint NOT NULL,created_at timestamptz NOT NULL DEFAULT now(),
 UNIQUE(tenant_id,id), FOREIGN KEY(tenant_id,scope_key) REFERENCES gov_scopes(tenant_id,scope_key),FOREIGN KEY(tenant_id,actor_id) REFERENCES iam_accounts(tenant_id,id)
);
CREATE TABLE gov_versions (
 id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,tenant_id text NOT NULL,domain text NOT NULL,resource_key text NOT NULL,scope_key text NOT NULL,entity_id bigint NOT NULL DEFAULT 0,
 revision bigint NOT NULL,payload jsonb NOT NULL,reason text NOT NULL,actor_id bigint NOT NULL,created_at timestamptz NOT NULL DEFAULT now(),
 UNIQUE(tenant_id,id),UNIQUE(tenant_id,resource_key,revision),FOREIGN KEY(tenant_id,scope_key) REFERENCES gov_scopes(tenant_id,scope_key),FOREIGN KEY(tenant_id,actor_id) REFERENCES iam_accounts(tenant_id,id)
);
CREATE TABLE gov_audit (
 id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,tenant_id text NOT NULL,scope_key text NOT NULL,actor_id bigint NOT NULL,action text NOT NULL,resource_key text NOT NULL,revision bigint NOT NULL,detail jsonb NOT NULL,created_at timestamptz NOT NULL DEFAULT now(),
 FOREIGN KEY(tenant_id,scope_key) REFERENCES gov_scopes(tenant_id,scope_key),FOREIGN KEY(tenant_id,actor_id) REFERENCES iam_accounts(tenant_id,id)
);
CREATE TABLE gov_reauth (
 token_hash text PRIMARY KEY,tenant_id text NOT NULL,session_hash text NOT NULL,department_id bigint NOT NULL,operation text NOT NULL,resource_id bigint NOT NULL,revision bigint NOT NULL,expires_at timestamptz NOT NULL,
 FOREIGN KEY(session_hash) REFERENCES iam_sessions(token_hash) ON DELETE CASCADE,FOREIGN KEY(tenant_id,department_id) REFERENCES iam_departments(tenant_id,id)
);
CREATE TABLE gov_dictionaries (
 id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,tenant_id text NOT NULL,scope_key text NOT NULL,code text NOT NULL,spec jsonb NOT NULL,revision bigint NOT NULL DEFAULT 0,
 UNIQUE(tenant_id,id),UNIQUE(tenant_id,code),FOREIGN KEY(tenant_id,scope_key) REFERENCES gov_scopes(tenant_id,scope_key)
);
CREATE TABLE gov_dictionary_items (
 id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,tenant_id text NOT NULL,dictionary_id bigint NOT NULL,scope_key text NOT NULL,code text NOT NULL,parent_code text NOT NULL DEFAULT '',spec jsonb NOT NULL,
 UNIQUE(tenant_id,id),UNIQUE(tenant_id,dictionary_id,code),UNIQUE(tenant_id,dictionary_id,id),
 FOREIGN KEY(tenant_id,dictionary_id) REFERENCES gov_dictionaries(tenant_id,id),FOREIGN KEY(tenant_id,scope_key) REFERENCES gov_scopes(tenant_id,scope_key)
);
CREATE TABLE gov_dictionary_overrides (
 tenant_id text NOT NULL,dictionary_id bigint NOT NULL,item_id bigint NOT NULL,scope_key text NOT NULL,spec jsonb NOT NULL,
 PRIMARY KEY(tenant_id,dictionary_id,item_id,scope_key),FOREIGN KEY(tenant_id,dictionary_id,item_id) REFERENCES gov_dictionary_items(tenant_id,dictionary_id,id),FOREIGN KEY(tenant_id,scope_key) REFERENCES gov_scopes(tenant_id,scope_key)
);
CREATE TABLE gov_dictionary_references (
 tenant_id text NOT NULL,dictionary_id bigint NOT NULL,item_id bigint NOT NULL,resource_type text NOT NULL,resource_id text NOT NULL,
 PRIMARY KEY(tenant_id,dictionary_id,item_id,resource_type,resource_id),FOREIGN KEY(tenant_id,dictionary_id,item_id) REFERENCES gov_dictionary_items(tenant_id,dictionary_id,id)
);
CREATE TABLE gov_storage_profiles (
 id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,tenant_id text NOT NULL,driver text NOT NULL,config_hash text NOT NULL,config jsonb NOT NULL,created_at timestamptz NOT NULL DEFAULT now(),
 UNIQUE(tenant_id,id),UNIQUE(tenant_id,config_hash)
);
CREATE TABLE gov_files (
 id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,tenant_id text NOT NULL,owner_id bigint NOT NULL,organization_id bigint NOT NULL,department_id bigint NOT NULL,
 storage_profile_id bigint NOT NULL,storage_key text NOT NULL,name text NOT NULL,mime text NOT NULL,size bigint NOT NULL CHECK(size>0),purpose text NOT NULL CHECK(purpose IN('logo','attachment')),sha256 text NOT NULL,
 created_at timestamptz NOT NULL DEFAULT now(),deleted_at timestamptz,purged_at timestamptz,
 UNIQUE(tenant_id,id),UNIQUE(tenant_id,storage_profile_id,storage_key),
 FOREIGN KEY(tenant_id,owner_id) REFERENCES iam_accounts(tenant_id,id),FOREIGN KEY(tenant_id,department_id,organization_id) REFERENCES iam_departments(tenant_id,id,organization_id),FOREIGN KEY(tenant_id,storage_profile_id) REFERENCES gov_storage_profiles(tenant_id,id)
);
CREATE TABLE gov_file_bindings (
 tenant_id text NOT NULL,file_id bigint NOT NULL,scope_key text NOT NULL,revision bigint NOT NULL,
 PRIMARY KEY(tenant_id,file_id,scope_key,revision),FOREIGN KEY(tenant_id,file_id) REFERENCES gov_files(tenant_id,id),FOREIGN KEY(tenant_id,scope_key) REFERENCES gov_scopes(tenant_id,scope_key)
);
CREATE TABLE gov_events (
 id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,tenant_id text NOT NULL,request_id bigint NOT NULL,event text NOT NULL,processed boolean NOT NULL DEFAULT false,lease_until timestamptz,attempts int NOT NULL DEFAULT 0,created_at timestamptz NOT NULL DEFAULT now(),
 UNIQUE(tenant_id,id),UNIQUE(tenant_id,request_id,event),FOREIGN KEY(tenant_id,request_id) REFERENCES iam_temporary_requests(tenant_id,id) ON DELETE CASCADE
);
CREATE TABLE gov_notifications (
 id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,tenant_id text NOT NULL,event_id bigint NOT NULL,recipient_id bigint NOT NULL,department_id bigint NOT NULL,payload jsonb NOT NULL,read_at timestamptz,created_at timestamptz NOT NULL DEFAULT now(),
 UNIQUE(tenant_id,id),UNIQUE(tenant_id,event_id,recipient_id,department_id),
 FOREIGN KEY(tenant_id,event_id) REFERENCES gov_events(tenant_id,id) ON DELETE CASCADE,FOREIGN KEY(tenant_id,recipient_id) REFERENCES iam_accounts(tenant_id,id) ON DELETE CASCADE,FOREIGN KEY(tenant_id,department_id) REFERENCES iam_departments(tenant_id,id)
);
CREATE TABLE gov_deliveries (
 id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,tenant_id text NOT NULL,message_id bigint NOT NULL,channel text NOT NULL DEFAULT 'smtp',status text NOT NULL DEFAULT 'pending' CHECK(status IN('pending','processing','sent','failed','skipped')),
 attempts int NOT NULL DEFAULT 0,error_code text NOT NULL DEFAULT '',next_attempt_at timestamptz NOT NULL DEFAULT now(),lease_until timestamptz,created_at timestamptz NOT NULL DEFAULT now(),
 UNIQUE(tenant_id,message_id,channel), FOREIGN KEY(tenant_id,message_id) REFERENCES gov_notifications(tenant_id,id) ON DELETE CASCADE
);
CREATE INDEX gov_events_pending ON gov_events(processed,lease_until);
CREATE INDEX gov_deliveries_pending ON gov_deliveries(status,next_attempt_at,lease_until);
CREATE INDEX gov_inbox ON gov_notifications(tenant_id,recipient_id,department_id,created_at DESC);
ALTER TABLE iam_sessions ADD COLUMN last_seen_at timestamptz NOT NULL DEFAULT now();
-- +goose StatementBegin
CREATE FUNCTION gov_temporary_event() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF TG_OP='INSERT' THEN
  INSERT INTO gov_events(tenant_id,request_id,event) VALUES(NEW.tenant_id,NEW.id,'requested') ON CONFLICT DO NOTHING;
 ELSIF NEW.status<>OLD.status AND NEW.status IN('approved','rejected','revoked') THEN
  INSERT INTO gov_events(tenant_id,request_id,event) VALUES(NEW.tenant_id,NEW.id,NEW.status) ON CONFLICT DO NOTHING;
 END IF;
 RETURN NEW;
END $$;
-- +goose StatementEnd
CREATE TRIGGER gov_temporary_event_trigger AFTER INSERT OR UPDATE ON iam_temporary_requests FOR EACH ROW EXECUTE FUNCTION gov_temporary_event();
-- +goose Down

DROP TRIGGER gov_temporary_event_trigger ON iam_temporary_requests;
DROP FUNCTION gov_temporary_event();
ALTER TABLE iam_sessions DROP COLUMN last_seen_at;
DROP TABLE gov_deliveries,gov_notifications,gov_events,gov_file_bindings,gov_files,gov_storage_profiles,gov_dictionary_references,gov_dictionary_overrides,gov_dictionary_items,gov_dictionaries,gov_reauth,gov_audit,gov_versions,gov_drafts,gov_values,gov_revisions,gov_scopes;

ALTER TABLE iam_temporary_requests DROP CONSTRAINT iam_temporary_tenant_id_unique;
