package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rophy/tostada/internal/auth"
	"github.com/rophy/tostada/internal/config"
	"github.com/rophy/tostada/internal/hub"
)

type mockHealthChecker struct {
	err error
}

func (m *mockHealthChecker) HealthCheck(_ context.Context) error {
	return m.err
}

func testConfig() *config.Config {
	return &config.Config{
		Guacamole: config.GuacamoleConfig{
			URL: "http://guacamole:8080",
		},
		JupyterHub: config.JupyterHubConfig{
			APIURL: "http://hub:8081/hub/api",
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
				Name:           "kasmvnc",
				DisplayName:    "Ubuntu Desktop (KasmVNC)",
				Description:    "KasmVNC desktop",
				Icon:           "desktop",
				Type:           "jupyterhub",
				Image:          "kasmweb/ubuntu-noble-desktop:1.19.0",
				Port:           6901,
				ServesFromRoot: true,
			},
		},
	}
}

func TestNewRouter(t *testing.T) {
	cfg := testConfig()
	hubClient := hub.NewClient("http://hub:8081", "test-token")
	authProvider := &auth.Auth{}
	store := testDeviceStore(t)

	mux := NewRouter(cfg, hubClient, authProvider, store, nil, nil, nil, "test-secret-key", &mockHealthChecker{})
	if mux == nil {
		t.Fatal("NewRouter returned nil")
	}
}

func TestNewRouter_WithOIDCProxy(t *testing.T) {
	cfg := testConfig()
	cfg.OIDC.InternalURL = "http://oidc-mock:8080"
	hubClient := hub.NewClient("http://hub:8081", "test-token")
	authProvider := &auth.Auth{}
	store := testDeviceStore(t)

	mux := NewRouter(cfg, hubClient, authProvider, store, nil, nil, nil, "test-secret-key", &mockHealthChecker{})
	if mux == nil {
		t.Fatal("NewRouter returned nil")
	}
}

func TestHealthz(t *testing.T) {
	cfg := testConfig()
	hubClient := hub.NewClient("http://hub:8081", "test-token")
	authProvider := &auth.Auth{}
	store := testDeviceStore(t)
	hc := &mockHealthChecker{}

	mux := NewRouter(cfg, hubClient, authProvider, store, nil, nil, nil, "test-secret-key", hc)

	req := httptest.NewRequest("GET", "/healthz", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Status = %d, want 200", rec.Code)
	}
}

func TestReadyz(t *testing.T) {
	cfg := testConfig()
	hubClient := hub.NewClient("http://hub:8081", "test-token")
	authProvider := &auth.Auth{}
	store := testDeviceStore(t)

	t.Run("healthy", func(t *testing.T) {
		hc := &mockHealthChecker{}
		mux := NewRouter(cfg, hubClient, authProvider, store, nil, nil, nil, "test-secret-key", hc)

		req := httptest.NewRequest("GET", "/readyz", nil)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("Status = %d, want 200", rec.Code)
		}
	})

	t.Run("unhealthy", func(t *testing.T) {
		hc := &mockHealthChecker{err: errors.New("db connection lost")}
		mux := NewRouter(cfg, hubClient, authProvider, store, nil, nil, nil, "test-secret-key", hc)

		req := httptest.NewRequest("GET", "/readyz", nil)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusServiceUnavailable {
			t.Errorf("Status = %d, want 503", rec.Code)
		}
	})
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
		guacURL:       cfg.Guacamole.URL,
		guacSecretKey: "4c0b569e4c96df157eee1b65dd0e4d41",
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
		guacURL:       cfg.Guacamole.URL,
		guacSecretKey: "4c0b569e4c96df157eee1b65dd0e4d41",
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
		guacURL:       cfg.Guacamole.URL,
		guacSecretKey: "4c0b569e4c96df157eee1b65dd0e4d41",
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

