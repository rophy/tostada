# Telemetry & Admin Backend Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add structured JSONL logging (access + audit) and admin API for managing users, devices, and workspace sessions.

**Architecture:** A `telemetry` package writes two JSONL log streams. A `model` package provides user persistence with admin flag. Admin API endpoints under `/api/admin/*` are gated by middleware and provide full CRUD for users, devices, and sessions. All mutating actions emit audit events.

**Tech Stack:** Go 1.22+, SQLite (GORM), stdlib `encoding/json` + `os.File` for telemetry.

**Spec:** `docs/superpowers/specs/2026-09-01-telemetry-admin-design.md`

## Global Constraints

- Go 1.22+ with net/http method-pattern routing
- SQLite via GORM (`github.com/glebarez/sqlite`) — existing pattern
- Module path: `github.com/rophy/tostada`
- No new dependencies for telemetry — stdlib only (`encoding/json`, `os`, `sync`)
- JSONL logs: append-only, no rotation, configurable directory (default `/data/logs`)
- Admin flag: boolean `is_admin` on users table — no RBAC
- All admin API responses are JSON
- All mutating admin actions must emit an audit event
- Existing non-admin endpoints must not change behavior
- Test coverage must stay above 80% for Go
- Run `make test` before committing — all tests must pass

---

### Task 1: Telemetry Package

Create the standalone `internal/telemetry` package with `AuditLog` and `AccessLogger`. No integration with other packages yet — pure library code with full test coverage.

**Files:**
- Create: `internal/telemetry/audit.go`
- Create: `internal/telemetry/audit_test.go`
- Create: `internal/telemetry/access.go`
- Create: `internal/telemetry/access_test.go`

**Interfaces:**
- Consumes: nothing (standalone package)
- Produces:
  - `func NewAuditLog(path string) (*AuditLog, error)` — opens file append-only
  - `func (a *AuditLog) Log(event, user, actor string, detail map[string]string)` — writes one JSONL line
  - `func (a *AuditLog) Close() error` — closes file
  - `func NewAccessLogger(path string) (*AccessLogger, error)` — opens file append-only
  - `func (a *AccessLogger) Middleware(next http.Handler) http.Handler` — wraps handler, logs request
  - `func (a *AccessLogger) Close() error` — closes file

- [ ] **Step 1: Write the audit log test**

Create `internal/telemetry/audit_test.go`:

```go
package telemetry

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestAuditLog_Log(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "audit.log")

	al, err := NewAuditLog(path)
	if err != nil {
		t.Fatalf("NewAuditLog: %v", err)
	}
	defer al.Close()

	al.Log("session.spawn", "alice", "", map[string]string{"workspace": "jupyter", "server": "jupyter-alice"})
	al.Log("admin.session.stop", "alice", "bob", map[string]string{"server": "jupyter-alice"})

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	lines := splitJSONL(data)
	if len(lines) != 2 {
		t.Fatalf("got %d lines, want 2", len(lines))
	}

	var entry1 auditEntry
	if err := json.Unmarshal(lines[0], &entry1); err != nil {
		t.Fatalf("Unmarshal line 1: %v", err)
	}
	if entry1.Event != "session.spawn" {
		t.Errorf("event = %q, want %q", entry1.Event, "session.spawn")
	}
	if entry1.User != "alice" {
		t.Errorf("user = %q, want %q", entry1.User, "alice")
	}
	if entry1.Actor != "" {
		t.Errorf("actor = %q, want empty", entry1.Actor)
	}
	if entry1.Detail["workspace"] != "jupyter" {
		t.Errorf("detail[workspace] = %q, want %q", entry1.Detail["workspace"], "jupyter")
	}
	if entry1.Ts == "" {
		t.Error("ts should not be empty")
	}

	var entry2 auditEntry
	if err := json.Unmarshal(lines[1], &entry2); err != nil {
		t.Fatalf("Unmarshal line 2: %v", err)
	}
	if entry2.Actor != "bob" {
		t.Errorf("actor = %q, want %q", entry2.Actor, "bob")
	}
}

func TestAuditLog_NilDetail(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "audit.log")

	al, err := NewAuditLog(path)
	if err != nil {
		t.Fatalf("NewAuditLog: %v", err)
	}
	defer al.Close()

	al.Log("auth.logout", "alice", "", nil)

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	var entry auditEntry
	if err := json.Unmarshal(splitJSONL(data)[0], &entry); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if entry.Event != "auth.logout" {
		t.Errorf("event = %q, want %q", entry.Event, "auth.logout")
	}
}

func TestAuditLog_AppendsToExistingFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "audit.log")

	al1, _ := NewAuditLog(path)
	al1.Log("auth.login", "alice", "", nil)
	al1.Close()

	al2, _ := NewAuditLog(path)
	al2.Log("auth.login", "bob", "", nil)
	al2.Close()

	data, _ := os.ReadFile(path)
	lines := splitJSONL(data)
	if len(lines) != 2 {
		t.Fatalf("got %d lines, want 2 (append)", len(lines))
	}
}

// splitJSONL splits JSONL bytes into individual JSON lines, skipping empty lines
func splitJSONL(data []byte) [][]byte {
	var lines [][]byte
	for _, line := range bytes.Split(data, []byte("\n")) {
		if len(line) > 0 {
			lines = append(lines, line)
		}
	}
	return lines
}
```

Note: add `"bytes"` to the import block — `splitJSONL` uses `bytes.Split`.

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /home/rophy/projects/tostada && go test ./internal/telemetry/ -run TestAuditLog -v > /tmp/test-telemetry.log 2>&1; cat /tmp/test-telemetry.log`

Expected: Compilation failure — `NewAuditLog`, `auditEntry` not defined.

- [ ] **Step 3: Implement the audit log**

Create `internal/telemetry/audit.go`:

```go
package telemetry

import (
	"encoding/json"
	"os"
	"sync"
	"time"
)

type auditEntry struct {
	Ts     string            `json:"ts"`
	Event  string            `json:"event"`
	User   string            `json:"user"`
	Actor  string            `json:"actor,omitempty"`
	Detail map[string]string `json:"detail,omitempty"`
}

type AuditLog struct {
	f  *os.File
	mu sync.Mutex
}

func NewAuditLog(path string) (*AuditLog, error) {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, err
	}
	return &AuditLog{f: f}, nil
}

func (a *AuditLog) Log(event, user, actor string, detail map[string]string) {
	entry := auditEntry{
		Ts:     time.Now().UTC().Format("2006-01-02T15:04:05.000Z"),
		Event:  event,
		User:   user,
		Actor:  actor,
		Detail: detail,
	}
	data, err := json.Marshal(entry)
	if err != nil {
		return
	}
	data = append(data, '\n')
	a.mu.Lock()
	defer a.mu.Unlock()
	a.f.Write(data)
}

func (a *AuditLog) Close() error {
	return a.f.Close()
}
```

- [ ] **Step 4: Run audit log tests to verify they pass**

Run: `cd /home/rophy/projects/tostada && go test ./internal/telemetry/ -run TestAuditLog -v > /tmp/test-telemetry.log 2>&1; cat /tmp/test-telemetry.log`

Expected: All 3 tests PASS.

- [ ] **Step 5: Write the access logger test**

Create `internal/telemetry/access_test.go`:

```go
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
```

- [ ] **Step 6: Run test to verify it fails**

Run: `cd /home/rophy/projects/tostada && go test ./internal/telemetry/ -run TestAccessLogger -v > /tmp/test-telemetry.log 2>&1; cat /tmp/test-telemetry.log`

Expected: Compilation failure — `NewAccessLogger`, `accessEntry` not defined.

- [ ] **Step 7: Implement the access logger**

Create `internal/telemetry/access.go`:

```go
package telemetry

import (
	"encoding/json"
	"net"
	"net/http"
	"os"
	"sync"
	"time"
)

type accessEntry struct {
	Ts         string `json:"ts"`
	Method     string `json:"method"`
	Path       string `json:"path"`
	User       string `json:"user"`
	Status     int    `json:"status"`
	DurationMs int64  `json:"duration_ms"`
	IP         string `json:"ip"`
	UserAgent  string `json:"user_agent,omitempty"`
}

