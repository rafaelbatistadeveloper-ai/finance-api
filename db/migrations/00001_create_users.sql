-- +goose Up
CREATE TABLE users (
    id            uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    email         text        NOT NULL,
    password_hash text        NOT NULL,
    created_at    timestamptz NOT NULL DEFAULT now()
);

-- R1.2: e-mail único sem diferenciar maiúsculas de minúsculas.
CREATE UNIQUE INDEX users_email_uq ON users (lower(email));

-- +goose Down
DROP TABLE users;
