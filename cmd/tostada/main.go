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
	"github.com/rophy/tostada/internal/audit"
	"github.com/rophy/tostada/web"
)

var registerCoverageHandler func(mux *http.ServeMux)

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

	logDir := cfg.AuditLog.LogDir
	if logDir == "" {
		logDir = "/data/logs"
	}
	if err := os.MkdirAll(logDir, 0755); err != nil {
		log.Fatalf("Failed to create log dir: %v", err)
	}
	maxSizeMB := cfg.AuditLog.MaxSizeMB
	if maxSizeMB == 0 {
		maxSizeMB = 5
	}
	maxBackups := cfg.AuditLog.MaxBackups
	if maxBackups == 0 {
		maxBackups = 3
	}
	auditLog := audit.NewAuditLog(filepath.Join(logDir, "audit.jsonl"), maxSizeMB, maxBackups)
	accessLogger := audit.NewAccessLogger(filepath.Join(logDir, "access.jsonl"), maxSizeMB, maxBackups)

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

	if registerCoverageHandler != nil {
		registerCoverageHandler(mux)
	}

	distFS, err := fs.Sub(web.DistFS, "dist")
	if err != nil {
		log.Fatalf("Failed to create sub FS: %v", err)
	}
	fileServer := http.FileServer(http.FS(distFS))
	mux.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Serve the file if it exists; otherwise fall back to index.html for SPA routing
		if r.URL.Path != "/" {
			if _, err := fs.Stat(distFS, r.URL.Path[1:]); err != nil {
				r.URL.Path = "/"
			}
		}
		fileServer.ServeHTTP(w, r)
	}))

	log.Printf("Tostada listening on %s", cfg.Server.Addr)
	log.Fatal(http.ListenAndServe(cfg.Server.Addr, mux))
}
