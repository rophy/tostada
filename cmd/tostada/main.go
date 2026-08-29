package main

import (
	"context"
	"flag"
	"io/fs"
	"log"
	"net/http"
	"time"

	"github.com/rophy/tostada/internal/api"
	"github.com/rophy/tostada/internal/auth"
	"github.com/rophy/tostada/internal/config"
	"github.com/rophy/tostada/internal/device"
	"github.com/rophy/tostada/internal/hub"
	"github.com/rophy/tostada/web"
)

func main() {
	configPath := flag.String("config", "config.yaml", "path to config file")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Retry OIDC discovery — sidecar proxy may not be ready yet
	var authProvider *auth.Auth
	for i := 0; i < 30; i++ {
		authProvider, err = auth.NewAuth(
			context.Background(),
			cfg.OIDC.IssuerURL,
			cfg.OIDC.InternalURL,
			cfg.OIDC.ClientID,
			cfg.OIDC.ClientSecret,
			cfg.OIDC.RedirectURL,
		)
		if err == nil {
			break
		}
		log.Printf("OIDC discovery attempt %d/30 failed: %v", i+1, err)
		time.Sleep(2 * time.Second)
	}
	if err != nil {
		log.Fatalf("Failed to initialize auth after retries: %v", err)
	}

	hubClient := hub.NewClient(cfg.JupyterHub.APIURL, cfg.JupyterHub.APIToken)

	dbPath := cfg.Database.Path
	if dbPath == "" {
		dbPath = "tostada.db"
	}
	deviceStore, err := device.NewGormStore(dbPath)
	if err != nil {
		log.Fatalf("Failed to initialize device store: %v", err)
	}

	mux := api.NewRouter(cfg, hubClient, authProvider, deviceStore)

	distFS, err := fs.Sub(web.DistFS, "dist")
	if err != nil {
		log.Fatalf("Failed to create sub FS: %v", err)
	}
	mux.Handle("/", http.FileServer(http.FS(distFS)))

	log.Printf("Tostada listening on %s", cfg.Server.Addr)
	log.Fatal(http.ListenAndServe(cfg.Server.Addr, mux))
}
