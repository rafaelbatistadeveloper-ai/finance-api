package database_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"finance-app/internal/platform/database/dbtest"
)

// Códigos SQLSTATE do PostgreSQL usados nos testes.
const (
	codeUnique     = "23505"
	codeForeignKey = "23503"
	codeCheck      = "23514"
)

type fixture struct {
	pool          *pgxpool.Pool
	userA, userB  uuid.UUID
	catA, catB    uuid.UUID
	roomA, roomB  uuid.UUID
	billA         uuid.UUID
	expenseCredit uuid.UUID
}

func scanID(t *testing.T, pool *pgxpool.Pool, query string, args ...any) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	if err := pool.QueryRow(context.Background(), query, args...).Scan(&id); err != nil {
		t.Fatalf("seed (%s): %v", query, err)
	}
	return id
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	pool := dbtest.NewPool(t)
	f := &fixture{pool: pool}

	f.userA = scanID(t, pool, `INSERT INTO users (email, password_hash) VALUES ('a@exemplo.com', 'h') RETURNING id`)
	f.userB = scanID(t, pool, `INSERT INTO users (email, password_hash) VALUES ('b@exemplo.com', 'h') RETURNING id`)
	f.catA = scanID(t, pool, `INSERT INTO categories (user_id, name) VALUES ($1, 'Moradia') RETURNING id`, f.userA)
	f.catB = scanID(t, pool, `INSERT INTO categories (user_id, name) VALUES ($1, 'Moradia') RETURNING id`, f.userB)
	f.roomA = scanID(t, pool, `INSERT INTO rooms (user_id, name) VALUES ($1, 'Sala') RETURNING id`, f.userA)
	f.roomB = scanID(t, pool, `INSERT INTO rooms (user_id, name) VALUES ($1, 'Sala') RETURNING id`, f.userB)
	f.billA = scanID(t, pool, `
		INSERT INTO recurring_bills (user_id, category_id, name, amount_cents, due_day, active_from)
		VALUES ($1, $2, 'Internet', 12000, 10, '2026-01-01') RETURNING id`, f.userA, f.catA)
	f.expenseCredit = scanID(t, pool, `
		INSERT INTO expenses (user_id, category_id, room_id, description, amount_cents, spent_at, payment_method, installments_n)
		VALUES ($1, $2, $3, 'Sofá', 320000, '2026-12-05', 'credit', 10) RETURNING id`, f.userA, f.catA, f.roomA)
	return f
}

func constraintOf(err error) (code, name string) {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code, pgErr.ConstraintName
	}
	return "", ""
}

func i16(v int16) *int16 { return &v }

func (f *fixture) expense(tx pgx.Tx, user, cat uuid.UUID, room *uuid.UUID, amount int64, method string, n *int16) error {
	_, err := tx.Exec(context.Background(), `
		INSERT INTO expenses (user_id, category_id, room_id, description, amount_cents, spent_at, payment_method, installments_n)
		VALUES ($1, $2, $3, 'teste', $4, '2026-12-05', $5, $6)`,
		user, cat, room, amount, method, n)
	return err
}

