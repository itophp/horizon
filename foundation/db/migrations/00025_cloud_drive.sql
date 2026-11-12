-- +goose Up
-- Cloud drive membership is tenant-scoped. Personal spaces are unique per
-- owner; shared-space membership can target an account or a department.
CREATE TABLE drive_spaces (
 id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
 tenant_id text NOT NULL,
 kind text NOT NULL CHECK(kind IN ('personal','shared')),
 name text NOT NULL CHECK(char_length(name) BETWEEN 1 AND 120),
 owner_id bigint NOT NULL,
 created_at timestamptz NOT NULL DEFAULT now(),
 updated_at timestamptz NOT NULL DEFAULT now(),
 UNIQUE(tenant_id,id),
 FOREIGN KEY(tenant_id,owner_id) REFERENCES gov_identity_refs(tenant_id,account_id)
);
CREATE UNIQUE INDEX drive_personal_owner ON drive_spaces(tenant_id,owner_id) WHERE kind='personal';

CREATE TABLE drive_members (
 id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
 tenant_id text NOT NULL,
 space_id bigint NOT NULL,
 account_id bigint,
 department_id bigint,
 role text NOT NULL CHECK(role IN ('admin','editor','viewer')),
 created_at timestamptz NOT NULL DEFAULT now(),
 CHECK ((account_id IS NOT NULL) <> (department_id IS NOT NULL)),
 UNIQUE(tenant_id,id),
 FOREIGN KEY(tenant_id,space_id) REFERENCES drive_spaces(tenant_id,id) ON DELETE CASCADE,
 FOREIGN KEY(tenant_id,account_id) REFERENCES gov_identity_refs(tenant_id,account_id),
 FOREIGN KEY(tenant_id,department_id) REFERENCES iam_departments(tenant_id,id)
);
CREATE UNIQUE INDEX drive_member_account ON drive_members(tenant_id,space_id,account_id) WHERE account_id IS NOT NULL;
CREATE UNIQUE INDEX drive_member_department ON drive_members(tenant_id,space_id,department_id) WHERE department_id IS NOT NULL;
CREATE INDEX drive_member_by_account ON drive_members(tenant_id,account_id,space_id) WHERE account_id IS NOT NULL;
CREATE INDEX drive_member_by_department ON drive_members(tenant_id,department_id,space_id) WHERE department_id IS NOT NULL;

CREATE TABLE drive_trash_batches (
 id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
 tenant_id text NOT NULL,
 space_id bigint NOT NULL,
 deleted_by bigint NOT NULL,
 deleted_at timestamptz NOT NULL DEFAULT now(),
 expires_at timestamptz NOT NULL DEFAULT (now() + interval '60 days'),
 purging_at timestamptz,
 UNIQUE(tenant_id,id),
 UNIQUE(tenant_id,space_id,id),
 FOREIGN KEY(tenant_id,space_id) REFERENCES drive_spaces(tenant_id,id),
 FOREIGN KEY(tenant_id,deleted_by) REFERENCES gov_identity_refs(tenant_id,account_id)
);
CREATE INDEX drive_trash_expiry ON drive_trash_batches(expires_at,tenant_id,id);

CREATE TABLE drive_folders (
 id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
 tenant_id text NOT NULL,
 space_id bigint NOT NULL,
 parent_id bigint,
 name text NOT NULL CHECK(char_length(name) BETWEEN 1 AND 200),
 created_by bigint NOT NULL,
 created_at timestamptz NOT NULL DEFAULT now(),
 updated_at timestamptz NOT NULL DEFAULT now(),
 deleted_at timestamptz,
 trash_batch_id bigint,
 trash_root boolean NOT NULL DEFAULT false,
 UNIQUE(tenant_id,id),
 UNIQUE(tenant_id,space_id,id),
 FOREIGN KEY(tenant_id,space_id) REFERENCES drive_spaces(tenant_id,id),
 FOREIGN KEY(tenant_id,space_id,parent_id) REFERENCES drive_folders(tenant_id,space_id,id),
 FOREIGN KEY(tenant_id,created_by) REFERENCES gov_identity_refs(tenant_id,account_id),
 FOREIGN KEY(tenant_id,space_id,trash_batch_id) REFERENCES drive_trash_batches(tenant_id,space_id,id),
 CHECK ((deleted_at IS NULL AND trash_batch_id IS NULL AND NOT trash_root) OR
        (deleted_at IS NOT NULL AND trash_batch_id IS NOT NULL))
);
CREATE UNIQUE INDEX drive_folder_name ON drive_folders(tenant_id,space_id,COALESCE(parent_id,0),name) WHERE deleted_at IS NULL;
CREATE INDEX drive_folder_parent ON drive_folders(tenant_id,space_id,parent_id,id) WHERE deleted_at IS NULL;
CREATE UNIQUE INDEX drive_folder_trash_root ON drive_folders(tenant_id,trash_batch_id) WHERE trash_root;
CREATE INDEX drive_folder_trash_batch ON drive_folders(tenant_id,trash_batch_id) WHERE trash_batch_id IS NOT NULL;

-- Cloud objects remain in the shared file store with a distinct purpose. The
-- original attachment/logo validation remains unchanged, including nonempty
-- content. Cloud files can be empty and are read only through drive bindings.
ALTER TABLE gov_files DROP CONSTRAINT gov_files_size_check;
ALTER TABLE gov_files DROP CONSTRAINT gov_files_purpose_check;
ALTER TABLE gov_files ADD CONSTRAINT gov_files_size_check CHECK(size>=0 AND (purpose='cloud_drive' OR size>0));
ALTER TABLE gov_files ADD CONSTRAINT gov_files_purpose_check CHECK(purpose IN ('logo','attachment','cloud_drive'));

