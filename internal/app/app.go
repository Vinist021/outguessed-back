package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"github.com/Vinist021/outguessed-back/internal/config"
	"github.com/Vinist021/outguessed-back/internal/health"
	"github.com/Vinist021/outguessed-back/internal/httpapi"
	"github.com/Vinist021/outguessed-back/internal/postgres/dbgen"
)

func Run(ctx context.Context, cfg config.Config, logger *slog.Logger) (runErr error) {
	if ctx.Err() != nil {
		return nil
	}
	logger.InfoContext(ctx, "Inicializando aplicação", "env", cfg.Env)
	startupCtx, cancelStartup := context.WithTimeout(ctx, cfg.StartupTimeout)
	defer cancelStartup()
	pool, err := pgxpool.NewWithConfig(startupCtx, cfg.Postgres)
	if err != nil {
		return &health.Error{Dependency: "PostgreSQL", Err: err}
	}
	defer pool.Close()
	redisOptions := *cfg.Redis // O driver normaliza opções; preserve a configuração recebida.
	client := redis.NewClient(&redisOptions)
	defer func() {
		if err := client.Close(); err != nil {
			runErr = errors.Join(runErr, &health.Error{Dependency: "Redis", Err: err})
		}
	}()
	readiness := health.New(dbgen.New(pool), redisPinger{client}, cfg.DependencyTimeout, logger)
	defer readiness.Stop()
	if err := readiness.Wait(startupCtx); err != nil {
		if ctx.Err() != nil {
			return nil
		}
		return err
	}
	cancelStartup()
	return runHTTP(ctx, cfg, readiness, logger)
}

type redisPinger struct{ client *redis.Client }

func (p redisPinger) Ping(ctx context.Context) error { return p.client.Ping(ctx).Err() }

func runHTTP(ctx context.Context, cfg config.Config, readiness *health.Service, logger *slog.Logger) error {
	var listenConfig net.ListenConfig
	listener, err := listenConfig.Listen(ctx, "tcp", cfg.HTTPAddr)
	if err != nil {
		if ctx.Err() != nil && errors.Is(err, ctx.Err()) {
			return nil
		}
		return fmt.Errorf("abrir endereço HTTP: %w", err)
	}

	server := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           httpapi.NewRouter(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
		ErrorLog:          slog.NewLogLogger(logger.Handler(), slog.LevelError),
	}

	return serve(ctx, server, listener, readiness, cfg.ShutdownTimeout, logger)
}

func serve(ctx context.Context, server *http.Server, listener net.Listener, readiness *health.Service, shutdownTimeout time.Duration, logger *slog.Logger) error {
	workCtx, cancelWork := context.WithCancel(context.WithoutCancel(ctx))
	defer cancelWork()
	server.BaseContext = func(net.Listener) context.Context { return workCtx }
	requests := &activeRequests{next: server.Handler}
	server.Handler = requests
	defer server.Close()

	serveErrors := make(chan error, 1)
	go func() {
		serveErrors <- server.Serve(listener)
	}()

	logger.InfoContext(ctx, "Servidor HTTP iniciado", "addr", listener.Addr().String())

	select {
	case err := <-serveErrors:
		readiness.Stop()
		requests.stop()
		cancelWork()
		server.Close()
		requests.wait()
		return fmt.Errorf("executar servidor HTTP: %w", err)
	case <-ctx.Done():
	}
	readiness.Stop()
	requests.stop()

	shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), shutdownTimeout)
	defer cancel()
	logger.InfoContext(shutdownCtx, "Encerrando servidor HTTP")

	shutdownErr := server.Shutdown(shutdownCtx)
	if shutdownErr != nil {
		cancelWork()
		shutdownErr = errors.Join(shutdownErr, server.Close())
	}
	requests.wait()

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

// A barreira impede novos trabalhos durante Shutdown e permite aguardar handlers
// também após Server.Close, que por si só não aguarda seu término.
type activeRequests struct {
	next    http.Handler
	mu      sync.Mutex
	stopped bool
	active  sync.WaitGroup
}

func (r *activeRequests) ServeHTTP(w http.ResponseWriter, request *http.Request) {
	r.mu.Lock()
	if r.stopped {
		r.mu.Unlock()
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}
	r.active.Add(1)
	r.mu.Unlock()
	defer r.active.Done()
	r.next.ServeHTTP(w, request)
}

func (r *activeRequests) stop() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.stopped = true
}

func (r *activeRequests) wait() { r.active.Wait() }
