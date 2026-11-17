-- +goose Up
UPDATE runtime_installation_templates
SET version = '1.0.3'
WHERE plugin_id = 'builtin:system-settings' AND version = '1.0.1';

-- +goose Down
UPDATE runtime_installation_templates
SET version = '1.0.1'
WHERE plugin_id = 'builtin:system-settings' AND version = '1.0.3';
