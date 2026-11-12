-- +goose Up
-- Sanitized canonical observations are independent of audit and business facts.
CREATE TABLE memory_events (
 id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
 tenant_id text NOT NULL,
 organization_id text NOT NULL DEFAULT '',
 event_id text NOT NULL,
 event_type text NOT NULL,
 event_version text NOT NULL,
 occurred_at timestamptz NOT NULL,
 ingested_at timestamptz NOT NULL DEFAULT now(),
 actor_id text NOT NULL,
 actor_type text NOT NULL,
 department_id bigint NOT NULL DEFAULT 0,
 source_type text NOT NULL,
 source_id text NOT NULL DEFAULT '',
 source_module text NOT NULL DEFAULT '',
 resource_type text NOT NULL DEFAULT '',
 resource_id text NOT NULL DEFAULT '',
 action_name text NOT NULL DEFAULT '',
 result text NOT NULL DEFAULT '',
 invocation_id text NOT NULL DEFAULT '',
 session_id text NOT NULL DEFAULT '',
 task_id text NOT NULL DEFAULT '',
 workflow_instance_id text NOT NULL DEFAULT '',
 trace_id text NOT NULL DEFAULT '',
 correlation_id text NOT NULL DEFAULT '',
 causation_id text NOT NULL DEFAULT '',
 trigger_depth integer NOT NULL DEFAULT 0 CHECK (trigger_depth >= 0),
 sensitivity text NOT NULL DEFAULT 'internal',
 candidate_partitions text[] NOT NULL DEFAULT '{}'::text[],
 payload jsonb NOT NULL DEFAULT '{}'::jsonb,
 UNIQUE (tenant_id, event_id)
);
CREATE INDEX memory_events_owner_recent ON memory_events (tenant_id, actor_type, actor_id, occurred_at DESC, id DESC);
CREATE INDEX memory_events_resource_recent ON memory_events (tenant_id, resource_type, resource_id, occurred_at DESC, id DESC);
CREATE INDEX memory_events_correlation ON memory_events (tenant_id, correlation_id) WHERE correlation_id <> '';

-- A row per source event is a durable inbox. A global high-water mark is not
-- safe: PostgreSQL sequence IDs can become visible out of commit order.
CREATE TABLE memory_event_inbox (
 source_event_id bigint PRIMARY KEY,
 tenant_id text NOT NULL,
 event_id text NOT NULL,
 attempts integer NOT NULL DEFAULT 0 CHECK (attempts >= 0),
 available_at timestamptz NOT NULL DEFAULT now(),
 completed_at timestamptz,
 dead_at timestamptz,
 error_code text NOT NULL DEFAULT '',
 UNIQUE (tenant_id, event_id)
);
CREATE INDEX memory_event_inbox_pending ON memory_event_inbox (available_at, source_event_id) WHERE completed_at IS NULL AND dead_at IS NULL;

CREATE TABLE memory_items (
 id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
 tenant_id text NOT NULL,
 partition text NOT NULL CHECK (partition IN ('agent','activity','exception','business','knowledge','execution')),
 memory_type text NOT NULL,
 scope_type text NOT NULL CHECK (scope_type IN ('user','project','department','organization','business_entity')),
 scope_id text NOT NULL,
 owner_id bigint NOT NULL,
 title text NOT NULL,
 summary text NOT NULL,
 sensitivity text NOT NULL DEFAULT 'internal',
 verification_status text NOT NULL DEFAULT 'unverified' CHECK (verification_status IN ('unverified','confirmed','rejected')),
 status text NOT NULL DEFAULT 'active' CHECK (status IN ('active','archived','deleted')),
 version bigint NOT NULL DEFAULT 1 CHECK (version > 0),
 valid_from timestamptz NOT NULL DEFAULT now(),
 valid_to timestamptz,
 created_at timestamptz NOT NULL DEFAULT now(),
 updated_at timestamptz NOT NULL DEFAULT now(),
 UNIQUE (tenant_id, id),
 CHECK (valid_to IS NULL OR valid_to >= valid_from)
);
CREATE INDEX memory_items_owner ON memory_items (tenant_id, owner_id, partition, id DESC) WHERE status = 'active';
CREATE INDEX memory_items_scope ON memory_items (tenant_id, scope_type, scope_id, partition, id DESC) WHERE status = 'active';

