package api

import (
	"net/http"
	"net/http/httputil"
	"net/url"

	"github.com/rophy/tostada/internal/auth"
	"github.com/rophy/tostada/internal/config"
	"github.com/rophy/tostada/internal/device"
	"github.com/rophy/tostada/internal/hub"
	"github.com/rophy/tostada/internal/model"
	"github.com/rophy/tostada/internal/telemetry"
)

func NewRouter(cfg *config.Config, hubClient *hub.Client, authProvider *auth.Auth, deviceStore device.Store, userStore model.UserStore, auditLog *telemetry.AuditLog, accessLogger *telemetry.AccessLogger) *http.ServeMux {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/auth/login", authProvider.LoginHandler())
	mux.HandleFunc("GET /api/auth/callback", authProvider.CallbackHandler())
	mux.HandleFunc("POST /api/auth/logout", authProvider.LogoutHandler())

	sessions := &sessionsHandler{
		hubClient:  hubClient,
		workspaces: cfg.Workspaces,
		guacCfg:    cfg.Guacamole,
		auditLog:   auditLog,
	}

	authed := http.NewServeMux()
	authed.HandleFunc("GET /api/auth/me", authProvider.CurrentUser)
	authed.Handle("GET /api/workspaces", &workspacesHandler{workspaces: cfg.Workspaces})
	authed.HandleFunc("GET /api/sessions", sessions.list)
	authed.HandleFunc("POST /api/sessions", sessions.create)
	authed.HandleFunc("DELETE /api/sessions/{name}", sessions.stop)
	authed.HandleFunc("GET /api/sessions/{name}/connect", sessions.connect)

	devices := &devicesHandler{
		store:    deviceStore,
		guacCfg:  cfg.Guacamole,
		auditLog: auditLog,
	}
	authed.HandleFunc("GET /api/devices", devices.list)
	authed.HandleFunc("GET /api/devices/{name}/connect", devices.connect)

	authedHandler := authProvider.Middleware(authed)
	if accessLogger != nil {
		authedHandler = accessLogger.Middleware(authedHandler, func(r *http.Request) string {
			return auth.UserFromContext(r.Context())
		})
	}
	mux.Handle("/api/", authedHandler)

	registerGuacamoleProxy(mux, cfg.Guacamole)

	if cfg.OIDC.InternalURL != "" {
		target, _ := url.Parse(cfg.OIDC.InternalURL)
		oidcProxy := &httputil.ReverseProxy{
			Director: func(req *http.Request) {
				req.URL.Scheme = target.Scheme
				req.URL.Host = target.Host
				req.URL.Path = req.URL.Path[len("/oidc"):]
				if req.URL.Path == "" {
					req.URL.Path = "/"
				}
				req.Host = target.Host
			},
		}
		mux.Handle("/oidc/", oidcProxy)

		// oidc-mock's authorize form POSTs to /authorize/callback (without /oidc prefix)
		authzProxy := &httputil.ReverseProxy{
			Director: func(req *http.Request) {
				req.URL.Scheme = target.Scheme
				req.URL.Host = target.Host
				req.Host = target.Host
			},
		}
		mux.Handle("/authorize/", authzProxy)
	}

	return mux
}
