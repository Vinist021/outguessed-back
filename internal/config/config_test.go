package config

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func environment() map[string]string {
	return map[string]string{
		"DATABASE_URL": "postgres://local:secret@127.0.0.1:5432/outguessed?sslmode=disable",
		"REDIS_URL":    "redis://127.0.0.1:6379/0",
	}
}

func lookup(values map[string]string) func(string) (string, bool) {
	return func(key string) (string, bool) { value, ok := values[key]; return value, ok }
}

func dotenv(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), ".env")
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestDefaultsWithoutDotenv(t *testing.T) {
	cfg, err := load(lookup(environment()), filepath.Join(t.TempDir(), "absent"))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Env != "development" || cfg.HTTPAddr != "127.0.0.1:8080" || cfg.LogLevel != slog.LevelInfo || cfg.StartupTimeout != 10*time.Second || cfg.DependencyTimeout != 2*time.Second || cfg.ShutdownTimeout != 10*time.Second {
		t.Fatalf("padrões inesperados: %s", cfg)
	}
	if cfg.Postgres.ConnConfig.ConnectTimeout != cfg.DependencyTimeout || !cfg.Redis.ContextTimeoutEnabled || cfg.Redis.ReadTimeout != cfg.DependencyTimeout || cfg.Redis.MaxRetries != -1 {
		t.Fatal("os drivers não receberam os limites de I/O")
	}
	if strings.Contains(fmt.Sprint(cfg), "secret") {
		t.Fatal("impressão da configuração expôs credenciais")
	}
}