func TestSessionsList(t *testing.T) {
	hubSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(hub.User{
			Name: "alice",
			Servers: map[string]hub.Server{
				"nb-1": {Name: "nb-1", Ready: true, URL: "/user/alice/nb-1/"},
				"nb-2": {Name: "nb-2", Ready: false, URL: ""},
			},
		})
	}))
	defer hubSrv.Close()

	cfg := testConfig()
	h := &sessionsHandler{
		hubClient:  hub.NewClient(hubSrv.URL, "test-token"),
		workspaces: cfg.Workspaces,
	}

	req := httptest.NewRequest("GET", "/api/sessions", nil)
	req = req.WithContext(auth.WithUser(req.Context(), "alice"))
	rec := httptest.NewRecorder()
	h.list(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("Status = %d, want 200", rec.Code)
	}
	var servers map[string]hub.Server
	if err := json.NewDecoder(rec.Body).Decode(&servers); err != nil {
		t.Fatalf("Decode error: %v", err)
	}
	if len(servers) != 2 {
		t.Errorf("len(servers) = %d, want 2", len(servers))
	}
}

func TestSessionsList_HubError(t *testing.T) {
	hubSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer hubSrv.Close()

	h := &sessionsHandler{
		hubClient: hub.NewClient(hubSrv.URL, "test-token"),
	}

	req := httptest.NewRequest("GET", "/api/sessions", nil)
	req = req.WithContext(auth.WithUser(req.Context(), "alice"))
	rec := httptest.NewRecorder()
	h.list(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("Status = %d, want 200 (empty fallback)", rec.Code)
	}
	var result map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&result); err != nil {
		t.Fatalf("Decode error: %v", err)
	}
	if len(result) != 0 {
		t.Errorf("expected empty object, got %v", result)
	}
}

func TestSessionsCreate(t *testing.T) {
	hubSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/users/alice":
			w.WriteHeader(http.StatusCreated)
		case r.Method == http.MethodPost && strings.HasPrefix(r.URL.Path, "/users/alice/servers/"):
			w.WriteHeader(http.StatusCreated)
		default:
			t.Errorf("unexpected: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer hubSrv.Close()

	cfg := testConfig()
	h := &sessionsHandler{
		hubClient:  hub.NewClient(hubSrv.URL, "test-token"),
		workspaces: cfg.Workspaces,
	}

	body := `{"workspace":"jupyter","serverName":"nb-123"}`
	req := httptest.NewRequest("POST", "/api/sessions", strings.NewReader(body))
	req = req.WithContext(auth.WithUser(req.Context(), "alice"))
	rec := httptest.NewRecorder()
	h.create(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("Status = %d, want 201; body: %s", rec.Code, rec.Body.String())
	}
}

func TestSessionsCreate_InvalidBody(t *testing.T) {
	h := &sessionsHandler{
		workspaces: testConfig().Workspaces,
	}

	req := httptest.NewRequest("POST", "/api/sessions", strings.NewReader("not json"))
	req = req.WithContext(auth.WithUser(req.Context(), "alice"))
	rec := httptest.NewRecorder()
	h.create(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("Status = %d, want 400", rec.Code)
	}
}

func TestSessionsCreate_UnknownWorkspace(t *testing.T) {
	h := &sessionsHandler{
		workspaces: testConfig().Workspaces,
	}

	body := `{"workspace":"nonexistent","serverName":"nb-123"}`
	req := httptest.NewRequest("POST", "/api/sessions", strings.NewReader(body))
	req = req.WithContext(auth.WithUser(req.Context(), "alice"))
	rec := httptest.NewRecorder()
	h.create(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("Status = %d, want 400", rec.Code)
	}
}

func TestSessionsStop(t *testing.T) {
	hubSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("Method = %q, want DELETE", r.Method)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer hubSrv.Close()

	h := &sessionsHandler{
		hubClient: hub.NewClient(hubSrv.URL, "test-token"),
	}

	req := httptest.NewRequest("DELETE", "/api/sessions/my-nb", nil)
	req.SetPathValue("name", "my-nb")
	req = req.WithContext(auth.WithUser(req.Context(), "alice"))
	rec := httptest.NewRecorder()
	h.stop(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Errorf("Status = %d, want 204", rec.Code)
	}
}

func TestSessionsStop_HubError(t *testing.T) {
	hubSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer hubSrv.Close()

	h := &sessionsHandler{
		hubClient: hub.NewClient(hubSrv.URL, "test-token"),
	}

	req := httptest.NewRequest("DELETE", "/api/sessions/my-nb", nil)
	req.SetPathValue("name", "my-nb")
	req = req.WithContext(auth.WithUser(req.Context(), "alice"))
	rec := httptest.NewRecorder()
	h.stop(rec, req)

	if rec.Code != http.StatusBadGateway {
		t.Errorf("Status = %d, want 502", rec.Code)
	}
}

func TestConnectServerNotReady(t *testing.T) {
	hubSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(hub.User{
			Name: "alice",
			Servers: map[string]hub.Server{
				"my-nb": {Name: "my-nb", Ready: false, URL: ""},
			},
		})
	}))
	defer hubSrv.Close()

	cfg := testConfig()
	h := &sessionsHandler{
		hubClient:  hub.NewClient(hubSrv.URL, "test-token"),
		workspaces: cfg.Workspaces,
		guacURL:       cfg.Guacamole.URL,
		guacSecretKey: "4c0b569e4c96df157eee1b65dd0e4d41",
	}

	req := httptest.NewRequest("GET", "/api/sessions/my-nb/connect", nil)
	req.SetPathValue("name", "my-nb")
	req = req.WithContext(auth.WithUser(req.Context(), "alice"))
	rec := httptest.NewRecorder()
	h.connect(rec, req)

	if rec.Code != http.StatusConflict {
		t.Errorf("Status = %d, want 409; body: %s", rec.Code, rec.Body.String())
	}
}

