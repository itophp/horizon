-- +goose Up
-- The capability console slimmed its page surface and moved the example
-- playground into the example application package, so the deployed console
-- version is 1.1.0. Provisioning templates must ship the deployed version;
-- otherwise a tenant created between deployments provisions the previous
-- package and the provisioning snapshot join produces no published head.
INSERT INTO runtime_installation_templates (plugin_id, version, application_id, enabled)
VALUES ('builtin:capabilities', '1.1.0', 'capabilities', true)
ON CONFLICT (plugin_id) DO UPDATE
SET version = EXCLUDED.version, application_id = EXCLUDED.application_id, enabled = EXCLUDED.enabled;

-- +goose Down
DELETE FROM runtime_installation_templates WHERE plugin_id = 'builtin:capabilities';
