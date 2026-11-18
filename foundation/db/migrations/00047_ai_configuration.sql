-- +goose Up
CREATE TABLE ai_configuration (
 tenant_id text PRIMARY KEY,
 revision bigint NOT NULL CHECK (revision > 0),
 values_json jsonb NOT NULL CHECK (jsonb_typeof(values_json) = 'object')
);

-- Move only effective workspace overrides. Secret envelopes remain bound to
-- their existing tenant, workspace and setting key. Drafts never take effect.
INSERT INTO ai_configuration (tenant_id, revision, values_json)
SELECT tenant_id, 1, jsonb_object_agg(config_key, value_json)
FROM gov_values
WHERE scope_key = 'workspace:0'
  AND (config_key LIKE 'aiModels.%' OR config_key LIKE 'knowledge.%'
       OR config_key IN ('agent.enabled', 'agent.baseUrl', 'assistant.enabled',
                         'assistant.baseUrl', 'assistant.model', 'assistant.token'))
GROUP BY tenant_id;

DELETE FROM gov_values
WHERE scope_key = 'workspace:0'
  AND (config_key LIKE 'aiModels.%' OR config_key LIKE 'knowledge.%'
       OR config_key IN ('agent.enabled', 'agent.baseUrl', 'assistant.enabled',
                         'assistant.baseUrl', 'assistant.model', 'assistant.token'));

-- +goose Down
INSERT INTO gov_scopes (tenant_id, scope_key, kind, entity_id)
SELECT tenant_id, 'workspace:0', 'workspace', 0 FROM ai_configuration
ON CONFLICT DO NOTHING;

INSERT INTO gov_values (tenant_id, scope_key, config_key, value_json)
SELECT tenant_id, 'workspace:0', item.key, item.value
FROM ai_configuration CROSS JOIN LATERAL jsonb_each(values_json) AS item
ON CONFLICT (tenant_id, scope_key, config_key)
DO UPDATE SET value_json = EXCLUDED.value_json;

DROP TABLE ai_configuration;
