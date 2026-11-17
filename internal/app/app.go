package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/Vinist021/outguessed-back/internal/httpapi"
)

const shutdownTimeout = 10 * time.Second

func Run(ctx context.Context, addr string, logger *slog.Logger) error {
	if ctx.Err() != nil {
		return nil
	}

	var listenConfig net.ListenConfig
	listener, err := listenConfig.Listen(ctx, "tcp", addr)
	if err != nil {
		if ctx.Err() != nil && errors.Is(err, ctx.Err()) {
			return nil
		}
		return fmt.Errorf("abrir endereço HTTP: %w", err)
	}

	server := &http.Server{
		Addr:              addr,
		Handler:           httpapi.NewRouter(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
		ErrorLog:          slog.NewLogLogger(logger.Handler(), slog.LevelError),
	}

	return serve(ctx, server, listener, logger)
}

func serve(ctx context.Context, server *http.Server, listener net.Listener, logger *slog.Logger) error {
	defer server.Close()

	serveErrors := make(chan error, 1)
	go func() {
		serveErrors <- server.Serve(listener)
	}()

	logger.InfoContext(ctx, "Servidor HTTP iniciado", "addr", listener.Addr().String())

	select {
	case err := <-serveErrors:
		return fmt.Errorf("executar servidor HTTP: %w", err)
	case <-ctx.Done():
	}

	shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), shutdownTimeout)
	defer cancel()
	logger.InfoContext(shutdownCtx, "Encerrando servidor HTTP")

	shutdownErr := server.Shutdown(shutdownCtx)
	if shutdownErr != nil {
		shutdownErr = errors.Join(shutdownErr, server.Close())
	}

	serveErr := <-serveErrors
	if errors.Is(serveErr, http.ErrServerClosed) {
		serveErr = nil
	} else if serveErr != nil {
		serveErr = fmt.Errorf("executar servidor HTTP: %w", serveErr)
	}
	if shutdownErr != nil {
		return errors.Join(fmt.Errorf("encerrar servidor HTTP: %w", shutdownErr), serveErr)
	}
	if serveErr != nil {
		return serveErr
	}

	logger.InfoContext(shutdownCtx, "Servidor HTTP encerrado")
	return nil
}
