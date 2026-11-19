package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Vinist021/outguessed-back/internal/health"
)

func TestRouterContracts(t *testing.T) {
	t.Parallel()
	router := NewRouter(func(context.Context) error { return nil }, discardLogger(), nil)
	for _, tc := range []struct {
		name, method, path string
		status             int
		code, suffix       string
		allow, cache       string
	}{
		{"vivacidade", "GET", "/health/live", 200, "", "", "", "no-store"},
		{"prontidão", "GET", "/health/ready", 200, "", "", "", "no-store"},
		{"caminho inexistente", "GET", "/caminho-inexistente", 404, "NOT_FOUND", "not-found", "", ""},
		{"método incompatível", "POST", "/health/live", 405, "METHOD_NOT_ALLOWED", "method-not-allowed", "GET", "no-store"},
		{"método desconhecido em rota existente", "CUSTOM", "/health/ready", 405, "METHOD_NOT_ALLOWED", "method-not-allowed", "GET", "no-store"},
		{"método desconhecido em rota inexistente", "CUSTOM", "/caminho-inexistente", 404, "NOT_FOUND", "not-found", "", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			request := httptest.NewRequest(tc.method, tc.path, nil)
			request.Header.Set("X-Request-ID", "fornecido-pelo-cliente")
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)
			if response.Code != tc.status {
				t.Fatalf("status = %d; esperado %d", response.Code, tc.status)
			}
			for name, value := range map[string]string{"Allow": tc.allow, "Cache-Control": tc.cache} {
				if got := response.Header().Get(name); got != value {
					t.Errorf("%s = %q; esperado %q", name, got, value)
				}
			}
			if id := response.Header().Get("X-Request-ID"); id == "" || id == "fornecido-pelo-cliente" {
				t.Errorf("identificador gerado inválido: %q", id)
			}
			if tc.code == "" {
				assertHealthy(t, response)
				return
			}
			assertProblem(t, response, tc.status, tc.code, tc.suffix)
		})
	}
}

func TestReadinessUsesFreshChecksAndSanitizedLogs(t *testing.T) {
	t.Parallel()
	var logs bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logs, nil))
	probe := &readinessProbe{schemaPresent: true}
	service := health.New(probe, probe, time.Second, discardLogger())
	router := NewRouter(service.Check, logger, nil)
	const secret = "segredo-simulado"
	check := func(path string) *httptest.ResponseRecorder {
		t.Helper()
		request := httptest.NewRequest(http.MethodGet, path+"?token="+secret, strings.NewReader(secret))
		request.Header.Set("Authorization", "Bearer "+secret)
		request.Header.Set("Cookie", "session="+secret)
		request.Header.Set("X-Request-ID", secret)
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)
		return response
	}

	first := check(readyPath)
	assertHealthy(t, first)
	probe.redisErr = errors.New("falha Redis: " + secret)
	second := check(readyPath)
	assertProblem(t, second, 503, "SERVICE_UNAVAILABLE", "service-unavailable")
	if second.Header().Get("Cache-Control") != "no-store" {
		t.Error("prontidão indisponível deve usar no-store")
	}
	if strings.Contains(second.Body.String(), secret) {
		t.Error("falha de prontidão expôs a causa bruta")
	}
	assertHealthy(t, check(livePath))
	if probe.schemaChecks != 2 || probe.redisChecks != 2 {
		t.Fatalf("vivacidade consultou dependências ou prontidão reutilizou resultado: %+v", probe)
	}

	probe.redisErr = nil
	assertHealthy(t, check(readyPath))
	probe.schemaPresent = false
	assertProblem(t, check(readyPath), 503, "SERVICE_UNAVAILABLE", "service-unavailable")
	probe.schemaPresent = true
	probe.schemaErr = errors.New("falha PostgreSQL: " + secret)
	assertProblem(t, check(readyPath), 503, "SERVICE_UNAVAILABLE", "service-unavailable")
	if probe.schemaChecks != 5 || probe.redisChecks != 3 {
		t.Errorf("consultas esperadas do PostgreSQL e do Redis: %+v", probe)
	}

	id := second.Header().Get("X-Request-ID")
	if id == first.Header().Get("X-Request-ID") {
		t.Error("identificador reutilizado entre requisições")
	}
	if strings.Contains(logs.String(), secret) {
		t.Error("logs expuseram dados da requisição ou da dependência")
	}
	assertLog(t, logs.String(), map[string]any{
		"msg": "Verificação HTTP de prontidão falhou", "request_id": id, "cause": "Redis indisponível",
	})
	assertLog(t, logs.String(), map[string]any{
		"msg": "Requisição HTTP", "request_id": id, "method": "GET", "path": readyPath, "status": float64(503),
	})
}

