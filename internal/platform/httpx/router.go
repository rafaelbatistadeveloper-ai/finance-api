package httpx

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
)

// NewRouter cria o roteador Gin com os middlewares globais e as rotas de infraestrutura.
// Os módulos de domínio registram suas rotas depois, em grupos próprios (ex.: /v1).
func NewRouter(logger *slog.Logger) *gin.Engine {
	r := gin.New()
	r.HandleMethodNotAllowed = true
	r.Use(Recovery(logger), Logging(logger))

	r.NoRoute(func(c *gin.Context) {
		RespondError(c, NotFound("route_not_found", "rota não encontrada"))
	})
	r.NoMethod(func(c *gin.Context) {
		RespondError(c, NewError(http.StatusMethodNotAllowed, "method_not_allowed", "método não permitido"))
	})

	r.GET("/healthz", healthz)

	return r
}

func healthz(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}
