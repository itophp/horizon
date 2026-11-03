-- +goose Up
CREATE EXTENSION IF NOT EXISTS pg_trgm;
CREATE TABLE kb_documents (
 id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
 tenant_id text NOT NULL,
 title text NOT NULL CHECK (length(title) BETWEEN 1 AND 200),
 source text NOT NULL DEFAULT 'manual' CHECK (source IN ('manual','file')),
 created_by bigint NOT NULL,
 created_at timestamptz NOT NULL DEFAULT now(),
 UNIQUE (tenant_id, id)
);
CREATE TABLE kb_chunks (
 id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
 tenant_id text NOT NULL,
 document_id bigint NOT NULL,
 ordinal int NOT NULL CHECK (ordinal >= 0),
 content text NOT NULL CHECK (length(content) BETWEEN 1 AND 4000),
 FOREIGN KEY (tenant_id, document_id) REFERENCES kb_documents(tenant_id, id) ON DELETE CASCADE,
 UNIQUE (document_id, ordinal)
);
-- Trigram index keeps retrieval usable for CJK text without a language pack.
CREATE INDEX kb_chunks_content_trgm ON kb_chunks USING gin (content gin_trgm_ops);
CREATE INDEX kb_chunks_tenant_document ON kb_chunks (tenant_id, document_id);

-- +goose Down
DROP TABLE IF EXISTS kb_chunks;
DROP TABLE IF EXISTS kb_documents;
