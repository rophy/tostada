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
	"github.com/rophy/tostada/internal/audit"
)

func NewRouter(cfg *config.Config, hubClient *hub.Client, authProvider *auth.Auth, deviceStore device.AdminStore, userStore model.UserStore, auditLog *audit.AuditLog, accessLogger *audit.AccessLogger) *http.ServeMux {
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

	adminMux := http.NewServeMux()
	admin := &adminHandler{
		userStore:   userStore,
		deviceStore: deviceStore,
		hubClient:   hubClient,
		auditLog:    auditLog,
	}
	adminMux.HandleFunc("GET /api/admin/users", admin.listUsers)
	adminMux.HandleFunc("PATCH /api/admin/users/{username}", admin.updateUser)
	adminMux.HandleFunc("DELETE /api/admin/users/{username}", admin.deleteUser)
	adminMux.HandleFunc("GET /api/admin/devices", admin.listDevices)
	adminMux.HandleFunc("POST /api/admin/devices", admin.createDevice)
	adminMux.HandleFunc("PUT /api/admin/devices/{name}", admin.updateDevice)
	adminMux.HandleFunc("DELETE /api/admin/devices/{name}", admin.deleteDevice)
	adminMux.HandleFunc("POST /api/admin/devices/{name}/grants", admin.grantAccess)
	adminMux.HandleFunc("DELETE /api/admin/devices/{name}/grants/{username}", admin.revokeAccess)
	adminMux.HandleFunc("GET /api/admin/sessions", admin.listSessions)
	adminMux.HandleFunc("DELETE /api/admin/sessions/{username}/{server}", admin.stopSession)

	authed.Handle("/api/admin/", AdminMiddleware(userStore)(adminMux))

	authedHandler := authProvider.Middleware(authed)
	if accessLogger != nil {
		authedHandler = accessLogger.Middleware(authedHandler, func(r *http.Request) string {
			return auth.UserFromContext(r.Context())
		})
	}
	mux.Handle("/api/", authedHandler)

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
