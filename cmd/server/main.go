package main

import (
	"flag"
	"log"
	"os"
	"path/filepath"

	"github.com/joho/godotenv"

	"github.com/morpheum-labs/OllamaPRAgent/internal/config"
	"github.com/morpheum-labs/OllamaPRAgent/internal/server"
	"github.com/morpheum-labs/OllamaPRAgent/internal/store"
)

func main() {
	_ = godotenv.Load()

	configPath := flag.String("c", "", "path to JSON or TOML config (environment variables override file values when set)")
	flag.Parse()

	cfg, err := config.LoadServer(*configPath)
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	if dir := filepath.Dir(cfg.DBPath); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			log.Fatalf("create db dir: %v", err)
		}
	}

	st, err := store.New(cfg.DBPath)
	if err != nil {
		log.Fatalf("store: %v", err)
	}

	srv := server.New(st, cfg, nil)
	addr := ":" + cfg.Port
	if err := srv.Listen(addr); err != nil {
		log.Fatalf("server: %v", err)
	}
}
