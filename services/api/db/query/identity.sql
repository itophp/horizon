-- name: IAMLogin :one
SELECT a.id,a.tenant_id,a.password_hash,a.status FROM iam_accounts a WHERE a.username=$1;

-- name: IAMLoginThrottle :one
SELECT failures,locked_until FROM iam_login_attempts WHERE attempt_key=$1;

-- name: IAMLoginFailure :exec
INSERT INTO iam_login_attempts(attempt_key,failures,locked_until) VALUES($1,1,now()) ON CONFLICT(attempt_key) DO UPDATE SET failures=CASE WHEN iam_login_attempts.locked_until < now()-interval '15 minutes' THEN 1 ELSE iam_login_attempts.failures+1 END,locked_until=CASE WHEN iam_login_attempts.failures>=4 THEN now()+interval '15 minutes' ELSE now() END;

-- name: IAMLoginClear :exec
DELETE FROM iam_login_attempts WHERE attempt_key=$1;

-- name: IAMCreateSession :exec
INSERT INTO iam_sessions(token_hash,tenant_id,account_id,csrf_token,expires_at) VALUES($1,$2,$3,$4,$5);

-- name: IAMSession :one
SELECT s.tenant_id,s.account_id,s.csrf_token,COALESCE(s.active_department_id,a.department_id)::bigint AS active_department_id FROM iam_sessions s JOIN iam_accounts a ON a.tenant_id=s.tenant_id AND a.id=s.account_id WHERE s.token_hash=$1 AND s.expires_at>now() AND a.status='active';

-- name: IAMDeleteSession :exec
DELETE FROM iam_sessions WHERE token_hash=$1;

-- name: IAMRevokeSessions :exec
DELETE FROM iam_sessions WHERE tenant_id=$1 AND account_id=$2;

-- name: IAMTouchLogin :exec
UPDATE iam_accounts SET last_login=now() WHERE tenant_id=$1 AND id=$2;

-- name: IAMRoleGrants :many
SELECT r.grants FROM iam_roles r JOIN iam_account_roles ar ON ar.tenant_id=r.tenant_id AND ar.role_id=r.id WHERE ar.tenant_id=$1 AND ar.account_id=$2 AND r.status='active';

-- name: IAMAccountRoles :many
SELECT role_id FROM iam_account_roles WHERE tenant_id=$1 AND account_id=$2 ORDER BY role_id;

-- name: IAMClearAccountRoles :exec
DELETE FROM iam_account_roles WHERE tenant_id=$1 AND account_id=$2;

-- name: IAMAssignRole :exec
INSERT INTO iam_account_roles(tenant_id,account_id,role_id) VALUES($1,$2,$3);

-- name: IAMCountAccounts :one
SELECT count(*) FROM iam_accounts WHERE iam_accounts.tenant_id=$1;

-- name: IAMListOrganizations :many
SELECT id,name,code,kind,COALESCE(parent_id,0)::bigint AS parent_id,status,description,created_at FROM iam_organizations WHERE tenant_id=sqlc.arg(tenant_id) AND (sqlc.arg(all_data)::boolean OR id=ANY(sqlc.arg(org_ids)::bigint[])) ORDER BY id;

-- name: IAMGetOrganization :one
SELECT id,name,code,kind,COALESCE(parent_id,0)::bigint AS parent_id,status,description,created_at FROM iam_organizations WHERE tenant_id=sqlc.arg(tenant_id) AND id=sqlc.arg(id) AND (sqlc.arg(all_data)::boolean OR id=ANY(sqlc.arg(org_ids)::bigint[]));

-- name: IAMCreateOrganization :one
INSERT INTO iam_organizations (tenant_id,name,code,kind,parent_id,status,description) VALUES (sqlc.arg(tenant_id),sqlc.arg(name),sqlc.arg(code),sqlc.arg(kind),NULLIF(sqlc.arg(parent_id)::bigint,0),sqlc.arg(status),sqlc.arg(description)) RETURNING id;

-- name: IAMUpdateOrganization :one
UPDATE iam_organizations SET name=sqlc.arg(name),code=sqlc.arg(code),kind=sqlc.arg(kind),parent_id=NULLIF(sqlc.arg(parent_id)::bigint,0),status=sqlc.arg(status),description=sqlc.arg(description) WHERE tenant_id=sqlc.arg(tenant_id) AND id=sqlc.arg(id) AND (sqlc.arg(all_data)::boolean OR id=ANY(sqlc.arg(org_ids)::bigint[])) RETURNING id;

