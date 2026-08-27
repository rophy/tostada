# Tostada Portal Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the Tostada portal — a Go + React web app that serves a workspace catalog and manages workspace lifecycle via JupyterHub's API.

**Architecture:** Go backend handles OIDC auth, JupyterHub API calls, and Guacamole JSON auth token generation. React frontend (built with Vite, embedded via Go's `embed`) shows a card grid of workspace types and active sessions. Deployed to a kind cluster alongside JupyterHub and shared guacamole-client+guacd.

**Tech Stack:** Go 1.22+, React 18 + TypeScript + Vite, JupyterHub Helm chart, Apache Guacamole, kind cluster, Skaffold

**Spec:** `docs/superpowers/specs/2026-08-28-tostada-portal-design.md`

## Global Constraints

- Go 1.22+ (use stdlib `net/http` ServeMux with method routing)
- Node 22 LTS for React build
- Kubernetes via kind cluster
- JupyterHub Helm chart v4.x
- All workspace types defined in `config.yaml` at repo root
- `mise` for runtime version management (activate if `MISE_SHELL` not set)

---

### Task 1: Config Loading + Go Project Scaffold

**Files:**
- Create: `go.mod`
- Create: `config.yaml`
- Create: `internal/config/config.go`
- Create: `internal/config/config_test.go`
- Create: `cmd/tostada/main.go`

**Interfaces:**
- Consumes: nothing
- Produces: `config.Load(path string) (*Config, error)`, `Config` struct with `Workspaces []Workspace`, `Workspace` struct with `Name`, `DisplayName`, `Description`, `Icon`, `Type` (enum: `"jupyterhub"` or `"guacamole"`), `Image`, `Port`, `Cmd`, `RDPCredentials`

- [ ] **Step 1: Initialize Go module**

```bash
cd /home/rophy/projects/tostada
go mod init github.com/rophy/tostada
```

- [ ] **Step 2: Create config.yaml**

```yaml
server:
  addr: ":8080"

oidc:
  issuerURL: "http://localhost:3000"
  clientID: "tostada"
  clientSecret: "changeme"
  redirectURL: "http://localhost:8080/api/auth/callback"

jupyterhub:
  apiURL: "http://jupyterhub-hub:8081/hub/api"
  apiToken: "changeme"

guacamole:
  url: "http://guacamole:8080"
  jsonSecretKey: "8bea5deb816809f814b83c28cf14d3e2"

workspaces:
  - name: jupyter
    displayName: Jupyter Notebook
    description: Python data science environment
    icon: notebook
    type: jupyterhub
    image: jupyter/minimal-notebook:latest
    port: 8888
    cmd: ["jupyterhub-singleuser"]

  - name: kasmvnc-ubuntu
    displayName: Ubuntu Desktop (KasmVNC)
    description: Full Ubuntu desktop in browser
    icon: desktop
    type: jupyterhub
    image: kasmweb/ubuntu-noble-desktop:1.16.1
    port: 6901
    cmd: ["/dockerstartup/kasm_default_profile.sh"]

  - name: xrdp-ubuntu
    displayName: Ubuntu Desktop (xRDP)
    description: Full Ubuntu desktop via RDP
    icon: desktop
    type: guacamole
    image: scottyhardy/docker-remote-desktop:latest
    port: 3389
    rdpCredentials:
      username: ubuntu
      password: ubuntu
```

- [ ] **Step 3: Write the failing test for config loading**

Create `internal/config/config_test.go`:

```go
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
  clientSecret: "test-secret"
  redirectURL: "http://localhost:9090/api/auth/callback"
jupyterhub:
  apiURL: "http://hub:8081/hub/api"
  apiToken: "test-token"
guacamole:
  url: "http://guacamole:8080"
  jsonSecretKey: "4c0b569e4c96df157eee1b65dd0e4d41"
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
	if cfg.Guacamole.JSONSecretKey != "4c0b569e4c96df157eee1b65dd0e4d41" {
		t.Errorf("Guacamole.JSONSecretKey = %q", cfg.Guacamole.JSONSecretKey)
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
```

- [ ] **Step 4: Run test to verify it fails**

```bash
go test ./internal/config/ -v
```

Expected: compilation error — `Load` not defined.

- [ ] **Step 5: Implement config.go**

Create `internal/config/config.go`:

```go
package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server     ServerConfig     `yaml:"server"`
	OIDC       OIDCConfig       `yaml:"oidc"`
	JupyterHub JupyterHubConfig `yaml:"jupyterhub"`
	Guacamole  GuacamoleConfig  `yaml:"guacamole"`
	Workspaces []Workspace      `yaml:"workspaces"`
}

type ServerConfig struct {
	Addr string `yaml:"addr"`
}

type OIDCConfig struct {
	IssuerURL    string `yaml:"issuerURL"`
	ClientID     string `yaml:"clientID"`
	ClientSecret string `yaml:"clientSecret"`
	RedirectURL  string `yaml:"redirectURL"`
}

type JupyterHubConfig struct {
	APIURL   string `yaml:"apiURL"`
	APIToken string `yaml:"apiToken"`
}

type GuacamoleConfig struct {
	URL           string `yaml:"url"`
	JSONSecretKey string `yaml:"jsonSecretKey"`
}

type Workspace struct {
	Name           string         `yaml:"name"`
	DisplayName    string         `yaml:"displayName"`
	Description    string         `yaml:"description"`
	Icon           string         `yaml:"icon"`
	Type           string         `yaml:"type"`
	Image          string         `yaml:"image"`
	Port           int            `yaml:"port"`
	Cmd            []string       `yaml:"cmd"`
	RDPCredentials RDPCredentials `yaml:"rdpCredentials"`
}

type RDPCredentials struct {
	Username string `yaml:"username"`
	Password string `yaml:"password"`
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config: %w", err)
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}
	return &cfg, nil
}
```

Then add the yaml dependency:

```bash
go get gopkg.in/yaml.v3
```

- [ ] **Step 6: Run tests to verify they pass**

```bash
go test ./internal/config/ -v
```

Expected: all PASS.

- [ ] **Step 7: Create minimal main.go**

Create `cmd/tostada/main.go`:

```go
package main

import (
	"flag"
	"log"

	"github.com/rophy/tostada/internal/config"
)

func main() {
	configPath := flag.String("config", "config.yaml", "path to config file")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}
	log.Printf("Loaded %d workspace(s), listening on %s", len(cfg.Workspaces), cfg.Server.Addr)
}
```

- [ ] **Step 8: Verify it compiles**

```bash
go build ./cmd/tostada/
```

- [ ] **Step 9: Commit**

```bash
git add go.mod go.sum config.yaml cmd/ internal/config/
git commit -m "feat: project scaffold with config loading"
```

---

### Task 2: Guacamole JSON Auth Token Generation

**Files:**
- Create: `internal/guacamole/token.go`
- Create: `internal/guacamole/token_test.go`

**Interfaces:**
- Consumes: nothing (standalone crypto package)
- Produces: `JSONAuthPayload` struct, `JSONAuthConnection` struct, `BuildToken(payload JSONAuthPayload, hexKey string) (string, error)`, `ExchangeToken(guacamoleURL string, token string) (authToken string, err error)`

Reference implementation: `~/projects/guacamole/portal/jsonauth.go` (branch `feat/guaca-idp-portal`).

- [ ] **Step 1: Write the failing test**

Create `internal/guacamole/token_test.go`:

```go
package guacamole

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"testing"
)

const testKey = "4c0b569e4c96df157eee1b65dd0e4d41"

func TestBuildToken(t *testing.T) {
	payload := JSONAuthPayload{
		Username: "testuser",
		Expires:  "1735689600000",
		Connections: map[string]JSONAuthConnection{
			"my-desktop": {
				Protocol: "rdp",
				Parameters: map[string]string{
					"hostname": "10.0.0.1",
					"port":     "3389",
					"username": "ubuntu",
					"password": "ubuntu",
				},
			},
		},
	}

	token, err := BuildToken(payload, testKey)
	if err != nil {
		t.Fatalf("BuildToken() error: %v", err)
	}
	if token == "" {
		t.Fatal("BuildToken() returned empty token")
	}

	// Verify by decrypting
	key, _ := hex.DecodeString(testKey)
	ciphertext, err := base64Decode(token)
	if err != nil {
		t.Fatalf("base64 decode error: %v", err)
	}

	block, _ := aes.NewCipher(key)
	iv := make([]byte, aes.BlockSize)
	mode := cipher.NewCBCDecrypter(block, iv)
	mode.CryptBlocks(ciphertext, ciphertext)

	// Remove PKCS7 padding
	padLen := int(ciphertext[len(ciphertext)-1])
	plaintext := ciphertext[:len(ciphertext)-padLen]

	// First 32 bytes are HMAC-SHA256 signature
	sig := plaintext[:32]
	jsonBytes := plaintext[32:]

	mac := hmac.New(sha256.New, key)
	mac.Write(jsonBytes)
	expectedSig := mac.Sum(nil)
	if !hmac.Equal(sig, expectedSig) {
		t.Error("HMAC signature mismatch")
	}

	var decoded JSONAuthPayload
	if err := json.Unmarshal(jsonBytes, &decoded); err != nil {
		t.Fatalf("JSON unmarshal error: %v", err)
	}
	if decoded.Username != "testuser" {
		t.Errorf("Username = %q, want %q", decoded.Username, "testuser")
	}
	if len(decoded.Connections) != 1 {
		t.Errorf("Connections count = %d, want 1", len(decoded.Connections))
	}
}

func TestBuildToken_InvalidKey(t *testing.T) {
	payload := JSONAuthPayload{Username: "test"}
	_, err := BuildToken(payload, "not-hex")
	if err == nil {
		t.Error("BuildToken() should error on invalid hex key")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
go test ./internal/guacamole/ -v
```

Expected: compilation error — types and functions not defined.

- [ ] **Step 3: Implement token.go**

Create `internal/guacamole/token.go`:

```go
package guacamole

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

type JSONAuthPayload struct {
	Username    string                       `json:"username"`
	Expires     string                       `json:"expires,omitempty"`
	Connections map[string]JSONAuthConnection `json:"connections"`
}

type JSONAuthConnection struct {
	Protocol   string            `json:"protocol"`
	Parameters map[string]string `json:"parameters"`
}

func BuildToken(payload JSONAuthPayload, hexKey string) (string, error) {
	key, err := hex.DecodeString(hexKey)
	if err != nil {
		return "", fmt.Errorf("decoding hex key: %w", err)
	}

	jsonBytes, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("marshaling payload: %w", err)
	}

	mac := hmac.New(sha256.New, key)
	mac.Write(jsonBytes)
	sig := mac.Sum(nil)

	plaintext := append(sig, jsonBytes...)

	// PKCS7 padding
	padLen := aes.BlockSize - (len(plaintext) % aes.BlockSize)
	plaintext = append(plaintext, bytes.Repeat([]byte{byte(padLen)}, padLen)...)

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("creating cipher: %w", err)
	}

	iv := make([]byte, aes.BlockSize)
	mode := cipher.NewCBCEncrypter(block, iv)
	ciphertext := make([]byte, len(plaintext))
	mode.CryptBlocks(ciphertext, plaintext)

	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

func ExchangeToken(guacamoleURL string, token string) (string, error) {
	resp, err := http.PostForm(guacamoleURL+"/api/tokens", url.Values{
		"data": {token},
	})
	if err != nil {
		return "", fmt.Errorf("exchanging token: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("reading response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("token exchange failed (%d): %s", resp.StatusCode, body)
	}

	var result struct {
		AuthToken string `json:"authToken"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("parsing response: %w", err)
	}
	return result.AuthToken, nil
}
```

Add a helper used in the test — or use `base64.StdEncoding.DecodeString` directly. Update the test to use `base64.StdEncoding.DecodeString` instead of `base64Decode`:

Replace `base64Decode(token)` in the test with `base64.StdEncoding.DecodeString(token)` and add `"encoding/base64"` to the test imports.

- [ ] **Step 4: Run tests to verify they pass**

```bash
go test ./internal/guacamole/ -v
```

Expected: all PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/guacamole/
git commit -m "feat: guacamole JSON auth token generation"
```

