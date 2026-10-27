-- +goose Up
UPDATE tasks SET area = 'Outros' WHERE area = '';
ALTER TABLE tasks ALTER COLUMN area DROP DEFAULT;
ALTER TABLE tasks DROP CONSTRAINT tasks_area_check;
ALTER TABLE tasks ADD CONSTRAINT tasks_area_check CHECK (char_length(area) BETWEEN 1 AND 50);

-- A fixed task may end the next day (end_minute < start_minute).
ALTER TABLE tasks DROP CONSTRAINT tasks_fixed_shape;
ALTER TABLE tasks ADD CONSTRAINT tasks_fixed_shape CHECK (kind <> 'fixed' OR (
    priority IS NULL AND duration_minutes IS NULL AND period IS NULL AND deadline IS NULL
    AND (weekdays > 0) <> (date IS NOT NULL)
    AND start_minute IS NOT NULL AND end_minute IS NOT NULL AND end_minute <> start_minute
));

-- +goose Down
-- Fails while a fixed task ends the next day: delete those tasks first.
ALTER TABLE tasks DROP CONSTRAINT tasks_fixed_shape;
ALTER TABLE tasks ADD CONSTRAINT tasks_fixed_shape CHECK (kind <> 'fixed' OR (
    priority IS NULL AND duration_minutes IS NULL AND period IS NULL AND deadline IS NULL
    AND (weekdays > 0) <> (date IS NOT NULL)
    AND start_minute IS NOT NULL AND end_minute > start_minute
));

ALTER TABLE tasks DROP CONSTRAINT tasks_area_check;
ALTER TABLE tasks ADD CONSTRAINT tasks_area_check CHECK (char_length(area) <= 50);
ALTER TABLE tasks ALTER COLUMN area SET DEFAULT '';