func TestDotenvPrecedenceAndFrozenMode(t *testing.T) {
	path := dotenv(t, "APP_ENV=production\nDATABASE_URL=postgres://file@localhost/local\nREDIS_URL=redis://localhost:6380/1\nHTTP_ADDR=127.0.0.1:9090\nLOG_LEVEL=debug\n")
	values := map[string]string{"DATABASE_URL": environment()["DATABASE_URL"], "HTTP_ADDR": "127.0.0.1:8088"}
	cfg, err := load(lookup(values), path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Env != "development" || cfg.HTTPAddr != values["HTTP_ADDR"] || cfg.Postgres.ConnConfig.Host != "127.0.0.1" || cfg.Redis.Addr != "localhost:6380" || cfg.LogLevel != slog.LevelDebug {
		t.Fatalf("precedência incorreta: %s", cfg)
	}
	values["APP_ENV"] = "development"
	values["REDIS_URL"] = "" // Até um valor vazio do processo prevalece sobre o arquivo.
	_, err = load(lookup(values), path)
	assertVariable(t, err, "REDIS_URL")
}

func TestTestAndProductionNeverReadDotenv(t *testing.T) {
	for _, mode := range []string{"test", "production"} {
		t.Run(mode, func(t *testing.T) {
			values := environment()
			values["APP_ENV"] = mode
			cfg, err := load(lookup(values), t.TempDir()) // Ler um diretório como dotenv produziria erro.
			if err != nil || cfg.Env != mode {
				t.Fatalf("modo %s: %v", mode, err)
			}
			delete(values, "DATABASE_URL")
			_, err = load(lookup(values), dotenv(t, "DATABASE_URL=postgres://localhost/from-file\n"))
			assertVariable(t, err, "DATABASE_URL")
		})
	}
}

func TestMalformedDotenvIsSanitized(t *testing.T) {
	_, err := load(lookup(environment()), dotenv(t, "DATABASE_URL=\"secret-without-closing-quote\n"))
	assertVariable(t, err, ".env")
	if strings.Contains(err.Error(), "secret") || errors.Unwrap(err) == nil {
		t.Fatal("erro deve preservar a causa sem expor conteúdo do arquivo")
	}
}

func TestInvalidValues(t *testing.T) {
	for index, test := range []struct{ key, value string }{
		{"APP_ENV", ""}, {"APP_ENV", "staging"},
		{"HTTP_ADDR", "localhost"}, {"HTTP_ADDR", "localhost:65536"}, {"HTTP_ADDR", "localhost:abc"},
		{"HTTP_ADDR", "local host:8080"}, {"HTTP_ADDR", "localhost/path:8080"}, {"HTTP_ADDR", "user:secret@localhost:8080"},
		{"DATABASE_URL", ""}, {"DATABASE_URL", "host=localhost"}, {"DATABASE_URL", "https://localhost/db"},
		{"DATABASE_URL", "postgres://user:secret@localhost/db?pool_max_conns=invalid"},
		{"DATABASE_URL", "postgres://localhost/db?pool_health_check_period=0s"},
		{"DATABASE_URL", "postgres://localhost/db?pool_min_conns=-1"},
		{"DATABASE_URL", "postgres://localhost/db?pool_max_conns=2&pool_min_conns=3"},
		{"REDIS_URL", ""}, {"REDIS_URL", "http://localhost"}, {"REDIS_URL", "redis://:secret@localhost/invalid"},
		{"REDIS_URL", "redis://localhost/-1"}, {"REDIS_URL", "redis://localhost?pool_size=-1"},
		{"REDIS_URL", "redis://localhost?write_buffer_size=-1"}, {"REDIS_URL", "redis://localhost?protocol=4"},
		{"STARTUP_TIMEOUT", "0s"}, {"STARTUP_TIMEOUT", "invalid"},
		{"DEPENDENCY_TIMEOUT", "-1s"}, {"SHUTDOWN_TIMEOUT", ""},
		{"LOG_LEVEL", ""}, {"LOG_LEVEL", "verbose"},
		{"CORS_ALLOWED_ORIGINS", "*"}, {"CORS_ALLOWED_ORIGINS", "https://*.example.com"},
		{"CORS_ALLOWED_ORIGINS", "http://localhost/path"}, {"CORS_ALLOWED_ORIGINS", "http://user:secret@localhost"},
		{"CORS_ALLOWED_ORIGINS", "http://localhost:5173,"}, {"CORS_ALLOWED_ORIGINS", "http://localhost:5173?"},
	} {
		t.Run(fmt.Sprintf("%s/%d", test.key, index), func(t *testing.T) {
			values := environment()
			values[test.key] = test.value
			_, err := load(lookup(values), filepath.Join(t.TempDir(), "absent"))
			assertVariable(t, err, test.key)
			if strings.Contains(err.Error(), "secret") {
				t.Fatal("erro expôs a senha")
			}
		})
	}
}

func TestRequiredValuesAndMissingDotenv(t *testing.T) {
	for _, key := range []string{"DATABASE_URL", "REDIS_URL"} {
		values := environment()
		delete(values, key)
		_, err := load(lookup(values), filepath.Join(t.TempDir(), "absent"))
		assertVariable(t, err, key)
	}
}

func TestCustomSettings(t *testing.T) {
	values := environment()
	values["APP_ENV"] = "test"
	values["CORS_ALLOWED_ORIGINS"] = "http://localhost:5173, https://example.com"
	values["LOG_LEVEL"] = "WARN"
	values["STARTUP_TIMEOUT"] = "3s"
	values["DEPENDENCY_TIMEOUT"] = "250ms"
	values["SHUTDOWN_TIMEOUT"] = "1s"
	cfg, err := load(lookup(values), "")
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.CORSAllowedOrigins) != 2 || cfg.CORSAllowedOrigins[1] != "https://example.com" || cfg.LogLevel != slog.LevelWarn || cfg.StartupTimeout != 3*time.Second || cfg.DependencyTimeout != 250*time.Millisecond || cfg.ShutdownTimeout != time.Second {
		t.Fatal("configuração personalizada não foi preservada")
	}
}

func assertVariable(t *testing.T, err error, variable string) {
	t.Helper()
	var invalid *Error
	if !errors.As(err, &invalid) || invalid.Variable != variable {
		t.Fatalf("erro = %v; esperada variável %s", err, variable)
	}
}
