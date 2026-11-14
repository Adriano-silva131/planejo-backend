-- +goose Up
-- When the task was really started that day, in the user's time zone.
ALTER TABLE checkins ADD COLUMN start_minute smallint CHECK (start_minute BETWEEN 0 AND 1439);

-- +goose Down
ALTER TABLE checkins DROP COLUMN start_minute;
