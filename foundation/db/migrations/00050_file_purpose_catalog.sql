-- +goose Up
-- File purposes become a plugin-declared catalog: the database enforces the
-- identifier format while the files service validates declared purposes at
-- runtime. New business purposes no longer require a migration.
ALTER TABLE gov_files DROP CONSTRAINT gov_files_purpose_check;
ALTER TABLE gov_files ADD CONSTRAINT gov_files_purpose_check
 CHECK (purpose ~ '^[a-z][a-z0-9_]{0,63}$');

-- +goose Down
ALTER TABLE gov_files DROP CONSTRAINT gov_files_purpose_check;
ALTER TABLE gov_files ADD CONSTRAINT gov_files_purpose_check
 CHECK (purpose IN ('logo','attachment','cloud_drive','mail_attachment'));
