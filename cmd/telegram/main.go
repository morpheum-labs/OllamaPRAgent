package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/morpheum-labs/OllamaPRAgent/internal/telegram"
)

func main() {
	cfg, err := telegram.LoadAppConfig()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	store, err := telegram.OpenSQLite(cfg.DBPath)
	if err != nil {
		log.Fatalf("database: %v", err)
	}
	defer store.Close()

	svc, err := telegram.NewService(cfg, store, nil)
	if err != nil {
		log.Fatalf("telegram: %v", err)
	}
	defer svc.Close()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := svc.Run(ctx); err != nil && err != context.Canceled {
		log.Fatalf("run: %v", err)
	}
}
