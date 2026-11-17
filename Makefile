.PHONY: run test vet migrate-up migrate-down sqlc

# Carrega o .env, se existir, para os comandos abaixo.
ifneq (,$(wildcard .env))
include .env
export
endif

run:
	go run ./cmd/api

# Usa TEST_DATABASE_URL (do .env) para os testes de integração; sem ela, eles são ignorados.
test:
	go test ./... -count=1

vet:
	go vet ./...

# Requer: go install github.com/pressly/goose/v3/cmd/goose@latest
migrate-up:
	goose -dir db/migrations postgres "$(DATABASE_URL)" up

migrate-down:
	goose -dir db/migrations postgres "$(DATABASE_URL)" down

# Requer: go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest
sqlc:
	sqlc generate