---

### Task 3: JupyterHub API Client

**Files:**
- Create: `internal/hub/client.go`
- Create: `internal/hub/client_test.go`

**Interfaces:**
- Consumes: nothing (standalone HTTP client)
- Produces: `Client` struct, `NewClient(apiURL string, apiToken string) *Client`, `Client.SpawnServer(username, serverName, profile string) error`, `Client.StopServer(username, serverName string) error`, `Client.GetUser(username string) (*User, error)`, `User` struct with `Name string`, `Servers map[string]Server`, `Server` struct with `Name`, `Ready`, `Pending`, `URL` fields

JupyterHub API auth: `Authorization: token <apiToken>` header on all requests.

- [ ] **Step 1: Write the failing test**

Create `internal/hub/client_test.go`:

```go
package hub

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGetUser(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "token test-token" {
			t.Errorf("Authorization = %q", r.Header.Get("Authorization"))
		}
		if r.URL.Path != "/users/alice" {
			t.Errorf("Path = %q, want /users/alice", r.URL.Path)
		}
		json.NewEncoder(w).Encode(User{
			Name: "alice",
			Servers: map[string]Server{
				"my-notebook": {Name: "my-notebook", Ready: true, URL: "/user/alice/my-notebook/"},
			},
		})
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "test-token")
	user, err := client.GetUser("alice")
	if err != nil {
		t.Fatalf("GetUser() error: %v", err)
	}
	if user.Name != "alice" {
		t.Errorf("Name = %q, want %q", user.Name, "alice")
	}
	if len(user.Servers) != 1 {
		t.Fatalf("len(Servers) = %d, want 1", len(user.Servers))
	}
	s := user.Servers["my-notebook"]
	if !s.Ready || s.URL != "/user/alice/my-notebook/" {
		t.Errorf("Server = %+v", s)
	}
}

func TestSpawnServer(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("Method = %q, want POST", r.Method)
		}
		if r.URL.Path != "/users/alice/servers/my-desktop" {
			t.Errorf("Path = %q", r.URL.Path)
		}
		var body map[string]string
		json.NewDecoder(r.Body).Decode(&body)
		if body["profile"] != "Ubuntu Desktop (KasmVNC)" {
			t.Errorf("profile = %q", body["profile"])
		}
		w.WriteHeader(http.StatusCreated)
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "test-token")
	err := client.SpawnServer("alice", "my-desktop", "Ubuntu Desktop (KasmVNC)")
	if err != nil {
		t.Fatalf("SpawnServer() error: %v", err)
	}
}

func TestStopServer(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("Method = %q, want DELETE", r.Method)
		}
		if r.URL.Path != "/users/alice/servers/my-desktop" {
			t.Errorf("Path = %q", r.URL.Path)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "test-token")
	err := client.StopServer("alice", "my-desktop")
	if err != nil {
		t.Fatalf("StopServer() error: %v", err)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
go test ./internal/hub/ -v
```

