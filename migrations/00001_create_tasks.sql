-- +goose Up
CREATE TABLE tasks (
    id               uuid PRIMARY KEY,
    user_id          uuid NOT NULL,
    title            text NOT NULL CHECK (char_length(title) BETWEEN 1 AND 120),
    area             text NOT NULL DEFAULT '' CHECK (char_length(area) <= 50),
    kind             text NOT NULL CHECK (kind IN ('fixed', 'recurring', 'one_off')),
    priority         text CHECK (priority IN ('high', 'medium', 'low')),
    -- bit n set = time.Weekday n (0 = Sunday)
    weekdays         smallint NOT NULL DEFAULT 0 CHECK (weekdays BETWEEN 0 AND 127),
    date             date,
    -- minutes since midnight in the user's time zone
    start_minute     smallint CHECK (start_minute BETWEEN 0 AND 1439),
    end_minute       smallint CHECK (end_minute BETWEEN 0 AND 1439),
    duration_minutes integer CHECK (duration_minutes > 0),
    period           text CHECK (period IN ('morning', 'afternoon', 'evening')),
    deadline         date,
    created_at       timestamptz NOT NULL DEFAULT now(),
    updated_at       timestamptz NOT NULL DEFAULT now(),

    CONSTRAINT tasks_fixed_shape CHECK (kind <> 'fixed' OR (
        priority IS NULL AND duration_minutes IS NULL AND period IS NULL AND deadline IS NULL
        AND (weekdays > 0) <> (date IS NOT NULL)
        AND start_minute IS NOT NULL AND end_minute > start_minute
    )),
    CONSTRAINT tasks_recurring_shape CHECK (kind <> 'recurring' OR (
        priority IS NOT NULL AND weekdays > 0 AND duration_minutes IS NOT NULL
        AND date IS NULL AND start_minute IS NULL AND end_minute IS NULL AND deadline IS NULL
    )),
    CONSTRAINT tasks_one_off_shape CHECK (kind <> 'one_off' OR (
        priority IS NOT NULL AND deadline IS NOT NULL AND duration_minutes IS NOT NULL
        AND weekdays = 0 AND date IS NULL AND start_minute IS NULL AND end_minute IS NULL
    ))
);

CREATE INDEX tasks_user_id_created_at_idx ON tasks (user_id, created_at DESC, id DESC);

-- +goose Down
DROP TABLE tasks;
