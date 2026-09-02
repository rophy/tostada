package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/rophy/tostada/internal/auth"
	"github.com/rophy/tostada/internal/device"
	"github.com/rophy/tostada/internal/guacamole"
	"github.com/rophy/tostada/internal/audit"
)

type devicesHandler struct {
	store         device.Store
	guacURL       string
	guacSecretKey string
	auditLog      *audit.AuditLog
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

func deviceParams(d *device.Device) map[string]string {
	params := map[string]string{
		"hostname": d.Host,
		"port":     fmt.Sprintf("%d", d.Port),
		"username": d.Username,
		"password": d.Password,
	}
	if d.Protocol == "rdp" {
		params["security"] = "any"
		params["ignore-cert"] = "true"
	}
	return params
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
				Parameters: deviceParams(d),
			},
		},
	}

	token, err := guacamole.BuildToken(payload, h.guacSecretKey)
	if err != nil {
		http.Error(w, "token generation failed", http.StatusInternalServerError)
		return
	}

	authToken, err := guacamole.ExchangeToken(h.guacURL, token)
	if err != nil {
		http.Error(w, "token exchange failed", http.StatusInternalServerError)
		return
	}

	if h.auditLog != nil {
		h.auditLog.Log("device.connect", username, "", map[string]string{
			"device": d.Name,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"token":        authToken,
		"connectionId": d.Name,
	})
}
