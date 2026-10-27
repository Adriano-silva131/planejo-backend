-- +goose Up
-- The weekdays a recurring task had before its schedule changed on
-- changed_on; until_week is the Monday of that week. Empty weekdays: not
-- recurring then.
CREATE TABLE habit_schedules (
    task_id    uuid NOT NULL,
    until_week date NOT NULL,
    changed_on date NOT NULL,
    user_id    uuid NOT NULL,
    weekdays   smallint NOT NULL CHECK (weekdays BETWEEN 0 AND 127),
    created_at timestamptz NOT NULL DEFAULT now(),

    PRIMARY KEY (task_id, until_week),
    CHECK (changed_on BETWEEN until_week AND until_week + 6),
    CONSTRAINT habit_schedules_task_fkey FOREIGN KEY (task_id, user_id)
        REFERENCES tasks (id, user_id) ON DELETE CASCADE
);

CREATE INDEX habit_schedules_user_id_idx ON habit_schedules (user_id);

-- +goose Down
DROP TABLE habit_schedules;
