package db_test

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"finance-app/internal/platform/database/db"
	"finance-app/internal/platform/database/dbtest"
)

func TestUserQueries(t *testing.T) {
	pool := dbtest.NewPool(t)
	q := db.New(pool)
	ctx := context.Background()

	created, err := q.CreateUser(ctx, db.CreateUserParams{Email: "Maria@Exemplo.com", PasswordHash: "hash"})
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	if created.ID.String() == "" || created.CreatedAt.IsZero() {
		t.Errorf("campos gerados pelo banco não vieram preenchidos: %+v", created)
	}

	t.Run("busca por e-mail ignora maiúsculas", func(t *testing.T) {
		got, err := q.GetUserByEmail(ctx, "maria@exemplo.com")
		if err != nil {
			t.Fatal(err)
		}
		if got.ID != created.ID {
			t.Errorf("id = %v, esperado %v", got.ID, created.ID)
		}
	})

	t.Run("busca por id", func(t *testing.T) {
		got, err := q.GetUserByID(ctx, created.ID)
		if err != nil {
			t.Fatal(err)
		}
		if got.Email != "Maria@Exemplo.com" {
			t.Errorf("email = %q, o e-mail original deve ser preservado", got.Email)
		}
	})

	t.Run("usuário inexistente devolve pgx.ErrNoRows", func(t *testing.T) {
		_, err := q.GetUserByEmail(ctx, "ninguem@exemplo.com")
		if !errors.Is(err, pgx.ErrNoRows) {
			t.Errorf("erro = %v, esperado pgx.ErrNoRows", err)
		}
	})

	t.Run("e-mail duplicado viola unicidade", func(t *testing.T) {
		_, err := q.CreateUser(ctx, db.CreateUserParams{Email: "MARIA@exemplo.com", PasswordHash: "outro"})
		var pgErr *pgconn.PgError
		if !errors.As(err, &pgErr) || pgErr.Code != "23505" {
			t.Errorf("esperava violação de unicidade (23505), obteve: %v", err)
		}
	})
}
