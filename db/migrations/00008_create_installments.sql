-- +goose Up
-- Somente para gastos no crédito. O isolamento entre usuários vem de expenses.user_id.
CREATE TABLE installments (
    id           uuid     PRIMARY KEY DEFAULT gen_random_uuid(),
    expense_id   uuid     NOT NULL REFERENCES expenses (id) ON DELETE CASCADE,
    number       smallint NOT NULL,
    due_month    date     NOT NULL,
    amount_cents bigint   NOT NULL,
    CONSTRAINT installments_number_positive CHECK (number >= 1),
    CONSTRAINT installments_amount_positive CHECK (amount_cents > 0),
    CONSTRAINT installments_due_month_first_day CHECK (date_part('day', due_month) = 1),
    CONSTRAINT installments_expense_number_uq UNIQUE (expense_id, number)
);

CREATE INDEX installments_due_month_idx ON installments (due_month);

-- +goose Down
DROP TABLE installments;
