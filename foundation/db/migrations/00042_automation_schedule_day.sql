-- +goose Up

-- Monthly schedules keep the day the owner asked for. The anchor instant is
-- clamped to the last day of a shorter month, so without this column a series
-- created on the 31st would drift to the 28th after February.
ALTER TABLE automation_tasks
 ADD COLUMN schedule_day int CHECK (schedule_day BETWEEN 1 AND 31);

-- +goose Down
ALTER TABLE automation_tasks DROP COLUMN IF EXISTS schedule_day;
