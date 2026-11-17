-- +goose Up
-- R3
CREATE TABLE categories (
    id      uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id uuid NOT NULL REFERENCES users (id),
    name    text NOT NULL,
    -- Permite chaves estrangeiras compostas (id, user_id): o banco impede que
    -- um registro aponte para a categoria de outro usuário.
    CONSTRAINT categories_id_user_uq UNIQUE (id, user_id)
);

CREATE UNIQUE INDEX categories_name_uq ON categories (user_id, lower(name));

-- +goose Down
DROP TABLE categories;
