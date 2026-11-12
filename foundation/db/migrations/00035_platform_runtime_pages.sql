-- +goose Up
INSERT INTO runtime_installation_templates (plugin_id, version, application_id, enabled)
VALUES
    ('builtin:agent-collaboration', '1.0.0', 'agent-collaboration', true),
    ('builtin:personal-preferences', '1.0.0', 'personal-preferences', true),
    ('builtin:system-settings', '1.0.0', 'system-settings', true);

INSERT INTO runtime_installations (tenant_id, plugin_id, version, status)
SELECT tenants.tenant_id, templates.plugin_id, templates.version, 'enabled'
FROM (SELECT DISTINCT tenant_id FROM iam_accounts) tenants
CROSS JOIN runtime_installation_templates templates
WHERE templates.plugin_id IN ('builtin:agent-collaboration', 'builtin:personal-preferences', 'builtin:system-settings')
ON CONFLICT (tenant_id, plugin_id) DO NOTHING;

-- +goose Down
DELETE FROM runtime_installation_templates
WHERE plugin_id IN ('builtin:agent-collaboration', 'builtin:personal-preferences', 'builtin:system-settings');
