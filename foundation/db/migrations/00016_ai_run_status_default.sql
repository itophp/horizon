-- +goose Up

-- A task is running from the moment it is recorded. The insert only carries the
-- immutable snapshot of the request, so the state machine is seeded here rather
-- than by every caller.
ALTER TABLE ai_runs ALTER COLUMN status SET DEFAULT 'running';