-- name: IAMDeleteOrganization :one
DELETE FROM iam_organizations WHERE tenant_id=sqlc.arg(tenant_id) AND id=sqlc.arg(id) AND (sqlc.arg(all_data)::boolean OR id=ANY(sqlc.arg(org_ids)::bigint[])) RETURNING id;

-- name: IAMListDepartments :many
SELECT id,name,code,organization_id,COALESCE(parent_id,0)::bigint AS parent_id,manager,status,description,created_at FROM iam_departments WHERE tenant_id=sqlc.arg(tenant_id) AND (sqlc.arg(all_data)::boolean OR organization_id=ANY(sqlc.arg(org_ids)::bigint[]) OR id=ANY(sqlc.arg(dept_ids)::bigint[])) ORDER BY id;

-- name: IAMGetDepartment :one
SELECT id,name,code,organization_id,COALESCE(parent_id,0)::bigint AS parent_id,manager,status,description,created_at FROM iam_departments WHERE tenant_id=sqlc.arg(tenant_id) AND id=sqlc.arg(id) AND (sqlc.arg(all_data)::boolean OR organization_id=ANY(sqlc.arg(org_ids)::bigint[]) OR id=ANY(sqlc.arg(dept_ids)::bigint[]));

-- name: IAMCreateDepartment :one
INSERT INTO iam_departments (tenant_id,name,code,organization_id,parent_id,manager,status,description) VALUES (sqlc.arg(tenant_id),sqlc.arg(name),sqlc.arg(code),sqlc.arg(organization_id),NULLIF(sqlc.arg(parent_id)::bigint,0),sqlc.arg(manager),sqlc.arg(status),sqlc.arg(description)) RETURNING id;

-- name: IAMUpdateDepartment :one
UPDATE iam_departments SET name=sqlc.arg(name),code=sqlc.arg(code),organization_id=sqlc.arg(organization_id),parent_id=NULLIF(sqlc.arg(parent_id)::bigint,0),manager=sqlc.arg(manager),status=sqlc.arg(status),description=sqlc.arg(description) WHERE tenant_id=sqlc.arg(tenant_id) AND id=sqlc.arg(id) AND (sqlc.arg(all_data)::boolean OR organization_id=ANY(sqlc.arg(org_ids)::bigint[]) OR id=ANY(sqlc.arg(dept_ids)::bigint[])) RETURNING id;

-- name: IAMDeleteDepartment :one
DELETE FROM iam_departments WHERE tenant_id=sqlc.arg(tenant_id) AND id=sqlc.arg(id) AND (sqlc.arg(all_data)::boolean OR organization_id=ANY(sqlc.arg(org_ids)::bigint[]) OR id=ANY(sqlc.arg(dept_ids)::bigint[])) RETURNING id;

-- name: IAMListRoles :many
SELECT id,name,code,description,status,grants,created_at FROM iam_roles WHERE tenant_id=sqlc.arg(tenant_id) AND sqlc.arg(all_data)::boolean ORDER BY id;

-- name: IAMGetRole :one
SELECT id,name,code,description,status,grants,created_at FROM iam_roles WHERE tenant_id=sqlc.arg(tenant_id) AND id=sqlc.arg(id) AND sqlc.arg(all_data)::boolean;

-- name: IAMCreateRole :one
INSERT INTO iam_roles (tenant_id,name,code,description,status,grants) VALUES (sqlc.arg(tenant_id),sqlc.arg(name),sqlc.arg(code),sqlc.arg(description),sqlc.arg(status),sqlc.arg(grants)) RETURNING id;

-- name: IAMUpdateRole :one
UPDATE iam_roles SET name=sqlc.arg(name),code=sqlc.arg(code),description=sqlc.arg(description),status=sqlc.arg(status),grants=sqlc.arg(grants) WHERE tenant_id=sqlc.arg(tenant_id) AND id=sqlc.arg(id) AND sqlc.arg(all_data)::boolean RETURNING id;

-- name: IAMDeleteRole :one
DELETE FROM iam_roles WHERE tenant_id=sqlc.arg(tenant_id) AND id=sqlc.arg(id) AND sqlc.arg(all_data)::boolean RETURNING id;

