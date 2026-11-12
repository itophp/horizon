-- +goose Up
INSERT INTO runtime_installation_templates (plugin_id, version, application_id, enabled)
VALUES ('builtin:analytics', '1.0.0', 'analytics', true);

INSERT INTO runtime_installations (tenant_id, plugin_id, version, status)
SELECT DISTINCT tenant_id, 'builtin:analytics', '1.0.0', 'enabled'
FROM iam_accounts
ON CONFLICT (tenant_id, plugin_id) DO NOTHING;

-- Runtime reconciliation initializes the application state and immutable
-- deployment definition from the compiled contribution.

-- +goose Down
DELETE FROM runtime_installation_templates WHERE plugin_id = 'builtin:analytics';
-- Preserve existing installations and published snapshots for tenant history.
