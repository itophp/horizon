-- +goose Up
-- TECHNICAL DEMONSTRATION ONLY: not a business model.
CREATE TABLE technical_demo_records (
 id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
 tenant_id text NOT NULL,
 organization_id text NOT NULL,
 department_id text NOT NULL,
 owner_id text NOT NULL,
 label text NOT NULL CHECK (char_length(label) BETWEEN 1 AND 80),
 internal_note text NOT NULL DEFAULT ''
);
CREATE INDEX technical_demo_scope ON technical_demo_records (tenant_id, owner_id);
-- +goose Down
DROP TABLE technical_demo_records;
