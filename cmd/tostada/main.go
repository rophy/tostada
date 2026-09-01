package main

import (
	"context"
	"flag"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/rophy/tostada/internal/api"
	"github.com/rophy/tostada/internal/auth"
	"github.com/rophy/tostada/internal/config"
	"github.com/rophy/tostada/internal/device"
	"github.com/rophy/tostada/internal/hub"
	"github.com/rophy/tostada/internal/model"
	"github.com/rophy/tostada/internal/telemetry"
	"github.com/rophy/tostada/web"
)

func main() {
	configPath := flag.String("config", "config.yaml", "path to config file")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	dbPath := cfg.Database.Path
	if dbPath == "" {
		dbPath = "tostada.db"
	}
	deviceStore, err := device.NewGormStore(dbPath)
	if err != nil {
		log.Fatalf("Failed to initialize device store: %v", err)
	}

	if err := deviceStore.DB().AutoMigrate(&model.User{}); err != nil {
		log.Fatalf("Failed to migrate user store: %v", err)
	}
	userStore := model.NewGormUserStore(deviceStore.DB())

	logDir := cfg.Telemetry.LogDir
	if logDir == "" {
		logDir = "/data/logs"
	}
	if err := os.MkdirAll(logDir, 0755); err != nil {
		log.Fatalf("Failed to create log dir: %v", err)
	}
	auditLog, err := telemetry.NewAuditLog(filepath.Join(logDir, "audit.jsonl"))
	if err != nil {
		log.Fatalf("Failed to initialize audit log: %v", err)
	}
	accessLogger, err := telemetry.NewAccessLogger(filepath.Join(logDir, "access.jsonl"))
	if err != nil {
		log.Fatalf("Failed to initialize access logger: %v", err)
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
			userStore,
			auditLog,
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

	mux := api.NewRouter(cfg, hubClient, authProvider, deviceStore, userStore, auditLog, accessLogger)

	distFS, err := fs.Sub(web.DistFS, "dist")
	if err != nil {
		log.Fatalf("Failed to create sub FS: %v", err)
	}
	mux.Handle("/", http.FileServer(http.FS(distFS)))

	log.Printf("Tostada listening on %s", cfg.Server.Addr)
	log.Fatal(http.ListenAndServe(cfg.Server.Addr, mux))
}
