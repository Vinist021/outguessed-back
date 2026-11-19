// Package config centraliza a configuração da aplicação, sem alterar o ambiente do processo.
package config

import (
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
	"github.com/redis/go-redis/v9"
)

type Config struct {
	Env                string
	HTTPAddr           string
	Postgres           *pgxpool.Config
	Redis              *redis.Options
	CORSAllowedOrigins []string
	LogLevel           slog.Level
	StartupTimeout     time.Duration
	DependencyTimeout  time.Duration
	ShutdownTimeout    time.Duration
}

// Error identifica o campo inválido, mantendo o valor e a causa fora da mensagem pública.
type Error struct {
	Variable string
	Reason   string
	Err      error
}

func (e *Error) Error() string { return "configuração inválida: " + e.Variable + ": " + e.Reason }
func (e *Error) Unwrap() error { return e.Err }

func Load() (Config, error) { return load(os.LookupEnv, ".env") }

func load(lookup func(string) (string, bool), filename string) (Config, error) {
	cfg := Config{Env: "development"}
	if value, ok := lookup("APP_ENV"); ok {
		cfg.Env = value
	}
	if cfg.Env != "development" && cfg.Env != "test" && cfg.Env != "production" {
		return Config{}, &Error{Variable: "APP_ENV", Reason: "ambiente desconhecido"}
	}

	var local map[string]string
	if cfg.Env == "development" {
		var err error
		local, err = godotenv.Read(filename)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return Config{}, &Error{Variable: ".env", Reason: "não foi possível ler o arquivo de configuração", Err: err}
		}
	}
	value := func(key, fallback string) string {
		if v, ok := lookup(key); ok {
			return v
		}
		if v, ok := local[key]; ok {
			return v
		}
		return fallback
	}

	cfg.HTTPAddr = value("HTTP_ADDR", "127.0.0.1:8080")
	_, port, err := net.SplitHostPort(cfg.HTTPAddr)
	portNumber, portErr := strconv.Atoi(port)
	addressURL, addressErr := url.Parse("http://" + cfg.HTTPAddr)
	if err != nil || portErr != nil || portNumber < 0 || portNumber > 65535 || addressErr != nil || addressURL.Host != cfg.HTTPAddr || addressURL.User != nil {
		return Config{}, &Error{Variable: "HTTP_ADDR", Reason: "endereço deve conter host e porta numérica válidos"}
	}

	level := value("LOG_LEVEL", "info")
	switch strings.ToLower(level) {
	case "debug", "info", "warn", "error":
		err = cfg.LogLevel.UnmarshalText([]byte(level))
	default:
		err = errors.New("nível desconhecido")
	}
	if err != nil {
		return Config{}, &Error{Variable: "LOG_LEVEL", Reason: "nível de log desconhecido", Err: err}
	}

	for _, setting := range []struct {
		key      string
		fallback string
		target   *time.Duration
	}{
		{"STARTUP_TIMEOUT", "10s", &cfg.StartupTimeout},
		{"DEPENDENCY_TIMEOUT", "2s", &cfg.DependencyTimeout},
		{"SHUTDOWN_TIMEOUT", "10s", &cfg.ShutdownTimeout},
	} {
		duration, err := time.ParseDuration(value(setting.key, setting.fallback))
		if err != nil || duration <= 0 {
			return Config{}, &Error{Variable: setting.key, Reason: "duração deve ser positiva", Err: err}
		}
		*setting.target = duration
	}

	databaseURL := value("DATABASE_URL", "")
	if !validURL(databaseURL, "postgres", "postgresql") {
		return Config{}, &Error{Variable: "DATABASE_URL", Reason: "URL PostgreSQL obrigatória e válida"}
	}
	cfg.Postgres, err = pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return Config{}, &Error{Variable: "DATABASE_URL", Reason: "URL PostgreSQL inválida", Err: err}
	}
	if cfg.Postgres.HealthCheckPeriod <= 0 || cfg.Postgres.MinConns < 0 || cfg.Postgres.MinIdleConns < 0 || cfg.Postgres.MinConns > cfg.Postgres.MaxConns || cfg.Postgres.MinIdleConns > cfg.Postgres.MaxConns {
		return Config{}, &Error{Variable: "DATABASE_URL", Reason: "parâmetros do pool PostgreSQL inválidos"}
	}
	cfg.Postgres.ConnConfig.ConnectTimeout = cfg.DependencyTimeout

	redisURL := value("REDIS_URL", "")
	if !validURL(redisURL, "redis", "rediss") {
		return Config{}, &Error{Variable: "REDIS_URL", Reason: "URL Redis obrigatória e válida"}
	}
	cfg.Redis, err = redis.ParseURL(redisURL)
	if err != nil {
		return Config{}, &Error{Variable: "REDIS_URL", Reason: "URL Redis inválida", Err: err}
	}
	if cfg.Redis.DB < 0 || cfg.Redis.PoolSize < 0 || cfg.Redis.MinIdleConns < 0 || cfg.Redis.MaxIdleConns < 0 || cfg.Redis.MaxActiveConns < 0 || cfg.Redis.ReadBufferSize < 0 || cfg.Redis.WriteBufferSize < 0 || (cfg.Redis.Protocol != 0 && cfg.Redis.Protocol != 2 && cfg.Redis.Protocol != 3) {
		return Config{}, &Error{Variable: "REDIS_URL", Reason: "parâmetros do cliente Redis inválidos"}
	}
	cfg.Redis.ContextTimeoutEnabled = true
	cfg.Redis.DialTimeout = cfg.DependencyTimeout
	cfg.Redis.ReadTimeout = cfg.DependencyTimeout
	cfg.Redis.WriteTimeout = cfg.DependencyTimeout
	cfg.Redis.PoolTimeout = cfg.DependencyTimeout
	cfg.Redis.MaxRetries = -1 // As tentativas de inicialização pertencem à aplicação.
	cfg.Redis.DialerRetries = 1

	if origins := value("CORS_ALLOWED_ORIGINS", ""); origins != "" {
		for _, item := range strings.Split(origins, ",") {
			origin := strings.TrimSpace(item)
			u, err := url.Parse(origin)
			if err != nil || !validURL(origin, "http", "https") || u.User != nil || u.Path != "" || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" || strings.Contains(origin, "*") {
				return Config{}, &Error{Variable: "CORS_ALLOWED_ORIGINS", Reason: "informe origens HTTP explícitas, sem caminho ou credenciais"}
			}
			cfg.CORSAllowedOrigins = append(cfg.CORSAllowedOrigins, origin)
		}
	}
	return cfg, nil
}

func validURL(raw string, schemes ...string) bool {
	u, err := url.Parse(raw)
	if err != nil || u.Hostname() == "" || u.Fragment != "" {
		return false
	}
	if port := u.Port(); port != "" {
		n, err := strconv.Atoi(port)
		if err != nil || n < 1 || n > 65535 {
			return false
		}
	}
	for _, scheme := range schemes {
		if u.Scheme == scheme {
			return true
		}
	}
	return false
}

// String evita que a impressão acidental da configuração revele credenciais.
func (c Config) String() string {
	return fmt.Sprintf("env=%s http_addr=%s", c.Env, c.HTTPAddr)
}
