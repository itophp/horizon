-- +goose Up
-- Preserve invocation and event metadata while removing historical credentials.
UPDATE cap_invocations
SET result = result - 'data'
WHERE capability IN ('mcp.token.create', 'mcp.token.rotate', 'settings.password.verify')
  AND jsonb_typeof(result) = 'object'
  AND result -> 'data' IS NOT NULL
  AND result -> 'data' <> 'null'::jsonb;

-- +goose Down
-- Redacted credentials cannot be recovered.
SELECT 1;