// TestRestricoesDoEsquema confirma que o banco rejeita dados inválidos mesmo que a aplicação falhe.
// Cada caso roda em uma transação desfeita ao final, então os casos não interferem entre si.
func TestRestricoesDoEsquema(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()

	tests := []struct {
		name           string
		run            func(tx pgx.Tx) error
		wantCode       string // vazio = deve ser aceito
		wantConstraint string
	}{
		// users
		{
			name: "e-mail duplicado ignorando maiúsculas",
			run: func(tx pgx.Tx) error {
				_, err := tx.Exec(ctx, `INSERT INTO users (email, password_hash) VALUES ('A@EXEMPLO.COM', 'h')`)
				return err
			},
			wantCode: codeUnique, wantConstraint: "users_email_uq",
		},

		// income
		{
			name: "renda zero",
			run: func(tx pgx.Tx) error {
				_, err := tx.Exec(ctx, `INSERT INTO income (user_id, amount_cents, valid_from) VALUES ($1, 0, '2026-01-01')`, f.userA)
				return err
			},
			wantCode: codeCheck, wantConstraint: "income_amount_positive",
		},
		{
			name: "vigência da renda fora do primeiro dia do mês",
			run: func(tx pgx.Tx) error {
				_, err := tx.Exec(ctx, `INSERT INTO income (user_id, amount_cents, valid_from) VALUES ($1, 500000, '2026-01-15')`, f.userA)
				return err
			},
			wantCode: codeCheck, wantConstraint: "income_valid_from_first_day",
		},
		{
			name: "renda válida",
			run: func(tx pgx.Tx) error {
				_, err := tx.Exec(ctx, `INSERT INTO income (user_id, amount_cents, valid_from) VALUES ($1, 500000, '2026-01-01')`, f.userA)
				return err
			},
		},

		// categories e rooms
		{
			name: "categoria duplicada ignorando maiúsculas",
			run: func(tx pgx.Tx) error {
				_, err := tx.Exec(ctx, `INSERT INTO categories (user_id, name) VALUES ($1, 'MORADIA')`, f.userA)
				return err
			},
			wantCode: codeUnique, wantConstraint: "categories_name_uq",
		},
		{
			name: "mesmo nome de categoria em outro usuário é permitido",
			run: func(tx pgx.Tx) error {
				_, err := tx.Exec(ctx, `INSERT INTO categories (user_id, name) VALUES ($1, 'Lazer')`, f.userA)
				if err != nil {
					return err
				}
				_, err = tx.Exec(ctx, `INSERT INTO categories (user_id, name) VALUES ($1, 'Lazer')`, f.userB)
				return err
			},
		},
		{
			name: "ambiente duplicado ignorando maiúsculas",
			run: func(tx pgx.Tx) error {
				_, err := tx.Exec(ctx, `INSERT INTO rooms (user_id, name) VALUES ($1, 'sala')`, f.userA)
				return err
			},
			wantCode: codeUnique, wantConstraint: "rooms_name_uq",
		},
		{
			name: "excluir categoria em uso",
			run: func(tx pgx.Tx) error {
				_, err := tx.Exec(ctx, `DELETE FROM categories WHERE id = $1`, f.catA)
				return err
			},
			wantCode: codeForeignKey,
		},
		{
			name: "excluir ambiente com gastos",
			run: func(tx pgx.Tx) error {
				_, err := tx.Exec(ctx, `DELETE FROM rooms WHERE id = $1`, f.roomA)
				return err
			},
			wantCode: codeForeignKey,
		},

		// recurring_bills
		{
			name: "dia de vencimento 0",
			run: func(tx pgx.Tx) error {
				_, err := tx.Exec(ctx, `
					INSERT INTO recurring_bills (user_id, category_id, name, amount_cents, due_day, active_from)
					VALUES ($1, $2, 'x', 100, 0, '2026-01-01')`, f.userA, f.catA)
				return err
			},
			wantCode: codeCheck, wantConstraint: "recurring_bills_due_day_range",
		},
		{
			name: "dia de vencimento 32",
			run: func(tx pgx.Tx) error {
				_, err := tx.Exec(ctx, `
					INSERT INTO recurring_bills (user_id, category_id, name, amount_cents, due_day, active_from)
					VALUES ($1, $2, 'x', 100, 32, '2026-01-01')`, f.userA, f.catA)
				return err
			},
			wantCode: codeCheck, wantConstraint: "recurring_bills_due_day_range",
		},
		{
			name: "dia de vencimento 31 é aceito",
			run: func(tx pgx.Tx) error {
				_, err := tx.Exec(ctx, `
					INSERT INTO recurring_bills (user_id, category_id, name, amount_cents, due_day, active_from)
					VALUES ($1, $2, 'x', 100, 31, '2026-01-01')`, f.userA, f.catA)
				return err
			},
		},
		{
			name: "período encerrado antes de começar",
			run: func(tx pgx.Tx) error {
				_, err := tx.Exec(ctx, `
					INSERT INTO recurring_bills (user_id, category_id, name, amount_cents, due_day, active_from, active_until)
					VALUES ($1, $2, 'x', 100, 5, '2026-03-01', '2026-02-01')`, f.userA, f.catA)
				return err
			},
			wantCode: codeCheck, wantConstraint: "recurring_bills_period_valid",
		},
		{
			name: "conta recorrente com categoria de outro usuário",
			run: func(tx pgx.Tx) error {
				_, err := tx.Exec(ctx, `
					INSERT INTO recurring_bills (user_id, category_id, name, amount_cents, due_day, active_from)
					VALUES ($1, $2, 'x', 100, 5, '2026-01-01')`, f.userA, f.catB)
				return err
			},
			wantCode: codeForeignKey, wantConstraint: "recurring_bills_category_fk",
		},

		// bill_payments
		{
			name: "pagamento duplicado da mesma conta no mesmo mês",
			run: func(tx pgx.Tx) error {
				q := `INSERT INTO bill_payments (user_id, recurring_bill_id, month, paid_amount_cents) VALUES ($1, $2, '2026-02-01', 11800)`
				if _, err := tx.Exec(ctx, q, f.userA, f.billA); err != nil {
					return err
				}
				_, err := tx.Exec(ctx, q, f.userA, f.billA)
				return err
			},
			wantCode: codeUnique, wantConstraint: "bill_payments_bill_month_uq",
		},
		{
			name: "pagamento de conta de outro usuário",
			run: func(tx pgx.Tx) error {
				_, err := tx.Exec(ctx, `INSERT INTO bill_payments (user_id, recurring_bill_id, month, paid_amount_cents) VALUES ($1, $2, '2026-02-01', 100)`, f.userB, f.billA)
				return err
			},
			wantCode: codeForeignKey, wantConstraint: "bill_payments_bill_fk",
		},

		// expenses
		{
			name:     "gasto com valor zero",
			run:      func(tx pgx.Tx) error { return f.expense(tx, f.userA, f.catA, nil, 0, "pix", nil) },
			wantCode: codeCheck, wantConstraint: "expenses_amount_positive",
		},
		{
			name:     "forma de pagamento inválida",
			run:      func(tx pgx.Tx) error { return f.expense(tx, f.userA, f.catA, nil, 100, "cheque", nil) },
			wantCode: codeCheck, wantConstraint: "expenses_payment_method_valid",
		},
		{
			name:     "crédito sem número de parcelas",
			run:      func(tx pgx.Tx) error { return f.expense(tx, f.userA, f.catA, nil, 100, "credit", nil) },
			wantCode: codeCheck, wantConstraint: "expenses_installments_only_credit",
		},
		{
			name:     "pix com número de parcelas",
			run:      func(tx pgx.Tx) error { return f.expense(tx, f.userA, f.catA, nil, 100, "pix", i16(3)) },
			wantCode: codeCheck, wantConstraint: "expenses_installments_only_credit",
		},
		{
			name:     "49 parcelas",
			run:      func(tx pgx.Tx) error { return f.expense(tx, f.userA, f.catA, nil, 100, "credit", i16(49)) },
			wantCode: codeCheck, wantConstraint: "expenses_installments_range",
		},
		{
			name:     "0 parcelas",
			run:      func(tx pgx.Tx) error { return f.expense(tx, f.userA, f.catA, nil, 100, "credit", i16(0)) },
			wantCode: codeCheck, wantConstraint: "expenses_installments_range",
		},
		{
			name: "48 parcelas é aceito",
			run:  func(tx pgx.Tx) error { return f.expense(tx, f.userA, f.catA, nil, 100, "credit", i16(48)) },
		},
		{
			name: "1 parcela é aceita",
			run:  func(tx pgx.Tx) error { return f.expense(tx, f.userA, f.catA, nil, 100, "credit", i16(1)) },
		},
		{
			name: "pix sem ambiente é aceito",
			run:  func(tx pgx.Tx) error { return f.expense(tx, f.userA, f.catA, nil, 100, "pix", nil) },
		},
		{
			name: "pix com ambiente próprio é aceito",
			run:  func(tx pgx.Tx) error { return f.expense(tx, f.userA, f.catA, &f.roomA, 100, "pix", nil) },
		},
		{
			name:     "gasto com categoria de outro usuário",
			run:      func(tx pgx.Tx) error { return f.expense(tx, f.userA, f.catB, nil, 100, "pix", nil) },
			wantCode: codeForeignKey, wantConstraint: "expenses_category_fk",
		},
		{
			name:     "gasto com ambiente de outro usuário",
			run:      func(tx pgx.Tx) error { return f.expense(tx, f.userA, f.catA, &f.roomB, 100, "pix", nil) },
			wantCode: codeForeignKey, wantConstraint: "expenses_room_fk",
		},

		// installments
		{
			name: "parcela com mês de vencimento fora do primeiro dia",
			run: func(tx pgx.Tx) error {
				_, err := tx.Exec(ctx, `INSERT INTO installments (expense_id, number, due_month, amount_cents) VALUES ($1, 1, '2026-12-15', 32000)`, f.expenseCredit)
				return err
			},
			wantCode: codeCheck, wantConstraint: "installments_due_month_first_day",
		},
		{
			name: "número de parcela duplicado no mesmo gasto",
			run: func(tx pgx.Tx) error {
				q := `INSERT INTO installments (expense_id, number, due_month, amount_cents) VALUES ($1, 1, '2026-12-01', 32000)`
				if _, err := tx.Exec(ctx, q, f.expenseCredit); err != nil {
					return err
				}
				_, err := tx.Exec(ctx, q, f.expenseCredit)
				return err
			},
			wantCode: codeUnique, wantConstraint: "installments_expense_number_uq",
		},
		{
			name: "parcela de gasto inexistente",
			run: func(tx pgx.Tx) error {
				_, err := tx.Exec(ctx, `INSERT INTO installments (expense_id, number, due_month, amount_cents) VALUES ($1, 1, '2026-12-01', 100)`, uuid.New())
				return err
			},
			wantCode: codeForeignKey,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tx, err := f.pool.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = tx.Rollback(ctx) }()

			err = tt.run(tx)

			if tt.wantCode == "" {
				if err != nil {
					t.Fatalf("esperava sucesso, obteve: %v", err)
				}
				return
			}
			code, constraint := constraintOf(err)
			if code != tt.wantCode {
				t.Fatalf("SQLSTATE = %q, esperado %q (erro: %v)", code, tt.wantCode, err)
			}
			if tt.wantConstraint != "" && constraint != tt.wantConstraint {
				t.Errorf("restrição = %q, esperado %q", constraint, tt.wantConstraint)
			}
		})
	}
}

