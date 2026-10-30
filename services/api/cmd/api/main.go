package main

import (
	"baseline/services/api/internal/config"
	"baseline/services/api/internal/server"
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))
	c, err := config.Load()
	if err != nil {
		slog.Error("invalid configuration", "error", err)
		os.Exit(1)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err = server.Run(ctx, c); err != nil {
		slog.Error("api stopped", "error", err)
		os.Exit(1)
	}
}
