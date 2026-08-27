package main

import (
	"flag"
	"log"

	"github.com/rophy/tostada/internal/config"
)

func main() {
	configPath := flag.String("config", "config.yaml", "path to config file")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}
	log.Printf("Loaded %d workspace(s), listening on %s", len(cfg.Workspaces), cfg.Server.Addr)
}
