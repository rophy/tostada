package api

import (
	"net/http"

	"github.com/rophy/tostada/internal/auth"
	"github.com/rophy/tostada/internal/config"
	"github.com/rophy/tostada/internal/hub"
)

func NewRouter(cfg *config.Config, hubClient *hub.Client, authProvider *auth.Auth) *http.ServeMux {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/auth/login", authProvider.LoginHandler())
	mux.HandleFunc("GET /api/auth/callback", authProvider.CallbackHandler())

	sessions := &sessionsHandler{
		hubClient:  hubClient,
		workspaces: cfg.Workspaces,
		guacCfg:    cfg.Guacamole,
	}

	authed := http.NewServeMux()
	authed.HandleFunc("GET /api/auth/me", authProvider.CurrentUser)
	authed.Handle("GET /api/workspaces", &workspacesHandler{workspaces: cfg.Workspaces})
	authed.HandleFunc("GET /api/sessions", sessions.list)
	authed.HandleFunc("POST /api/sessions", sessions.create)
	authed.HandleFunc("DELETE /api/sessions/{name}", sessions.stop)
	authed.HandleFunc("GET /api/sessions/{name}/connect", sessions.connect)

	mux.Handle("/api/", authProvider.Middleware(authed))

	return mux
}
