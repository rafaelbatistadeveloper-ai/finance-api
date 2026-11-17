// Package dbtest cria bancos PostgreSQL isolados para testes de integração.
//
// Os testes usam a variável TEST_DATABASE_URL, que aponta para um servidor PostgreSQL
// com permissão para criar bancos (ex.: postgres://finance:finance@localhost:5432/postgres?sslmode=disable).
// Sem ela, os testes de integração são ignorados (skip), e `go test ./...` continua passando.
package dbtest

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib" // registra o driver "pgx" para o goose
	"github.com/pressly/goose/v3"

	"finance-app/db/migrations"
)

const envName = "TEST_DATABASE_URL"

// NewDatabase cria um banco vazio, exclusivo do teste, e devolve a URL de conexão.
// O banco é removido ao final do teste.
func NewDatabase(t testing.TB) string {
	t.Helper()

	adminURL := os.Getenv(envName)
	if adminURL == "" {
		t.Skipf("%s não definida: teste de integração ignorado", envName)
	}

	ctx := context.Background()
	admin, err := pgxpool.New(ctx, adminURL)
	if err != nil {
		t.Fatalf("conectar ao servidor de testes: %v", err)
	}

	name := "finance_test_" + randomSuffix(t)
	// O nome é gerado por nós (hex), então é seguro para uso como identificador.
	if _, err := admin.Exec(ctx, "CREATE DATABASE "+name); err != nil {
		admin.Close()
		t.Fatalf("criar banco %s: %v", name, err)
	}

	t.Cleanup(func() {
		defer admin.Close()
		if _, err := admin.Exec(context.Background(), "DROP DATABASE IF EXISTS "+name+" WITH (FORCE)"); err != nil {
			t.Errorf("remover banco %s: %v", name, err)
		}
	})

	u, err := url.Parse(adminURL)
	if err != nil {
		t.Fatalf("%s inválida: %v", envName, err)
	}
	u.Path = "/" + name
	return u.String()
}

// Migrate aplica todas as migrations ao banco indicado.
func Migrate(t testing.TB, databaseURL string) {
	t.Helper()
	withGoose(t, databaseURL, func(db *sql.DB) error { return goose.Up(db, ".") })
}

// MigrateDown desfaz todas as migrations do banco indicado.
func MigrateDown(t testing.TB, databaseURL string) {
	t.Helper()
	withGoose(t, databaseURL, func(db *sql.DB) error { return goose.DownTo(db, ".", 0) })
}

// NewPool devolve um pool conectado a um banco novo, já migrado.
func NewPool(t testing.TB) *pgxpool.Pool {
	t.Helper()

	databaseURL := NewDatabase(t)
	Migrate(t, databaseURL)

	pool, err := pgxpool.New(context.Background(), databaseURL)
	if err != nil {
		t.Fatalf("abrir pool de testes: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		t.Fatalf("banco de testes indisponível: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

func withGoose(t testing.TB, databaseURL string, fn func(*sql.DB) error) {
	t.Helper()

	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		t.Fatalf("abrir conexão para migrations: %v", err)
	}
	defer db.Close()

	goose.SetBaseFS(migrations.FS)
	goose.SetLogger(goose.NopLogger())
	if err := goose.SetDialect("postgres"); err != nil {
		t.Fatalf("dialeto goose: %v", err)
	}
	if err := fn(db); err != nil {
		t.Fatalf("migrations: %v", err)
	}
}

func randomSuffix(t testing.TB) string {
	t.Helper()
	b := make([]byte, 6)
	if _, err := rand.Read(b); err != nil {
		t.Fatal(fmt.Errorf("gerar sufixo aleatório: %w", err))
	}
	return hex.EncodeToString(b)
}
