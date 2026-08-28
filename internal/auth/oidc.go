package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strings"
	"sync"
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
	mu           sync.RWMutex
}

type session struct {
	username string
	expiry   time.Time
}

func NewAuth(ctx context.Context, issuerURL, internalURL, clientID, clientSecret, redirectURL string) (*Auth, error) {
	discoveryURL := issuerURL
	if internalURL != "" {
		discoveryURL = internalURL
		ctx = oidc.InsecureIssuerURLContext(ctx, issuerURL)
	}
	provider, err := oidc.NewProvider(ctx, discoveryURL)
	if err != nil {
		return nil, err
	}

	endpoint := provider.Endpoint()
	if internalURL != "" {
		// AuthURL is browser-facing, must use public issuer URL
		endpoint.AuthURL = strings.Replace(endpoint.AuthURL, internalURL, issuerURL, 1)
		// TokenURL is server-to-server, keep internal (already from discovery)
	}

	return &Auth{
		oauth2Config: &oauth2.Config{
			ClientID:     clientID,
			ClientSecret: clientSecret,
			RedirectURL:  redirectURL,
			Endpoint:     endpoint,
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
			Email             string `json:"email"`
		}
		idToken.Claims(&claims)

		username := claims.PreferredUsername
		if username == "" {
			username = claims.Email
		}

		sessionID := generateState()
		a.mu.Lock()
		a.sessions[sessionID] = session{
			username: username,
			expiry:   time.Now().Add(24 * time.Hour),
		}
		a.mu.Unlock()

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
		a.mu.RLock()
		sess, ok := a.sessions[cookie.Value]
		a.mu.RUnlock()
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
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"username": username})
}

func generateState() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}
