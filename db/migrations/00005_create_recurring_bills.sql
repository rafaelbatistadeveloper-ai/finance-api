-- +goose Up
-- R4: cada edição cria uma nova versão e encerra a anterior (design, seção 3).
CREATE TABLE recurring_bills (
    id           uuid     PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id      uuid     NOT NULL REFERENCES users (id),
    category_id  uuid     NOT NULL,
    name         text     NOT NULL,
    amount_cents bigint   NOT NULL,
    is_estimate  boolean  NOT NULL DEFAULT false,
    due_day      smallint NOT NULL,
    active_from  date     NOT NULL, -- primeiro dia do primeiro mês ativo
    active_until date,              -- primeiro dia do último mês ativo; NULL = sem fim
    CONSTRAINT recurring_bills_category_fk
        FOREIGN KEY (category_id, user_id) REFERENCES categories (id, user_id),
    CONSTRAINT recurring_bills_id_user_uq UNIQUE (id, user_id),
    CONSTRAINT recurring_bills_amount_positive CHECK (amount_cents > 0),
    CONSTRAINT recurring_bills_due_day_range CHECK (due_day BETWEEN 1 AND 31),
    CONSTRAINT recurring_bills_active_from_first_day CHECK (date_part('day', active_from) = 1),
    CONSTRAINT recurring_bills_active_until_first_day
        CHECK (active_until IS NULL OR date_part('day', active_until) = 1),
    CONSTRAINT recurring_bills_period_valid
        CHECK (active_until IS NULL OR active_until >= active_from)
);

CREATE INDEX recurring_bills_user_idx ON recurring_bills (user_id, active_from);

-- +goose Down
DROP TABLE recurring_bills;
