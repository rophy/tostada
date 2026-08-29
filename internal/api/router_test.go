package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rophy/tostada/internal/auth"
	"github.com/rophy/tostada/internal/config"
	"github.com/rophy/tostada/internal/hub"
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
			{
				Name:        "kasmvnc",
				DisplayName: "Ubuntu Desktop (KasmVNC)",
				Description: "KasmVNC desktop",
				Icon:        "desktop",
				Type:        "jupyterhub",
				Image:       "kasmweb/ubuntu-noble-desktop:1.19.0",
				Port:        6901,
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
	if len(workspaces) != 3 {
		t.Errorf("len(workspaces) = %d, want 3", len(workspaces))
	}
	if workspaces[0].Name != "jupyter" {
		t.Errorf("workspaces[0].Name = %q", workspaces[0].Name)
	}
}

func TestProfileSlug(t *testing.T) {
	tests := []struct {
		displayName string
		want        string
	}{
		{"Jupyter Notebook", "jupyter-notebook"},
		{"Ubuntu Desktop (KasmVNC)", "ubuntu-desktop-kasmvnc"},
		{"Ubuntu Desktop", "ubuntu-desktop"},
	}
	for _, tt := range tests {
		got := profileSlug(tt.displayName)
		if got != tt.want {
			t.Errorf("profileSlug(%q) = %q, want %q", tt.displayName, got, tt.want)
		}
	}
}

func TestWorkspaceByProfile(t *testing.T) {
	cfg := testConfig()
	h := &sessionsHandler{workspaces: cfg.Workspaces}

	ws := h.workspaceByProfile("ubuntu-desktop-kasmvnc")
	if ws == nil {
		t.Fatal("workspaceByProfile(ubuntu-desktop-kasmvnc) = nil")
	}
	if ws.Name != "kasmvnc" {
		t.Errorf("Name = %q, want %q", ws.Name, "kasmvnc")
	}

	ws = h.workspaceByProfile("jupyter-notebook")
	if ws == nil {
		t.Fatal("workspaceByProfile(jupyter-notebook) = nil")
	}
	if ws.Name != "jupyter" {
		t.Errorf("Name = %q, want %q", ws.Name, "jupyter")
	}

	ws = h.workspaceByProfile("nonexistent")
	if ws != nil {
		t.Errorf("workspaceByProfile(nonexistent) = %+v, want nil", ws)
	}
}

func TestConnectKasmVNC(t *testing.T) {
	hubSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/users/alice":
			json.NewEncoder(w).Encode(hub.User{
				Name: "alice",
				Servers: map[string]hub.Server{
					"my-kasm": {
						Name:        "my-kasm",
						Ready:       true,
						URL:         "/user/alice/my-kasm/",
						UserOptions: map[string]string{"profile": "ubuntu-desktop-kasmvnc"},
					},
				},
			})
		case "/users/alice/tokens":
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(map[string]string{"token": "fake-token"})
		default:
			t.Errorf("unexpected hub request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer hubSrv.Close()

	cfg := testConfig()
	hubClient := hub.NewClient(hubSrv.URL, "test-token")
	h := &sessionsHandler{
		hubClient:  hubClient,
		workspaces: cfg.Workspaces,
		guacCfg:    cfg.Guacamole,
	}

	req := httptest.NewRequest("GET", "/api/sessions/my-kasm/connect", nil)
	req.SetPathValue("name", "my-kasm")
	req = req.WithContext(auth.WithUser(req.Context(), "alice"))
	rec := httptest.NewRecorder()

	h.connect(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("Status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("Decode error: %v", err)
	}

	url := resp["url"]
	if url == "" {
		t.Fatal("url is empty")
	}
	if !strings.Contains(url, "token=fake-token") {
		t.Errorf("url missing token: %s", url)
	}
	if !strings.Contains(url, "path=") {
		t.Errorf("url missing path= param for KasmVNC websockify: %s", url)
	}
	if !strings.Contains(url, "websockify") {
		t.Errorf("url missing websockify path for KasmVNC: %s", url)
	}
}

func TestConnectJupyter(t *testing.T) {
	hubSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/users/alice":
			json.NewEncoder(w).Encode(hub.User{
				Name: "alice",
				Servers: map[string]hub.Server{
					"my-jupyter": {
						Name:        "my-jupyter",
						Ready:       true,
						URL:         "/user/alice/my-jupyter/",
						UserOptions: map[string]string{"profile": "jupyter-notebook"},
					},
				},
			})
		case "/users/alice/tokens":
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(map[string]string{"token": "fake-token"})
		default:
			t.Errorf("unexpected hub request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer hubSrv.Close()

	cfg := testConfig()
	hubClient := hub.NewClient(hubSrv.URL, "test-token")
	h := &sessionsHandler{
		hubClient:  hubClient,
		workspaces: cfg.Workspaces,
		guacCfg:    cfg.Guacamole,
	}

	req := httptest.NewRequest("GET", "/api/sessions/my-jupyter/connect", nil)
	req.SetPathValue("name", "my-jupyter")
	req = req.WithContext(auth.WithUser(req.Context(), "alice"))
	rec := httptest.NewRecorder()

	h.connect(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("Status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("Decode error: %v", err)
	}

	url := resp["url"]
	if url == "" {
		t.Fatal("url is empty")
	}
	if !strings.Contains(url, "token=fake-token") {
		t.Errorf("url missing token: %s", url)
	}
	if strings.Contains(url, "path=") {
		t.Errorf("Jupyter url should not have path= param: %s", url)
	}
}

func TestConnectServerNotFound(t *testing.T) {
	hubSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(hub.User{
			Name:    "alice",
			Servers: map[string]hub.Server{},
		})
	}))
	defer hubSrv.Close()

	cfg := testConfig()
	hubClient := hub.NewClient(hubSrv.URL, "test-token")
	h := &sessionsHandler{
		hubClient:  hubClient,
		workspaces: cfg.Workspaces,
		guacCfg:    cfg.Guacamole,
	}

	req := httptest.NewRequest("GET", "/api/sessions/missing/connect", nil)
	req.SetPathValue("name", "missing")
	req = req.WithContext(auth.WithUser(req.Context(), "alice"))
	rec := httptest.NewRecorder()

	h.connect(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("Status = %d, want 404", rec.Code)
	}
}

