-- +goose Up
-- A published Runtime definition is immutable, and 1.0.0 shipped the automation
-- task form without the name and schedule targets of the AI form assistant.
-- Move the provisioning template and un-customized installations to 1.0.1;
-- tenants that published a definition use the Runtime installation upgrade flow
-- so their customization is rebased instead of silently overwritten.
UPDATE runtime_installation_templates
SET version = '1.0.1'
WHERE plugin_id = 'builtin:automation' AND version = '1.0.0';

UPDATE runtime_installations AS installation
SET version = '1.0.1',
    previous_version = '1.0.0',
    revision = installation.revision + 1,
    updated_at = now()
WHERE installation.plugin_id = 'builtin:automation'
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
WHERE plugin_id = 'builtin:automation' AND version = '1.0.1';
