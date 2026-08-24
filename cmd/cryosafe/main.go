package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	cfg, err := loadConfig()
	if err != nil {
		slog.Error("configuration failed", "error", err)
		os.Exit(1)
	}
	runtime, err := buildRuntime(cfg, time.Now)
	if err != nil {
		slog.Error("runtime initialization failed", "error", err)
		os.Exit(1)
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	slog.Info("CryoSafe listening", "address", cfg.Address, "data_dir", cfg.DataDir)
	if err := runtime.run(ctx); err != nil {
		slog.Error("CryoSafe stopped with an error", "error", err)
		os.Exit(1)
	}
}
