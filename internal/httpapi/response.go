package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/Vinist021/outguessed-back/internal/health"
)

type problemType struct {
	status                      int
	code, suffix, title, detail string
}

var (
	notFound = problemType{
		status: http.StatusNotFound, code: "NOT_FOUND", suffix: "not-found",
		title: "Caminho não encontrado", detail: "O caminho solicitado não existe.",
	}
	methodNotAllowed = problemType{
		status: http.StatusMethodNotAllowed, code: "METHOD_NOT_ALLOWED", suffix: "method-not-allowed",
		title: "Método não permitido", detail: "O método solicitado não é permitido para este caminho.",
	}
	internalError = problemType{
		status: http.StatusInternalServerError, code: "INTERNAL_ERROR", suffix: "internal-error",
		title: "Erro interno", detail: "Não foi possível processar a requisição.",
	}
	serviceUnavailable = problemType{
		status: http.StatusServiceUnavailable, code: "SERVICE_UNAVAILABLE", suffix: "service-unavailable",
		title: "Serviço indisponível", detail: "O serviço não está pronto para receber requisições.",
	}
)

type problemDetails struct {
	Type      string       `json:"type"`
	Title     string       `json:"title"`
	Status    int          `json:"status"`
	Detail    string       `json:"detail"`
	Instance  string       `json:"instance"`
	Code      string       `json:"code"`
	RequestID string       `json:"requestId"`
	Errors    []fieldError `json:"errors,omitempty"`
}

type fieldError struct {
	Field   string `json:"field"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

func writeProblem(w http.ResponseWriter, r *http.Request, kind problemType) {
	id := requestID(r.Context())
	writeJSON(w, kind.status, "application/problem+json", problemDetails{
		Type:      "urn:outguessed:problem:" + kind.suffix,
		Title:     kind.title,
		Status:    kind.status,
		Detail:    kind.detail,
		Instance:  "urn:outguessed:request:" + id,
		Code:      kind.code,
		RequestID: id,
	})
}

func writeHealthy(w http.ResponseWriter) {
	writeJSON(w, http.StatusOK, "application/json", struct {
		Status string `json:"status"`
	}{Status: "ok"})
}

func writeJSON(w http.ResponseWriter, status int, contentType string, body any) {
	w.Header().Set("Content-Type", contentType)
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func readinessFailure(err error) string {
	switch {
	case errors.Is(err, health.ErrStopping):
		return "aplicação em encerramento"
	case errors.Is(err, health.ErrSchemaMissing):
		return "schema outguessed ausente"
	case errors.Is(err, context.DeadlineExceeded):
		return "prazo de prontidão excedido"
	case errors.Is(err, context.Canceled):
		return "verificação cancelada"
	}
	var dependency *health.Error
	if errors.As(err, &dependency) {
		switch dependency.Dependency {
		case "PostgreSQL", "Redis":
			return dependency.Dependency + " indisponível"
		}
	}
	return "falha na verificação de prontidão"
}
