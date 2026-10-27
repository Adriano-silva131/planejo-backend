-- +goose Up
-- Changes one occurrence of a fixed task, the one starting on date, without
-- changing the task: skipped, or moved to start_minute–end_minute.
CREATE TABLE routine_adjustments (
    task_id      uuid NOT NULL,
    date         date NOT NULL,
    user_id      uuid NOT NULL,
    skipped      boolean NOT NULL DEFAULT false,
    start_minute smallint CHECK (start_minute BETWEEN 0 AND 1439),
    end_minute   smallint CHECK (end_minute BETWEEN 0 AND 1439),
    created_at   timestamptz NOT NULL DEFAULT now(),
    updated_at   timestamptz NOT NULL DEFAULT now(),

    PRIMARY KEY (task_id, date),
    CONSTRAINT routine_adjustments_task_fkey FOREIGN KEY (task_id, user_id)
        REFERENCES tasks (id, user_id) ON DELETE CASCADE,
    CONSTRAINT routine_adjustments_shape CHECK (
        (skipped AND start_minute IS NULL AND end_minute IS NULL)
        OR (NOT skipped AND start_minute IS NOT NULL AND end_minute IS NOT NULL AND start_minute <> end_minute)
    )
);

CREATE INDEX routine_adjustments_user_id_date_idx ON routine_adjustments (user_id, date);

-- +goose Down
DROP TABLE routine_adjustments;
