package config

import (
	"strings"
	"testing"
)

const validSecret = "0123456789abcdef0123456789abcdef"

func env(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

func TestLoad(t *testing.T) {
	tests := []struct {
		name     string
		env      map[string]string
		want     Config
		wantErrs []string
	}{
		{
			name: "valores completos",
			env:  map[string]string{"PORT": "9000", "DATABASE_URL": "postgres://x", "JWT_SECRET": validSecret},
			want: Config{Port: "9000", DatabaseURL: "postgres://x", JWTSecret: validSecret},
		},
		{
			name: "porta padrão quando ausente",
			env:  map[string]string{"DATABASE_URL": "postgres://x", "JWT_SECRET": validSecret},
			want: Config{Port: "8080", DatabaseURL: "postgres://x", JWTSecret: validSecret},
		},
		{
			name:     "variáveis obrigatórias ausentes",
			env:      map[string]string{},
			wantErrs: []string{"DATABASE_URL é obrigatória", "JWT_SECRET é obrigatória"},
		},
		{
			name:     "segredo curto",
			env:      map[string]string{"DATABASE_URL": "postgres://x", "JWT_SECRET": "curto"},
			wantErrs: []string{"JWT_SECRET deve ter ao menos 32 caracteres"},
		},
		{
			name:     "porta não numérica",
			env:      map[string]string{"PORT": "abc", "DATABASE_URL": "postgres://x", "JWT_SECRET": validSecret},
			wantErrs: []string{"PORT inválida"},
		},
		{
			name:     "porta fora do intervalo",
			env:      map[string]string{"PORT": "70000", "DATABASE_URL": "postgres://x", "JWT_SECRET": validSecret},
			wantErrs: []string{"PORT inválida"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Load(env(tt.env))

			if len(tt.wantErrs) == 0 {
				if err != nil {
					t.Fatalf("erro inesperado: %v", err)
				}
				if got != tt.want {
					t.Fatalf("config = %+v, esperado %+v", got, tt.want)
				}
				return
			}

			if err == nil {
				t.Fatal("esperava erro, obteve nil")
			}
			for _, w := range tt.wantErrs {
				if !strings.Contains(err.Error(), w) {
					t.Errorf("erro %q não contém %q", err.Error(), w)
				}
			}
		})
	}
}
