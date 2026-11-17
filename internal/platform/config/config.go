// Package config carrega a configuração da aplicação a partir de variáveis de ambiente.
package config

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

const (
	defaultPort = "8080"
	// minJWTSecretLen evita segredos triviais para assinar tokens.
	minJWTSecretLen = 32
)

// Config reúne os valores de configuração da API.
type Config struct {
	Port        string
	DatabaseURL string
	JWTSecret   string
}

// Load lê a configuração usando getenv (normalmente os.Getenv).
// Receber a função como parâmetro torna o código testável sem alterar o ambiente do processo.
// Todos os problemas encontrados são reportados juntos em um único erro.
func Load(getenv func(string) string) (Config, error) {
	cfg := Config{
		Port:        strings.TrimSpace(getenv("PORT")),
		DatabaseURL: strings.TrimSpace(getenv("DATABASE_URL")),
		JWTSecret:   getenv("JWT_SECRET"),
	}

	if cfg.Port == "" {
		cfg.Port = defaultPort
	}

	var errs []error

	if n, err := strconv.Atoi(cfg.Port); err != nil || n < 1 || n > 65535 {
		errs = append(errs, fmt.Errorf("PORT inválida: %q (use um número entre 1 e 65535)", cfg.Port))
	}
	if cfg.DatabaseURL == "" {
		errs = append(errs, errors.New("DATABASE_URL é obrigatória"))
	}
	switch {
	case cfg.JWTSecret == "":
		errs = append(errs, errors.New("JWT_SECRET é obrigatória"))
	case len(cfg.JWTSecret) < minJWTSecretLen:
		errs = append(errs, fmt.Errorf("JWT_SECRET deve ter ao menos %d caracteres", minJWTSecretLen))
	}

	if len(errs) > 0 {
		return Config{}, errors.Join(errs...)
	}
	return cfg, nil
}
