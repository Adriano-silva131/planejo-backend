-- +goose Up
-- The weekdays a recurring task had in the weeks before until_week, the
-- Monday of the week its schedule changed. Empty weekdays: not recurring then.
CREATE TABLE habit_schedules (
    task_id    uuid NOT NULL,
    until_week date NOT NULL,
    user_id    uuid NOT NULL,
    weekdays   smallint NOT NULL CHECK (weekdays BETWEEN 0 AND 127),
    created_at timestamptz NOT NULL DEFAULT now(),

    PRIMARY KEY (task_id, until_week),
    CONSTRAINT habit_schedules_task_fkey FOREIGN KEY (task_id, user_id)
        REFERENCES tasks (id, user_id) ON DELETE CASCADE
);

CREATE INDEX habit_schedules_user_id_idx ON habit_schedules (user_id);

-- +goose Down
DROP TABLE habit_schedules;