Expected: compilation error — types not defined.

- [ ] **Step 3: Implement client.go**

Create `internal/hub/client.go`:

```go
package hub

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

type Client struct {
	apiURL   string
	apiToken string
	http     *http.Client
}

type User struct {
	Name    string            `json:"name"`
	Servers map[string]Server `json:"servers"`
}

type Server struct {
	Name    string `json:"name"`
	Ready   bool   `json:"ready"`
	Pending bool   `json:"pending"`
	URL     string `json:"url"`
}

func NewClient(apiURL, apiToken string) *Client {
	return &Client{
		apiURL:   apiURL,
		apiToken: apiToken,
		http:     &http.Client{},
	}
}

func (c *Client) do(method, path string, body any) (*http.Response, error) {
	var reqBody io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		reqBody = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, c.apiURL+path, reqBody)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "token "+c.apiToken)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return c.http.Do(req)
}

func (c *Client) GetUser(username string) (*User, error) {
	resp, err := c.do(http.MethodGet, "/users/"+username, nil)
	if err != nil {
		return nil, fmt.Errorf("getting user: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("get user failed (%d): %s", resp.StatusCode, body)
	}

	var user User
	if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
		return nil, fmt.Errorf("decoding user: %w", err)
	}
	return &user, nil
}

func (c *Client) SpawnServer(username, serverName, profile string) error {
	body := map[string]string{"profile": profile}
	resp, err := c.do(http.MethodPost, "/users/"+username+"/servers/"+serverName, body)
	if err != nil {
		return fmt.Errorf("spawning server: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("spawn failed (%d): %s", resp.StatusCode, b)
	}
	return nil
}

func (c *Client) StopServer(username, serverName string) error {
	resp, err := c.do(http.MethodDelete, "/users/"+username+"/servers/"+serverName, nil)
	if err != nil {
		return fmt.Errorf("stopping server: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("stop failed (%d): %s", resp.StatusCode, b)
	}
	return nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
go test ./internal/hub/ -v
```

