-- +goose Up
-- R2: renda com vigência a partir de um mês (sempre o primeiro dia do mês).
CREATE TABLE income (
    id           uuid   PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id      uuid   NOT NULL REFERENCES users (id),
    amount_cents bigint NOT NULL,
    valid_from   date   NOT NULL,
    CONSTRAINT income_amount_positive CHECK (amount_cents > 0),
    CONSTRAINT income_valid_from_first_day CHECK (date_part('day', valid_from) = 1),
    CONSTRAINT income_user_valid_from_uq UNIQUE (user_id, valid_from)
);

-- +goose Down
DROP TABLE income;
