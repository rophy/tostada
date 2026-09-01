package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"log"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/rophy/tostada/internal/model"
	"github.com/rophy/tostada/internal/audit"
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
	oidcCtx      context.Context
	sessions     map[string]session
	mu           sync.RWMutex
	userStore    model.UserStore
	auditLog     *audit.AuditLog
}

type session struct {
	username string
	expiry   time.Time
}

func NewAuth(ctx context.Context, issuerURL, internalURL, clientID, clientSecret, redirectURL string, userStore model.UserStore, auditLog *audit.AuditLog) (*Auth, error) {
	oidcCtx := ctx
	if internalURL != "" {
		oidcCtx = oidc.InsecureIssuerURLContext(ctx, issuerURL)
		oidcCtx = context.WithValue(oidcCtx, oauth2.HTTPClient, &http.Client{
			Transport: &issuerRewriteTransport{
				issuerURL:   issuerURL,
				internalURL: internalURL,
				base:        http.DefaultTransport,
			},
		})
	}
	discoveryURL := issuerURL
	if internalURL != "" {
		discoveryURL = internalURL
	}
	provider, err := oidc.NewProvider(oidcCtx, discoveryURL)
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
		verifier:  provider.Verifier(&oidc.Config{ClientID: clientID}),
		oidcCtx:   oidcCtx,
		sessions:  make(map[string]session),
		userStore: userStore,
		auditLog:  auditLog,
	}, nil
}

type issuerRewriteTransport struct {
	issuerURL   string
	internalURL string
	base        http.RoundTripper
}

func (t *issuerRewriteTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	reqURL := req.URL.String()
	if strings.HasPrefix(reqURL, t.issuerURL) {
		rewritten := strings.Replace(reqURL, t.issuerURL, t.internalURL, 1)
		u, err := url.Parse(rewritten)
		if err != nil {
			return nil, err
		}
		req = req.Clone(req.Context())
		req.URL = u
		req.Host = u.Host
	}
	return t.base.RoundTrip(req)
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

		ctx := r.Context()
		if a.oidcCtx != nil {
			ctx = a.oidcCtx
		}

		token, err := a.oauth2Config.Exchange(ctx, r.URL.Query().Get("code"))
		if err != nil {
			log.Printf("Token exchange failed: %v", err)
			http.Error(w, "token exchange failed", http.StatusInternalServerError)
			return
		}

		rawIDToken, ok := token.Extra("id_token").(string)
		if !ok {
			http.Error(w, "no id_token", http.StatusInternalServerError)
			return
		}

		idToken, err := a.verifier.Verify(ctx, rawIDToken)
		if err != nil {
			log.Printf("Token verification failed: %v", err)
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
		if username == "" {
			username = idToken.Subject
		}

		if a.userStore != nil {
			a.userStore.EnsureUser(r.Context(), username)
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

		if a.auditLog != nil {
			ip, _, _ := net.SplitHostPort(r.RemoteAddr)
			a.auditLog.Log("auth.login", username, "", map[string]string{"ip": ip})
		}

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

func WithUser(ctx context.Context, username string) context.Context {
	return context.WithValue(ctx, userContextKey, username)
}

func (a *Auth) LogoutHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(sessionCookie)
		if err == nil {
			if a.auditLog != nil {
				a.mu.RLock()
				sess, ok := a.sessions[cookie.Value]
				a.mu.RUnlock()
				if ok {
					a.auditLog.Log("auth.logout", sess.username, "", nil)
				}
			}
			a.mu.Lock()
			delete(a.sessions, cookie.Value)
			a.mu.Unlock()
		}
		http.SetCookie(w, &http.Cookie{
			Name:   sessionCookie,
			Value:  "",
			Path:   "/",
			MaxAge: -1,
		})
		http.Redirect(w, r, "/", http.StatusFound)
	}
}

func (a *Auth) CurrentUser(w http.ResponseWriter, r *http.Request) {
	username := UserFromContext(r.Context())
	isAdmin := false
	if a.userStore != nil {
		isAdmin, _ = a.userStore.IsAdmin(r.Context(), username)
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"username": username, "isAdmin": isAdmin})
}

func generateState() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}