Expected: all PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/hub/
git commit -m "feat: JupyterHub API client"
```

---

### Task 4: OIDC Auth Middleware

**Files:**
- Create: `internal/auth/oidc.go`
- Create: `internal/auth/oidc_test.go`

**Interfaces:**
- Consumes: `config.OIDCConfig` (from Task 1)
- Produces: `Auth` struct, `NewAuth(cfg config.OIDCConfig) (*Auth, error)`, `Auth.LoginHandler() http.HandlerFunc`, `Auth.CallbackHandler() http.HandlerFunc`, `Auth.Middleware(next http.Handler) http.Handler`, `Auth.UserFromContext(ctx context.Context) string`

Dependencies: `github.com/coreos/go-oidc/v3/oidc`, `golang.org/x/oauth2`

- [ ] **Step 1: Write the failing test**

Create `internal/auth/oidc_test.go`:

```go
package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestUserFromContext(t *testing.T) {
	ctx := context.WithValue(context.Background(), userContextKey, "alice")
	if got := UserFromContext(ctx); got != "alice" {
		t.Errorf("UserFromContext() = %q, want %q", got, "alice")
	}
}

func TestUserFromContext_Empty(t *testing.T) {
	if got := UserFromContext(context.Background()); got != "" {
		t.Errorf("UserFromContext() = %q, want empty", got)
	}
}

func TestMiddleware_NoCookie(t *testing.T) {
	a := &Auth{}
	handler := a.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called")
	}))

	req := httptest.NewRequest("GET", "/api/sessions", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("Status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
go test ./internal/auth/ -v
```

Expected: compilation error.

- [ ] **Step 3: Implement oidc.go**

Create `internal/auth/oidc.go`:

```go
package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
)

type contextKey string

const (
	userContextKey contextKey = "user"
	sessionCookie             = "tostada_session"
)

type Auth struct {
	oauth2Config *oauth2.Config
	verifier     *oidc.IDTokenVerifier
	sessions     map[string]session
}

type session struct {
	username string
	expiry   time.Time
}

func NewAuth(ctx context.Context, issuerURL, clientID, clientSecret, redirectURL string) (*Auth, error) {
	provider, err := oidc.NewProvider(ctx, issuerURL)
	if err != nil {
		return nil, err
	}
	return &Auth{
		oauth2Config: &oauth2.Config{
			ClientID:     clientID,
			ClientSecret: clientSecret,
			RedirectURL:  redirectURL,
			Endpoint:     provider.Endpoint(),
			Scopes:       []string{oidc.ScopeOpenID, "profile", "email"},
		},
		verifier: provider.Verifier(&oidc.Config{ClientID: clientID}),
		sessions: make(map[string]session),
	}, nil
}

func (a *Auth) LoginHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		state := generateState()
		http.SetCookie(w, &http.Cookie{
			Name:     "oauth_state",
			Value:    state,
			Path:     "/",
			MaxAge:   300,
			HttpOnly: true,
			SameSite: http.SameSiteLaxMode,
		})
		http.Redirect(w, r, a.oauth2Config.AuthCodeURL(state), http.StatusFound)
	}
}

func (a *Auth) CallbackHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		stateCookie, err := r.Cookie("oauth_state")
		if err != nil || stateCookie.Value != r.URL.Query().Get("state") {
			http.Error(w, "invalid state", http.StatusBadRequest)
			return
		}

		token, err := a.oauth2Config.Exchange(r.Context(), r.URL.Query().Get("code"))
		if err != nil {
			http.Error(w, "token exchange failed", http.StatusInternalServerError)
			return
		}

		rawIDToken, ok := token.Extra("id_token").(string)
		if !ok {
			http.Error(w, "no id_token", http.StatusInternalServerError)
			return
		}

		idToken, err := a.verifier.Verify(r.Context(), rawIDToken)
		if err != nil {
			http.Error(w, "token verification failed", http.StatusInternalServerError)
			return
		}

		var claims struct {
			PreferredUsername string `json:"preferred_username"`
			Email            string `json:"email"`
		}
		idToken.Claims(&claims)

		username := claims.PreferredUsername
		if username == "" {
			username = claims.Email
		}

		sessionID := generateState()
		a.sessions[sessionID] = session{
			username: username,
			expiry:   time.Now().Add(24 * time.Hour),
		}

		http.SetCookie(w, &http.Cookie{
			Name:     sessionCookie,
			Value:    sessionID,
			Path:     "/",
			MaxAge:   86400,
			HttpOnly: true,
			SameSite: http.SameSiteLaxMode,
		})
		http.Redirect(w, r, "/", http.StatusFound)
	}
}

