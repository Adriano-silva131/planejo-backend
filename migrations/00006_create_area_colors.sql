-- +goose Up
-- The color the user chose for an area, shared by every task in it. area is
-- written exactly as in tasks.area.
CREATE TABLE area_colors (
    user_id    uuid NOT NULL,
    area       text NOT NULL CHECK (char_length(area) BETWEEN 1 AND 50),
    color      text NOT NULL CHECK (color ~ '^#[0-9a-f]{6}$'),
    updated_at timestamptz NOT NULL DEFAULT now(),

    PRIMARY KEY (user_id, area)
);

-- +goose Down
DROP TABLE area_colors;
