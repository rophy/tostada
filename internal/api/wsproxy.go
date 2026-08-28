package api

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"
)

type wsProxyHandler struct {
	proxyAPIURL   string
	proxyAPIToken string
}

type chpRoute struct {
	Target string `json:"target"`
}

func (h *wsProxyHandler) lookupTarget(routePath string) (string, error) {
	client := &http.Client{Timeout: 5 * time.Second}
	req, err := http.NewRequest("GET", h.proxyAPIURL+"/api/routes"+routePath, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "token "+h.proxyAPIToken)

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("CHP API returned %d", resp.StatusCode)
	}

	var route chpRoute
	if err := json.NewDecoder(resp.Body).Decode(&route); err != nil {
		return "", err
	}
	return route.Target, nil
}

func (h *wsProxyHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Path: /api/ws/user/{user}/{server}/websockify
	// Extract the JupyterHub route prefix: /user/{user}/{server}
	path := strings.TrimPrefix(r.URL.Path, "/api/ws")
	parts := strings.SplitN(path, "/", 5) // ["", "user", "{user}", "{server}", "websockify"]
	if len(parts) < 4 {
		http.Error(w, "invalid path", http.StatusBadRequest)
		return
	}

	routePrefix := strings.Join(parts[:4], "/") // /user/{user}/{server}
	remainder := ""
	if len(parts) > 4 {
		remainder = "/" + parts[4]
	}

	target, err := h.lookupTarget(routePrefix)
	if err != nil {
		log.Printf("wsProxy: lookup failed for %s: %v", routePrefix, err)
		http.Error(w, "route not found", http.StatusBadGateway)
		return
	}

	targetURL, err := url.Parse(target)
	if err != nil {
		log.Printf("wsProxy: invalid target URL %s: %v", target, err)
		http.Error(w, "invalid target", http.StatusBadGateway)
		return
	}

	proxy := &httputil.ReverseProxy{
		Director: func(req *http.Request) {
			req.URL.Scheme = targetURL.Scheme
			req.URL.Host = targetURL.Host
			req.URL.Path = remainder
			req.Host = targetURL.Host
		},
	}
	proxy.ServeHTTP(w, r)
}
