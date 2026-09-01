package auth

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	josejwt "github.com/go-jose/go-jose/v4"
	"github.com/rophy/tostada/internal/model"
	"github.com/rophy/tostada/internal/audit"
	"golang.org/x/oauth2"
)

type fakeUserStore struct {
	ensured []string
}

func (f *fakeUserStore) GetUser(ctx context.Context, username string) (*model.User, error) {
	return nil, nil
}
func (f *fakeUserStore) ListUsers(ctx context.Context) ([]model.User, error) { return nil, nil }
func (f *fakeUserStore) EnsureUser(ctx context.Context, username string) (*model.User, error) {
	f.ensured = append(f.ensured, username)
	return &model.User{Username: username}, nil
}
func (f *fakeUserStore) UpdateUser(ctx context.Context, username string, updates map[string]any) error {
	return nil
}
func (f *fakeUserStore) DeleteUser(ctx context.Context, username string) error { return nil }
func (f *fakeUserStore) IsAdmin(ctx context.Context, username string) (bool, error) {
	return false, nil
}

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

func TestWithUser(t *testing.T) {
	ctx := WithUser(context.Background(), "bob")
	if got := UserFromContext(ctx); got != "bob" {
		t.Errorf("UserFromContext(WithUser(bob)) = %q, want %q", got, "bob")
	}
}

func TestLogoutHandler(t *testing.T) {
	a := &Auth{sessions: map[string]session{
		"sess123": {username: "alice"},
	}}

	req := httptest.NewRequest("POST", "/api/auth/logout", nil)
	req.AddCookie(&http.Cookie{Name: "tostada_session", Value: "sess123"})
	rec := httptest.NewRecorder()

	a.LogoutHandler()(rec, req)

	if rec.Code != http.StatusFound {
		t.Errorf("Status = %d, want %d", rec.Code, http.StatusFound)
	}
	if _, ok := a.sessions["sess123"]; ok {
		t.Error("session should have been deleted")
	}
	cookies := rec.Result().Cookies()
	found := false
	for _, c := range cookies {
		if c.Name == "tostada_session" && c.MaxAge < 0 {
			found = true
		}
	}
	if !found {
		t.Error("session cookie should be cleared with MaxAge < 0")
	}
}

func TestLogoutHandler_NoCookie(t *testing.T) {
	a := &Auth{sessions: map[string]session{}}

	req := httptest.NewRequest("POST", "/api/auth/logout", nil)
	rec := httptest.NewRecorder()

	a.LogoutHandler()(rec, req)

	if rec.Code != http.StatusFound {
		t.Errorf("Status = %d, want %d", rec.Code, http.StatusFound)
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

func TestMiddleware_ValidSession(t *testing.T) {
	a := &Auth{sessions: map[string]session{
		"valid-sess": {username: "alice", expiry: time.Now().Add(1 * time.Hour)},
	}}

	var gotUsername string
	handler := a.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUsername = UserFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/api/sessions", nil)
	req.AddCookie(&http.Cookie{Name: "tostada_session", Value: "valid-sess"})
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Status = %d, want %d", rec.Code, http.StatusOK)
	}
	if gotUsername != "alice" {
		t.Errorf("username = %q, want %q", gotUsername, "alice")
	}
}

func TestMiddleware_ExpiredSession(t *testing.T) {
	a := &Auth{sessions: map[string]session{
		"expired-sess": {username: "alice", expiry: time.Now().Add(-1 * time.Hour)},
	}}

	handler := a.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called for expired session")
	}))

	req := httptest.NewRequest("GET", "/api/sessions", nil)
	req.AddCookie(&http.Cookie{Name: "tostada_session", Value: "expired-sess"})
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("Status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestMiddleware_InvalidSession(t *testing.T) {
	a := &Auth{sessions: map[string]session{
		"real-sess": {username: "alice", expiry: time.Now().Add(1 * time.Hour)},
	}}

	handler := a.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called for invalid session")
	}))

	req := httptest.NewRequest("GET", "/api/sessions", nil)
	req.AddCookie(&http.Cookie{Name: "tostada_session", Value: "wrong-sess"})
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("Status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestCurrentUser(t *testing.T) {
	a := &Auth{}

	req := httptest.NewRequest("GET", "/api/auth/me", nil)
	req = req.WithContext(WithUser(req.Context(), "bob"))
	rec := httptest.NewRecorder()

	a.CurrentUser(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Status = %d, want %d", rec.Code, http.StatusOK)
	}

	var resp map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("Decode error: %v", err)
	}
	if resp["username"] != "bob" {
		t.Errorf("username = %v, want %q", resp["username"], "bob")
	}
	if resp["isAdmin"] != false {
		t.Errorf("isAdmin = %v, want false", resp["isAdmin"])
	}
}

