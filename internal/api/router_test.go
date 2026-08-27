package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rophy/tostada/internal/config"
)

func testConfig() *config.Config {
	return &config.Config{
		Guacamole: config.GuacamoleConfig{
			URL:           "http://guacamole:8080",
			JSONSecretKey: "4c0b569e4c96df157eee1b65dd0e4d41",
		},
		JupyterHub: config.JupyterHubConfig{
			APIURL:   "http://hub:8081/hub/api",
			APIToken: "test-token",
		},
		Workspaces: []config.Workspace{
			{
				Name:        "jupyter",
				DisplayName: "Jupyter Notebook",
				Description: "Python notebook",
				Icon:        "notebook",
				Type:        "jupyterhub",
				Image:       "jupyter/minimal-notebook:latest",
				Port:        8888,
			},
			{
				Name:        "xrdp-ubuntu",
				DisplayName: "Ubuntu Desktop",
				Description: "RDP desktop",
				Icon:        "desktop",
				Type:        "guacamole",
				Image:       "scottyhardy/docker-remote-desktop:latest",
				Port:        3389,
			},
		},
	}
}

func TestListWorkspaces(t *testing.T) {
	cfg := testConfig()
	h := &workspacesHandler{workspaces: cfg.Workspaces}

	req := httptest.NewRequest("GET", "/api/workspaces", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("Status = %d, want 200", rec.Code)
	}

	var workspaces []config.Workspace
	if err := json.NewDecoder(rec.Body).Decode(&workspaces); err != nil {
		t.Fatalf("Decode error: %v", err)
	}
	if len(workspaces) != 2 {
		t.Errorf("len(workspaces) = %d, want 2", len(workspaces))
	}
	if workspaces[0].Name != "jupyter" {
		t.Errorf("workspaces[0].Name = %q", workspaces[0].Name)
	}
}