func (a *Auth) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(sessionCookie)
		if err != nil {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		sess, ok := a.sessions[cookie.Value]
		if !ok || time.Now().After(sess.expiry) {
			http.Error(w, "session expired", http.StatusUnauthorized)
			return
		}
		ctx := context.WithValue(r.Context(), userContextKey, sess.username)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func UserFromContext(ctx context.Context) string {
	v, _ := ctx.Value(userContextKey).(string)
	return v
}

func (a *Auth) CurrentUser(w http.ResponseWriter, r *http.Request) {
	username := UserFromContext(r.Context())
	json.NewEncoder(w).Encode(map[string]string{"username": username})
}

func generateState() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}
```

Then add dependencies:

```bash
go get github.com/coreos/go-oidc/v3/oidc
go get golang.org/x/oauth2
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
go test ./internal/auth/ -v
```

Expected: all PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/auth/ go.mod go.sum
git commit -m "feat: OIDC auth with session middleware"
```

---

### Task 5: API Routes + Session Management

**Files:**
- Create: `internal/api/router.go`
- Create: `internal/api/workspaces.go`
- Create: `internal/api/sessions.go`
- Create: `internal/api/router_test.go`

**Interfaces:**
- Consumes: `config.Config` (Task 1), `hub.Client` (Task 3), `guacamole.BuildToken` + `guacamole.ExchangeToken` (Task 2), `auth.Auth` + `auth.UserFromContext` (Task 4)
- Produces: `NewRouter(cfg *config.Config, hubClient *hub.Client, authProvider *auth.Auth) http.Handler`

- [ ] **Step 1: Write the failing test**

Create `internal/api/router_test.go`:

```go
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
```

- [ ] **Step 2: Run test to verify it fails**

```bash
go test ./internal/api/ -v
```

Expected: compilation error.

- [ ] **Step 3: Implement workspaces.go**

Create `internal/api/workspaces.go`:

```go
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
```

- [ ] **Step 4: Implement sessions.go**

Create `internal/api/sessions.go`:

```go
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
	serverName := strings.TrimPrefix(r.URL.Path, "/api/sessions/")

	if err := h.hubClient.StopServer(username, serverName); err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *sessionsHandler) connect(w http.ResponseWriter, r *http.Request) {
	username := auth.UserFromContext(r.Context())
	path := strings.TrimPrefix(r.URL.Path, "/api/sessions/")
	serverName := strings.TrimSuffix(path, "/connect")

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
```

- [ ] **Step 5: Implement router.go**

Create `internal/api/router.go`:

```go
package api

import (
	"net/http"

	"github.com/rophy/tostada/internal/auth"
	"github.com/rophy/tostada/internal/config"
	"github.com/rophy/tostada/internal/hub"
)

func NewRouter(cfg *config.Config, hubClient *hub.Client, authProvider *auth.Auth) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/auth/login", authProvider.LoginHandler())
	mux.HandleFunc("GET /api/auth/callback", authProvider.CallbackHandler())

	sessions := &sessionsHandler{
		hubClient:  hubClient,
		workspaces: cfg.Workspaces,
		guacCfg:    cfg.Guacamole,
		hubCfg:     cfg.JupyterHub,
	}

	authed := http.NewServeMux()
	authed.HandleFunc("GET /api/auth/me", authProvider.CurrentUser)
	authed.Handle("GET /api/workspaces", &workspacesHandler{workspaces: cfg.Workspaces})
	authed.HandleFunc("GET /api/sessions", sessions.list)
	authed.HandleFunc("POST /api/sessions", sessions.create)
	authed.HandleFunc("DELETE /api/sessions/{name}", sessions.stop)
	authed.HandleFunc("GET /api/sessions/{name}/connect", sessions.connect)

	mux.Handle("/api/", authProvider.Middleware(authed))

	return mux
}
```

Update `sessions.go` to use `r.PathValue("name")` instead of manual path parsing for the `stop` and `connect` handlers:

Replace the `stop` method body:
```go
func (h *sessionsHandler) stop(w http.ResponseWriter, r *http.Request) {
	username := auth.UserFromContext(r.Context())
	serverName := r.PathValue("name")
	// ... rest same
}
```

Replace the `connect` method body:
```go
func (h *sessionsHandler) connect(w http.ResponseWriter, r *http.Request) {
	username := auth.UserFromContext(r.Context())
	serverName := r.PathValue("name")
	// ... rest same
}
```

- [ ] **Step 6: Run tests to verify they pass**

```bash
go test ./internal/api/ -v
```

Expected: all PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/api/
git commit -m "feat: API routes with session management"
```

---

### Task 6: React Frontend

**Files:**
- Create: `web/package.json`
- Create: `web/vite.config.ts`
- Create: `web/tsconfig.json`
- Create: `web/index.html`
- Create: `web/src/App.tsx`
- Create: `web/src/main.tsx`
- Create: `web/src/pages/Dashboard.tsx`
- Create: `web/src/components/WorkspaceCard.tsx`
- Create: `web/src/components/SessionList.tsx`
- Create: `web/src/api.ts`

**Interfaces:**
- Consumes: Go backend API (`/api/workspaces`, `/api/sessions`, `/api/sessions/:name/connect`, `/api/auth/me`)
- Produces: Built static files in `web/dist/` for Go embed

- [ ] **Step 1: Scaffold Vite + React project**

```bash
cd /home/rophy/projects/tostada
mkdir -p web/src/pages web/src/components
```

Create `web/package.json`:

```json
{
  "name": "tostada-web",
  "private": true,
  "type": "module",
  "scripts": {
    "dev": "vite",
    "build": "tsc && vite build",
    "preview": "vite preview"
  },
  "dependencies": {
    "react": "^18.3.1",
    "react-dom": "^18.3.1"
  },
  "devDependencies": {
    "@types/react": "^18.3.18",
    "@types/react-dom": "^18.3.5",
    "@vitejs/plugin-react": "^4.3.4",
    "typescript": "^5.6.3",
    "vite": "^6.0.0"
  }
}
```

Create `web/vite.config.ts`:

```typescript
import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

