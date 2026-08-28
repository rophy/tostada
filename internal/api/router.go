package api

import (
	"net/http"
	"net/http/httputil"
	"net/url"

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

	if cfg.JupyterHub.ProxyAPIURL != "" {
		wsProxy := &wsProxyHandler{
			proxyAPIURL:   cfg.JupyterHub.ProxyAPIURL,
			proxyAPIToken: cfg.JupyterHub.ProxyAPIToken,
		}
		authed.Handle("/api/ws/", wsProxy)
	}

	mux.Handle("/api/", authProvider.Middleware(authed))

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
