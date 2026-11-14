-- +goose Up
-- A recurring task is done times_per_week times a week, on any days. Its
-- weekdays become the days the user usually does it, which may be none.
ALTER TABLE tasks ADD COLUMN times_per_week smallint CHECK (times_per_week BETWEEN 1 AND 7);
UPDATE tasks SET times_per_week = (
    SELECT count(*) FROM generate_series(0, 6) AS day WHERE weekdays & (1 << day) <> 0
) WHERE kind = 'recurring';
ALTER TABLE tasks DROP CONSTRAINT tasks_recurring_shape;
ALTER TABLE tasks ADD CONSTRAINT tasks_recurring_shape CHECK (kind <> 'recurring' OR (
    priority IS NOT NULL AND times_per_week IS NOT NULL AND duration_minutes IS NOT NULL
    AND date IS NULL AND start_minute IS NULL AND end_minute IS NULL AND deadline IS NULL
));
ALTER TABLE tasks ADD CONSTRAINT tasks_times_per_week_recurring
    CHECK (kind = 'recurring' OR times_per_week IS NULL);

-- Habit weeks are judged by how many times a week the task was set for.
-- times_per_week 0: not recurring then.
ALTER TABLE habit_schedules ADD COLUMN times_per_week smallint NOT NULL DEFAULT 0
    CHECK (times_per_week BETWEEN 0 AND 7);
UPDATE habit_schedules SET times_per_week = (
    SELECT count(*) FROM generate_series(0, 6) AS day WHERE weekdays & (1 << day) <> 0
);
ALTER TABLE habit_schedules ALTER COLUMN times_per_week DROP DEFAULT;

-- A session the user placed for a recurring or one-off task: on date, from
-- start_minute to end_minute (1440 = midnight), at most one per task and day.
CREATE TABLE routine_sessions (
    task_id      uuid NOT NULL,
    date         date NOT NULL,
    user_id      uuid NOT NULL,
    start_minute smallint NOT NULL CHECK (start_minute BETWEEN 0 AND 1439),
    end_minute   smallint NOT NULL CHECK (end_minute BETWEEN 1 AND 1440),
    created_at   timestamptz NOT NULL DEFAULT now(),
    updated_at   timestamptz NOT NULL DEFAULT now(),

    PRIMARY KEY (task_id, date),
    CONSTRAINT routine_sessions_task_fkey FOREIGN KEY (task_id, user_id)
        REFERENCES tasks (id, user_id) ON DELETE CASCADE,
    CHECK (end_minute > start_minute)
);

CREATE INDEX routine_sessions_user_id_date_idx ON routine_sessions (user_id, date);

-- Recurring sessions the user moved become placed sessions, cut at midnight.
INSERT INTO routine_sessions (task_id, date, user_id, start_minute, end_minute, created_at, updated_at)
SELECT a.task_id, a.date, a.user_id, a.start_minute,
       CASE WHEN a.end_minute > a.start_minute THEN a.end_minute ELSE 1440 END,
       a.created_at, a.updated_at
FROM routine_adjustments a
JOIN tasks t ON t.id = a.task_id
WHERE t.kind = 'recurring' AND NOT a.skipped;
DELETE FROM routine_adjustments a USING tasks t WHERE a.task_id = t.id AND t.kind = 'recurring';

-- +goose Down
-- Sessions are lost. A recurring task without weekdays gets its first
-- times_per_week weekdays, from Sunday.
DROP TABLE routine_sessions;

ALTER TABLE habit_schedules DROP COLUMN times_per_week;

UPDATE tasks SET weekdays = (1 << times_per_week) - 1 WHERE kind = 'recurring' AND weekdays = 0;
ALTER TABLE tasks DROP CONSTRAINT tasks_times_per_week_recurring;
ALTER TABLE tasks DROP CONSTRAINT tasks_recurring_shape;
ALTER TABLE tasks ADD CONSTRAINT tasks_recurring_shape CHECK (kind <> 'recurring' OR (
    priority IS NOT NULL AND weekdays > 0 AND duration_minutes IS NOT NULL
    AND date IS NULL AND start_minute IS NULL AND end_minute IS NULL AND deadline IS NULL
));
ALTER TABLE tasks DROP COLUMN times_per_week;
