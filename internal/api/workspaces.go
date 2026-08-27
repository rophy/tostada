package api

import (
	"encoding/json"
	"net/http"

	"github.com/rophy/tostada/internal/config"
)

type workspacesHandler struct {
	workspaces []config.Workspace
}

func (h *workspacesHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(h.workspaces)
}
