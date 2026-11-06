-- +goose Up
ALTER TABLE cap_execution_plans ADD COLUMN workflow_hash text NOT NULL DEFAULT '';
ALTER TABLE cap_execution_records ADD COLUMN run_id bigint;
ALTER TABLE cap_execution_records ADD CONSTRAINT execution_record_run FOREIGN KEY(tenant_id,run_id) REFERENCES ai_runs(tenant_id,id);
-- +goose Down
ALTER TABLE cap_execution_records DROP COLUMN run_id;
ALTER TABLE cap_execution_plans DROP COLUMN workflow_hash;
