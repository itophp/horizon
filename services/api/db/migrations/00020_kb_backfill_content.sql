-- +goose Up

-- Documents indexed before fragments moved to the vector store never had their
-- source text kept: only kb_chunks held it. Rebuild it once so a re-index can
-- re-embed the existing corpus instead of asking operators to paste it again.
-- Fragments overlap by design, so the restored text repeats a few sentences;
-- the re-index splits it again, which is acceptable for a one-off backfill.
UPDATE kb_documents d
SET content = COALESCE((
      SELECT string_agg(c.content, '' ORDER BY c.ordinal)
      FROM kb_chunks c
      WHERE c.tenant_id = d.tenant_id AND c.document_id = d.id
    ), ''),
    chunks = COALESCE((
      SELECT count(*)
      FROM kb_chunks c
      WHERE c.tenant_id = d.tenant_id AND c.document_id = d.id
    ), 0)
WHERE d.content = '';
