package telemetry

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

type ctxKey string

const testUserKey ctxKey = "user"

func TestAccessLogger_Middleware(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "access.log")

	al, err := NewAccessLogger(path)
	if err != nil {
		t.Fatalf("NewAccessLogger: %v", err)
	}
	defer al.Close()

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte("ok"))
	})

	handler := al.Middleware(inner, func(r *http.Request) string {
		v, _ := r.Context().Value(testUserKey).(string)
		return v
	})

	req := httptest.NewRequest("POST", "/api/sessions", nil)
	req = req.WithContext(context.WithValue(req.Context(), testUserKey, "alice"))
	req.RemoteAddr = "10.0.0.1:12345"
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("Status = %d, want 201", rec.Code)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	lines := splitJSONL(data)
	if len(lines) != 1 {
		t.Fatalf("got %d lines, want 1", len(lines))
	}

	var entry accessEntry
	if err := json.Unmarshal(lines[0], &entry); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if entry.Method != "POST" {
		t.Errorf("method = %q, want POST", entry.Method)
	}
	if entry.Path != "/api/sessions" {
		t.Errorf("path = %q, want /api/sessions", entry.Path)
	}
	if entry.User != "alice" {
		t.Errorf("user = %q, want alice", entry.User)
	}
	if entry.Status != 201 {
		t.Errorf("status = %d, want 201", entry.Status)
	}
	if entry.IP != "10.0.0.1" {
		t.Errorf("ip = %q, want 10.0.0.1", entry.IP)
	}
	if entry.DurationMs < 0 {
		t.Errorf("duration_ms = %d, want >= 0", entry.DurationMs)
	}
}

func TestAccessLogger_UnauthenticatedRequest(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "access.log")

	al, err := NewAccessLogger(path)
	if err != nil {
		t.Fatalf("NewAccessLogger: %v", err)
	}
	defer al.Close()

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := al.Middleware(inner, func(r *http.Request) string { return "" })

	req := httptest.NewRequest("GET", "/api/workspaces", nil)
	req.RemoteAddr = "192.168.1.1:9999"
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	data, _ := os.ReadFile(path)
	var entry accessEntry
	json.Unmarshal(splitJSONL(data)[0], &entry)
	if entry.User != "" {
		t.Errorf("user = %q, want empty", entry.User)
	}
}
