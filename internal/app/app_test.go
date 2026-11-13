package app

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"testing"
	"time"

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
	err = Run(ctx, listener.Addr().String(), logger)
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
	server := &http.Server{Handler: httpapi.NewRouter()}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	finished := make(chan struct{})
	var runErr error
	go func() {
		runErr = serve(ctx, server, listener, logger)
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