-- name: IAMListAccounts :many
SELECT iam_accounts.id,username,full_name,email,status,organization_id,department_id,overrides,is_admin,created_at,last_login FROM iam_accounts WHERE iam_accounts.tenant_id=sqlc.arg(tenant_id) AND (sqlc.arg(all_data)::boolean OR iam_accounts.id=sqlc.arg(own_id)::bigint OR iam_accounts.organization_id=ANY(sqlc.arg(org_ids)::bigint[]) OR iam_accounts.department_id=ANY(sqlc.arg(dept_ids)::bigint[]) OR EXISTS (SELECT 1 FROM iam_department_memberships m WHERE m.tenant_id=iam_accounts.tenant_id AND m.account_id=iam_accounts.id AND (m.organization_id=ANY(sqlc.arg(org_ids)::bigint[]) OR m.department_id=ANY(sqlc.arg(dept_ids)::bigint[])))) ORDER BY iam_accounts.id;

-- name: IAMGetAccount :one
SELECT iam_accounts.id,username,full_name,email,status,organization_id,department_id,overrides,is_admin,created_at,last_login FROM iam_accounts WHERE iam_accounts.tenant_id=sqlc.arg(tenant_id) AND iam_accounts.id=sqlc.arg(id) AND (sqlc.arg(all_data)::boolean OR iam_accounts.id=sqlc.arg(own_id)::bigint OR iam_accounts.organization_id=ANY(sqlc.arg(org_ids)::bigint[]) OR iam_accounts.department_id=ANY(sqlc.arg(dept_ids)::bigint[]) OR EXISTS (SELECT 1 FROM iam_department_memberships m WHERE m.tenant_id=iam_accounts.tenant_id AND m.account_id=iam_accounts.id AND (m.organization_id=ANY(sqlc.arg(org_ids)::bigint[]) OR m.department_id=ANY(sqlc.arg(dept_ids)::bigint[]))));

-- name: IAMCreateAccount :one
INSERT INTO iam_accounts (tenant_id,username,full_name,email,status,organization_id,department_id,overrides,password_hash) VALUES (sqlc.arg(tenant_id),sqlc.arg(username),sqlc.arg(full_name),sqlc.arg(email),sqlc.arg(status),sqlc.arg(organization_id),sqlc.arg(department_id),sqlc.arg(overrides),sqlc.arg(password_hash)) RETURNING id;

-- name: IAMUpdateAccount :one
UPDATE iam_accounts SET username=sqlc.arg(username),full_name=sqlc.arg(full_name),email=sqlc.arg(email),status=sqlc.arg(status),organization_id=sqlc.arg(organization_id),department_id=sqlc.arg(department_id),overrides=sqlc.arg(overrides) WHERE iam_accounts.tenant_id=sqlc.arg(tenant_id) AND iam_accounts.id=sqlc.arg(id) AND (sqlc.arg(all_data)::boolean OR iam_accounts.id=sqlc.arg(own_id)::bigint OR iam_accounts.organization_id=ANY(sqlc.arg(org_ids)::bigint[]) OR iam_accounts.department_id=ANY(sqlc.arg(dept_ids)::bigint[]) OR EXISTS (SELECT 1 FROM iam_department_memberships m WHERE m.tenant_id=iam_accounts.tenant_id AND m.account_id=iam_accounts.id AND (m.organization_id=ANY(sqlc.arg(org_ids)::bigint[]) OR m.department_id=ANY(sqlc.arg(dept_ids)::bigint[])))) RETURNING iam_accounts.id;

-- name: IAMDeleteAccount :one
DELETE FROM iam_accounts WHERE iam_accounts.tenant_id=sqlc.arg(tenant_id) AND iam_accounts.id=sqlc.arg(id) AND (sqlc.arg(all_data)::boolean OR iam_accounts.id=sqlc.arg(own_id)::bigint OR iam_accounts.organization_id=ANY(sqlc.arg(org_ids)::bigint[]) OR iam_accounts.department_id=ANY(sqlc.arg(dept_ids)::bigint[]) OR EXISTS (SELECT 1 FROM iam_department_memberships m WHERE m.tenant_id=iam_accounts.tenant_id AND m.account_id=iam_accounts.id AND (m.organization_id=ANY(sqlc.arg(org_ids)::bigint[]) OR m.department_id=ANY(sqlc.arg(dept_ids)::bigint[])))) RETURNING iam_accounts.id;

-- name: IAMSetPassword :exec
UPDATE iam_accounts SET password_hash=$3 WHERE tenant_id=$1 AND id=$2;

