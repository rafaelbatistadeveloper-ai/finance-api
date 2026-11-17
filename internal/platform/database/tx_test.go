package database_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"

	"finance-app/internal/platform/database"
	"finance-app/internal/platform/database/dbtest"
)

func countUsers(t *testing.T, ctx context.Context, q interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}) int {
	t.Helper()
	var n int
	if err := q.QueryRow(ctx, `SELECT count(*) FROM users`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestInTx(t *testing.T) {
	pool := dbtest.NewPool(t)
	ctx := context.Background()
	insert := `INSERT INTO users (email, password_hash) VALUES ($1, 'h')`

	t.Run("confirma quando a função retorna nil", func(t *testing.T) {
		err := database.InTx(ctx, pool, func(tx pgx.Tx) error {
			_, err := tx.Exec(ctx, insert, "commit@exemplo.com")
			return err
		})
		if err != nil {
			t.Fatal(err)
		}
		if n := countUsers(t, ctx, pool); n != 1 {
			t.Errorf("usuários = %d, esperado 1", n)
		}
	})

	t.Run("desfaz tudo quando a função retorna erro", func(t *testing.T) {
		before := countUsers(t, ctx, pool)
		boom := errors.New("falha de negócio")

		err := database.InTx(ctx, pool, func(tx pgx.Tx) error {
			if _, err := tx.Exec(ctx, insert, "rollback1@exemplo.com"); err != nil {
				return err
			}
			if _, err := tx.Exec(ctx, insert, "rollback2@exemplo.com"); err != nil {
				return err
			}
			return boom
		})

		if !errors.Is(err, boom) {
			t.Fatalf("erro = %v, esperado %v", err, boom)
		}
		if after := countUsers(t, ctx, pool); after != before {
			t.Errorf("usuários = %d, esperado %d (rollback não desfez)", after, before)
		}
	})

	t.Run("desfaz quando uma restrição do banco falha no meio", func(t *testing.T) {
		before := countUsers(t, ctx, pool)

		err := database.InTx(ctx, pool, func(tx pgx.Tx) error {
			if _, err := tx.Exec(ctx, insert, "meio@exemplo.com"); err != nil {
				return err
			}
			_, err := tx.Exec(ctx, insert, "MEIO@exemplo.com") // duplicado
			return err
		})

		if code, _ := constraintOf(err); code != codeUnique {
			t.Fatalf("esperava violação de unicidade, obteve: %v", err)
		}
		if after := countUsers(t, ctx, pool); after != before {
			t.Errorf("usuários = %d, esperado %d", after, before)
		}
	})

	t.Run("desfaz quando a função entra em panic", func(t *testing.T) {
		before := countUsers(t, ctx, pool)

		func() {
			defer func() { _ = recover() }()
			_ = database.InTx(ctx, pool, func(tx pgx.Tx) error {
				if _, err := tx.Exec(ctx, insert, "panic@exemplo.com"); err != nil {
					return err
				}
				panic("boom")
			})
		}()

		if after := countUsers(t, ctx, pool); after != before {
			t.Errorf("usuários = %d, esperado %d", after, before)
		}
	})
}

func TestNewPoolURLInvalidaNaoVazaSenha(t *testing.T) {
	_, err := database.NewPool(context.Background(), "postgres://usuario:segredo123@host:abc/db")
	if err == nil {
		t.Fatal("esperava erro")
	}
	if msg := err.Error(); strings.Contains(msg, "segredo123") {
		t.Errorf("mensagem de erro vazou a senha: %s", msg)
	}
}

func TestNewPoolBancoIndisponivel(t *testing.T) {
	// Porta 1 não tem Postgres: deve falhar rápido e com erro claro.
	_, err := database.NewPool(context.Background(), "postgres://u:p@127.0.0.1:1/db?sslmode=disable&connect_timeout=2")
	if err == nil {
		t.Fatal("esperava erro de conexão")
	}
	if strings.Contains(err.Error(), ":p@") {
		t.Errorf("mensagem de erro vazou credenciais: %s", err)
	}
}
