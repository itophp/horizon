-- +goose Up
-- Configuration is shared within a tenant. Keep historical scoped settings and
-- overlays for audit/recovery, but no longer resolve them at runtime.
INSERT INTO gov_scopes(tenant_id,scope_key,kind,entity_id)
SELECT DISTINCT tenant_id,'workspace:0','workspace',0 FROM gov_dictionaries
ON CONFLICT DO NOTHING;
-- Preserve stable item IDs when former local extensions become shared items.
-- If formerly independent scopes each selected a default, retain one deterministically.
WITH ranked AS (
 SELECT i.id,row_number() OVER(PARTITION BY i.tenant_id,i.dictionary_id ORDER BY (i.scope_key=d.scope_key) DESC,COALESCE((i.spec->>'Sort')::integer,0),i.id) AS position
 FROM gov_dictionary_items i JOIN gov_dictionaries d ON d.tenant_id=i.tenant_id AND d.id=i.dictionary_id
 WHERE i.spec->>'Status'='active' AND i.spec->>'IsDefault'='true'
)
UPDATE gov_dictionary_items i SET spec=jsonb_set(i.spec,'{IsDefault}','false') FROM ranked r WHERE r.id=i.id AND r.position>1;
UPDATE gov_dictionary_items SET scope_key='workspace:0',spec=jsonb_set(spec,'{Owner}','{"Kind":"workspace","ID":0}');
UPDATE gov_dictionaries SET scope_key='workspace:0',spec=jsonb_set(jsonb_set(spec,'{Target}','{"Kind":"workspace","ID":0}'),'{AllowExtensions}','false');
UPDATE gov_revisions r SET scope_key='workspace:0' FROM gov_dictionaries d WHERE r.tenant_id=d.tenant_id AND r.resource_key='dictionary:'||d.id::text;
UPDATE gov_drafts SET scope_key='workspace:0' WHERE domain='dictionary';
UPDATE gov_versions SET scope_key='workspace:0' WHERE domain='dictionary';

-- +goose Down
-- Data consolidation deliberately does not split shared dictionaries back into
-- organizational scopes or remove stable references. Reverting code retains data.
SELECT 1;