func TestLoginHandler(t *testing.T) {
	a := &Auth{
		oauth2Config: &oauth2.Config{
			ClientID: "test-client",
			Endpoint: oauth2.Endpoint{
				AuthURL: "https://example.com/authorize",
			},
			RedirectURL: "https://example.com/callback",
			Scopes:      []string{"openid"},
		},
	}

	req := httptest.NewRequest("GET", "/api/auth/login", nil)
	rec := httptest.NewRecorder()

	a.LoginHandler()(rec, req)

	if rec.Code != http.StatusFound {
		t.Errorf("Status = %d, want %d", rec.Code, http.StatusFound)
	}

	location := rec.Header().Get("Location")
	if location == "" {
		t.Fatal("Location header is empty")
	}

	var foundStateCookie bool
	for _, c := range rec.Result().Cookies() {
		if c.Name == "oauth_state" && c.Value != "" {
			foundStateCookie = true
		}
	}
	if !foundStateCookie {
		t.Error("oauth_state cookie not set")
	}
}

func TestGenerateState(t *testing.T) {
	s1 := generateState()
	s2 := generateState()

	if len(s1) != 32 {
		t.Errorf("len(generateState()) = %d, want 32", len(s1))
	}
	if s1 == s2 {
		t.Error("two calls to generateState() returned the same value")
	}
}

func TestNewAuth(t *testing.T) {
	oidcSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/.well-known/openid-configuration" {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{
				"issuer":                 "http://" + r.Host,
				"authorization_endpoint": "http://" + r.Host + "/authorize",
				"token_endpoint":         "http://" + r.Host + "/token",
				"jwks_uri":               "http://" + r.Host + "/jwks",
			})
			return
		}
		if r.URL.Path == "/jwks" {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{"keys": []any{}})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer oidcSrv.Close()

	a, err := NewAuth(context.Background(), oidcSrv.URL, "", "client-id", "client-secret", "http://localhost/callback", nil, nil)
	if err != nil {
		t.Fatalf("NewAuth: %v", err)
	}
	if a == nil {
		t.Fatal("NewAuth returned nil")
	}
	if a.oauth2Config.ClientID != "client-id" {
		t.Errorf("ClientID = %q, want %q", a.oauth2Config.ClientID, "client-id")
	}
}

func TestNewAuth_WithInternalURL(t *testing.T) {
	oidcSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/.well-known/openid-configuration" {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{
				"issuer":                 "https://external-issuer.example.com",
				"authorization_endpoint": "https://external-issuer.example.com/authorize",
				"token_endpoint":         "http://" + r.Host + "/token",
				"jwks_uri":               "http://" + r.Host + "/jwks",
			})
			return
		}
		if r.URL.Path == "/jwks" {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{"keys": []any{}})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer oidcSrv.Close()

	a, err := NewAuth(context.Background(), "https://external-issuer.example.com", oidcSrv.URL, "client-id", "secret", "http://localhost/callback", nil, nil)
	if err != nil {
		t.Fatalf("NewAuth with internalURL: %v", err)
	}
	if a == nil {
		t.Fatal("NewAuth returned nil")
	}
}

func TestNewAuth_DiscoveryFails(t *testing.T) {
	_, err := NewAuth(context.Background(), "http://127.0.0.1:1", "", "client-id", "secret", "http://localhost/callback", nil, nil)
	if err == nil {
		t.Fatal("expected error when discovery endpoint unreachable")
	}
}