type readinessProbe struct {
	schemaPresent bool
	schemaErr     error
	redisErr      error
	schemaChecks  int
	redisChecks   int
}

func (p *readinessProbe) CheckSchema(context.Context) (bool, error) {
	p.schemaChecks++
	return p.schemaPresent, p.schemaErr
}

func (p *readinessProbe) Ping(context.Context) error {
	p.redisChecks++
	return p.redisErr
}

func TestPanicUsesCorrelatedProblem(t *testing.T) {
	t.Parallel()
	var logs bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logs, nil))
	const secret = "credencial-simulada"
	var fromContext string
	router := NewRouter(func(ctx context.Context) error {
		fromContext = requestID(ctx)
		panic([]string{secret})
	}, logger, []string{"http://localhost:8081"})
	request := httptest.NewRequest(http.MethodGet, readyPath, nil)
	request.Header.Set("Origin", "http://localhost:8081")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	assertProblem(t, response, 500, "INTERNAL_ERROR", "internal-error")
	if response.Header().Get("Cache-Control") != "no-store" ||
		response.Header().Get("Access-Control-Expose-Headers") != "X-Request-ID" {
		t.Errorf("cabeçalhos do 500: %v", response.Header())
	}
	id := response.Header().Get("X-Request-ID")
	if id != fromContext {
		t.Error("identificador do contexto difere do cabeçalho")
	}
	if strings.Contains(logs.String(), secret) || strings.Contains(response.Body.String(), secret) {
		t.Error("panic expôs seu valor bruto")
	}
	assertLog(t, logs.String(), map[string]any{"msg": "Pânico durante requisição HTTP", "request_id": id})
	assertLog(t, logs.String(), map[string]any{"msg": "Requisição HTTP", "request_id": id, "status": float64(500)})
}

func TestPanicAfterCommitAbortsResponse(t *testing.T) {
	t.Parallel()
	handler := observeRequests(discardLogger())(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, "parcial")
		panic("falha após iniciar a resposta")
	}))
	response := httptest.NewRecorder()
	defer func() {
		if cause := recover(); cause != http.ErrAbortHandler {
			t.Errorf("abort = %v; esperado http.ErrAbortHandler", cause)
		}
		if response.Code != 200 || response.Body.String() != "parcial" {
			t.Errorf("resposta escrita antes do panic foi alterada: %d %s", response.Code, response.Body)
		}
	}()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/", nil))
}

