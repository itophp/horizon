-- +goose Up
UPDATE runtime_installation_templates
SET version = '1.0.1'
WHERE plugin_id = 'builtin:analytics' AND version = '1.0.0';

-- Only advance provisioned installations that have never received a definition.
-- Published or proposed definitions are upgraded through the Runtime lifecycle.
UPDATE runtime_installations AS installation
SET version = '1.0.1',
    previous_version = '1.0.0',
    revision = installation.revision + 1,
    updated_at = now()
WHERE installation.plugin_id = 'builtin:analytics'
  AND installation.version = '1.0.0'
  AND installation.status IN ('enabled', 'installed')
  AND installation.digest = ''
  AND NOT EXISTS (
    SELECT 1 FROM runtime_application_heads AS head
    WHERE head.tenant_id = installation.tenant_id
      AND head.plugin_id = installation.plugin_id
  );

-- +goose Down
UPDATE runtime_installation_templates
SET version = '1.0.0'
WHERE plugin_id = 'builtin:analytics' AND version = '1.0.1';
-- Retain installations and their immutable definitions on rollback.
