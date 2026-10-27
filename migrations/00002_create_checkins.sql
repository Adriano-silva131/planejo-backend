-- +goose Up
-- Lets checkins reference the task together with its owner.
ALTER TABLE tasks ADD CONSTRAINT tasks_id_user_id_key UNIQUE (id, user_id);

CREATE TABLE checkins (
    task_id    uuid NOT NULL,
    date       date NOT NULL,
    user_id    uuid NOT NULL,
    minutes    integer NOT NULL DEFAULT 0 CHECK (minutes BETWEEN 0 AND 1440),
    completed  boolean NOT NULL DEFAULT false,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),

    PRIMARY KEY (task_id, date),
    -- A check-in always belongs to the owner of its task and goes away with it.
    CONSTRAINT checkins_task_fkey FOREIGN KEY (task_id, user_id)
        REFERENCES tasks (id, user_id) ON DELETE CASCADE
);

CREATE INDEX checkins_user_id_date_idx ON checkins (user_id, date);

-- +goose Down
DROP TABLE checkins;
ALTER TABLE tasks DROP CONSTRAINT tasks_id_user_id_key;
