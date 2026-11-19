package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/redis/go-redis/v9"

	"github.com/Vinist021/outguessed-back/internal/app"
	"github.com/Vinist021/outguessed-back/internal/config"
)

func main() {
	os.Exit(run())
}

func run() int {
	logger := slog.New(slog.NewJSONHandler(os.Stderr, nil))
	cfg, err := config.Load()
	if err != nil {
		logger.Error("Falha na configuração da API", "error", err)
		return 1
	}
	logger = slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: cfg.LogLevel}))
	redis.SetLogger(redisLogger{logger})
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := app.Run(ctx, cfg, logger); err != nil {
		logger.Error("Falha na execução da API", "error", err)
		return 1
	}

	return 0
}

type redisLogger struct{ logger *slog.Logger }

// Os diagnósticos brutos do driver podem conter dados fornecidos na conexão.
// A disponibilidade e sua causa sanitizada são registradas pelo serviço de saúde.
func (l redisLogger) Printf(ctx context.Context, _ string, _ ...interface{}) {
	l.logger.DebugContext(ctx, "Diagnóstico interno do cliente Redis")
}
