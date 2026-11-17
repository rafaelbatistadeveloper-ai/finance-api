-- +goose Up
-- R7: ambientes da casa.
CREATE TABLE rooms (
    id      uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id uuid NOT NULL REFERENCES users (id),
    name    text NOT NULL,
    CONSTRAINT rooms_id_user_uq UNIQUE (id, user_id)
);

CREATE UNIQUE INDEX rooms_name_uq ON rooms (user_id, lower(name));

-- +goose Down
DROP TABLE rooms;
