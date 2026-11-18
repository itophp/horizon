-- +goose Up
ALTER TABLE kb_documents ADD COLUMN version text NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE kb_documents DROP COLUMN version;
