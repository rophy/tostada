package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/rophy/tostada/internal/auth"
	"github.com/rophy/tostada/internal/audit"
)

func TestAccessLogMiddleware_WritesJSONL(t *testing.T) {
	dir := t.TempDir()
	accessPath := filepath.Join(dir, "access.log")

	al := audit.NewAccessLogger(accessPath, 50, 5)
	defer al.Close()

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	})

	handler := al.Middleware(inner, func(r *http.Request) string {
		return auth.UserFromContext(r.Context())
	})

	req := httptest.NewRequest("GET", "/api/workspaces", nil)
	req = req.WithContext(auth.WithUser(req.Context(), "alice"))
	req.RemoteAddr = "10.0.0.1:5555"
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("Status = %d, want 200", rec.Code)
	}

	data, err := os.ReadFile(accessPath)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("access.log is empty")
	}

	var entry map[string]any
	if err := json.Unmarshal(data[:len(data)-1], &entry); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if entry["method"] != "GET" {
		t.Errorf("method = %v, want GET", entry["method"])
	}
	if entry["path"] != "/api/workspaces" {
		t.Errorf("path = %v, want /api/workspaces", entry["path"])
	}
	if entry["user"] != "alice" {
		t.Errorf("user = %v, want alice", entry["user"])
	}
	if entry["status"].(float64) != 200 {
		t.Errorf("status = %v, want 200", entry["status"])
	}
	if entry["ip"] != "10.0.0.1" {
		t.Errorf("ip = %v, want 10.0.0.1", entry["ip"])
	}
}
