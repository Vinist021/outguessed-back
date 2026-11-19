//go:build integration

package app

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"github.com/Vinist021/outguessed-back/internal/config"
	"github.com/Vinist021/outguessed-back/internal/health"
	"github.com/Vinist021/outguessed-back/internal/postgres/dbgen"
)

func integrationConfig(t *testing.T) config.Config {
	t.Helper()
	for _, pair := range [][2]string{{"TEST_DATABASE_URL", "DATABASE_URL"}, {"TEST_REDIS_URL", "REDIS_URL"}} {
		value := os.Getenv(pair[0])
		if value == "" {
			t.Fatalf("%s obrigatória; use os serviços descartáveis do perfil test", pair[0])
		}
		t.Setenv(pair[1], value)
	}
	t.Setenv("APP_ENV", "test")
	t.Setenv("HTTP_ADDR", freeAddress(t))
	t.Setenv("STARTUP_TIMEOUT", "3s")
	t.Setenv("DEPENDENCY_TIMEOUT", "1s")
	t.Setenv("SHUTDOWN_TIMEOUT", "1s")
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Postgres.ConnConfig.Database != "outguessed_test" {
		t.Fatal("a integração exige o banco descartável outguessed_test")
	}
	return cfg
}

func freeAddress(t *testing.T) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	address := listener.Addr().String()
	listener.Close()
	return address
}

// Os cenários alteram o mesmo schema descartável e por isso são sequenciais.
func TestFoundationIntegration(t *testing.T) {
	cfg := integrationConfig(t)
	ctx, cancel := context.WithTimeout(t.Context(), 20*time.Second)
	defer cancel()
	pool, err := pgxpool.NewWithConfig(ctx, cfg.Postgres)
	if err != nil {
		t.Fatal("criar pool de teste")
	}
	defer pool.Close()
	queries := dbgen.New(pool)
	if ready, err := queries.CheckSchema(ctx); err != nil || !ready {
		t.Fatal("schema indisponível; inicie os serviços de teste e execute migrate-test up")
	}
	redisClient := redis.NewClient(cfg.Redis)
	defer redisClient.Close()
	if err := redisClient.Ping(ctx).Err(); err != nil {
		t.Fatal("Redis de teste indisponível")
	}
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))

	t.Run("generated query in both schema states", func(t *testing.T) {
		tx, err := pool.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer tx.Rollback(ctx)
		if _, err := tx.Exec(ctx, "DROP SCHEMA outguessed"); err != nil {
			t.Fatal(err)
		}
		if ready, err := queries.WithTx(tx).CheckSchema(ctx); err != nil || ready {
			t.Fatalf("schema ausente: ready=%v erro=%v", ready, err)
		}
		if _, err := tx.Exec(ctx, "CREATE SCHEMA outguessed"); err != nil {
			t.Fatal(err)
		}
		if ready, err := queries.WithTx(tx).CheckSchema(ctx); err != nil || !ready {
			t.Fatalf("schema presente: ready=%v erro=%v", ready, err)
		}
	})

	t.Run("rollback protects dependent objects", func(t *testing.T) {
		tx, err := pool.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer tx.Rollback(ctx)
		if _, err := tx.Exec(ctx, "CREATE TABLE outguessed.rollback_guard (id integer)"); err != nil {
			t.Fatal(err)
		}
		sql, err := os.ReadFile("../../db/migrations/000001_create_outguessed_schema.down.sql")
		if err != nil {
			t.Fatal(err)
		}
		_, err = tx.Exec(ctx, string(sql))
		var pgErr *pgconn.PgError
		if !errors.As(err, &pgErr) || pgErr.Code != "2BP01" {
			t.Fatalf("rollback deveria preservar objetos dependentes: %v", err)
		}
	})

	t.Run("healthy startup and cleanup", func(t *testing.T) {
		cfg := integrationConfig(t)
		cfg.Postgres.ConnConfig.RuntimeParams["application_name"] = "outguessed_stage03_cleanup"
		ctx, cancel := context.WithCancel(ctx)
		defer cancel()
		finished := make(chan error, 1)
		go func() { finished <- Run(ctx, cfg, logger); close(finished) }()
		t.Cleanup(func() {
			cancel()
			select {
			case <-finished:
			case <-time.After(2 * time.Second):
				t.Error("API não terminou durante a limpeza")
			}
		})
		transport := &http.Transport{}
		defer transport.CloseIdleConnections()
		client := &http.Client{Timeout: time.Second, Transport: transport}
		deadline := time.NewTimer(3 * time.Second)
		defer deadline.Stop()
		ticker := time.NewTicker(10 * time.Millisecond)
		defer ticker.Stop()
		for {
			response, err := client.Get("http://" + cfg.HTTPAddr + "/unconfigured")
			if err == nil {
				response.Body.Close()
				if response.StatusCode != http.StatusNotFound {
					t.Fatalf("status = %d", response.StatusCode)
				}
				break
			}
			select {
			case err := <-finished:
				t.Fatalf("API terminou antes de aceitar HTTP: %v", err)
			case <-deadline.C:
				t.Fatal("API não iniciou")
			case <-ticker.C:
			}
		}
		cancel()
		if err := <-finished; err != nil {
			t.Fatal(err)
		}
		var connections int
		if err := pool.QueryRow(t.Context(), "SELECT count(*) FROM pg_stat_activity WHERE application_name = $1", "outguessed_stage03_cleanup").Scan(&connections); err != nil || connections != 0 {
			t.Fatalf("conexões não liberadas: quantidade=%d erro=%v", connections, err)
		}
	})

	t.Run("database unavailable prevents HTTP", func(t *testing.T) {
		cfg := integrationConfig(t)
		cfg.Postgres.ConnConfig.Host = "127.0.0.1"
		cfg.Postgres.ConnConfig.Port = portNumber(t, freeAddress(t))
		assertUnavailable(t, cfg)
	})
	t.Run("Redis unavailable prevents HTTP", func(t *testing.T) {
		cfg := integrationConfig(t)
		cfg.Redis.Addr = freeAddress(t)
		assertUnavailable(t, cfg)
	})
	t.Run("missing schema prevents HTTP and readiness recovers", func(t *testing.T) {
		service := health.New(queries, redisPinger{redisClient}, time.Second, logger)
		if err := service.Check(ctx); err != nil {
			t.Fatal(err)
		}
		if _, err := pool.Exec(ctx, "DROP SCHEMA outguessed"); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			cleanupCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			if _, err := pool.Exec(cleanupCtx, "CREATE SCHEMA IF NOT EXISTS outguessed"); err != nil {
				t.Error("restaurar schema descartável")
			}
		})
		if err := service.Check(ctx); !errors.Is(err, health.ErrSchemaMissing) {
			t.Fatalf("schema ausente: %v", err)
		}
		assertUnavailable(t, integrationConfig(t))
		if _, err := pool.Exec(ctx, "CREATE SCHEMA outguessed"); err != nil {
			t.Fatal(err)
		}
		if err := service.Check(ctx); err != nil {
			t.Fatalf("prontidão não recuperou com os mesmos clientes: %v", err)
		}
	})
}

