package main

import (
	"context"
	"flag"
	"io/fs"
	"log"
	"net/http"

	"github.com/rophy/tostada/internal/api"
	"github.com/rophy/tostada/internal/auth"
	"github.com/rophy/tostada/internal/config"
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

	authProvider, err := auth.NewAuth(
		context.Background(),
		cfg.OIDC.IssuerURL,
		cfg.OIDC.ClientID,
		cfg.OIDC.ClientSecret,
		cfg.OIDC.RedirectURL,
	)
	if err != nil {
		log.Fatalf("Failed to initialize auth: %v", err)
	}

	hubClient := hub.NewClient(cfg.JupyterHub.APIURL, cfg.JupyterHub.APIToken)

	mux := api.NewRouter(cfg, hubClient, authProvider)

	distFS, err := fs.Sub(web.DistFS, "dist")
	if err != nil {
		log.Fatalf("Failed to create sub FS: %v", err)
	}
	mux.Handle("GET /", http.FileServer(http.FS(distFS)))

	log.Printf("Tostada listening on %s", cfg.Server.Addr)
	log.Fatal(http.ListenAndServe(cfg.Server.Addr, mux))
}
