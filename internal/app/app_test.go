package app

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Vinist021/outguessed-back/internal/config"
	"github.com/Vinist021/outguessed-back/internal/health"
	"github.com/Vinist021/outguessed-back/internal/httpapi"
)

func TestRunAddressInUse(t *testing.T) {
	t.Parallel()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reservar endereço: %v", err)
	}
	t.Cleanup(func() { listener.Close() })

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	err = runHTTP(ctx, config.Config{HTTPAddr: listener.Addr().String()}, testReadiness(), logger)
	var networkError *net.OpError
	if !errors.As(err, &networkError) || networkError.Op != "listen" {
		t.Fatalf("erro recebido = %v; esperada falha de abertura do endereço", err)
	}
}

func TestServeCancellationClosesListener(t *testing.T) {
	t.Parallel()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("abrir endereço de teste: %v", err)
	}
	t.Cleanup(func() { listener.Close() })

	ctx, cancel := context.WithCancel(context.Background())
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	readiness := testReadiness()
	router := httpapi.NewRouter(readiness.Check, logger, nil)
	server := &http.Server{Handler: router}
	finished := make(chan struct{})
	var runErr error
	go func() {
		runErr = serve(ctx, server, listener, readiness, time.Second, logger, router)
		close(finished)
	}()
	t.Cleanup(func() {
		cancel()
		server.Close()
		select {
		case <-finished:
		case <-time.After(5 * time.Second):
			t.Error("a execução do servidor não terminou durante a limpeza")
		}
	})

	transport := &http.Transport{}
	t.Cleanup(transport.CloseIdleConnections)
	client := &http.Client{Timeout: 2 * time.Second, Transport: transport}
	response, err := client.Get("http://" + listener.Addr().String() + "/caminho-inexistente")
	if err != nil {
		t.Fatalf("consultar servidor antes do cancelamento: %v", err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusNotFound {
		t.Fatalf("status recebido = %d; esperado = %d", response.StatusCode, http.StatusNotFound)
	}

	cancel()
	select {
	case <-finished:
		if runErr != nil {
			t.Fatalf("encerramento por cancelamento: %v", runErr)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("o servidor não encerrou após o cancelamento")
	}

	connection, err := net.DialTimeout("tcp", listener.Addr().String(), time.Second)
	if err == nil {
		connection.Close()
		t.Fatal("o servidor ainda aceita conexões após o encerramento")
	}
}

type readySchema struct{}

func (readySchema) CheckSchema(context.Context) (bool, error) { return true, nil }

type readyRedis struct{}

func (readyRedis) Ping(context.Context) error { return nil }

func testReadiness() *health.Service {
	return health.New(readySchema{}, readyRedis{}, time.Second, slog.New(slog.NewJSONHandler(io.Discard, nil)))
}

func TestRunAlreadyCancelledDoesNotOpenClients(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := Run(ctx, config.Config{}, slog.Default()); err != nil {
		t.Fatal(err)
	}
}

func TestServeDrainsActiveRequests(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	entered, release, draining := make(chan struct{}), make(chan struct{}), make(chan struct{})
	server := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(entered)
		select {
		case <-release:
			w.WriteHeader(http.StatusNoContent)
		case <-r.Context().Done():
			t.Error("requisição cancelada antes de esgotar o prazo de encerramento")
		}
	})}
	server.RegisterOnShutdown(func() { close(draining) })
	readiness := testReadiness()
	finished := make(chan error, 1)
	go func() {
		finished <- serve(ctx, server, listener, readiness, time.Second, slog.Default(), http.NotFoundHandler())
	}()
	t.Cleanup(func() { cancel(); server.Close() })
	response := make(chan error, 1)
	go func() {
		client := &http.Client{Timeout: 2 * time.Second}
		res, err := client.Get("http://" + listener.Addr().String())
		if err == nil {
			res.Body.Close()
			if res.StatusCode != http.StatusNoContent {
				err = errors.New("requisição ativa não foi concluída")
			}
		}
		response <- err
	}()
	<-entered
	cancel()
	<-draining
	if err := readiness.Check(context.Background()); !errors.Is(err, health.ErrStopping) {
		t.Errorf("prontidão durante encerramento: %v", err)
	}
	close(release)
	if err := <-response; err != nil {
		t.Fatal(err)
	}
	if err := <-finished; err != nil {
		t.Fatal(err)
	}
}

func TestServeCancelsAndWaitsForRequestsAfterDeadline(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	entered, exited := make(chan struct{}), make(chan struct{})
	server := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(entered)
		<-r.Context().Done()
		close(exited)
	})}
	finished := make(chan error, 1)
	go func() {
		finished <- serve(ctx, server, listener, testReadiness(), 20*time.Millisecond, slog.Default(), http.NotFoundHandler())
	}()
	t.Cleanup(func() { cancel(); server.Close() })
	response := make(chan struct{})
	go func() {
		defer close(response)
		client := &http.Client{Timeout: 2 * time.Second}
		res, err := client.Get("http://" + listener.Addr().String())
		if err == nil {
			res.Body.Close()
		}
	}()
	<-entered
	cancel()
	select {
	case err := <-finished:
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("prazo de encerramento: %v", err)
		}
		select {
		case <-exited:
		default:
			t.Fatal("servidor retornou enquanto o handler ainda executava")
		}
	case <-time.After(time.Second):
		t.Fatal("servidor não encerrou dentro do limite")
	}
	<-response
}

func TestShutdownGatePreservesHealthContracts(t *testing.T) {
	readiness := testReadiness()
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	router := httpapi.NewRouter(readiness.Check, logger, []string{"http://localhost:8081"})
	requests := &activeRequests{
		next: http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
			t.Error("requisição admitida após o início do encerramento")
		}),
		duringShutdown: router,
	}
	readiness.Stop()
	requests.stop()
	for _, tc := range []struct {
		path string
		code int
		body string
	}{
		{"/health/live", http.StatusOK, `"status":"ok"`},
		{"/health/ready", http.StatusServiceUnavailable, `"code":"SERVICE_UNAVAILABLE"`},
	} {
		request := httptest.NewRequest(http.MethodGet, tc.path, nil)
		request.Header.Set("Origin", "http://localhost:8081")
		response := httptest.NewRecorder()
		requests.ServeHTTP(response, request)
		if response.Code != tc.code || !strings.Contains(response.Body.String(), tc.body) {
			t.Errorf("%s respondeu %d %s", tc.path, response.Code, response.Body)
		}
		if response.Header().Get("X-Request-ID") == "" ||
			response.Header().Get("Cache-Control") != "no-store" ||
			response.Header().Get("Access-Control-Allow-Origin") != "http://localhost:8081" {
			t.Errorf("cabeçalhos de %s: %v", tc.path, response.Header())
		}
	}
}
