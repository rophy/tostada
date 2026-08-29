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