-- The caller supplies scopes resolved by IAM from the trusted principal.
-- Both the read and write scope must include the same tenant resource.
CREATE FUNCTION memory_group_authorized(text,text,text,boolean,bigint[],bigint[],boolean,bigint[],bigint[],bigint)
RETURNS boolean LANGUAGE sql STABLE AS $$
 SELECT CASE $2
 WHEN 'department' THEN EXISTS (
  SELECT 1 FROM iam_departments d WHERE d.tenant_id=$1 AND d.id::text=$3
   AND ($4 OR d.organization_id=ANY($5) OR d.id=ANY($6))
   AND ($7 OR d.organization_id=ANY($8) OR d.id=ANY($9)))
 WHEN 'organization' THEN EXISTS (
  SELECT 1 FROM iam_organizations o WHERE o.tenant_id=$1 AND o.id::text=$3
   AND ($4 OR o.id=ANY($5)) AND ($7 OR o.id=ANY($8)))
 WHEN 'business_entity' THEN EXISTS (
  SELECT 1 FROM example_records e WHERE e.tenant_id=$1 AND ('example_record:'||e.id::text)=$3 AND e.created_by=$10)
 ELSE false END
$$;

CREATE TABLE memory_sources (
 id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
 tenant_id text NOT NULL,
 memory_id bigint NOT NULL,
 event_id text NOT NULL,
 created_at timestamptz NOT NULL DEFAULT now(),
 UNIQUE (tenant_id, memory_id, event_id),
 FOREIGN KEY (tenant_id, memory_id) REFERENCES memory_items (tenant_id, id) ON DELETE CASCADE,
 FOREIGN KEY (tenant_id, event_id) REFERENCES memory_events (tenant_id, event_id)
);

CREATE TABLE memory_relations (
 id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
 tenant_id text NOT NULL,
 source_memory_id bigint NOT NULL,
 target_memory_id bigint NOT NULL,
 relation_type text NOT NULL,
 created_at timestamptz NOT NULL DEFAULT now(),
 CHECK (source_memory_id <> target_memory_id),
 UNIQUE (tenant_id, source_memory_id, target_memory_id, relation_type),
 FOREIGN KEY (tenant_id, source_memory_id) REFERENCES memory_items (tenant_id, id) ON DELETE CASCADE,
 FOREIGN KEY (tenant_id, target_memory_id) REFERENCES memory_items (tenant_id, id) ON DELETE CASCADE
);
CREATE INDEX memory_relations_target ON memory_relations (tenant_id, target_memory_id);

CREATE TABLE memory_corrections (
 id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
 tenant_id text NOT NULL,
 memory_id bigint NOT NULL,
 actor_id bigint NOT NULL,
 previous_version bigint NOT NULL,
 previous_title text NOT NULL,
 previous_summary text NOT NULL,
 created_at timestamptz NOT NULL DEFAULT now(),
 FOREIGN KEY (tenant_id, memory_id) REFERENCES memory_items (tenant_id, id) ON DELETE CASCADE,
 UNIQUE (tenant_id, memory_id, previous_version)
);

CREATE TABLE memory_access_audit (
 id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
 tenant_id text NOT NULL,
 actor_id bigint NOT NULL,
 action text NOT NULL,
 object_id bigint NOT NULL DEFAULT 0,
 correlation_id text NOT NULL DEFAULT '',
 created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX memory_access_audit_actor ON memory_access_audit (tenant_id, actor_id, created_at DESC, id DESC);

-- Zero retention days means no automatic expiry has been configured. This
-- phase stores policy only; it does not purge events or memories.
CREATE TABLE memory_partition_policies (
 id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
 tenant_id text NOT NULL,
 partition text NOT NULL CHECK (partition IN ('agent','activity','exception','business','knowledge','execution')),
 retention_days integer NOT NULL DEFAULT 0 CHECK (retention_days >= 0),
 revision bigint NOT NULL DEFAULT 1 CHECK (revision > 0),
 updated_by bigint NOT NULL,
 updated_at timestamptz NOT NULL DEFAULT now(),
 UNIQUE (tenant_id, partition)
);

-- +goose Down
DROP TABLE IF EXISTS memory_partition_policies;
DROP TABLE IF EXISTS memory_access_audit;
DROP TABLE IF EXISTS memory_corrections;
DROP TABLE IF EXISTS memory_relations;
DROP TABLE IF EXISTS memory_sources;
DROP FUNCTION IF EXISTS memory_group_authorized(text,text,text,boolean,bigint[],bigint[],boolean,bigint[],bigint[],bigint);
DROP TABLE IF EXISTS memory_items;
DROP TABLE IF EXISTS memory_event_inbox;
DROP TABLE IF EXISTS memory_events;
