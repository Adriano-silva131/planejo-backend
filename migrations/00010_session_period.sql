-- +goose Up
-- A session is placed at a time, from start_minute to end_minute, or only in
-- a part of the day, never both.
ALTER TABLE routine_sessions
    ADD COLUMN period text CHECK (period IN ('morning', 'afternoon', 'evening')),
    ALTER COLUMN start_minute DROP NOT NULL,
    ALTER COLUMN end_minute DROP NOT NULL,
    ADD CONSTRAINT routine_sessions_time_or_period CHECK (
        (start_minute IS NOT NULL AND end_minute IS NOT NULL AND period IS NULL)
        OR (period IS NOT NULL AND start_minute IS NULL AND end_minute IS NULL)
    );

-- +goose Down
-- Sessions placed only in a part of the day are lost.
DELETE FROM routine_sessions WHERE period IS NOT NULL;
ALTER TABLE routine_sessions
    DROP CONSTRAINT routine_sessions_time_or_period,
    DROP COLUMN period,
    ALTER COLUMN start_minute SET NOT NULL,
    ALTER COLUMN end_minute SET NOT NULL;