func TestCallbackHandler_NoStateCookie(t *testing.T) {
	a := &Auth{
		oauth2Config: &oauth2.Config{},
		sessions:     make(map[string]session),
	}

	req := httptest.NewRequest("GET", "/api/auth/callback?state=abc&code=xyz", nil)
	rec := httptest.NewRecorder()
	a.CallbackHandler()(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("Status = %d, want 400", rec.Code)
	}
}

func TestCallbackHandler_StateMismatch(t *testing.T) {
	a := &Auth{
		oauth2Config: &oauth2.Config{},
		sessions:     make(map[string]session),
	}

	req := httptest.NewRequest("GET", "/api/auth/callback?state=wrong&code=xyz", nil)
	req.AddCookie(&http.Cookie{Name: "oauth_state", Value: "correct"})
	rec := httptest.NewRecorder()
	a.CallbackHandler()(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("Status = %d, want 400", rec.Code)
	}
}

func TestCallbackHandler_NoIDToken(t *testing.T) {
	tokenSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"access_token": "access-tok",
			"token_type":   "Bearer",
		})
	}))
	defer tokenSrv.Close()

	a := &Auth{
		oauth2Config: &oauth2.Config{
			ClientID: "test",
			Endpoint: oauth2.Endpoint{
				TokenURL: tokenSrv.URL + "/token",
			},
		},
		sessions: make(map[string]session),
	}

	req := httptest.NewRequest("GET", "/api/auth/callback?state=abc&code=xyz", nil)
	req.AddCookie(&http.Cookie{Name: "oauth_state", Value: "abc"})
	rec := httptest.NewRecorder()
	a.CallbackHandler()(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("Status = %d, want 500 (no id_token)", rec.Code)
	}
}

func TestCallbackHandler_VerificationFails(t *testing.T) {
	tokenSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"access_token": "access-tok",
			"token_type":   "Bearer",
			"id_token":     "not-a-valid-jwt",
		})
	}))
	defer tokenSrv.Close()

	oidcSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/.well-known/openid-configuration" {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{
				"issuer":                 "http://" + r.Host,
				"authorization_endpoint": "http://" + r.Host + "/authorize",
				"token_endpoint":         tokenSrv.URL + "/token",
				"jwks_uri":               "http://" + r.Host + "/jwks",
			})
			return
		}
		if r.URL.Path == "/jwks" {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{"keys": []any{}})
			return
		}
	}))
	defer oidcSrv.Close()

	a, err := NewAuth(context.Background(), oidcSrv.URL, "", "test-client", "secret", "http://localhost/callback", nil, nil)
	if err != nil {
		t.Fatalf("NewAuth: %v", err)
	}
	a.oauth2Config.Endpoint.TokenURL = tokenSrv.URL + "/token"

	req := httptest.NewRequest("GET", "/api/auth/callback?state=abc&code=xyz", nil)
	req.AddCookie(&http.Cookie{Name: "oauth_state", Value: "abc"})
	rec := httptest.NewRecorder()
	a.CallbackHandler()(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("Status = %d, want 500 (verification should fail)", rec.Code)
	}
}

func TestCallbackHandler_TokenExchangeFails(t *testing.T) {
	tokenSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error":"invalid_grant"}`))
	}))
	defer tokenSrv.Close()

	a := &Auth{
		oauth2Config: &oauth2.Config{
			ClientID: "test",
			Endpoint: oauth2.Endpoint{
				TokenURL: tokenSrv.URL + "/token",
			},
		},
		sessions: make(map[string]session),
	}

	req := httptest.NewRequest("GET", "/api/auth/callback?state=abc&code=xyz", nil)
	req.AddCookie(&http.Cookie{Name: "oauth_state", Value: "abc"})
	rec := httptest.NewRecorder()
	a.CallbackHandler()(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("Status = %d, want 500", rec.Code)
	}
}

