package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/Vinist021/outguessed-back/internal/app"
)

func main() {
	os.Exit(run())
}

func run() int {
	logger := slog.New(slog.NewJSONHandler(os.Stderr, nil))
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := app.Run(ctx, "127.0.0.1:8080", logger); err != nil {
		logger.Error("Falha na execução da API", "error", err)
		return 1
	}

	return 0
}
