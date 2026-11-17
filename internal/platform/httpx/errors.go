// Package httpx reúne a infraestrutura HTTP compartilhada: roteador, middlewares e formato de erro.
package httpx

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
)

// AppError é um erro com tradução direta para uma resposta HTTP.
// Os services devolvem erros de domínio e os handlers os convertem em AppError.
type AppError struct {
	Status  int
	Code    string
	Message string
	Err     error // causa original, nunca enviada ao cliente
}

func (e *AppError) Error() string {
	if e.Err != nil {
		return e.Code + ": " + e.Message + ": " + e.Err.Error()
	}
	return e.Code + ": " + e.Message
}

func (e *AppError) Unwrap() error { return e.Err }

// NewError cria um AppError com status, código estável e mensagem para o cliente.
func NewError(status int, code, message string) *AppError {
	return &AppError{Status: status, Code: code, Message: message}
}

func BadRequest(code, message string) *AppError {
	return NewError(http.StatusBadRequest, code, message)
}
func Unauthorized(code, message string) *AppError {
	return NewError(http.StatusUnauthorized, code, message)
}
func NotFound(code, message string) *AppError { return NewError(http.StatusNotFound, code, message) }
func Conflict(code, message string) *AppError { return NewError(http.StatusConflict, code, message) }

// ErrorBody é o formato único de erro da API (design, seção 5).
type ErrorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// RespondError escreve o erro no formato padrão e interrompe a cadeia de handlers.
// Erros que não são AppError viram 500 com mensagem genérica: detalhes internos
// não vão para o cliente, apenas para o log (via c.Error).
func RespondError(c *gin.Context, err error) {
	var appErr *AppError
	if errors.As(err, &appErr) {
		if appErr.Err != nil {
			_ = c.Error(appErr.Err)
		}
		c.AbortWithStatusJSON(appErr.Status, ErrorBody{Code: appErr.Code, Message: appErr.Message})
		return
	}

	_ = c.Error(err)
	c.AbortWithStatusJSON(http.StatusInternalServerError, ErrorBody{
		Code:    "internal_error",
		Message: "erro interno do servidor",
	})
}
