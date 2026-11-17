// Package database cuida da conexão com o PostgreSQL e das transações.
// O código em ./db é gerado pelo sqlc (veja sqlc.yaml); não edite à mão.
package database

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const pingTimeout = 5 * time.Second

// NewPool abre o pool de conexões e confirma que o banco responde.
func NewPool(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		// O erro original é descartado de propósito: ele pode ecoar trechos da URL, que contém a senha.
		return nil, errors.New("DATABASE_URL inválida (esperado postgres://usuario:senha@host:porta/banco)")
	}

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("criar pool de conexões: %w", err)
	}

	pingCtx, cancel := context.WithTimeout(ctx, pingTimeout)
	defer cancel()
	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("banco de dados indisponível: %w", err)
	}
	return pool, nil
}

// Beginner é satisfeito por *pgxpool.Pool e permite trocar a implementação nos testes.
type Beginner interface {
	Begin(ctx context.Context) (pgx.Tx, error)
}

// InTx executa fn dentro de uma transação.
// Confirma (commit) se fn retornar nil; desfaz (rollback) se retornar erro ou entrar em panic.
func InTx(ctx context.Context, b Beginner, fn func(tx pgx.Tx) error) error {
	return pgx.BeginFunc(ctx, b, fn)
}