type AccessLogger struct {
	f  *os.File
	mu sync.Mutex
}

func NewAccessLogger(path string) (*AccessLogger, error) {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, err
	}
	return &AccessLogger{f: f}, nil
}

// UserFunc extracts the authenticated username from a request.
type UserFunc func(r *http.Request) string

// Middleware returns an HTTP middleware that logs each request as a JSONL line.
// userFn extracts the authenticated username (empty string if unauthenticated).
func (a *AccessLogger) Middleware(next http.Handler, userFn UserFunc) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(sw, r)

		ip, _, _ := net.SplitHostPort(r.RemoteAddr)
		if ip == "" {
			ip = r.RemoteAddr
		}

		entry := accessEntry{
			Ts:         start.UTC().Format("2006-01-02T15:04:05.000Z"),
			Method:     r.Method,
			Path:       r.URL.Path,
			User:       userFn(r),
			Status:     sw.status,
			DurationMs: time.Since(start).Milliseconds(),
			IP:         ip,
			UserAgent:  r.UserAgent(),
		}
		data, err := json.Marshal(entry)
		if err != nil {
			return
		}
		data = append(data, '\n')
		a.mu.Lock()
		defer a.mu.Unlock()
		a.f.Write(data)
	})
}

func (a *AccessLogger) Close() error {
	return a.f.Close()
}

type statusWriter struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
}

