package api

import (
	"net/http"
	"net/http/httputil"
	"net/url"

	"github.com/rophy/tostada/internal/config"
)

func registerGuacamoleProxy(mux *http.ServeMux, cfg config.GuacamoleConfig) {
	target, _ := url.Parse(cfg.URL)

	proxy := &httputil.ReverseProxy{
		Director: func(req *http.Request) {
			req.URL.Scheme = target.Scheme
			req.URL.Host = target.Host
			req.Host = target.Host
		},
	}

	mux.HandleFunc("/api/guacamole/tunnel", func(w http.ResponseWriter, r *http.Request) {
		r.URL.Path = "/tunnel"
		proxy.ServeHTTP(w, r)
	})

	mux.Handle("/guacamole-common-js/", proxy)
}
