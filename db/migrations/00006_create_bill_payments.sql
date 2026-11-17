-- +goose Up
-- R5: a ausência de linha para (conta, mês) significa "pendente".
CREATE TABLE bill_payments (
    id                uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id           uuid        NOT NULL REFERENCES users (id),
    recurring_bill_id uuid        NOT NULL,
    month             date        NOT NULL,
    paid_amount_cents bigint      NOT NULL,
    paid_at           timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT bill_payments_bill_fk
        FOREIGN KEY (recurring_bill_id, user_id) REFERENCES recurring_bills (id, user_id),
    CONSTRAINT bill_payments_amount_positive CHECK (paid_amount_cents > 0),
    CONSTRAINT bill_payments_month_first_day CHECK (date_part('day', month) = 1),
    CONSTRAINT bill_payments_bill_month_uq UNIQUE (recurring_bill_id, month)
);

CREATE INDEX bill_payments_user_month_idx ON bill_payments (user_id, month);

-- +goose Down
DROP TABLE bill_payments;
