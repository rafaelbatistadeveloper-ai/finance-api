// Package migrations embute os arquivos SQL do goose no binário.
// Assim os testes (e, no futuro, a própria API) aplicam o esquema sem depender de caminhos em disco.
package migrations

import "embed"

//go:embed *.sql
var FS embed.FS
