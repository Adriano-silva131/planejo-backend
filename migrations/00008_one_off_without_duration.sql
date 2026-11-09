-- +goose Up
-- A one-off task has a deadline but no estimate, and its check-in is the day
-- it was completed. Time logged without completing it goes away.
ALTER TABLE tasks DROP CONSTRAINT tasks_one_off_shape;
UPDATE tasks SET duration_minutes = NULL WHERE kind = 'one_off';
ALTER TABLE tasks ADD CONSTRAINT tasks_one_off_shape CHECK (kind <> 'one_off' OR (
    priority IS NOT NULL AND deadline IS NOT NULL AND duration_minutes IS NULL
    AND weekdays = 0 AND date IS NULL AND start_minute IS NULL AND end_minute IS NULL
));

DELETE FROM checkins c USING tasks t
WHERE c.task_id = t.id AND t.kind = 'one_off' AND NOT c.completed;
ALTER TABLE checkins DROP COLUMN minutes, DROP COLUMN completed;

-- +goose Down
ALTER TABLE checkins
    ADD COLUMN minutes integer NOT NULL DEFAULT 0 CHECK (minutes BETWEEN 0 AND 1440),
    ADD COLUMN completed boolean NOT NULL DEFAULT false;
UPDATE checkins c SET completed = true FROM tasks t WHERE c.task_id = t.id AND t.kind = 'one_off';

-- The estimates are gone; one hour stands in for them.
ALTER TABLE tasks DROP CONSTRAINT tasks_one_off_shape;
UPDATE tasks SET duration_minutes = 60 WHERE kind = 'one_off';
ALTER TABLE tasks ADD CONSTRAINT tasks_one_off_shape CHECK (kind <> 'one_off' OR (
    priority IS NOT NULL AND deadline IS NOT NULL AND duration_minutes IS NOT NULL
    AND weekdays = 0 AND date IS NULL AND start_minute IS NULL AND end_minute IS NULL
));
