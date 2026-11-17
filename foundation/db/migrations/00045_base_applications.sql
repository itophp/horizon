-- +goose Up
-- Platform-managed base applications ship with every tenant. The identity
-- package covers accounts, organizations, permissions and the runtime protocol
-- application; the other applications of a package are provisioned from the
-- same deployment snapshot by RuntimeProvisionTenant and reconciliation.
INSERT INTO runtime_installation_templates (plugin_id, version, application_id, enabled)
VALUES ('system.identity', '2.3.0', 'accounts', true)
ON CONFLICT (plugin_id) DO NOTHING;

-- +goose Down
DELETE FROM runtime_installation_templates WHERE plugin_id = 'system.identity';
