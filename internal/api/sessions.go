package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/rophy/tostada/internal/auth"
	"github.com/rophy/tostada/internal/config"
	"github.com/rophy/tostada/internal/guacamole"
	"github.com/rophy/tostada/internal/hub"
)

type sessionsHandler struct {
	hubClient  *hub.Client
	workspaces []config.Workspace
	guacCfg    config.GuacamoleConfig
	hubCfg     config.JupyterHubConfig
}

func (h *sessionsHandler) list(w http.ResponseWriter, r *http.Request) {
	username := auth.UserFromContext(r.Context())
	user, err := h.hubClient.GetUser(username)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(user.Servers)
}

func (h *sessionsHandler) create(w http.ResponseWriter, r *http.Request) {
	username := auth.UserFromContext(r.Context())

	var req struct {
		Workspace  string `json:"workspace"`
		ServerName string `json:"serverName"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	var ws *config.Workspace
	for i := range h.workspaces {
		if h.workspaces[i].Name == req.Workspace {
			ws = &h.workspaces[i]
			break
		}
	}
	if ws == nil {
		http.Error(w, "unknown workspace type", http.StatusBadRequest)
		return
	}

	if err := h.hubClient.SpawnServer(username, req.ServerName, ws.DisplayName); err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{"status": "spawning"})
}

func (h *sessionsHandler) stop(w http.ResponseWriter, r *http.Request) {
	username := auth.UserFromContext(r.Context())
	serverName := r.PathValue("name")

	if err := h.hubClient.StopServer(username, serverName); err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *sessionsHandler) connect(w http.ResponseWriter, r *http.Request) {
	username := auth.UserFromContext(r.Context())
	serverName := r.PathValue("name")

	user, err := h.hubClient.GetUser(username)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}

	srv, ok := user.Servers[serverName]
	if !ok {
		http.Error(w, "server not found", http.StatusNotFound)
		return
	}
	if !srv.Ready {
		http.Error(w, "server not ready", http.StatusConflict)
		return
	}

	// Determine workspace type from server name prefix
	var ws *config.Workspace
	for i := range h.workspaces {
		if strings.HasPrefix(serverName, h.workspaces[i].Name) {
			ws = &h.workspaces[i]
			break
		}
	}

	var connectURL string
	if ws != nil && ws.Type == "guacamole" {
		payload := guacamole.JSONAuthPayload{
			Username: username,
			Expires:  fmt.Sprintf("%d", time.Now().Add(5*time.Minute).UnixMilli()),
			Connections: map[string]guacamole.JSONAuthConnection{
				serverName: {
					Protocol: "rdp",
					Parameters: map[string]string{
						"hostname": fmt.Sprintf("jupyter-%s--%s", username, serverName),
						"port":     fmt.Sprintf("%d", ws.Port),
						"username": ws.RDPCredentials.Username,
						"password": ws.RDPCredentials.Password,
					},
				},
			},
		}
		token, err := guacamole.BuildToken(payload, h.guacCfg.JSONSecretKey)
		if err != nil {
			http.Error(w, "token generation failed", http.StatusInternalServerError)
			return
		}
		connectURL = fmt.Sprintf("%s/#/client/%s?token=%s", h.guacCfg.URL, serverName, token)
	} else {
		connectURL = srv.URL
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"url": connectURL})
}
