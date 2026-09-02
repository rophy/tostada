package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoad(t *testing.T) {
	content := `
server:
  addr: ":9090"
oidc:
  issuerURL: "https://auth.example.com"
  clientID: "test-client"
  redirectURL: "http://localhost:9090/api/auth/callback"
jupyterhub:
  apiURL: "http://hub:8081/hub/api"
guacamole:
  url: "http://guacamole:8080"
workspaces:
  - name: jupyter
    displayName: Jupyter Notebook
    description: Python notebook
    icon: notebook
    type: jupyterhub
    image: jupyter/minimal-notebook:latest
    port: 8888
    cmd: ["jupyterhub-singleuser"]
  - name: xrdp-ubuntu
    displayName: Ubuntu Desktop
    description: RDP desktop
    icon: desktop
    type: guacamole
    image: scottyhardy/docker-remote-desktop:latest
    port: 3389
    rdpCredentials:
      username: ubuntu
      password: ubuntu
`
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if cfg.Server.Addr != ":9090" {
		t.Errorf("Server.Addr = %q, want %q", cfg.Server.Addr, ":9090")
	}
	if cfg.OIDC.IssuerURL != "https://auth.example.com" {
		t.Errorf("OIDC.IssuerURL = %q, want %q", cfg.OIDC.IssuerURL, "https://auth.example.com")
	}
	if cfg.JupyterHub.APIURL != "http://hub:8081/hub/api" {
		t.Errorf("JupyterHub.APIURL = %q, want %q", cfg.JupyterHub.APIURL, "http://hub:8081/hub/api")
	}
	if cfg.Guacamole.URL != "http://guacamole:8080" {
		t.Errorf("Guacamole.URL = %q, want %q", cfg.Guacamole.URL, "http://guacamole:8080")
	}
	if len(cfg.Workspaces) != 2 {
		t.Fatalf("len(Workspaces) = %d, want 2", len(cfg.Workspaces))
	}

	ws := cfg.Workspaces[0]
	if ws.Name != "jupyter" || ws.Type != "jupyterhub" || ws.Port != 8888 {
		t.Errorf("Workspace[0] = %+v", ws)
	}

	ws = cfg.Workspaces[1]
	if ws.Name != "xrdp-ubuntu" || ws.Type != "guacamole" || ws.RDPCredentials.Username != "ubuntu" {
		t.Errorf("Workspace[1] = %+v", ws)
	}
}

func TestLoad_FileNotFound(t *testing.T) {
	_, err := Load("/nonexistent/config.yaml")
	if err == nil {
		t.Error("Load() should error on missing file")
	}
}
