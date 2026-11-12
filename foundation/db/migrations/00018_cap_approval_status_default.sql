-- +goose Up

-- A confirmation card is pending from the moment it is recorded. The insert
-- carries the reviewed payload and the requester, not the state, so the state
-- machine is seeded here. Without this default every automation-origin call to
-- an approval-bound capability failed inside the booking transaction: the card
-- insert violated NOT NULL, the whole transaction rolled back and the caller saw
-- a bare internal error with no invocation, no card and no audit trail.
ALTER TABLE cap_approvals ALTER COLUMN status SET DEFAULT 'pending';