func TestExcluirGastoRemoveParcelasEmCascata(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()

	for n := 1; n <= 3; n++ {
		due := time.Date(2026, time.Month(11+n), 1, 0, 0, 0, 0, time.UTC)
		if _, err := f.pool.Exec(ctx,
			`INSERT INTO installments (expense_id, number, due_month, amount_cents) VALUES ($1, $2, $3, 100)`,
			f.expenseCredit, n, due); err != nil {
			t.Fatal(err)
		}
	}

	if _, err := f.pool.Exec(ctx, `DELETE FROM expenses WHERE id = $1`, f.expenseCredit); err != nil {
		t.Fatalf("excluir gasto: %v", err)
	}

	var remaining int
	if err := f.pool.QueryRow(ctx, `SELECT count(*) FROM installments WHERE expense_id = $1`, f.expenseCredit).Scan(&remaining); err != nil {
		t.Fatal(err)
	}
	if remaining != 0 {
		t.Errorf("restaram %d parcelas, esperado 0", remaining)
	}
}

func TestMigrationsSubirDescerESubirDeNovo(t *testing.T) {
	url := dbtest.NewDatabase(t)
	ctx := context.Background()

	countTables := func() int {
		t.Helper()
		pool, err := pgxpool.New(ctx, url)
		if err != nil {
			t.Fatal(err)
		}
		defer pool.Close()
		var n int
		err = pool.QueryRow(ctx, `
			SELECT count(*) FROM information_schema.tables
			WHERE table_schema = 'public' AND table_name <> 'goose_db_version'`).Scan(&n)
		if err != nil {
			t.Fatal(err)
		}
		return n
	}

	if n := countTables(); n != 0 {
		t.Fatalf("banco novo deveria estar vazio, tem %d tabelas", n)
	}
	dbtest.Migrate(t, url)
	const want = 8
	if n := countTables(); n != want {
		t.Fatalf("após subir: %d tabelas, esperado %d", n, want)
	}
	dbtest.MigrateDown(t, url)
	if n := countTables(); n != 0 {
		t.Fatalf("após descer: %d tabelas, esperado 0", n)
	}
	dbtest.Migrate(t, url)
	if n := countTables(); n != want {
		t.Fatalf("após subir de novo: %d tabelas, esperado %d", n, want)
	}
}
