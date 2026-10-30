-- +goose Up
CREATE TABLE iam_temporary_requests (
 id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
 tenant_id text NOT NULL, applicant_id bigint NOT NULL,
 requested_grants jsonb NOT NULL CHECK(jsonb_typeof(requested_grants)='array'),
 requested_minutes integer NOT NULL CHECK(requested_minutes IN (30,60,120,240,480,1440,4320,10080)),
 reason text NOT NULL CHECK(char_length(reason) BETWEEN 5 AND 500),
 status text NOT NULL DEFAULT 'pending' CHECK(status IN ('pending','approved','rejected','cancelled','revoked')),
 approved_grants jsonb NOT NULL DEFAULT '[]', resolved_scopes jsonb NOT NULL DEFAULT '{}',
 approved_minutes integer NOT NULL DEFAULT 0, reviewer_id bigint, review_note text NOT NULL DEFAULT '',
 created_at timestamptz NOT NULL DEFAULT now(), reviewed_at timestamptz, expires_at timestamptz,
 cancelled_at timestamptz, revoked_at timestamptz, revoked_by bigint,
 FOREIGN KEY(tenant_id,applicant_id) REFERENCES iam_accounts(tenant_id,id) ON DELETE CASCADE,
 FOREIGN KEY(tenant_id,reviewer_id) REFERENCES iam_accounts(tenant_id,id),
 FOREIGN KEY(tenant_id,revoked_by) REFERENCES iam_accounts(tenant_id,id),
 CHECK(reviewer_id IS NULL OR reviewer_id<>applicant_id),
 CHECK(status NOT IN ('approved','revoked') OR (reviewer_id IS NOT NULL AND reviewed_at IS NOT NULL AND expires_at>reviewed_at AND approved_minutes IN (30,60,120,240,480,1440,4320,10080)))
);
CREATE INDEX iam_temporary_applicant ON iam_temporary_requests(tenant_id,applicant_id,created_at DESC);
CREATE INDEX iam_temporary_active ON iam_temporary_requests(tenant_id,applicant_id,expires_at) WHERE status='approved';
-- +goose Down
DROP TABLE iam_temporary_requests;
