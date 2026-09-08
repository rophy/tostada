package main

import (
	"context"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	"github.com/rophy/tostada/internal/api"
	"github.com/rophy/tostada/internal/audit"
	"github.com/rophy/tostada/internal/auth"
	"github.com/rophy/tostada/internal/config"
	"github.com/rophy/tostada/internal/device"
	"github.com/rophy/tostada/internal/hub"
	"github.com/rophy/tostada/internal/kube"
	"github.com/rophy/tostada/internal/model"
	"github.com/rophy/tostada/web"
)

var registerCoverageHandler func(mux *http.ServeMux)

func main() {
	rootCmd := &cobra.Command{
		Use:   "tostada",
		Short: "Tostada workspace portal",
	}

	rootCmd.AddCommand(serveCmd())
	rootCmd.AddCommand(deviceCmd())
	rootCmd.AddCommand(userCmd())

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func serveCmd() *cobra.Command {
	var configPath string
	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Start the HTTP server",
		Run: func(cmd *cobra.Command, args []string) {
			serve(configPath)
		},
	}
	cmd.Flags().StringVarP(&configPath, "config", "c", "config.yaml", "path to config file")
	return cmd
}

func serve(configPath string) {
	cfg, err := config.Load(configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	var deviceStore *device.GormStore
	if dsn := os.Getenv("DATABASE_DSN"); dsn != "" {
		deviceStore, err = device.NewGormStorePostgres(dsn)
	} else {
		dbPath := cfg.Database.Path
		if dbPath == "" {
			dbPath = "tostada.db"
		}
		deviceStore, err = device.NewGormStore(dbPath)
	}
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

	oidcClientSecret := os.Getenv("OIDC_CLIENT_SECRET")
	if oidcClientSecret == "" {
		log.Fatal("OIDC_CLIENT_SECRET environment variable is required")
	}
	guacJSONSecretKey := os.Getenv("GUACAMOLE_JSON_SECRET_KEY")
	if guacJSONSecretKey == "" {
		log.Fatal("GUACAMOLE_JSON_SECRET_KEY environment variable is required")
	}
	hubAPIToken := os.Getenv("JUPYTERHUB_API_TOKEN")
	if hubAPIToken == "" {
		log.Fatal("JUPYTERHUB_API_TOKEN environment variable is required")
	}

	var authProvider *auth.Auth
	for i := 0; i < 30; i++ {
		authProvider, err = auth.NewAuth(
			context.Background(),
			cfg.OIDC.IssuerURL,
			cfg.OIDC.InternalURL,
			cfg.OIDC.ClientID,
			oidcClientSecret,
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

	hubClient := hub.NewClient(cfg.JupyterHub.APIURL, hubAPIToken)

	namespace := os.Getenv("TOSTADA_NAMESPACE")
	if namespace == "" {
		namespace = "tostada"
	}
	quotaClient, err := kube.NewQuotaClient(namespace)
	if err != nil {
		log.Printf("Kubernetes quota client unavailable: %v", err)
	}

	mux := api.NewRouter(cfg, hubClient, authProvider, deviceStore, userStore, auditLog, accessLogger, guacJSONSecretKey, deviceStore, quotaClient)

	if registerCoverageHandler != nil {
		registerCoverageHandler(mux)
	}

	distFS, err := fs.Sub(web.DistFS, "dist")
	if err != nil {
		log.Fatalf("Failed to create sub FS: %v", err)
	}
	fileServer := http.FileServer(http.FS(distFS))
	mux.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