func (w *statusWriter) WriteHeader(code int) {
	if !w.wroteHeader {
		w.status = code
		w.wroteHeader = true
	}
	w.ResponseWriter.WriteHeader(code)
}
```

- [ ] **Step 8: Run all telemetry tests**

Run: `cd /home/rophy/projects/tostada && go test ./internal/telemetry/ -v > /tmp/test-telemetry.log 2>&1; cat /tmp/test-telemetry.log`

Expected: All 5 tests PASS.

- [ ] **Step 9: Run full test suite to verify no regressions**

Run: `cd /home/rophy/projects/tostada && make test > /tmp/test-full.log 2>&1; grep -E 'PASS|FAIL|ok' /tmp/test-full.log`

Expected: All existing tests still pass.

- [ ] **Step 10: Commit**

```bash
git add internal/telemetry/
git commit -m "feat: add telemetry package with audit log and access logger"
```

---

### Task 2: User Model & Store

Create `internal/model` package with GORM User model and `UserStore` interface.

**Files:**
- Create: `internal/model/user.go`
- Create: `internal/model/store.go`
- Create: `internal/model/store_test.go`

**Interfaces:**
- Consumes: `gorm.io/gorm`, `github.com/glebarez/sqlite` (existing dependencies)
- Produces:
  - `model.User` struct with `ID`, `Username`, `IsAdmin`, `LastLogin`, `CreatedAt`, `UpdatedAt`
  - `model.UserStore` interface: `GetUser`, `ListUsers`, `EnsureUser`, `UpdateUser`, `DeleteUser`, `IsAdmin`
  - `model.NewGormUserStore(db *gorm.DB) *GormUserStore` — reuses an existing `*gorm.DB` connection
  - `model.WithSilentLogger()` option for `NewGormUserStoreFromPath`

- [ ] **Step 1: Write the user store tests**

Create `internal/model/store_test.go`:

```go
package model

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func testDB(t *testing.T) *gorm.DB {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&User{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func TestEnsureUser_CreatesNew(t *testing.T) {
	store := NewGormUserStore(testDB(t))
	ctx := context.Background()

	user, err := store.EnsureUser(ctx, "alice")
	if err != nil {
		t.Fatalf("EnsureUser: %v", err)
	}
	if user.Username != "alice" {
		t.Errorf("Username = %q, want alice", user.Username)
	}
	if user.IsAdmin {
		t.Error("new user should not be admin")
	}
	if user.LastLogin.IsZero() {
		t.Error("LastLogin should be set")
	}
}

func TestEnsureUser_UpdatesLastLogin(t *testing.T) {
	store := NewGormUserStore(testDB(t))
	ctx := context.Background()

	u1, _ := store.EnsureUser(ctx, "alice")
	firstLogin := u1.LastLogin

	u2, _ := store.EnsureUser(ctx, "alice")
	if !u2.LastLogin.After(firstLogin) && !u2.LastLogin.Equal(firstLogin) {
		t.Error("LastLogin should be updated on second call")
	}
}

func TestGetUser(t *testing.T) {
	store := NewGormUserStore(testDB(t))
	ctx := context.Background()

	store.EnsureUser(ctx, "alice")

	user, err := store.GetUser(ctx, "alice")
	if err != nil {
		t.Fatalf("GetUser: %v", err)
	}
	if user.Username != "alice" {
		t.Errorf("Username = %q, want alice", user.Username)
	}
}

func TestGetUser_NotFound(t *testing.T) {
	store := NewGormUserStore(testDB(t))
	ctx := context.Background()

	_, err := store.GetUser(ctx, "nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent user")
	}
}

func TestListUsers(t *testing.T) {
	store := NewGormUserStore(testDB(t))
	ctx := context.Background()

	store.EnsureUser(ctx, "alice")
	store.EnsureUser(ctx, "bob")

	users, err := store.ListUsers(ctx)
	if err != nil {
		t.Fatalf("ListUsers: %v", err)
	}
	if len(users) != 2 {
		t.Fatalf("len = %d, want 2", len(users))
	}
}

func TestUpdateUser(t *testing.T) {
	store := NewGormUserStore(testDB(t))
	ctx := context.Background()

	store.EnsureUser(ctx, "alice")

	err := store.UpdateUser(ctx, "alice", map[string]any{"is_admin": true})
	if err != nil {
		t.Fatalf("UpdateUser: %v", err)
	}

	user, _ := store.GetUser(ctx, "alice")
	if !user.IsAdmin {
		t.Error("IsAdmin should be true after update")
	}
}

func TestDeleteUser(t *testing.T) {
	store := NewGormUserStore(testDB(t))
	ctx := context.Background()

	store.EnsureUser(ctx, "alice")

	err := store.DeleteUser(ctx, "alice")
	if err != nil {
		t.Fatalf("DeleteUser: %v", err)
	}

	_, err = store.GetUser(ctx, "alice")
	if err == nil {
		t.Error("expected error after delete")
	}
}

func TestIsAdmin(t *testing.T) {
	store := NewGormUserStore(testDB(t))
	ctx := context.Background()

	store.EnsureUser(ctx, "alice")

	isAdmin, _ := store.IsAdmin(ctx, "alice")
	if isAdmin {
		t.Error("should not be admin initially")
	}

	store.UpdateUser(ctx, "alice", map[string]any{"is_admin": true})

	isAdmin, _ = store.IsAdmin(ctx, "alice")
	if !isAdmin {
		t.Error("should be admin after update")
	}
}

func TestIsAdmin_NonexistentUser(t *testing.T) {
	store := NewGormUserStore(testDB(t))
	ctx := context.Background()

	isAdmin, err := store.IsAdmin(ctx, "ghost")
	if err != nil {
		t.Fatalf("IsAdmin should not error for missing user: %v", err)
	}
	if isAdmin {
		t.Error("nonexistent user should not be admin")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /home/rophy/projects/tostada && go test ./internal/model/ -v > /tmp/test-model.log 2>&1; cat /tmp/test-model.log`

Expected: Compilation failure — package does not exist.

- [ ] **Step 3: Implement the User model**

Create `internal/model/user.go`:

```go
package model

import "time"

type User struct {
	ID        uint      `gorm:"primarykey" json:"id"`
	Username  string    `gorm:"uniqueIndex;not null" json:"username"`
	IsAdmin   bool      `gorm:"default:false" json:"isAdmin"`
	LastLogin time.Time `json:"lastLogin"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}
```

- [ ] **Step 4: Implement the UserStore interface and GormUserStore**

Create `internal/model/store.go`:

```go
package model

import (
	"context"
	"time"

	"gorm.io/gorm"
)

type UserStore interface {
	GetUser(ctx context.Context, username string) (*User, error)
	ListUsers(ctx context.Context) ([]User, error)
	EnsureUser(ctx context.Context, username string) (*User, error)
	UpdateUser(ctx context.Context, username string, updates map[string]any) error
	DeleteUser(ctx context.Context, username string) error
	IsAdmin(ctx context.Context, username string) (bool, error)
}

type GormUserStore struct {
	db *gorm.DB
}

func NewGormUserStore(db *gorm.DB) *GormUserStore {
	return &GormUserStore{db: db}
}

func (s *GormUserStore) GetUser(_ context.Context, username string) (*User, error) {
	var u User
	if err := s.db.Where("username = ?", username).First(&u).Error; err != nil {
		return nil, err
	}
	return &u, nil
}

func (s *GormUserStore) ListUsers(_ context.Context) ([]User, error) {
	var users []User
	err := s.db.Order("username").Find(&users).Error
	return users, err
}

func (s *GormUserStore) EnsureUser(_ context.Context, username string) (*User, error) {
	var u User
	result := s.db.Where("username = ?", username).First(&u)
	if result.Error != nil {
		u = User{Username: username, LastLogin: time.Now().UTC()}
		if err := s.db.Create(&u).Error; err != nil {
			return nil, err
		}
		return &u, nil
	}
	s.db.Model(&u).Update("last_login", time.Now().UTC())
	return &u, nil
}

func (s *GormUserStore) UpdateUser(_ context.Context, username string, updates map[string]any) error {
	return s.db.Model(&User{}).Where("username = ?", username).Updates(updates).Error
}

func (s *GormUserStore) DeleteUser(_ context.Context, username string) error {
	return s.db.Where("username = ?", username).Delete(&User{}).Error
}

func (s *GormUserStore) IsAdmin(_ context.Context, username string) (bool, error) {
	var u User
	if err := s.db.Where("username = ?", username).First(&u).Error; err != nil {
		return false, nil
	}
	return u.IsAdmin, nil
}
```

- [ ] **Step 5: Run user store tests**

Run: `cd /home/rophy/projects/tostada && go test ./internal/model/ -v > /tmp/test-model.log 2>&1; cat /tmp/test-model.log`

Expected: All 8 tests PASS.

- [ ] **Step 6: Run full test suite**

Run: `cd /home/rophy/projects/tostada && make test > /tmp/test-full.log 2>&1; grep -E 'PASS|FAIL|ok' /tmp/test-full.log`

Expected: All tests pass.

- [ ] **Step 7: Commit**

```bash
git add internal/model/
git commit -m "feat: add user model and store with admin flag"
```

---

### Task 3: Config & Main Wiring

Add `TelemetryConfig` to config, create telemetry + user store in `main.go`, and update `NewRouter` signature to accept the new dependencies. This task bridges the standalone packages from Tasks 1-2 into the application.

**Files:**
- Modify: `internal/config/config.go` — add `TelemetryConfig` struct and field
- Modify: `cmd/tostada/main.go` — create telemetry loggers, user store, pass to NewRouter
- Modify: `internal/api/router.go` — accept new parameters, no new routes yet
- Modify: `internal/api/devices_test.go` — update NewRouter calls if any (check first)
- Modify: `internal/api/sessions.go` — add `auditLog` field to `sessionsHandler` (no usage yet)
- Modify: `internal/api/devices.go` — add `auditLog` field to `devicesHandler` (no usage yet)

**Interfaces:**
- Consumes:
  - `telemetry.NewAuditLog(path string) (*AuditLog, error)` from Task 1
  - `telemetry.NewAccessLogger(path string) (*AccessLogger, error)` from Task 1
  - `model.NewGormUserStore(db *gorm.DB) *GormUserStore` from Task 2
  - `model.UserStore` interface from Task 2
- Produces:
  - `config.TelemetryConfig` struct with `LogDir string`
  - Updated `api.NewRouter` signature: `func NewRouter(cfg *config.Config, hubClient *hub.Client, authProvider *auth.Auth, deviceStore device.Store, userStore model.UserStore, auditLog *telemetry.AuditLog, accessLogger *telemetry.AccessLogger) *http.ServeMux`

- [ ] **Step 1: Add TelemetryConfig to config**

In `internal/config/config.go`, add the struct and field:

```go
type TelemetryConfig struct {
	LogDir string `yaml:"logDir"`
}
```

Add to `Config` struct:

```go
Telemetry TelemetryConfig `yaml:"telemetry"`
```

- [ ] **Step 2: Update NewRouter signature**

In `internal/api/router.go`, update the import block to add:

```go
"github.com/rophy/tostada/internal/model"
"github.com/rophy/tostada/internal/telemetry"
```

Change the function signature:

```go
func NewRouter(cfg *config.Config, hubClient *hub.Client, authProvider *auth.Auth, deviceStore device.Store, userStore model.UserStore, auditLog *telemetry.AuditLog, accessLogger *telemetry.AccessLogger) *http.ServeMux {
```

Add `auditLog` field to `sessionsHandler` and `devicesHandler` struct initialization (no usage yet):

In the `sessions` initialization, add `auditLog` field:
```go
sessions := &sessionsHandler{
    hubClient:  hubClient,
    workspaces: cfg.Workspaces,
    guacCfg:    cfg.Guacamole,
    auditLog:   auditLog,
}
```

In the `devices` initialization, add `auditLog` field:
```go
devices := &devicesHandler{
    store:    deviceStore,
    guacCfg:  cfg.Guacamole,
    auditLog: auditLog,
}
```

Wrap the authenticated mux with the access logger. Change:
```go
mux.Handle("/api/", authProvider.Middleware(authed))
```
to:
```go
authedHandler := authProvider.Middleware(authed)
if accessLogger != nil {
    authedHandler = accessLogger.Middleware(authedHandler, func(r *http.Request) string {
        return auth.UserFromContext(r.Context())
    })
}
mux.Handle("/api/", authedHandler)
```

- [ ] **Step 3: Add auditLog field to handler structs**

In `internal/api/sessions.go`, add to the struct:

```go
type sessionsHandler struct {
	hubClient  *hub.Client
	workspaces []config.Workspace
	guacCfg    config.GuacamoleConfig
	auditLog   *telemetry.AuditLog
}
```

Add import: `"github.com/rophy/tostada/internal/telemetry"`

In `internal/api/devices.go`, add to the struct:

```go
type devicesHandler struct {
	store    device.Store
	guacCfg  config.GuacamoleConfig
	auditLog *telemetry.AuditLog
}
```

Add import: `"github.com/rophy/tostada/internal/telemetry"`

- [ ] **Step 4: Update main.go**

In `cmd/tostada/main.go`, add imports:

```go
"os"
"path/filepath"

"github.com/rophy/tostada/internal/model"
"github.com/rophy/tostada/internal/telemetry"
```

After the `deviceStore` creation, add user store (reusing the same DB):

```go
userStore := model.NewGormUserStore(deviceStore.DB())
if err := deviceStore.DB().AutoMigrate(&model.User{}); err != nil {
    log.Fatalf("Failed to migrate user model: %v", err)
}
```

Create telemetry loggers:

```go
logDir := cfg.Telemetry.LogDir
if logDir == "" {
    logDir = "/data/logs"
}
if err := os.MkdirAll(logDir, 0755); err != nil {
    log.Fatalf("Failed to create log directory: %v", err)
}

auditLog, err := telemetry.NewAuditLog(filepath.Join(logDir, "audit.log"))
if err != nil {
    log.Fatalf("Failed to create audit log: %v", err)
}
defer auditLog.Close()

accessLogger, err := telemetry.NewAccessLogger(filepath.Join(logDir, "access.log"))
if err != nil {
    log.Fatalf("Failed to create access logger: %v", err)
}
defer accessLogger.Close()
```

Update the `NewRouter` call:

```go
mux := api.NewRouter(cfg, hubClient, authProvider, deviceStore, userStore, auditLog, accessLogger)
```

- [ ] **Step 5: Update existing tests**

In `internal/api/devices_test.go`, the tests create `devicesHandler` directly (not via `NewRouter`), so they need the `auditLog` field. Add `auditLog: nil` to each handler initialization — nil is safe because the existing handlers don't emit events yet.

Update `seedTestDevice` caller in `TestDevicesConnect` — the handler struct now has `auditLog`:

```go
h := &devicesHandler{
    store:    store,
    guacCfg:  config.GuacamoleConfig{...},
    auditLog: nil,
}
```

Do the same for all `devicesHandler` and `sessionsHandler` instantiations in test files.

- [ ] **Step 6: Build and run full test suite**

Run: `cd /home/rophy/projects/tostada && make test > /tmp/test-full.log 2>&1; grep -E 'PASS|FAIL|ok' /tmp/test-full.log`

Expected: All tests pass. The new imports compile, handler structs accept the new field, `NewRouter` accepts the updated signature.

- [ ] **Step 7: Commit**

```bash
git add internal/config/config.go internal/api/router.go internal/api/sessions.go internal/api/devices.go internal/api/devices_test.go cmd/tostada/main.go
git commit -m "feat: wire telemetry and user store into application"
```

---

### Task 4: Admin Device Store

Extend the device package with `AdminStore` interface and add admin methods to `GormStore`.

**Files:**
- Modify: `internal/device/store.go` — add `AdminStore` interface, `DeviceWithGrants` type
- Modify: `internal/device/gorm_store.go` — add admin methods
- Create: `internal/device/gorm_store_test.go` — tests for admin methods (new file; existing test is in `internal/api/devices_test.go`)

**Interfaces:**
- Consumes: existing `Device`, `UserAccess`, `GormStore` types
- Produces:
  - `device.DeviceWithGrants` struct — Device + `Grants []string`
  - `device.AdminStore` interface embedding `Store` plus: `ListAllDevices`, `CreateDevice`, `UpdateDevice`, `DeleteDevice`, `GrantAccess`, `RevokeAccess`
  - All methods implemented on `*GormStore`

- [ ] **Step 1: Write admin store tests**

Create `internal/device/gorm_store_test.go`:

```go
package device

import (
	"context"
	"path/filepath"
	"testing"
)

func testStore(t *testing.T) *GormStore {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	store, err := NewGormStore(dbPath, WithSilentLogger())
	if err != nil {
		t.Fatalf("NewGormStore: %v", err)
	}
	return store
}

func seedDevice(t *testing.T, store *GormStore) Device {
	t.Helper()
	d := Device{Name: "test-rdp", Display: "Test RDP", Protocol: "rdp", Host: "10.0.0.1", Port: 3389, Username: "user", Password: "pass"}
	if err := store.db.Create(&d).Error; err != nil {
		t.Fatalf("create device: %v", err)
	}
	return d
}

func TestListAllDevices(t *testing.T) {
	store := testStore(t)
	d := seedDevice(t, store)
	store.db.Create(&UserAccess{Username: "alice", DeviceID: d.ID})
	store.db.Create(&UserAccess{Username: "bob", DeviceID: d.ID})

	ctx := context.Background()
	devices, err := store.ListAllDevices(ctx)
	if err != nil {
		t.Fatalf("ListAllDevices: %v", err)
	}
	if len(devices) != 1 {
		t.Fatalf("len = %d, want 1", len(devices))
	}
	if devices[0].Name != "test-rdp" {
		t.Errorf("Name = %q, want test-rdp", devices[0].Name)
	}
	if len(devices[0].Grants) != 2 {
		t.Fatalf("grants len = %d, want 2", len(devices[0].Grants))
	}
}

func TestCreateDevice(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()

	d := &Device{Name: "new-dev", Display: "New", Protocol: "vnc", Host: "10.0.0.2", Port: 5900, Username: "u", Password: "p"}
	if err := store.CreateDevice(ctx, d); err != nil {
		t.Fatalf("CreateDevice: %v", err)
	}

	devices, _ := store.ListAllDevices(ctx)
	if len(devices) != 1 {
		t.Fatalf("len = %d, want 1", len(devices))
	}
	if devices[0].Name != "new-dev" {
		t.Errorf("Name = %q, want new-dev", devices[0].Name)
	}
}

func TestUpdateDevice(t *testing.T) {
	store := testStore(t)
	seedDevice(t, store)
	ctx := context.Background()

	err := store.UpdateDevice(ctx, "test-rdp", map[string]any{"host": "10.0.0.99"})
	if err != nil {
		t.Fatalf("UpdateDevice: %v", err)
	}

	var d Device
	store.db.Where("name = ?", "test-rdp").First(&d)
	if d.Host != "10.0.0.99" {
		t.Errorf("Host = %q, want 10.0.0.99", d.Host)
	}
}

func TestDeleteDevice(t *testing.T) {
	store := testStore(t)
	d := seedDevice(t, store)
	store.db.Create(&UserAccess{Username: "alice", DeviceID: d.ID})
	ctx := context.Background()

	if err := store.DeleteDevice(ctx, "test-rdp"); err != nil {
		t.Fatalf("DeleteDevice: %v", err)
	}

	var count int64
	store.db.Model(&Device{}).Count(&count)
	if count != 0 {
		t.Errorf("device count = %d, want 0", count)
	}

	store.db.Model(&UserAccess{}).Count(&count)
	if count != 0 {
		t.Errorf("access count = %d, want 0 (cascade)", count)
	}
}

func TestGrantAccess(t *testing.T) {
	store := testStore(t)
	seedDevice(t, store)
	ctx := context.Background()

	if err := store.GrantAccess(ctx, "test-rdp", "alice"); err != nil {
		t.Fatalf("GrantAccess: %v", err)
	}

	devices, _ := store.ListAllDevices(ctx)
	if len(devices[0].Grants) != 1 || devices[0].Grants[0] != "alice" {
		t.Errorf("grants = %v, want [alice]", devices[0].Grants)
	}
}

func TestGrantAccess_Duplicate(t *testing.T) {
	store := testStore(t)
	seedDevice(t, store)
	ctx := context.Background()

	store.GrantAccess(ctx, "test-rdp", "alice")
	err := store.GrantAccess(ctx, "test-rdp", "alice")
	if err == nil {
		t.Error("expected error on duplicate grant")
	}
}

func TestRevokeAccess(t *testing.T) {
	store := testStore(t)
	d := seedDevice(t, store)
	store.db.Create(&UserAccess{Username: "alice", DeviceID: d.ID})
	ctx := context.Background()

	if err := store.RevokeAccess(ctx, "test-rdp", "alice"); err != nil {
		t.Fatalf("RevokeAccess: %v", err)
	}

	devices, _ := store.ListAllDevices(ctx)
	if len(devices[0].Grants) != 0 {
		t.Errorf("grants = %v, want []", devices[0].Grants)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /home/rophy/projects/tostada && go test ./internal/device/ -v > /tmp/test-device.log 2>&1; cat /tmp/test-device.log`

Expected: Compilation failure — `ListAllDevices`, `CreateDevice`, etc. not defined.

- [ ] **Step 3: Add AdminStore interface and DeviceWithGrants to store.go**

In `internal/device/store.go`, add:

```go
type DeviceWithGrants struct {
	Device
	Grants []string `json:"grants"`
}

type AdminStore interface {
	Store
	ListAllDevices(ctx context.Context) ([]DeviceWithGrants, error)
	CreateDevice(ctx context.Context, d *Device) error
	UpdateDevice(ctx context.Context, name string, updates map[string]any) error
	DeleteDevice(ctx context.Context, name string) error
	GrantAccess(ctx context.Context, deviceName string, username string) error
	RevokeAccess(ctx context.Context, deviceName string, username string) error
}
```

- [ ] **Step 4: Implement admin methods on GormStore**

In `internal/device/gorm_store.go`, add:

```go
func (s *GormStore) ListAllDevices(_ context.Context) ([]DeviceWithGrants, error) {
	var devices []Device
	if err := s.db.Find(&devices).Error; err != nil {
		return nil, err
	}
	result := make([]DeviceWithGrants, len(devices))
	for i, d := range devices {
		var accesses []UserAccess
		s.db.Where("device_id = ?", d.ID).Find(&accesses)
		grants := make([]string, len(accesses))
		for j, a := range accesses {
			grants[j] = a.Username
		}
		result[i] = DeviceWithGrants{Device: d, Grants: grants}
	}
	return result, nil
}

func (s *GormStore) CreateDevice(_ context.Context, d *Device) error {
	return s.db.Create(d).Error
}

func (s *GormStore) UpdateDevice(_ context.Context, name string, updates map[string]any) error {
	return s.db.Model(&Device{}).Where("name = ?", name).Updates(updates).Error
}

func (s *GormStore) DeleteDevice(_ context.Context, name string) error {
	var d Device
	if err := s.db.Where("name = ?", name).First(&d).Error; err != nil {
		return err
	}
	s.db.Where("device_id = ?", d.ID).Delete(&UserAccess{})
	return s.db.Delete(&d).Error
}

func (s *GormStore) GrantAccess(_ context.Context, deviceName string, username string) error {
	var d Device
	if err := s.db.Where("name = ?", deviceName).First(&d).Error; err != nil {
		return err
	}
	return s.db.Create(&UserAccess{Username: username, DeviceID: d.ID}).Error
}

func (s *GormStore) RevokeAccess(_ context.Context, deviceName string, username string) error {
	var d Device
	if err := s.db.Where("name = ?", deviceName).First(&d).Error; err != nil {
		return err
	}
	return s.db.Where("username = ? AND device_id = ?", username, d.ID).Delete(&UserAccess{}).Error
}
```

- [ ] **Step 5: Run device store tests**

Run: `cd /home/rophy/projects/tostada && go test ./internal/device/ -v > /tmp/test-device.log 2>&1; cat /tmp/test-device.log`

Expected: All 7 new tests PASS.

- [ ] **Step 6: Run full test suite**

Run: `cd /home/rophy/projects/tostada && make test > /tmp/test-full.log 2>&1; grep -E 'PASS|FAIL|ok' /tmp/test-full.log`

Expected: All tests pass.

- [ ] **Step 7: Commit**

```bash
git add internal/device/store.go internal/device/gorm_store.go internal/device/gorm_store_test.go
git commit -m "feat: add admin device store with CRUD and access management"
```

---

### Task 5: Admin API

Implement `AdminMiddleware` and `adminHandler` with all admin endpoints — user CRUD, device CRUD, session management. Every mutating endpoint emits an audit event.

**Files:**
- Create: `internal/api/admin.go`
- Create: `internal/api/admin_test.go`
- Modify: `internal/api/router.go` — register admin routes

**Interfaces:**
- Consumes:
  - `model.UserStore` interface from Task 2
  - `device.AdminStore` interface from Task 4
  - `telemetry.AuditLog` from Task 1
  - `hub.Client` from existing code (for session management)
  - `auth.UserFromContext(ctx) string` from existing code
- Produces:
  - `AdminMiddleware(store model.UserStore) func(http.Handler) http.Handler`
  - `adminHandler` struct with methods: `listUsers`, `updateUser`, `deleteUser`, `listDevices`, `createDevice`, `updateDevice`, `deleteDevice`, `grantAccess`, `revokeAccess`, `listSessions`, `stopSession`

- [ ] **Step 1: Write admin middleware test**

Create `internal/api/admin_test.go`:

```go
package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rophy/tostada/internal/auth"
	"github.com/rophy/tostada/internal/device"
	"github.com/rophy/tostada/internal/model"
	"github.com/rophy/tostada/internal/telemetry"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func testUserStore(t *testing.T) (*model.GormUserStore, *gorm.DB) {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	db.AutoMigrate(&model.User{}, &device.Device{}, &device.UserAccess{})
	return model.NewGormUserStore(db), db
}

func testAuditLog(t *testing.T) *telemetry.AuditLog {
	t.Helper()
	path := filepath.Join(t.TempDir(), "audit.log")
	al, err := telemetry.NewAuditLog(path)
	if err != nil {
		t.Fatalf("NewAuditLog: %v", err)
	}
	t.Cleanup(func() { al.Close() })
	return al
}

func TestAdminMiddleware_AllowsAdmin(t *testing.T) {
	userStore, _ := testUserStore(t)
	ctx := context.Background()
	userStore.EnsureUser(ctx, "admin")
	userStore.UpdateUser(ctx, "admin", map[string]any{"is_admin": true})

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := AdminMiddleware(userStore)(inner)

	req := httptest.NewRequest("GET", "/api/admin/users", nil)
	req = req.WithContext(auth.WithUser(req.Context(), "admin"))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Status = %d, want 200", rec.Code)
	}
}

func TestAdminMiddleware_DeniesNonAdmin(t *testing.T) {
	userStore, _ := testUserStore(t)
	ctx := context.Background()
	userStore.EnsureUser(ctx, "alice")

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("inner handler should not be called")
	})

	handler := AdminMiddleware(userStore)(inner)

	req := httptest.NewRequest("GET", "/api/admin/users", nil)
	req = req.WithContext(auth.WithUser(req.Context(), "alice"))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("Status = %d, want 403", rec.Code)
	}
}

func TestAdminListUsers(t *testing.T) {
	userStore, _ := testUserStore(t)
	ctx := context.Background()
	userStore.EnsureUser(ctx, "alice")
	userStore.EnsureUser(ctx, "bob")

	h := &adminHandler{userStore: userStore, auditLog: testAuditLog(t)}

	req := httptest.NewRequest("GET", "/api/admin/users", nil)
	req = req.WithContext(auth.WithUser(req.Context(), "admin"))
	rec := httptest.NewRecorder()
	h.listUsers(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("Status = %d, want 200", rec.Code)
	}

	var users []model.User
	json.NewDecoder(rec.Body).Decode(&users)
	if len(users) != 2 {
		t.Fatalf("len = %d, want 2", len(users))
	}
}

func TestAdminUpdateUser(t *testing.T) {
	userStore, _ := testUserStore(t)
	ctx := context.Background()
	userStore.EnsureUser(ctx, "alice")

	h := &adminHandler{userStore: userStore, auditLog: testAuditLog(t)}

	body := strings.NewReader(`{"isAdmin": true}`)
	req := httptest.NewRequest("PATCH", "/api/admin/users/alice", body)
	req.SetPathValue("username", "alice")
	req = req.WithContext(auth.WithUser(req.Context(), "admin"))
	rec := httptest.NewRecorder()
	h.updateUser(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("Status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}

	user, _ := userStore.GetUser(ctx, "alice")
	if !user.IsAdmin {
		t.Error("IsAdmin should be true")
	}
}

func TestAdminUpdateUser_CannotDemoteSelf(t *testing.T) {
	userStore, _ := testUserStore(t)
	ctx := context.Background()
	userStore.EnsureUser(ctx, "admin")
	userStore.UpdateUser(ctx, "admin", map[string]any{"is_admin": true})

	h := &adminHandler{userStore: userStore, auditLog: testAuditLog(t)}

	body := strings.NewReader(`{"isAdmin": false}`)
	req := httptest.NewRequest("PATCH", "/api/admin/users/admin", body)
	req.SetPathValue("username", "admin")
	req = req.WithContext(auth.WithUser(req.Context(), "admin"))
	rec := httptest.NewRecorder()
	h.updateUser(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("Status = %d, want 400", rec.Code)
	}
}

func TestAdminDeleteUser(t *testing.T) {
	userStore, _ := testUserStore(t)
	ctx := context.Background()
	userStore.EnsureUser(ctx, "alice")

	h := &adminHandler{userStore: userStore, auditLog: testAuditLog(t)}

	req := httptest.NewRequest("DELETE", "/api/admin/users/alice", nil)
	req.SetPathValue("username", "alice")
	req = req.WithContext(auth.WithUser(req.Context(), "admin"))
	rec := httptest.NewRecorder()
	h.deleteUser(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Errorf("Status = %d, want 204", rec.Code)
	}
}

func TestAdminDeleteUser_CannotDeleteSelf(t *testing.T) {
	userStore, _ := testUserStore(t)
	ctx := context.Background()
	userStore.EnsureUser(ctx, "admin")

	h := &adminHandler{userStore: userStore, auditLog: testAuditLog(t)}

	req := httptest.NewRequest("DELETE", "/api/admin/users/admin", nil)
	req.SetPathValue("username", "admin")
	req = req.WithContext(auth.WithUser(req.Context(), "admin"))
	rec := httptest.NewRecorder()
	h.deleteUser(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("Status = %d, want 400", rec.Code)
	}
}

func TestAdminListDevices(t *testing.T) {
	userStore, db := testUserStore(t)
	devStore, _ := device.NewGormStore(filepath.Join(t.TempDir(), "dev.db"), device.WithSilentLogger())
	// Seed via the admin store's DB
	db.Create(&device.Device{Name: "dev1", Display: "Dev 1", Protocol: "rdp", Host: "10.0.0.1", Port: 3389, Username: "u", Password: "p"})

	h := &adminHandler{userStore: userStore, deviceStore: devStore, auditLog: testAuditLog(t)}

	req := httptest.NewRequest("GET", "/api/admin/devices", nil)
	req = req.WithContext(auth.WithUser(req.Context(), "admin"))
	rec := httptest.NewRecorder()
	h.listDevices(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("Status = %d, want 200", rec.Code)
	}
}

func TestAdminCreateDevice(t *testing.T) {
	userStore, _ := testUserStore(t)
	devStore := testDeviceStore(t)

	h := &adminHandler{userStore: userStore, deviceStore: devStore, auditLog: testAuditLog(t)}

	body := strings.NewReader(`{"name":"new","displayName":"New Device","protocol":"vnc","host":"10.0.0.5","port":5900,"username":"u","password":"p"}`)
	req := httptest.NewRequest("POST", "/api/admin/devices", body)
	req = req.WithContext(auth.WithUser(req.Context(), "admin"))
	rec := httptest.NewRecorder()
	h.createDevice(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("Status = %d, want 201; body: %s", rec.Code, rec.Body.String())
	}
}

func TestAdminDeleteDevice(t *testing.T) {
	userStore, _ := testUserStore(t)
	devStore := testDeviceStore(t)
	seedTestDevice(t, devStore)

	h := &adminHandler{userStore: userStore, deviceStore: devStore, auditLog: testAuditLog(t)}

	req := httptest.NewRequest("DELETE", "/api/admin/devices/test-rdp", nil)
	req.SetPathValue("name", "test-rdp")
	req = req.WithContext(auth.WithUser(req.Context(), "admin"))
	rec := httptest.NewRecorder()
	h.deleteDevice(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Errorf("Status = %d, want 204", rec.Code)
	}
}

func TestAdminGrantAccess(t *testing.T) {
	userStore, _ := testUserStore(t)
	devStore := testDeviceStore(t)
	seedTestDevice(t, devStore)

	h := &adminHandler{userStore: userStore, deviceStore: devStore, auditLog: testAuditLog(t)}

	body := strings.NewReader(`{"username":"bob"}`)
	req := httptest.NewRequest("POST", "/api/admin/devices/test-rdp/grants", body)
	req.SetPathValue("name", "test-rdp")
	req = req.WithContext(auth.WithUser(req.Context(), "admin"))
	rec := httptest.NewRecorder()
	h.grantAccess(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("Status = %d, want 201; body: %s", rec.Code, rec.Body.String())
	}
}

func TestAdminRevokeAccess(t *testing.T) {
	userStore, _ := testUserStore(t)
	devStore := testDeviceStore(t)
	seedTestDevice(t, devStore)

	h := &adminHandler{userStore: userStore, deviceStore: devStore, auditLog: testAuditLog(t)}

	req := httptest.NewRequest("DELETE", "/api/admin/devices/test-rdp/grants/alice", nil)
	req.SetPathValue("name", "test-rdp")
	req.SetPathValue("username", "alice")
	req = req.WithContext(auth.WithUser(req.Context(), "admin"))
	rec := httptest.NewRecorder()
	h.revokeAccess(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Errorf("Status = %d, want 204", rec.Code)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /home/rophy/projects/tostada && go test ./internal/api/ -run TestAdmin -v > /tmp/test-admin.log 2>&1; cat /tmp/test-admin.log`

Expected: Compilation failure — `AdminMiddleware`, `adminHandler` not defined.

- [ ] **Step 3: Implement AdminMiddleware and adminHandler**

Create `internal/api/admin.go`:

```go
package api

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/rophy/tostada/internal/auth"
	"github.com/rophy/tostada/internal/device"
	"github.com/rophy/tostada/internal/hub"
	"github.com/rophy/tostada/internal/model"
	"github.com/rophy/tostada/internal/telemetry"
)

func AdminMiddleware(store model.UserStore) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			username := auth.UserFromContext(r.Context())
			isAdmin, _ := store.IsAdmin(r.Context(), username)
			if !isAdmin {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusForbidden)
				json.NewEncoder(w).Encode(map[string]string{"error": "admin access required"})
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

type adminHandler struct {
	userStore   model.UserStore
	deviceStore device.AdminStore
	hubClient   *hub.Client
	auditLog    *telemetry.AuditLog
}

func (h *adminHandler) listUsers(w http.ResponseWriter, r *http.Request) {
	users, err := h.userStore.ListUsers(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(users)
}

func (h *adminHandler) updateUser(w http.ResponseWriter, r *http.Request) {
	actor := auth.UserFromContext(r.Context())
	target := r.PathValue("username")

	var req struct {
		IsAdmin *bool `json:"isAdmin"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	if req.IsAdmin != nil && !*req.IsAdmin && target == actor {
		http.Error(w, "cannot remove admin from yourself", http.StatusBadRequest)
		return
	}

	updates := map[string]any{}
	if req.IsAdmin != nil {
		updates["is_admin"] = *req.IsAdmin
	}

	if err := h.userStore.UpdateUser(r.Context(), target, updates); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if h.auditLog != nil {
		h.auditLog.Log("admin.user.update", target, actor, map[string]string{
			"target":  target,
			"changes": fmt.Sprintf("%v", updates),
		})
	}

	user, err := h.userStore.GetUser(r.Context(), target)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(user)
}

func (h *adminHandler) deleteUser(w http.ResponseWriter, r *http.Request) {
	actor := auth.UserFromContext(r.Context())
	target := r.PathValue("username")

	if target == actor {
		http.Error(w, "cannot delete yourself", http.StatusBadRequest)
		return
	}

	if err := h.userStore.DeleteUser(r.Context(), target); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if h.auditLog != nil {
		h.auditLog.Log("admin.user.delete", target, actor, map[string]string{"target": target})
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *adminHandler) listDevices(w http.ResponseWriter, r *http.Request) {
	devices, err := h.deviceStore.ListAllDevices(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(devices)
}

func (h *adminHandler) createDevice(w http.ResponseWriter, r *http.Request) {
	actor := auth.UserFromContext(r.Context())

	var d device.Device
	if err := json.NewDecoder(r.Body).Decode(&d); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	if err := h.deviceStore.CreateDevice(r.Context(), &d); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if h.auditLog != nil {
		h.auditLog.Log("admin.device.add", "", actor, map[string]string{"device": d.Name})
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(d)
}

func (h *adminHandler) updateDevice(w http.ResponseWriter, r *http.Request) {
	actor := auth.UserFromContext(r.Context())
	name := r.PathValue("name")

	var updates map[string]any
	if err := json.NewDecoder(r.Body).Decode(&updates); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	if err := h.deviceStore.UpdateDevice(r.Context(), name, updates); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if h.auditLog != nil {
		h.auditLog.Log("admin.device.update", "", actor, map[string]string{"device": name})
	}

	w.WriteHeader(http.StatusOK)
}

func (h *adminHandler) deleteDevice(w http.ResponseWriter, r *http.Request) {
	actor := auth.UserFromContext(r.Context())
	name := r.PathValue("name")

	if err := h.deviceStore.DeleteDevice(r.Context(), name); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if h.auditLog != nil {
		h.auditLog.Log("admin.device.remove", "", actor, map[string]string{"device": name})
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *adminHandler) grantAccess(w http.ResponseWriter, r *http.Request) {
	actor := auth.UserFromContext(r.Context())
	deviceName := r.PathValue("name")

	var req struct {
		Username string `json:"username"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	if err := h.deviceStore.GrantAccess(r.Context(), deviceName, req.Username); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if h.auditLog != nil {
		h.auditLog.Log("admin.device.grant", req.Username, actor, map[string]string{
			"device": deviceName,
			"target": req.Username,
		})
	}

	w.WriteHeader(http.StatusCreated)
}

func (h *adminHandler) revokeAccess(w http.ResponseWriter, r *http.Request) {
	actor := auth.UserFromContext(r.Context())
	deviceName := r.PathValue("name")
	username := r.PathValue("username")

	if err := h.deviceStore.RevokeAccess(r.Context(), deviceName, username); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if h.auditLog != nil {
		h.auditLog.Log("admin.device.revoke", username, actor, map[string]string{
			"device": deviceName,
			"target": username,
		})
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *adminHandler) listSessions(w http.ResponseWriter, r *http.Request) {
	if h.hubClient == nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]any{})
		return
	}

	// JupyterHub list-all-users requires admin scope, which we have via service token
	// For now, return empty — full implementation requires hub.ListUsers method
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode([]any{})
}

func (h *adminHandler) stopSession(w http.ResponseWriter, r *http.Request) {
	actor := auth.UserFromContext(r.Context())
	username := r.PathValue("username")
	server := r.PathValue("server")

	if h.hubClient == nil {
		http.Error(w, "hub client not configured", http.StatusServiceUnavailable)
		return
	}

	if err := h.hubClient.StopServer(username, server); err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}

	if h.auditLog != nil {
		h.auditLog.Log("admin.session.stop", username, actor, map[string]string{
			"target": username,
			"server": server,
		})
	}

	w.WriteHeader(http.StatusNoContent)
}
```

- [ ] **Step 4: Register admin routes in router.go**

In `internal/api/router.go`, after the existing authenticated route registrations, add:

```go
adminMux := http.NewServeMux()
admin := &adminHandler{
    userStore:   userStore,
    deviceStore: deviceStore.(device.AdminStore),
    hubClient:   hubClient,
    auditLog:    auditLog,
}
adminMux.HandleFunc("GET /api/admin/users", admin.listUsers)
adminMux.HandleFunc("PATCH /api/admin/users/{username}", admin.updateUser)
adminMux.HandleFunc("DELETE /api/admin/users/{username}", admin.deleteUser)
adminMux.HandleFunc("GET /api/admin/devices", admin.listDevices)
adminMux.HandleFunc("POST /api/admin/devices", admin.createDevice)
adminMux.HandleFunc("PUT /api/admin/devices/{name}", admin.updateDevice)
adminMux.HandleFunc("DELETE /api/admin/devices/{name}", admin.deleteDevice)
adminMux.HandleFunc("POST /api/admin/devices/{name}/grants", admin.grantAccess)
adminMux.HandleFunc("DELETE /api/admin/devices/{name}/grants/{username}", admin.revokeAccess)
adminMux.HandleFunc("GET /api/admin/sessions", admin.listSessions)
adminMux.HandleFunc("DELETE /api/admin/sessions/{username}/{server}", admin.stopSession)

authed.Handle("/api/admin/", AdminMiddleware(userStore)(adminMux))
```

Note: The `deviceStore` parameter type in `NewRouter` should be changed to `device.AdminStore` (which embeds `device.Store`, so it's backward-compatible). Update the parameter type:

```go
func NewRouter(cfg *config.Config, hubClient *hub.Client, authProvider *auth.Auth, deviceStore device.AdminStore, userStore model.UserStore, auditLog *telemetry.AuditLog, accessLogger *telemetry.AccessLogger) *http.ServeMux {
```

And update `cmd/tostada/main.go` to pass `deviceStore` (which is `*device.GormStore`, satisfying `device.AdminStore`).

- [ ] **Step 5: Run admin tests**

Run: `cd /home/rophy/projects/tostada && go test ./internal/api/ -run TestAdmin -v > /tmp/test-admin.log 2>&1; cat /tmp/test-admin.log`

Expected: All 12 admin tests PASS.

- [ ] **Step 6: Run full test suite**

Run: `cd /home/rophy/projects/tostada && make test > /tmp/test-full.log 2>&1; grep -E 'PASS|FAIL|ok' /tmp/test-full.log`

Expected: All tests pass.

- [ ] **Step 7: Commit**

```bash
git add internal/api/admin.go internal/api/admin_test.go internal/api/router.go
git commit -m "feat: add admin API with user, device, and session management"
```

---

### Task 6: Audit Event Integration

Add audit event emission to existing session, device, and auth handlers. Add user commands to `tostada-cli`.

**Files:**
- Modify: `internal/api/sessions.go` — emit `session.spawn`, `session.stop`, `session.connect`
- Modify: `internal/api/devices.go` — emit `device.connect`
- Modify: `internal/auth/oidc.go` — add `userStore` + `auditLog` fields, call `EnsureUser` on callback, emit `auth.login` + `auth.logout`
- Modify: `cmd/tostada/main.go` — pass `userStore` + `auditLog` to `Auth`
- Modify: `cmd/tostada-cli/main.go` — add `user` subcommand

**Interfaces:**
- Consumes:
  - `telemetry.AuditLog.Log(event, user, actor string, detail map[string]string)` from Task 1
  - `model.UserStore.EnsureUser(ctx, username) (*User, error)` from Task 2
- Produces: audit events from all existing handlers

- [ ] **Step 1: Add audit events to sessions handler**

In `internal/api/sessions.go`, add audit logging to `create`, `stop`, and `connect` methods.

In `create`, after successful spawn:
```go
if h.auditLog != nil {
    h.auditLog.Log("session.spawn", username, "", map[string]string{
        "workspace": req.Workspace,
        "server":    req.ServerName,
    })
}
```

In `stop`, after successful stop:
```go
if h.auditLog != nil {
    h.auditLog.Log("session.stop", username, "", map[string]string{
        "server": serverName,
    })
}
```

In `connect`, after generating the connect URL (before the final response):
```go
if h.auditLog != nil {
    connType := "jupyterhub"
    if ws != nil && ws.Type == "guacamole" {
        connType = "guacamole"
    }
    h.auditLog.Log("session.connect", username, "", map[string]string{
        "server": serverName,
        "type":   connType,
    })
}
```

- [ ] **Step 2: Add audit events to devices handler**

In `internal/api/devices.go`, add audit logging to `connect` method.

After the successful token exchange (before the final response):
```go
if h.auditLog != nil {
    h.auditLog.Log("device.connect", username, "", map[string]string{
        "device": d.Name,
    })
}
```

- [ ] **Step 3: Add userStore and auditLog to Auth**

In `internal/auth/oidc.go`, add fields to the `Auth` struct:

```go
type Auth struct {
    oauth2Config *oauth2.Config
    verifier     *oidc.IDTokenVerifier
    sessions     map[string]session
    mu           sync.RWMutex
    userStore    model.UserStore
    auditLog     *telemetry.AuditLog
}
```

Add imports:
```go
"github.com/rophy/tostada/internal/model"
"github.com/rophy/tostada/internal/telemetry"
```

Update `NewAuth` to accept and store these:

```go
func NewAuth(ctx context.Context, issuerURL, internalURL, clientID, clientSecret, redirectURL string, userStore model.UserStore, auditLog *telemetry.AuditLog) (*Auth, error) {
```

Store them in the returned struct:
```go
return &Auth{
    oauth2Config: ...,
    verifier:     ...,
    sessions:     make(map[string]session),
    userStore:    userStore,
    auditLog:     auditLog,
}, nil
```

In `CallbackHandler`, after extracting `username`, call `EnsureUser`:
```go
if a.userStore != nil {
    a.userStore.EnsureUser(r.Context(), username)
}
```

After setting the session cookie, emit login event:
```go
if a.auditLog != nil {
    ip, _, _ := net.SplitHostPort(r.RemoteAddr)
    a.auditLog.Log("auth.login", username, "", map[string]string{"ip": ip})
}
```

Add `"net"` to the import block.

In `LogoutHandler`, emit logout event before clearing the session:
```go
if a.auditLog != nil {
    a.mu.RLock()
    sess, ok := a.sessions[cookie.Value]
    a.mu.RUnlock()
    if ok {
        a.auditLog.Log("auth.logout", sess.username, "", nil)
    }
}
```

- [ ] **Step 4: Update main.go to pass new Auth parameters**

In `cmd/tostada/main.go`, update the `auth.NewAuth` call to pass `userStore` and `auditLog`:

```go
authProvider, err = auth.NewAuth(
    context.Background(),
    cfg.OIDC.IssuerURL,
    cfg.OIDC.InternalURL,
    cfg.OIDC.ClientID,
    cfg.OIDC.ClientSecret,
    cfg.OIDC.RedirectURL,
    userStore,
    auditLog,
)
```

Note: The user store and audit log must be created before the auth provider. Reorder the initialization in main:
1. Open database, create device store
2. Create user store (from same DB)
3. Create telemetry loggers
4. Create auth provider (with user store + audit log)
5. Create hub client
6. Create router

- [ ] **Step 5: Add user commands to tostada-cli**

In `cmd/tostada-cli/main.go`, update `usage()` to include user commands:

```go
fmt.Fprintf(os.Stderr, `Usage: tostada-cli <command> [args]

Commands:
  device list                                   List all devices
  device add <name> <display> <proto> <host> <port> <user> <pass>  Add a device
  device remove <name>                          Remove a device
  device grant <device> <username>              Grant user access
  device revoke <device> <username>             Revoke user access
  device import <file.yaml>                     Import devices from YAML
  device access <device>                        List users with access
  user list                                     List all users
  user set-admin <username> <true|false>        Set admin flag
  user delete <username>                        Remove user

Environment:
  TOSTADA_DB   Path to SQLite database (default: tostada.db)
`)
```

Update `main()` to handle the `user` command:

```go
switch os.Args[1] {
case "device":
    // existing device switch
case "user":
    if len(os.Args) < 3 {
        usage()
    }
    switch os.Args[2] {
    case "list":
        cmdUserList(store)
    case "set-admin":
        cmdUserSetAdmin(store)
    case "delete":
        cmdUserDelete(store)
    default:
        usage()
    }
default:
    usage()
}
```

Add the import for the model package and implement user commands:

```go
import (
    "github.com/rophy/tostada/internal/model"
)
```

The store needs to auto-migrate the User model. After opening the DB, add:

```go
store.DB().AutoMigrate(&model.User{})
```

Implement user commands:

```go
func cmdUserList(store *device.GormStore) {
	var users []model.User
	store.DB().Order("username").Find(&users)
	if len(users) == 0 {
		fmt.Println("No users.")
		return
	}
	fmt.Printf("%-20s %-8s %s\n", "USERNAME", "ADMIN", "LAST LOGIN")
	for _, u := range users {
		admin := "no"
		if u.IsAdmin {
			admin = "yes"
		}
		lastLogin := "never"
		if !u.LastLogin.IsZero() {
			lastLogin = u.LastLogin.Format("2006-01-02 15:04")
		}
		fmt.Printf("%-20s %-8s %s\n", u.Username, admin, lastLogin)
	}
}

func cmdUserSetAdmin(store *device.GormStore) {
	if len(os.Args) < 5 {
		fatal("usage: tostada-cli user set-admin <username> <true|false>")
	}
	username := os.Args[3]
	isAdmin := os.Args[4] == "true"

	var u model.User
	if store.DB().Where("username = ?", username).First(&u).Error != nil {
		u = model.User{Username: username, IsAdmin: isAdmin}
		store.DB().Create(&u)
		fmt.Printf("Created user %q (admin=%v).\n", username, isAdmin)
		return
	}

	store.DB().Model(&u).Update("is_admin", isAdmin)
	fmt.Printf("Updated user %q (admin=%v).\n", username, isAdmin)
}

func cmdUserDelete(store *device.GormStore) {
	if len(os.Args) < 4 {
		fatal("usage: tostada-cli user delete <username>")
	}
	username := os.Args[3]
	result := store.DB().Where("username = ?", username).Delete(&model.User{})
	if result.RowsAffected == 0 {
		fmt.Printf("User %q not found.\n", username)
		return
	}
	store.DB().Where("username = ?", username).Delete(&device.UserAccess{})
	fmt.Printf("User %q deleted.\n", username)
}
```

- [ ] **Step 6: Run full test suite**

Run: `cd /home/rophy/projects/tostada && make test > /tmp/test-full.log 2>&1; grep -E 'PASS|FAIL|ok' /tmp/test-full.log`

Expected: All tests pass. Note: some existing tests may need the updated `auth.NewAuth` signature — add `nil, nil` for the new parameters where tests create `Auth` directly.

- [ ] **Step 7: Commit**

```bash
git add internal/api/sessions.go internal/api/devices.go internal/auth/oidc.go cmd/tostada/main.go cmd/tostada-cli/main.go
git commit -m "feat: integrate audit events into handlers and add CLI user commands"
```

---

### Task 7: Access Log Integration & End-to-End Verification

Wire the access logger middleware into the router and verify the complete telemetry pipeline works end-to-end. Add a `hub.ListUsers` method for the admin sessions endpoint.

**Files:**
- Modify: `internal/hub/client.go` — add `ListUsers()` method
- Modify: `internal/api/admin.go` — update `listSessions` to use `ListUsers`
- Create: `internal/api/integration_test.go` — end-to-end test that verifies access + audit logs

**Interfaces:**
- Consumes: all packages from Tasks 1-6
- Produces:
  - `hub.Client.ListUsers() ([]User, error)` — calls JupyterHub `GET /users`
  - Complete telemetry pipeline verified

- [ ] **Step 1: Add ListUsers to hub client**

In `internal/hub/client.go`, add:

```go
func (c *Client) ListUsers() ([]User, error) {
	resp, err := c.do(http.MethodGet, "/users", nil)
	if err != nil {
		return nil, fmt.Errorf("listing users: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("list users failed (%d): %s", resp.StatusCode, body)
	}

	var users []User
	if err := json.NewDecoder(resp.Body).Decode(&users); err != nil {
		return nil, fmt.Errorf("decoding users: %w", err)
	}
	return users, nil
}
```

- [ ] **Step 2: Update admin listSessions**

In `internal/api/admin.go`, replace the placeholder `listSessions`:

```go
func (h *adminHandler) listSessions(w http.ResponseWriter, r *http.Request) {
	if h.hubClient == nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]any{})
		return
	}

	users, err := h.hubClient.ListUsers()
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}

	type sessionInfo struct {
		Username   string `json:"username"`
		ServerName string `json:"serverName"`
		Ready      bool   `json:"ready"`
		URL        string `json:"url"`
	}

	var sessions []sessionInfo
	for _, u := range users {
		for name, srv := range u.Servers {
			sessions = append(sessions, sessionInfo{
				Username:   u.Name,
				ServerName: name,
				Ready:      srv.Ready,
				URL:        srv.URL,
			})
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(sessions)
}
```

- [ ] **Step 3: Write integration test**

Create `internal/api/integration_test.go`:

```go
package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/rophy/tostada/internal/auth"
	"github.com/rophy/tostada/internal/telemetry"
)

func TestAccessLogMiddleware_WritesJSONL(t *testing.T) {
	dir := t.TempDir()
	accessPath := filepath.Join(dir, "access.log")

	al, err := telemetry.NewAccessLogger(accessPath)
	if err != nil {
		t.Fatalf("NewAccessLogger: %v", err)
	}
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
```

- [ ] **Step 4: Run integration test**

Run: `cd /home/rophy/projects/tostada && go test ./internal/api/ -run TestAccessLog -v > /tmp/test-integration.log 2>&1; cat /tmp/test-integration.log`

Expected: PASS.

- [ ] **Step 5: Run full test suite**

Run: `cd /home/rophy/projects/tostada && make test > /tmp/test-full.log 2>&1; grep -E 'PASS|FAIL|ok' /tmp/test-full.log`

Expected: All tests pass.

- [ ] **Step 6: Commit**

```bash
git add internal/hub/client.go internal/api/admin.go internal/api/integration_test.go
git commit -m "feat: complete telemetry integration and admin session listing"
```