func portNumber(t *testing.T, address string) uint16 {
	t.Helper()
	tcp, err := net.ResolveTCPAddr("tcp", address)
	if err != nil {
		t.Fatal(err)
	}
	return uint16(tcp.Port)
}

func assertUnavailable(t *testing.T, cfg config.Config) {
	t.Helper()
	cfg.StartupTimeout = 250 * time.Millisecond
	cfg.DependencyTimeout = 100 * time.Millisecond
	cfg.Postgres.ConnConfig.ConnectTimeout = cfg.DependencyTimeout
	cfg.Redis.DialTimeout = cfg.DependencyTimeout
	cfg.Redis.ReadTimeout = cfg.DependencyTimeout
	cfg.Redis.WriteTimeout = cfg.DependencyTimeout
	var logs bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logs, nil))
	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()
	finished := make(chan error, 1)
	go func() { finished <- Run(ctx, cfg, logger); close(finished) }()
	t.Cleanup(func() {
		cancel()
		select {
		case <-finished:
		case <-time.After(time.Second):
			t.Error("inicialização não terminou durante a limpeza")
		}
	})
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case err := <-finished:
			if !errors.Is(err, context.DeadlineExceeded) {
				t.Fatalf("esperado prazo global de inicialização: %v", err)
			}
			if strings.Contains(logs.String(), "postgres://") || strings.Contains(logs.String(), "redis://") || strings.Contains(logs.String(), "Servidor HTTP iniciado") {
				t.Fatal("logs expuseram URL ou HTTP iniciou antes das dependências")
			}
			return
		case <-ctx.Done():
			t.Fatal("inicialização não terminou dentro do limite")
		case <-ticker.C:
			connection, err := net.DialTimeout("tcp", cfg.HTTPAddr, 20*time.Millisecond)
			if err == nil {
				connection.Close()
				t.Fatal("API aceitou conexão antes das dependências")
			}
		}
	}
}