-- name: IAMMarkAdmin :exec
UPDATE iam_accounts SET is_admin=true WHERE tenant_id=$1 AND id=$2;

-- name: IAMDepartmentTree :many
WITH RECURSIVE tree AS (SELECT root.id FROM iam_departments root WHERE root.tenant_id=$1 AND root.id=$2 UNION SELECT d.id FROM iam_departments d JOIN tree t ON d.parent_id=t.id WHERE d.tenant_id=$1) SELECT id FROM tree;

-- name: IAMOrganizationTree :many
WITH RECURSIVE tree AS (SELECT root.id FROM iam_organizations root WHERE root.tenant_id=$1 AND root.id=$2 UNION SELECT o.id FROM iam_organizations o JOIN tree t ON o.parent_id=t.id WHERE o.tenant_id=$1) SELECT id FROM tree;

-- name: IAMRoleMemberCount :one
SELECT count(DISTINCT account_id) FROM (SELECT account_id FROM iam_account_roles WHERE iam_account_roles.tenant_id=$1 AND iam_account_roles.role_id=$2 UNION SELECT account_id FROM iam_department_membership_roles WHERE iam_department_membership_roles.tenant_id=$1 AND iam_department_membership_roles.role_id=$2) members;

-- name: IAMRootAnchors :many
SELECT organization_id,department_id FROM iam_accounts WHERE iam_accounts.tenant_id=$1 AND is_admin=true;

-- name: IAMBootstrapLock :exec
SELECT pg_advisory_xact_lock(hashtextextended(sqlc.arg(tenant_id)::text,0));

-- name: IAMMemberships :many
SELECT m.organization_id,m.department_id,d.name AS department_name,m.overrides FROM iam_department_memberships m JOIN iam_departments d ON d.tenant_id=m.tenant_id AND d.id=m.department_id WHERE m.tenant_id=$1 AND m.account_id=$2 ORDER BY m.department_id;
-- name: IAMMembershipRoles :many
SELECT role_id FROM iam_department_membership_roles WHERE tenant_id=$1 AND account_id=$2 AND department_id=$3 ORDER BY role_id;
-- name: IAMMembershipRoleGrants :many
SELECT r.grants FROM iam_roles r JOIN iam_department_membership_roles m ON m.tenant_id=r.tenant_id AND m.role_id=r.id WHERE m.tenant_id=$1 AND m.account_id=$2 AND m.department_id=$3 AND r.status='active';
-- name: IAMClearMemberships :exec
DELETE FROM iam_department_memberships WHERE tenant_id=$1 AND account_id=$2;
-- name: IAMCreateMembership :exec
INSERT INTO iam_department_memberships(tenant_id,account_id,organization_id,department_id,overrides) VALUES($1,$2,$3,$4,$5);
-- name: IAMAssignMembershipRole :exec
INSERT INTO iam_department_membership_roles(tenant_id,account_id,department_id,role_id) VALUES($1,$2,$3,$4);
-- name: IAMActivePlacement :one
SELECT d.id FROM iam_departments d JOIN iam_organizations o ON o.tenant_id=d.tenant_id AND o.id=d.organization_id JOIN iam_organizations parent ON parent.tenant_id=o.tenant_id AND parent.id=o.parent_id WHERE d.tenant_id=$1 AND d.id=$2 AND d.organization_id=$3 AND d.status='active' AND o.status='active' AND parent.status='active';
-- name: IAMSwitchDepartment :one
UPDATE iam_sessions SET active_department_id=$2,csrf_token=$3 WHERE token_hash=$1 AND expires_at>now() RETURNING token_hash;

-- name: IAMInvalidateDetachedTemporary :exec
UPDATE iam_temporary_requests r SET status=CASE WHEN r.status='pending' THEN 'cancelled' ELSE 'revoked' END,cancelled_at=CASE WHEN r.status='pending' THEN now() ELSE r.cancelled_at END,revoked_at=CASE WHEN r.status='approved' THEN now() ELSE r.revoked_at END
WHERE r.tenant_id=$1 AND r.applicant_id=$2 AND r.status IN ('pending','approved') AND NOT EXISTS (SELECT 1 FROM iam_accounts a WHERE a.tenant_id=r.tenant_id AND a.id=r.applicant_id AND a.department_id=r.department_id) AND NOT EXISTS (SELECT 1 FROM iam_department_memberships m WHERE m.tenant_id=r.tenant_id AND m.account_id=r.applicant_id AND m.department_id=r.department_id);
