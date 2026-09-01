package api

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/rophy/tostada/internal/auth"
	"github.com/rophy/tostada/internal/device"
	"github.com/rophy/tostada/internal/hub"
	"github.com/rophy/tostada/internal/model"
	"github.com/rophy/tostada/internal/audit"
)

func AdminMiddleware(store model.UserStore) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			username := auth.UserFromContext(r.Context())
			isAdmin, _ := store.IsAdmin(r.Context(), username)
			if !isAdmin {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusForbidden)
				json.NewEncoder(w).Encode(map[string]string{"error": "admin access required"})
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

type adminHandler struct {
	userStore   model.UserStore
	deviceStore device.AdminStore
	hubClient   *hub.Client
	auditLog    *audit.AuditLog
}

func (h *adminHandler) listUsers(w http.ResponseWriter, r *http.Request) {
	users, err := h.userStore.ListUsers(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(users)
}

func (h *adminHandler) updateUser(w http.ResponseWriter, r *http.Request) {
	actor := auth.UserFromContext(r.Context())
	target := r.PathValue("username")

	var req struct {
		IsAdmin *bool `json:"isAdmin"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	if req.IsAdmin != nil && !*req.IsAdmin && target == actor {
		http.Error(w, "cannot remove admin from yourself", http.StatusBadRequest)
		return
	}

	updates := map[string]any{}
	if req.IsAdmin != nil {
		updates["is_admin"] = *req.IsAdmin
	}

	if err := h.userStore.UpdateUser(r.Context(), target, updates); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if h.auditLog != nil {
		h.auditLog.Log("admin.user.update", target, actor, map[string]string{
			"target":  target,
			"changes": fmt.Sprintf("%v", updates),
		})
	}

	user, err := h.userStore.GetUser(r.Context(), target)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(user)
}

func (h *adminHandler) deleteUser(w http.ResponseWriter, r *http.Request) {
	actor := auth.UserFromContext(r.Context())
	target := r.PathValue("username")

	if target == actor {
		http.Error(w, "cannot delete yourself", http.StatusBadRequest)
		return
	}

	if err := h.userStore.DeleteUser(r.Context(), target); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if h.auditLog != nil {
		h.auditLog.Log("admin.user.delete", target, actor, map[string]string{"target": target})
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *adminHandler) listDevices(w http.ResponseWriter, r *http.Request) {
	devices, err := h.deviceStore.ListAllDevices(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(devices)
}

func (h *adminHandler) createDevice(w http.ResponseWriter, r *http.Request) {
	actor := auth.UserFromContext(r.Context())

	var d device.Device
	if err := json.NewDecoder(r.Body).Decode(&d); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	if err := h.deviceStore.CreateDevice(r.Context(), &d); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if h.auditLog != nil {
		h.auditLog.Log("admin.device.add", "", actor, map[string]string{"device": d.Name})
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(d)
}

func (h *adminHandler) updateDevice(w http.ResponseWriter, r *http.Request) {
	actor := auth.UserFromContext(r.Context())
	name := r.PathValue("name")

	var updates map[string]any
	if err := json.NewDecoder(r.Body).Decode(&updates); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	if err := h.deviceStore.UpdateDevice(r.Context(), name, updates); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if h.auditLog != nil {
		h.auditLog.Log("admin.device.update", "", actor, map[string]string{"device": name})
	}

	w.WriteHeader(http.StatusOK)
}

func (h *adminHandler) deleteDevice(w http.ResponseWriter, r *http.Request) {
	actor := auth.UserFromContext(r.Context())
	name := r.PathValue("name")

	if err := h.deviceStore.DeleteDevice(r.Context(), name); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if h.auditLog != nil {
		h.auditLog.Log("admin.device.remove", "", actor, map[string]string{"device": name})
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *adminHandler) grantAccess(w http.ResponseWriter, r *http.Request) {
	actor := auth.UserFromContext(r.Context())
	deviceName := r.PathValue("name")

	var req struct {
		Username string `json:"username"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	if err := h.deviceStore.GrantAccess(r.Context(), deviceName, req.Username); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if h.auditLog != nil {
		h.auditLog.Log("admin.device.grant", req.Username, actor, map[string]string{
			"device": deviceName,
			"target": req.Username,
		})
	}

	w.WriteHeader(http.StatusCreated)
}

func (h *adminHandler) revokeAccess(w http.ResponseWriter, r *http.Request) {
	actor := auth.UserFromContext(r.Context())
	deviceName := r.PathValue("name")
	username := r.PathValue("username")

	if err := h.deviceStore.RevokeAccess(r.Context(), deviceName, username); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if h.auditLog != nil {
		h.auditLog.Log("admin.device.revoke", username, actor, map[string]string{
			"device": deviceName,
			"target": username,
		})
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *adminHandler) listSessions(w http.ResponseWriter, r *http.Request) {
	if h.hubClient == nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]any{})
		return
	}

	users, err := h.hubClient.ListUsers()
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}

	type sessionInfo struct {
		Username   string `json:"username"`
		ServerName string `json:"serverName"`
		Ready      bool   `json:"ready"`
		URL        string `json:"url"`
	}

	var sessions []sessionInfo
	for _, u := range users {
		for name, srv := range u.Servers {
			sessions = append(sessions, sessionInfo{
				Username:   u.Name,
				ServerName: name,
				Ready:      srv.Ready,
				URL:        srv.URL,
			})
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(sessions)
}

func (h *adminHandler) stopSession(w http.ResponseWriter, r *http.Request) {
	actor := auth.UserFromContext(r.Context())
	username := r.PathValue("username")
	server := r.PathValue("server")

	if h.hubClient == nil {
		http.Error(w, "hub client not configured", http.StatusServiceUnavailable)
		return
	}

	if err := h.hubClient.StopServer(username, server); err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}

	if h.auditLog != nil {
		h.auditLog.Log("admin.session.stop", username, actor, map[string]string{
			"target": username,
			"server": server,
		})
	}

	w.WriteHeader(http.StatusNoContent)
}