CREATE TABLE drive_files (
 id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
 tenant_id text NOT NULL,
 space_id bigint NOT NULL,
 folder_id bigint,
 file_id bigint NOT NULL,
 name text NOT NULL CHECK(char_length(name) BETWEEN 1 AND 200),
 created_by bigint NOT NULL,
 created_at timestamptz NOT NULL DEFAULT now(),
 updated_at timestamptz NOT NULL DEFAULT now(),
 deleted_at timestamptz,
 trash_batch_id bigint,
 trash_root boolean NOT NULL DEFAULT false,
 UNIQUE(tenant_id,id),
 UNIQUE(tenant_id,file_id),
 FOREIGN KEY(tenant_id,space_id) REFERENCES drive_spaces(tenant_id,id),
 FOREIGN KEY(tenant_id,space_id,folder_id) REFERENCES drive_folders(tenant_id,space_id,id),
 FOREIGN KEY(tenant_id,file_id) REFERENCES gov_files(tenant_id,id),
 FOREIGN KEY(tenant_id,created_by) REFERENCES gov_identity_refs(tenant_id,account_id),
 FOREIGN KEY(tenant_id,space_id,trash_batch_id) REFERENCES drive_trash_batches(tenant_id,space_id,id),
 CHECK ((deleted_at IS NULL AND trash_batch_id IS NULL AND NOT trash_root) OR
        (deleted_at IS NOT NULL AND trash_batch_id IS NOT NULL))
);
CREATE UNIQUE INDEX drive_file_name ON drive_files(tenant_id,space_id,COALESCE(folder_id,0),name) WHERE deleted_at IS NULL;
CREATE UNIQUE INDEX drive_file_trash_root ON drive_files(tenant_id,trash_batch_id) WHERE trash_root;
CREATE INDEX drive_file_folder ON drive_files(tenant_id,space_id,folder_id,id) WHERE deleted_at IS NULL;
CREATE INDEX drive_file_trash_batch ON drive_files(tenant_id,trash_batch_id) WHERE trash_batch_id IS NOT NULL;

-- Every temporary object key is recorded before it can be written, so a crash
-- cannot make an untracked chunk or assembled object impossible to clean up.
CREATE TABLE drive_uploads (
 id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
 tenant_id text NOT NULL,
 space_id bigint NOT NULL,
 folder_id bigint,
 owner_id bigint NOT NULL,
 storage_profile_id bigint NOT NULL,
 object_key text NOT NULL,
 name text NOT NULL CHECK(char_length(name) BETWEEN 1 AND 200),
 size bigint NOT NULL CHECK(size BETWEEN 0 AND 1073741824),
 state text NOT NULL DEFAULT 'open' CHECK(state IN ('open','completed','cancelled','cleaned')),
 file_id bigint,
 created_at timestamptz NOT NULL DEFAULT now(),
 expires_at timestamptz NOT NULL DEFAULT (now()+interval '24 hours'),
 UNIQUE(tenant_id,id),
 UNIQUE(tenant_id,space_id,id),
 UNIQUE(tenant_id,storage_profile_id,object_key),
 FOREIGN KEY(tenant_id,space_id) REFERENCES drive_spaces(tenant_id,id),
 FOREIGN KEY(tenant_id,space_id,folder_id) REFERENCES drive_folders(tenant_id,space_id,id),
 FOREIGN KEY(tenant_id,owner_id) REFERENCES gov_identity_refs(tenant_id,account_id),
 FOREIGN KEY(tenant_id,storage_profile_id) REFERENCES gov_storage_profiles(tenant_id,id),
 FOREIGN KEY(tenant_id,file_id) REFERENCES gov_files(tenant_id,id)
);
CREATE INDEX drive_upload_expiry ON drive_uploads(state,expires_at,tenant_id,id);

CREATE TABLE drive_upload_chunks (
 id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
 tenant_id text NOT NULL,
 upload_id bigint NOT NULL,
 chunk_index integer NOT NULL CHECK(chunk_index BETWEEN 0 AND 255),
 storage_key text NOT NULL,
 expected_size bigint NOT NULL CHECK(expected_size BETWEEN 1 AND 4194304),
 sha256 text,
 uploaded_at timestamptz,
 UNIQUE(tenant_id,id),
 UNIQUE(tenant_id,upload_id,chunk_index),
 UNIQUE(tenant_id,upload_id,storage_key),
 FOREIGN KEY(tenant_id,upload_id) REFERENCES drive_uploads(tenant_id,id),
 CHECK ((sha256 IS NULL AND uploaded_at IS NULL) OR (sha256 IS NOT NULL AND uploaded_at IS NOT NULL))
);

-- +goose Down
DROP TABLE drive_upload_chunks;
DROP TABLE drive_uploads;
DROP TABLE drive_files;
-- Downgrade is intentionally rejected by PostgreSQL if cloud-drive objects
-- still exist; operators must migrate those objects before rolling back.
ALTER TABLE gov_files DROP CONSTRAINT gov_files_size_check;
ALTER TABLE gov_files DROP CONSTRAINT gov_files_purpose_check;
ALTER TABLE gov_files ADD CONSTRAINT gov_files_size_check CHECK(size>0);
ALTER TABLE gov_files ADD CONSTRAINT gov_files_purpose_check CHECK(purpose IN ('logo','attachment'));
DROP TABLE drive_folders;
DROP TABLE drive_trash_batches;
DROP TABLE drive_members;
DROP TABLE drive_spaces;
