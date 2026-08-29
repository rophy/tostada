package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/rophy/tostada/internal/auth"
	"github.com/rophy/tostada/internal/config"
	"github.com/rophy/tostada/internal/device"
	"github.com/rophy/tostada/internal/guacamole"
)

type devicesHandler struct {
	store   device.Store
	guacCfg config.GuacamoleConfig
}

func (h *devicesHandler) list(w http.ResponseWriter, r *http.Request) {
	username := auth.UserFromContext(r.Context())
	devices, err := h.store.ListDevices(r.Context(), username)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(devices)
}

func (h *devicesHandler) connect(w http.ResponseWriter, r *http.Request) {
	username := auth.UserFromContext(r.Context())
	name := r.PathValue("name")

	d, err := h.store.GetDevice(r.Context(), username, name)
	if err != nil {
		http.Error(w, "device not found", http.StatusNotFound)
		return
	}

	payload := guacamole.JSONAuthPayload{
		Username: username,
		Expires:  fmt.Sprintf("%d", time.Now().Add(5*time.Minute).UnixMilli()),
		Connections: map[string]guacamole.JSONAuthConnection{
			d.Name: {
				Protocol: d.Protocol,
				Parameters: map[string]string{
					"hostname": d.Host,
					"port":     fmt.Sprintf("%d", d.Port),
					"username": d.Username,
					"password": d.Password,
				},
			},
		},
	}

	token, err := guacamole.BuildToken(payload, h.guacCfg.JSONSecretKey)
	if err != nil {
		http.Error(w, "token generation failed", http.StatusInternalServerError)
		return
	}

	connectURL := fmt.Sprintf("%s/#/client/%s?token=%s", h.guacCfg.URL, d.Name, url.QueryEscape(token))

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"url": connectURL})
}