func TestSessionsCreate_EnsureUserFails(t *testing.T) {
	hubSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer hubSrv.Close()

	cfg := testConfig()
	h := &sessionsHandler{
		hubClient:  hub.NewClient(hubSrv.URL, "test-token"),
		workspaces: cfg.Workspaces,
	}

	body := `{"workspace":"jupyter","serverName":"nb-123"}`
	req := httptest.NewRequest("POST", "/api/sessions", strings.NewReader(body))
	req = req.WithContext(auth.WithUser(req.Context(), "alice"))
	rec := httptest.NewRecorder()
	h.create(rec, req)

	if rec.Code != http.StatusBadGateway {
		t.Errorf("Status = %d, want 502", rec.Code)
	}
}

func TestSessionsCreate_SpawnFails(t *testing.T) {
	callCount := 0
	hubSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if callCount == 1 {
			w.WriteHeader(http.StatusCreated)
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer hubSrv.Close()

	cfg := testConfig()
	h := &sessionsHandler{
		hubClient:  hub.NewClient(hubSrv.URL, "test-token"),
		workspaces: cfg.Workspaces,
	}

	body := `{"workspace":"jupyter","serverName":"nb-123"}`
	req := httptest.NewRequest("POST", "/api/sessions", strings.NewReader(body))
	req = req.WithContext(auth.WithUser(req.Context(), "alice"))
	rec := httptest.NewRecorder()
	h.create(rec, req)

	if rec.Code != http.StatusBadGateway {
		t.Errorf("Status = %d, want 502", rec.Code)
	}
}

func TestConnectHubError(t *testing.T) {
	hubSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer hubSrv.Close()

	h := &sessionsHandler{
		hubClient: hub.NewClient(hubSrv.URL, "test-token"),
	}

	req := httptest.NewRequest("GET", "/api/sessions/nb/connect", nil)
	req.SetPathValue("name", "nb")
	req = req.WithContext(auth.WithUser(req.Context(), "alice"))
	rec := httptest.NewRecorder()
	h.connect(rec, req)

	if rec.Code != http.StatusBadGateway {
		t.Errorf("Status = %d, want 502", rec.Code)
	}
}

func TestConnectGuacamoleType(t *testing.T) {
	hubSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(hub.User{
			Name: "alice",
			Servers: map[string]hub.Server{
				"my-rdp": {
					Name:        "my-rdp",
					Ready:       true,
					URL:         "/user/alice/my-rdp/",
					UserOptions: map[string]string{"profile": "rdp-desktop"},
				},
			},
		})
	}))
	defer hubSrv.Close()

	cfg := testConfig()
	cfg.Workspaces = append(cfg.Workspaces, config.Workspace{
		Name:        "rdp",
		DisplayName: "RDP Desktop",
		Type:        "guacamole",
		Image:       "xrdp:latest",
		Port:        3389,
		RDPCredentials: config.RDPCredentials{
			Username: "user",
			Password: "pass",
		},
	})
	h := &sessionsHandler{
		hubClient:  hub.NewClient(hubSrv.URL, "test-token"),
		workspaces: cfg.Workspaces,
		guacURL:       cfg.Guacamole.URL,
		guacSecretKey: "4c0b569e4c96df157eee1b65dd0e4d41",
	}

	req := httptest.NewRequest("GET", "/api/sessions/my-rdp/connect", nil)
	req.SetPathValue("name", "my-rdp")
	req = req.WithContext(auth.WithUser(req.Context(), "alice"))
	rec := httptest.NewRecorder()
	h.connect(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("Status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]string
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp["url"] == "" {
		t.Error("url is empty")
	}
	if !strings.Contains(resp["url"], "token=") {
		t.Errorf("url missing token param: %s", resp["url"])
	}
}

func TestConnectTokenCreateFails(t *testing.T) {
	hubSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/users/alice":
			json.NewEncoder(w).Encode(hub.User{
				Name: "alice",
				Servers: map[string]hub.Server{
					"nb": {
						Name:        "nb",
						Ready:       true,
						URL:         "/user/alice/nb/",
						UserOptions: map[string]string{"profile": "jupyter-notebook"},
					},
				},
			})
		case "/users/alice/tokens":
			w.WriteHeader(http.StatusInternalServerError)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer hubSrv.Close()

	cfg := testConfig()
	h := &sessionsHandler{
		hubClient:  hub.NewClient(hubSrv.URL, "test-token"),
		workspaces: cfg.Workspaces,
		guacURL:       cfg.Guacamole.URL,
		guacSecretKey: "4c0b569e4c96df157eee1b65dd0e4d41",
	}

	req := httptest.NewRequest("GET", "/api/sessions/nb/connect", nil)
	req.SetPathValue("name", "nb")
	req = req.WithContext(auth.WithUser(req.Context(), "alice"))
	rec := httptest.NewRecorder()
	h.connect(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("Status = %d, want 200 (falls back to URL without token)", rec.Code)
	}
	var resp map[string]string
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp["url"] != "/user/alice/nb/" {
		t.Errorf("url = %q, want fallback URL", resp["url"])
	}
}

func TestSessionProgress_HubError(t *testing.T) {
	hubSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer hubSrv.Close()

	h := &sessionsHandler{
		hubClient: hub.NewClient(hubSrv.URL, "test-token"),
	}

	req := httptest.NewRequest("GET", "/api/sessions/my-nb/progress", nil)
	req.SetPathValue("name", "my-nb")
	req = req.WithContext(auth.WithUser(req.Context(), "alice"))
	rec := httptest.NewRecorder()
	h.progress(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("Status = %d, want 500", rec.Code)
	}
}

func TestSessionProgress_HubConnectionRefused(t *testing.T) {
	hubClient := hub.NewClient("http://127.0.0.1:1", "test-token")

	h := &sessionsHandler{
		hubClient: hubClient,
	}

	req := httptest.NewRequest("GET", "/api/sessions/my-nb/progress", nil)
	req.SetPathValue("name", "my-nb")
	req = req.WithContext(auth.WithUser(req.Context(), "alice"))
	rec := httptest.NewRecorder()
	h.progress(rec, req)

	if rec.Code != http.StatusBadGateway {
		t.Errorf("Status = %d, want 502", rec.Code)
	}
}

func TestSessionProgress(t *testing.T) {
	hubSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/users/alice/servers/my-nb/progress" {
			t.Errorf("unexpected hub request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("data: {\"progress\": 0, \"message\": \"Server requested\"}\n\n"))
		w.Write([]byte("data: {\"progress\": 100, \"ready\": true, \"message\": \"Server ready\"}\n\n"))
	}))
	defer hubSrv.Close()

	cfg := testConfig()
	hubClient := hub.NewClient(hubSrv.URL, "test-token")
	h := &sessionsHandler{
		hubClient:  hubClient,
		workspaces: cfg.Workspaces,
	}

	req := httptest.NewRequest("GET", "/api/sessions/my-nb/progress", nil)
	req.SetPathValue("name", "my-nb")
	req = req.WithContext(auth.WithUser(req.Context(), "alice"))
	rec := httptest.NewRecorder()

	h.progress(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("Status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); ct != "text/event-stream" {
		t.Errorf("Content-Type = %q, want text/event-stream", ct)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Server requested") {
		t.Errorf("body missing 'Server requested': %s", body)
	}
	if !strings.Contains(body, "Server ready") {
		t.Errorf("body missing 'Server ready': %s", body)
	}
}