func TestCORS(t *testing.T) {
	t.Parallel()
	const origin = "http://localhost:8081"
	router := NewRouter(func(context.Context) error { return nil }, discardLogger(), []string{origin})
	for _, tc := range []struct {
		name, method, path, origin, preflightMethod, preflightHeaders string
		status                                                        int
		authorized                                                    bool
	}{
		{"permitida", "GET", livePath, origin, "", "", 200, true},
		{"permitida em erro", "GET", "/inexistente", origin, "", "", 404, true},
		{"recusada", "GET", livePath, origin + ".example", "", "", 200, false},
		{"sem origem", "GET", livePath, "", "", "", 200, false},
		{"preflight permitido", "OPTIONS", livePath, origin, "GET", "content-tYPE", 204, true},
		{"preflight sem cabeçalhos", "OPTIONS", readyPath, origin, "GET", "", 204, true},
		{"preflight de origem recusada", "OPTIONS", livePath, "https://example.com", "GET", "Content-Type", 405, false},
		{"preflight com método recusado", "OPTIONS", livePath, origin, "POST", "", 405, false},
		{"preflight com cabeçalho recusado", "OPTIONS", livePath, origin, "GET", "Content-Type, Authorization", 405, false},
		{"preflight em rota inexistente", "OPTIONS", "/inexistente", origin, "GET", "", 404, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			request := httptest.NewRequest(tc.method, tc.path, nil)
			if tc.origin != "" {
				request.Header.Set("Origin", tc.origin)
			}
			if tc.preflightMethod != "" {
				request.Header.Set("Access-Control-Request-Method", tc.preflightMethod)
			}
			if tc.preflightHeaders != "" {
				request.Header.Set("Access-Control-Request-Headers", tc.preflightHeaders)
			}
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)
			if response.Code != tc.status {
				t.Errorf("status = %d; esperado %d", response.Code, tc.status)
			}
			header := response.Header()
			wantedOrigin := ""
			if tc.authorized {
				wantedOrigin = tc.origin
			}
			if header.Get("Access-Control-Allow-Origin") != wantedOrigin || header.Get("Access-Control-Allow-Credentials") != "" {
				t.Errorf("cabeçalhos de autorização CORS: %v", header)
			}
			if !strings.Contains(strings.Join(header.Values("Vary"), ","), "Origin") ||
				header.Get("X-Request-ID") == "" {
				t.Errorf("cabeçalhos gerais: %v", header)
			}
			if tc.status == 204 {
				if header.Get("Access-Control-Allow-Methods") != "GET" ||
					header.Get("Access-Control-Allow-Headers") != "Content-Type" ||
					!strings.Contains(strings.Join(header.Values("Vary"), ","), "Access-Control-Request-Headers") ||
					response.Body.Len() != 0 {
					t.Errorf("resposta ao preflight: %v %s", header, response.Body)
				}
			} else {
				wantedExpose := ""
				if tc.authorized {
					wantedExpose = "X-Request-ID"
				}
				if header.Get("Access-Control-Expose-Headers") != wantedExpose {
					t.Errorf("cabeçalhos expostos: %v", header)
				}
			}
		})
	}
}

func assertHealthy(t *testing.T, response *httptest.ResponseRecorder) {
	t.Helper()
	if response.Code != 200 || response.Header().Get("Content-Type") != "application/json" ||
		response.Header().Get("Cache-Control") != "no-store" || strings.TrimSpace(response.Body.String()) != `{"status":"ok"}` {
		t.Fatalf("resposta de saúde inválida: %d %v %s", response.Code, response.Header(), response.Body)
	}
}

func assertProblem(t *testing.T, response *httptest.ResponseRecorder, status int, code, suffix string) {
	t.Helper()
	if response.Code != status || response.Header().Get("Content-Type") != "application/problem+json" {
		t.Fatalf("resposta de erro inválida: %d %v %s", response.Code, response.Header(), response.Body)
	}
	var body map[string]any
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	id := response.Header().Get("X-Request-ID")
	title, _ := body["title"].(string)
	detail, _ := body["detail"].(string)
	if id == "" || len(body) != 7 || title == "" || detail == "" ||
		body["status"] != float64(status) || body["code"] != code ||
		body["type"] != "urn:outguessed:problem:"+suffix ||
		body["requestId"] != id || body["instance"] != "urn:outguessed:request:"+id {
		t.Errorf("Problem Details inválido: %s", response.Body)
	}
}

func assertLog(t *testing.T, logs string, expected map[string]any) {
	t.Helper()
	decoder := json.NewDecoder(strings.NewReader(logs))
	for decoder.More() {
		var log map[string]any
		if err := decoder.Decode(&log); err != nil {
			t.Fatal(err)
		}
		matches := true
		for key, value := range expected {
			if log[key] != value {
				matches = false
				break
			}
		}
		if matches {
			if log["msg"] == "Requisição HTTP" {
				if duration, ok := log["duration_ms"].(float64); !ok || duration < 0 {
					t.Errorf("duração ausente ou inválida: %v", log)
				}
			}
			return
		}
	}
	t.Errorf("log esperado não encontrado: %v", expected)
}

func discardLogger() *slog.Logger {
	return slog.New(slog.NewJSONHandler(io.Discard, nil))
}
