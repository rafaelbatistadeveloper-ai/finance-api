-- +goose Up
-- R6, R7: room_id preenchido indica compra da casa. A loja, se houver, fica em description.
CREATE TABLE expenses (
    id             uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id        uuid        NOT NULL REFERENCES users (id),
    category_id    uuid        NOT NULL,
    room_id        uuid,
    description    text        NOT NULL,
    amount_cents   bigint      NOT NULL,
    spent_at       date        NOT NULL,
    payment_method text        NOT NULL,
    installments_n smallint,
    created_at     timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT expenses_category_fk
        FOREIGN KEY (category_id, user_id) REFERENCES categories (id, user_id),
    -- Com room_id NULL a chave composta não é verificada (MATCH SIMPLE).
    CONSTRAINT expenses_room_fk
        FOREIGN KEY (room_id, user_id) REFERENCES rooms (id, user_id),
    CONSTRAINT expenses_amount_positive CHECK (amount_cents > 0),
    CONSTRAINT expenses_payment_method_valid
        CHECK (payment_method IN ('pix', 'debit', 'credit', 'cash', 'boleto')),
    CONSTRAINT expenses_installments_range CHECK (installments_n BETWEEN 1 AND 48),
    -- Parcelas só existem no crédito, e todo crédito tem número de parcelas.
    CONSTRAINT expenses_installments_only_credit
        CHECK ((payment_method = 'credit') = (installments_n IS NOT NULL))
);

CREATE INDEX expenses_user_date_idx ON expenses (user_id, spent_at);
CREATE INDEX expenses_user_room_idx ON expenses (user_id, room_id) WHERE room_id IS NOT NULL;

-- +goose Down
DROP TABLE expenses;