func TestCallbackHandler_Success(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}

	jwk := josejwt.JSONWebKey{Key: &key.PublicKey, KeyID: "test-key", Algorithm: "RS256", Use: "sig"}

	var oidcSrv *httptest.Server
	oidcSrv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/.well-known/openid-configuration":
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{
				"issuer":                 oidcSrv.URL,
				"authorization_endpoint": oidcSrv.URL + "/authorize",
				"token_endpoint":         oidcSrv.URL + "/token",
				"jwks_uri":               oidcSrv.URL + "/jwks",
			})
		case "/jwks":
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(josejwt.JSONWebKeySet{Keys: []josejwt.JSONWebKey{jwk}})
		case "/token":
			signer, err := josejwt.NewSigner(josejwt.SigningKey{Algorithm: josejwt.RS256, Key: key}, (&josejwt.SignerOptions{}).WithHeader("kid", "test-key"))
			if err != nil {
				t.Fatalf("NewSigner: %v", err)
			}
			claims := map[string]any{
				"iss":                oidcSrv.URL,
				"aud":                "client-id",
				"sub":                "user-123",
				"exp":                time.Now().Add(1 * time.Hour).Unix(),
				"iat":                time.Now().Unix(),
				"preferred_username": "alice",
			}
			payload, _ := json.Marshal(claims)
			jws, err := signer.Sign(payload)
			if err != nil {
				t.Fatalf("Sign: %v", err)
			}
			idToken, err := jws.CompactSerialize()
			if err != nil {
				t.Fatalf("CompactSerialize: %v", err)
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{
				"access_token": "access-tok",
				"token_type":   "Bearer",
				"id_token":     idToken,
			})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer oidcSrv.Close()

	logPath := filepath.Join(t.TempDir(), "audit.jsonl")
	auditLog := audit.NewAuditLog(logPath, 50, 5)
	defer auditLog.Close()

	userStore := &fakeUserStore{}

	a, err := NewAuth(context.Background(), oidcSrv.URL, "", "client-id", "secret", "http://localhost/callback", userStore, auditLog)
	if err != nil {
		t.Fatalf("NewAuth: %v", err)
	}

	req := httptest.NewRequest("GET", "/api/auth/callback?state=abc&code=xyz", nil)
	req.AddCookie(&http.Cookie{Name: "oauth_state", Value: "abc"})
	rec := httptest.NewRecorder()
	a.CallbackHandler()(rec, req)

	if rec.Code != http.StatusFound {
		t.Fatalf("Status = %d, want %d, body=%s", rec.Code, http.StatusFound, rec.Body.String())
	}

	if len(userStore.ensured) != 1 || userStore.ensured[0] != "alice" {
		t.Errorf("EnsureUser calls = %v, want [alice]", userStore.ensured)
	}

	var sessionCookieFound bool
	for _, c := range rec.Result().Cookies() {
		if c.Name == sessionCookie && c.Value != "" {
			sessionCookieFound = true
		}
	}
	if !sessionCookieFound {
		t.Error("session cookie not set")
	}

	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if !strings.Contains(string(data), "auth.login") {
		t.Errorf("audit log missing auth.login event, got: %s", data)
	}
	if !strings.Contains(string(data), "alice") {
		t.Errorf("audit log missing username, got: %s", data)
	}
}

func TestLogoutHandler_WithAuditLog(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "audit.jsonl")
	auditLog := audit.NewAuditLog(logPath, 50, 5)
	defer auditLog.Close()

	a := &Auth{
		sessions: map[string]session{
			"sess123": {username: "alice"},
		},
		auditLog: auditLog,
	}

	req := httptest.NewRequest("POST", "/api/auth/logout", nil)
	req.AddCookie(&http.Cookie{Name: "tostada_session", Value: "sess123"})
	rec := httptest.NewRecorder()

	a.LogoutHandler()(rec, req)

	if rec.Code != http.StatusFound {
		t.Errorf("Status = %d, want %d", rec.Code, http.StatusFound)
	}

	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if !strings.Contains(string(data), "auth.logout") {
		t.Errorf("audit log missing auth.logout event, got: %s", data)
	}
	if !strings.Contains(string(data), "alice") {
		t.Errorf("audit log missing username, got: %s", data)
	}
}
