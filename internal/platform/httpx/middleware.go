package httpx

import (
	"errors"
	"log/slog"
	"net/http"
	"runtime/debug"
	"time"

	"github.com/gin-gonic/gin"
)

// Recovery captura panics, registra a pilha no log e responde 500 no formato padrão.
// O conteúdo do panic nunca é enviado ao cliente.
func Recovery(logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			rec := recover()
			if rec == nil {
				return
			}
			// Sinal usado pelo net/http para abortar a resposta; deve continuar subindo.
			if err, ok := rec.(error); ok && errors.Is(err, http.ErrAbortHandler) {
				panic(rec)
			}
			logger.Error("panic recuperado",
				slog.Any("panic", rec),
				slog.String("stack", string(debug.Stack())),
				slog.String("route", routeOf(c)),
			)
			c.AbortWithStatusJSON(http.StatusInternalServerError, ErrorBody{
				Code:    "internal_error",
				Message: "erro interno do servidor",
			})
		}()
		c.Next()
	}
}

// Logging registra uma linha por requisição.
// Usa o padrão da rota (ex.: /v1/expenses/:id) e não o caminho real nem a query string,
// para não gravar identificadores ou dados sensíveis nos logs.
func Logging(logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()

		status := c.Writer.Status()
		attrs := []any{
			slog.String("method", c.Request.Method),
			slog.String("route", routeOf(c)),
			slog.Int("status", status),
			slog.Duration("duration", time.Since(start)),
		}
		if len(c.Errors) > 0 {
			attrs = append(attrs, slog.String("errors", c.Errors.String()))
		}

		switch {
		case status >= http.StatusInternalServerError:
			logger.Error("requisição", attrs...)
		case status >= http.StatusBadRequest:
			logger.Warn("requisição", attrs...)
		default:
			logger.Info("requisição", attrs...)
		}
	}
}

func routeOf(c *gin.Context) string {
	if p := c.FullPath(); p != "" {
		return p
	}
	return "(sem rota)"
}
