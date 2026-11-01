-- +goose Up

-- Fragments and their vectors live in the vector store; the document keeps the
-- source text and the fragment count so a re-index can rebuild every vector
-- without asking anyone to paste the document again.
ALTER TABLE kb_documents ADD COLUMN chunks int NOT NULL DEFAULT 0 CHECK (chunks >= 0);
ALTER TABLE kb_documents ADD COLUMN content text NOT NULL DEFAULT '';
