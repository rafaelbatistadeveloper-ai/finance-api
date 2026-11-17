package main

import (
	"fmt"
	"os"

	"finance-app/internal/platform/config"
)

func main() {
	cfg, err := config.Load(os.Getenv)
	if err != nil {
		fmt.Fprintf(os.Stderr, "configuração inválida:\n%v\n", err)
		os.Exit(1)
	}
	// O servidor HTTP (Gin) é adicionado na tarefa 2.
	fmt.Printf("configuração carregada (porta %s)\n", cfg.Port)
}
