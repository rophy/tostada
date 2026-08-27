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
