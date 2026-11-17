package httpx

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func init() { gin.SetMode(gin.TestMode) }

func newTestRouter() (*gin.Engine, *bytes.Buffer) {
	buf := &bytes.Buffer{}
	logger := slog.New(slog.NewTextHandler(buf, nil))
	return NewRouter(logger), buf
}

func do(r http.Handler, method, path string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func decodeError(t *testing.T, body io.Reader) ErrorBody {
	t.Helper()
	var eb ErrorBody
	if err := json.NewDecoder(body).Decode(&eb); err != nil {
		t.Fatalf("corpo não é JSON no formato de erro: %v", err)
	}
	return eb
}

func TestHealthz(t *testing.T) {
	r, _ := newTestRouter()
	w := do(r, http.MethodGet, "/healthz")

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, esperado 200", w.Code)
	}
	var body map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["status"] != "ok" {
		t.Errorf("body = %v, esperado status=ok", body)
	}
}

func TestRotaInexistente(t *testing.T) {
	r, _ := newTestRouter()
	w := do(r, http.MethodGet, "/nao-existe")

	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, esperado 404", w.Code)
	}
	if eb := decodeError(t, w.Body); eb.Code != "route_not_found" {
		t.Errorf("code = %q, esperado route_not_found", eb.Code)
	}
}

func TestMetodoNaoPermitido(t *testing.T) {
	r, _ := newTestRouter()
	w := do(r, http.MethodPost, "/healthz")

	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, esperado 405", w.Code)
	}
	if eb := decodeError(t, w.Body); eb.Code != "method_not_allowed" {
		t.Errorf("code = %q, esperado method_not_allowed", eb.Code)
	}
}

func TestRespondError(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantStatus int
		wantCode   string
		wantMsg    string
	}{
		{"bad request", BadRequest("invalid_amount", "valor inválido"), 400, "invalid_amount", "valor inválido"},
		{"unauthorized", Unauthorized("invalid_credentials", "credenciais inválidas"), 401, "invalid_credentials", "credenciais inválidas"},
		{"not found", NotFound("expense_not_found", "gasto não encontrado"), 404, "expense_not_found", "gasto não encontrado"},
		{"conflict", Conflict("category_in_use", "categoria em uso"), 409, "category_in_use", "categoria em uso"},
		{"erro genérico vira 500 sem vazar detalhe", errors.New("senha do banco: hunter2"), 500, "internal_error", "erro interno do servidor"},
		{
			"AppError com causa interna não vaza a causa",
			&AppError{Status: 409, Code: "x", Message: "conflito", Err: errors.New("detalhe interno secreto")},
			409, "x", "conflito",
		},
		{
			"AppError embrulhado com fmt.Errorf",
			fmt.Errorf("ao buscar ambiente: %w", NotFound("room_not_found", "ambiente não encontrado")),
			404, "room_not_found", "ambiente não encontrado",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, _ := newTestRouter()
			r.GET("/erro", func(c *gin.Context) { RespondError(c, tt.err) })

			w := do(r, http.MethodGet, "/erro")

			if w.Code != tt.wantStatus {
				t.Fatalf("status = %d, esperado %d", w.Code, tt.wantStatus)
			}
			eb := decodeError(t, bytes.NewReader(w.Body.Bytes()))
			if eb.Code != tt.wantCode || eb.Message != tt.wantMsg {
				t.Errorf("corpo = %+v, esperado code=%q message=%q", eb, tt.wantCode, tt.wantMsg)
			}
			for _, segredo := range []string{"hunter2", "detalhe interno secreto"} {
				if strings.Contains(w.Body.String(), segredo) {
					t.Errorf("resposta vazou detalhe interno %q: %s", segredo, w.Body.String())
				}
			}
		})
	}
}

func TestRecoveryConverteEmErro500(t *testing.T) {
	r, logs := newTestRouter()
	r.GET("/panic", func(c *gin.Context) { panic("falha com dado sensível: token-abc") })

	w := do(r, http.MethodGet, "/panic")

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, esperado 500", w.Code)
	}
	if eb := decodeError(t, bytes.NewReader(w.Body.Bytes())); eb.Code != "internal_error" {
		t.Errorf("code = %q, esperado internal_error", eb.Code)
	}
	if strings.Contains(w.Body.String(), "token-abc") {
		t.Error("o conteúdo do panic vazou na resposta")
	}
	if !strings.Contains(logs.String(), "panic recuperado") || !strings.Contains(logs.String(), "token-abc") {
		t.Error("o panic deveria estar registrado no log")
	}

	// O servidor continua atendendo depois do panic.
	if w2 := do(r, http.MethodGet, "/healthz"); w2.Code != http.StatusOK {
		t.Errorf("healthz após panic = %d, esperado 200", w2.Code)
	}
}

func TestLoggingNaoRegistraQueryNemIdReal(t *testing.T) {
	r, logs := newTestRouter()
	r.GET("/v1/expenses/:id", func(c *gin.Context) { c.Status(http.StatusOK) })

	do(r, http.MethodGet, "/v1/expenses/123e4567-segredo?token=abc")

	out := logs.String()
	if !strings.Contains(out, "/v1/expenses/:id") {
		t.Errorf("log deveria conter o padrão da rota: %s", out)
	}
	for _, proibido := range []string{"123e4567-segredo", "token=abc"} {
		if strings.Contains(out, proibido) {
			t.Errorf("log contém dado que não deveria: %q\n%s", proibido, out)
		}
	}
}

func TestLoggingNiveisPorStatus(t *testing.T) {
	r, logs := newTestRouter()
	r.GET("/ok", func(c *gin.Context) { c.Status(200) })
	r.GET("/ruim", func(c *gin.Context) { RespondError(c, BadRequest("x", "y")) })
	r.GET("/quebra", func(c *gin.Context) { RespondError(c, errors.New("boom")) })

	do(r, http.MethodGet, "/ok")
	do(r, http.MethodGet, "/ruim")
	do(r, http.MethodGet, "/quebra")

	out := logs.String()
	for _, nivel := range []string{"level=INFO", "level=WARN", "level=ERROR"} {
		if !strings.Contains(out, nivel) {
			t.Errorf("log deveria conter %s:\n%s", nivel, out)
		}
	}
}