export default defineConfig({
  plugins: [react()],
  server: {
    proxy: {
      '/api': 'http://localhost:8080',
    },
  },
})
```

Create `web/tsconfig.json`:

```json
{
  "compilerOptions": {
    "target": "ES2020",
    "module": "ESNext",
    "lib": ["ES2020", "DOM", "DOM.Iterable"],
    "jsx": "react-jsx",
    "moduleResolution": "bundler",
    "strict": true,
    "outDir": "./dist",
    "skipLibCheck": true
  },
  "include": ["src"]
}
```

Create `web/index.html`:

```html
<!doctype html>
<html lang="en">
  <head>
    <meta charset="UTF-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1.0" />
    <title>Tostada</title>
  </head>
  <body>
    <div id="root"></div>
    <script type="module" src="/src/main.tsx"></script>
  </body>
</html>
```

- [ ] **Step 2: Create API client**

Create `web/src/api.ts`:

```typescript
export interface Workspace {
  name: string
  displayName: string
  description: string
  icon: string
  type: 'jupyterhub' | 'guacamole'
  image: string
  port: number
}

export interface Server {
  name: string
  ready: boolean
  pending: boolean
  url: string
}

export async function fetchWorkspaces(): Promise<Workspace[]> {
  const res = await fetch('/api/workspaces')
  if (res.status === 401) {
    window.location.href = '/api/auth/login'
    return []
  }
  return res.json()
}

export async function fetchSessions(): Promise<Record<string, Server>> {
  const res = await fetch('/api/sessions')
  if (res.status === 401) {
    window.location.href = '/api/auth/login'
    return {}
  }
  return res.json()
}

export async function launchWorkspace(workspace: string, serverName: string): Promise<void> {
  await fetch('/api/sessions', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ workspace, serverName }),
  })
}

export async function stopSession(name: string): Promise<void> {
  await fetch(`/api/sessions/${name}`, { method: 'DELETE' })
}

export async function getConnectURL(name: string): Promise<string> {
  const res = await fetch(`/api/sessions/${name}/connect`)
  const data = await res.json()
  return data.url
}
```

- [ ] **Step 3: Create WorkspaceCard component**

Create `web/src/components/WorkspaceCard.tsx`:

```tsx
import { Workspace } from '../api'

const iconMap: Record<string, string> = {
  notebook: '\u{1F4D3}',
  desktop: '\u{1F5A5}',
  terminal: '\u{1F4BB}',
}

interface Props {
  workspace: Workspace
  onLaunch: (workspace: Workspace) => void
}

export function WorkspaceCard({ workspace, onLaunch }: Props) {
  return (
    <div style={{
      border: '1px solid #ddd',
      borderRadius: '8px',
      padding: '20px',
      display: 'flex',
      flexDirection: 'column',
      gap: '8px',
    }}>
      <div style={{ fontSize: '2rem' }}>
        {iconMap[workspace.icon] || '\u{1F4E6}'}
      </div>
      <h3 style={{ margin: 0 }}>{workspace.displayName}</h3>
      <p style={{ margin: 0, color: '#666', fontSize: '0.9rem' }}>
        {workspace.description}
      </p>
      <button
        onClick={() => onLaunch(workspace)}
        style={{
          marginTop: 'auto',
          padding: '8px 16px',
          background: '#2563eb',
          color: '#fff',
          border: 'none',
          borderRadius: '4px',
          cursor: 'pointer',
        }}
      >
        Launch
      </button>
    </div>
  )
}
```

- [ ] **Step 4: Create SessionList component**

Create `web/src/components/SessionList.tsx`:

```tsx
import { Server } from '../api'

interface Props {
  sessions: Record<string, Server>
  onConnect: (name: string) => void
  onStop: (name: string) => void
}

export function SessionList({ sessions, onConnect, onStop }: Props) {
  const entries = Object.entries(sessions)
  if (entries.length === 0) return null

  return (
    <div>
      <h2>Active Sessions</h2>
      <table style={{ width: '100%', borderCollapse: 'collapse' }}>
        <thead>
          <tr>
            <th style={{ textAlign: 'left', padding: '8px', borderBottom: '1px solid #ddd' }}>Name</th>
            <th style={{ textAlign: 'left', padding: '8px', borderBottom: '1px solid #ddd' }}>Status</th>
            <th style={{ textAlign: 'right', padding: '8px', borderBottom: '1px solid #ddd' }}>Actions</th>
          </tr>
        </thead>
        <tbody>
          {entries.map(([name, server]) => (
            <tr key={name}>
              <td style={{ padding: '8px' }}>{name}</td>
              <td style={{ padding: '8px' }}>
                {server.ready ? '✅ Ready' : server.pending ? '⏳ Starting...' : '⚠️ Unknown'}
              </td>
              <td style={{ padding: '8px', textAlign: 'right' }}>
                {server.ready && (
                  <button
                    onClick={() => onConnect(name)}
                    style={{ marginRight: '8px', padding: '4px 12px', cursor: 'pointer' }}
                  >
                    Connect
                  </button>
                )}
                <button
                  onClick={() => onStop(name)}
                  style={{ padding: '4px 12px', cursor: 'pointer', color: '#dc2626' }}
                >
                  Stop
                </button>
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  )
}
```

- [ ] **Step 5: Create Dashboard page**

Create `web/src/pages/Dashboard.tsx`:

```tsx
import { useEffect, useState } from 'react'
import {
  Workspace, Server,
  fetchWorkspaces, fetchSessions,
  launchWorkspace, stopSession, getConnectURL,
} from '../api'
import { WorkspaceCard } from '../components/WorkspaceCard'
import { SessionList } from '../components/SessionList'

export function Dashboard() {
  const [workspaces, setWorkspaces] = useState<Workspace[]>([])
  const [sessions, setSessions] = useState<Record<string, Server>>({})

  const refresh = async () => {
    const [ws, sess] = await Promise.all([fetchWorkspaces(), fetchSessions()])
    setWorkspaces(ws)
    setSessions(sess)
  }

  useEffect(() => { refresh() }, [])

  const handleLaunch = async (ws: Workspace) => {
    const name = `${ws.name}-${Date.now().toString(36)}`
    await launchWorkspace(ws.name, name)
    refresh()
  }

  const handleConnect = async (name: string) => {
    const url = await getConnectURL(name)
    window.open(url, '_blank')
  }

  const handleStop = async (name: string) => {
    await stopSession(name)
    refresh()
  }

  return (
    <div style={{ maxWidth: '960px', margin: '0 auto', padding: '20px' }}>
      <h1>Tostada</h1>

      <h2>Workspaces</h2>
      <div style={{
        display: 'grid',
        gridTemplateColumns: 'repeat(auto-fill, minmax(260px, 1fr))',
        gap: '16px',
      }}>
        {workspaces.map(ws => (
          <WorkspaceCard key={ws.name} workspace={ws} onLaunch={handleLaunch} />
        ))}
      </div>

      <SessionList sessions={sessions} onConnect={handleConnect} onStop={handleStop} />
    </div>
  )
}
```

- [ ] **Step 6: Create App and main entry point**

Create `web/src/App.tsx`:

```tsx
import { Dashboard } from './pages/Dashboard'

export default function App() {
  return <Dashboard />
}
```

Create `web/src/main.tsx`:

```tsx
import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import App from './App'

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <App />
  </StrictMode>
)
```

- [ ] **Step 7: Install dependencies and verify build**

```bash
cd /home/rophy/projects/tostada/web
npm install
npm run build
```

Expected: `web/dist/` directory created with bundled assets.

- [ ] **Step 8: Commit**

```bash
cd /home/rophy/projects/tostada
git add web/
git commit -m "feat: React frontend with workspace catalog and session management"
```

---

### Task 7: Go Embed, Dockerfile, and K8s Deployment

**Files:**
- Modify: `cmd/tostada/main.go`
- Create: `web/embed.go`
- Create: `Dockerfile`
- Create: `Makefile`
- Create: `skaffold.yaml`
- Create: `k8s/jupyterhub-values.yaml`
- Create: `k8s/guacamole.yaml`
- Create: `k8s/tostada.yaml`

**Interfaces:**
- Consumes: all previous tasks
- Produces: deployable system in kind cluster

- [ ] **Step 1: Create web/embed.go for static file serving**

Create `web/embed.go`:

```go
package web

import "embed"

//go:embed dist/*
var DistFS embed.FS
```

- [ ] **Step 2: Update main.go to wire everything together**

Update `cmd/tostada/main.go`:

```go
package main

import (
	"context"
	"flag"
	"io/fs"
	"log"
	"net/http"

	"github.com/rophy/tostada/internal/api"
	"github.com/rophy/tostada/internal/auth"
	"github.com/rophy/tostada/internal/config"
	"github.com/rophy/tostada/internal/hub"
	"github.com/rophy/tostada/web"
)

func main() {
	configPath := flag.String("config", "config.yaml", "path to config file")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	authProvider, err := auth.NewAuth(
		context.Background(),
		cfg.OIDC.IssuerURL,
		cfg.OIDC.ClientID,
		cfg.OIDC.ClientSecret,
		cfg.OIDC.RedirectURL,
	)
	if err != nil {
		log.Fatalf("Failed to initialize auth: %v", err)
	}

	hubClient := hub.NewClient(cfg.JupyterHub.APIURL, cfg.JupyterHub.APIToken)

	mux := api.NewRouter(cfg, hubClient, authProvider)

	distFS, err := fs.Sub(web.DistFS, "dist")
	if err != nil {
		log.Fatalf("Failed to create sub FS: %v", err)
	}
	mux.(*http.ServeMux).Handle("GET /", http.FileServer(http.FS(distFS)))

	log.Printf("Tostada listening on %s", cfg.Server.Addr)
	log.Fatal(http.ListenAndServe(cfg.Server.Addr, mux))
}
```

Note: This requires `NewRouter` to return `*http.ServeMux` instead of `http.Handler`, or use a wrapper. Simpler: update `router.go` to return `*http.ServeMux` and update `main.go` to use the mux directly. Alternatively, register the file server inside `NewRouter` by passing the `fs.FS`. Choose whichever is cleaner at implementation time.

- [ ] **Step 3: Create Dockerfile**

Create `Dockerfile`:

```dockerfile
FROM node:22-alpine AS frontend
WORKDIR /app/web
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ ./
RUN npm run build

FROM golang:1.22-alpine AS backend
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=frontend /app/web/dist ./web/dist
RUN CGO_ENABLED=0 go build -o /tostada ./cmd/tostada/

FROM alpine:3.20
RUN apk add --no-cache ca-certificates
COPY --from=backend /tostada /tostada
COPY config.yaml /etc/tostada/config.yaml
ENTRYPOINT ["/tostada", "-config", "/etc/tostada/config.yaml"]
```

- [ ] **Step 4: Create JupyterHub Helm values**

Create `k8s/jupyterhub-values.yaml`:

```yaml
hub:
  config:
    JupyterHub:
      allow_named_servers: true
    Authenticator:
      enable_auth_state: true
      admin_users:
        - admin
  services:
    tostada:
      api_token: "changeme-tostada-api-token"
  loadRoles:
    - name: tostada-service
      scopes:
        - "admin:users"
        - "admin:servers"
        - "read:users"
      services:
        - tostada

singleuser:
  profileList:
    - display_name: "Jupyter Notebook"
      kubespawner_override:
        image: jupyter/minimal-notebook:latest
        cmd: ["jupyterhub-singleuser"]
        default_url: "/lab"
    - display_name: "Ubuntu Desktop (KasmVNC)"
      kubespawner_override:
        image: kasmweb/ubuntu-noble-desktop:1.16.1
        cmd: ["/dockerstartup/kasm_default_profile.sh"]
        port: 6901
    - display_name: "Ubuntu Desktop (xRDP)"
      kubespawner_override:
        image: scottyhardy/docker-remote-desktop:latest
        cmd: null
        port: 3389
        extra_annotations:
          tostada.dev/type: guacamole
        volume_mounts:
          - name: dshm
            mountPath: /dev/shm
        volumes:
          - name: dshm
            emptyDir:
              medium: Memory
  storage:
    capacity: 256Mi

proxy:
  service:
    type: NodePort
```

- [ ] **Step 5: Create guacamole Kubernetes manifest**

Create `k8s/guacamole.yaml`:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: guacamole
  namespace: tostada
spec:
  replicas: 1
  selector:
    matchLabels:
      app: guacamole
  template:
    metadata:
      labels:
        app: guacamole
    spec:
      containers:
        - name: guacd
          image: guacamole/guacd:1.6.0
          ports:
            - containerPort: 4822
        - name: guacamole
          image: guacamole/guacamole:1.6.0
          ports:
            - containerPort: 8080
          env:
            - name: GUACD_HOSTNAME
              value: "localhost"
            - name: GUACD_PORT
              value: "4822"
            - name: JSON_SECRET_KEY
              value: "8bea5deb816809f814b83c28cf14d3e2"
            - name: WEBAPP_CONTEXT
              value: "ROOT"
---
apiVersion: v1
kind: Service
metadata:
  name: guacamole
  namespace: tostada
spec:
  selector:
    app: guacamole
  ports:
    - port: 8080
      targetPort: 8080
```

- [ ] **Step 6: Create Tostada portal Kubernetes manifest**

Create `k8s/tostada.yaml`:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: tostada
  namespace: tostada
spec:
  replicas: 1
  selector:
    matchLabels:
      app: tostada
  template:
    metadata:
      labels:
        app: tostada
    spec:
      containers:
        - name: tostada
          image: tostada
          ports:
            - containerPort: 8080
---
apiVersion: v1
kind: Service
metadata:
  name: tostada
  namespace: tostada
spec:
  selector:
    app: tostada
  ports:
    - port: 8080
      targetPort: 8080
```

- [ ] **Step 7: Create skaffold.yaml**

Create `skaffold.yaml`:

```yaml
apiVersion: skaffold/v4beta11
kind: Config
metadata:
  name: tostada
build:
  artifacts:
    - image: tostada
      docker:
        dockerfile: Dockerfile
deploy:
  helm:
    releases:
      - name: jupyterhub
        remoteChart: jupyterhub
        repo: https://hub.jupyter.org/helm-chart/
        version: 4.1.0
        namespace: tostada
        createNamespace: true
        valuesFiles:
          - k8s/jupyterhub-values.yaml
  kubectl:
    manifests:
      - k8s/guacamole.yaml
      - k8s/tostada.yaml
```

- [ ] **Step 8: Create Makefile**

Create `Makefile`:

```makefile
.PHONY: up dev down build test

CLUSTER_NAME := tostada

up:
	kind create cluster --name $(CLUSTER_NAME) 2>/dev/null || true
	kubectl create namespace tostada 2>/dev/null || true
	skaffold run

dev:
	kind create cluster --name $(CLUSTER_NAME) 2>/dev/null || true
	kubectl create namespace tostada 2>/dev/null || true
	skaffold dev

down:
	skaffold delete
	kind delete cluster --name $(CLUSTER_NAME)

build:
	cd web && npm run build
	go build ./cmd/tostada/

test:
	go test ./... -v
```

- [ ] **Step 9: Verify Go build with embed**

```bash
cd /home/rophy/projects/tostada/web
npm run build
cd /home/rophy/projects/tostada
go build ./cmd/tostada/
```

- [ ] **Step 10: Commit**

```bash
git add web/embed.go cmd/tostada/main.go Dockerfile Makefile skaffold.yaml k8s/
git commit -m "feat: Dockerfile, k8s manifests, and Makefile for kind deployment"
```
