# Telemetry & Admin Backend Design

> **Spec for:** tostada workspace telemetry and admin management API

**Goal:** Add structured JSONL logging (access + audit) and a full admin API for managing users, devices, and workspace sessions — enabling operational visibility, compliance audit trails, and admin self-service without CLI access.

**Architecture:** A telemetry package writes two JSONL log streams (access log via middleware, audit log via explicit calls from handlers). A new `users` table in SQLite tracks admin status. Admin API endpoints under `/api/admin/*` are gated by admin middleware and cover user CRUD, device CRUD, and cross-user session management. All mutating actions emit audit events.

**Tech Stack:** Go, SQLite (GORM), structured JSONL logging, existing JupyterHub API integration.

## Global Constraints

- Go 1.22+ (net/http routing with method patterns)
- SQLite via GORM for persistence (existing pattern)
- JSONL logs written to configurable directory, append-only, no rotation (external concern)
- No new dependencies beyond stdlib for telemetry (use `encoding/json` + `os.File`)
- Admin flag is a boolean on the users table — no RBAC, no roles beyond admin/non-admin
- All admin API responses are JSON
- All mutating admin actions must emit an audit event
- Existing non-admin endpoints must not change behavior
- Test coverage must stay above 80% for Go

---

## 1. JSONL Telemetry

### 1.1 Access Log

An HTTP middleware that logs every request to `access.log` in JSONL format.

**Log entry schema:**

```json
{
  "ts": "2026-09-01T00:15:00.123Z",
  "method": "POST",
  "path": "/api/sessions",
  "user": "alice",
  "status": 200,
  "duration_ms": 342,
  "ip": "10.0.0.1",
  "user_agent": "Mozilla/5.0 ..."
}
```

- `ts`: RFC3339 with milliseconds, UTC
- `user`: authenticated username or `""` for unauthenticated requests
- `ip`: from `r.RemoteAddr`, stripped of port
- Requests to static assets (`/guacamole-common-js/`, `/oidc/`, `/authorize/`) are excluded from the access log to reduce noise

**Implementation:** `internal/telemetry/access.go`

- `AccessLogger` struct holds an `*os.File` and a `sync.Mutex`
- `NewAccessLogger(path string) (*AccessLogger, error)` opens file append-only, creates if missing
- `Middleware(next http.Handler) http.Handler` wraps the response writer to capture status code and measures duration
- `Close()` flushes and closes the file
- The middleware extracts `user` from context (empty string if not authenticated) — it must wrap the auth middleware's output, not the raw mux

### 1.2 Audit Log

Explicit audit events emitted by handlers when significant actions occur.

**Log entry schema:**

```json
{
  "ts": "2026-09-01T00:15:00.123Z",
  "event": "session.spawn",
  "user": "alice",
  "actor": "",
  "detail": {
    "workspace": "jupyter",
    "server": "jupyter-alice"
  }
}
```

- `user`: the user being acted upon (the subject)
- `actor`: the admin performing the action (empty if user is acting on their own behalf)
- `detail`: event-specific key-value pairs

**Event types:**

| Event | Emitted when | Detail fields |
|-------|-------------|---------------|
| `auth.login` | OIDC callback succeeds | `ip` |
| `auth.logout` | User logs out | |
| `session.spawn` | Workspace session created | `workspace`, `server` |
| `session.stop` | Workspace session stopped | `server` |
| `session.connect` | User connects to workspace | `server`, `type` (jupyterhub/guacamole) |
| `device.connect` | User connects to device | `device` |
| `admin.user.create` | Admin creates user | `target` |
| `admin.user.update` | Admin updates user | `target`, `changes` |
| `admin.user.delete` | Admin deletes user | `target` |
| `admin.device.add` | Admin adds device | `device` |
| `admin.device.update` | Admin updates device | `device` |
| `admin.device.remove` | Admin removes device | `device` |
| `admin.device.grant` | Admin grants device access | `device`, `target` |
| `admin.device.revoke` | Admin revokes device access | `device`, `target` |
| `admin.session.stop` | Admin stops another user's session | `target`, `server` |

**Implementation:** `internal/telemetry/audit.go`

- `AuditLog` struct holds an `*os.File` and a `sync.Mutex`
- `NewAuditLog(path string) (*AuditLog, error)` opens file append-only
- `Log(event string, user string, actor string, detail map[string]string)` writes one JSON line
- `Close()` flushes and closes

### 1.3 Configuration

Add to `config.Config`:

```go
type TelemetryConfig struct {
    LogDir string `yaml:"logDir"` // default: "/data/logs"
}
```

Add `Telemetry TelemetryConfig` field to `Config`. The Helm chart sets this via ConfigMap. The directory is created on startup if it doesn't exist.

### 1.4 Integration Points

- Access logger middleware wraps the authenticated mux in `router.go`
- Audit log is passed to handler structs (`sessionsHandler`, `devicesHandler`, `adminHandler`) as a dependency
- Auth callback handler (`oidc.go`) emits `auth.login`; logout emits `auth.logout`
- Existing session/device handlers emit their respective events
- `tostada-cli` device commands also emit audit events (they open the audit log file directly)

---

## 2. User Model & Admin Role

### 2.1 Users Table

```go
// internal/model/user.go
type User struct {
    ID        uint      `gorm:"primarykey" json:"id"`
    Username  string    `gorm:"uniqueIndex;not null" json:"username"`
    IsAdmin   bool      `gorm:"default:false" json:"isAdmin"`
    LastLogin time.Time `json:"lastLogin"`
    CreatedAt time.Time `json:"createdAt"`
    UpdatedAt time.Time `json:"updatedAt"`
}
```

GORM auto-migrates on startup (existing pattern from `device.GormStore`).

### 2.2 User Store

```go
// internal/model/store.go
type UserStore interface {
    GetUser(ctx context.Context, username string) (*User, error)
    ListUsers(ctx context.Context) ([]User, error)
    EnsureUser(ctx context.Context, username string) (*User, error) // create if not exists
    UpdateUser(ctx context.Context, username string, updates map[string]any) error
    DeleteUser(ctx context.Context, username string) error
    IsAdmin(ctx context.Context, username string) (bool, error)
}
```

`EnsureUser` is called from the OIDC callback — it creates the user record on first login and updates `LastLogin` on every login.

### 2.3 Migration

The `user_accesses` table currently stores `username` as a string. This spec does NOT migrate it to a foreign key — the join is done by username string matching. This avoids a complex migration and keeps the device access system working without changes. A future spec can normalize this if needed.

### 2.4 Admin Middleware

```go
// internal/api/admin.go
func AdminMiddleware(store model.UserStore) func(http.Handler) http.Handler
```

- Extracts username from context (already authenticated by auth middleware)
- Calls `store.IsAdmin(ctx, username)`
- Returns 403 `{"error": "admin access required"}` if not admin
- Applied to all `/api/admin/*` routes in `router.go`

### 2.5 Bootstrap

The first admin is set via CLI:

```bash
tostada-cli user set-admin <username> true
```

This directly updates the SQLite database. The user must have logged in at least once (so the record exists). If the user doesn't exist, the CLI creates the record.

---

## 3. Admin API

All endpoints require authentication + admin middleware.

### 3.1 User Management

**`GET /api/admin/users`**
- Returns: `[{"id":1, "username":"alice", "isAdmin":true, "lastLogin":"...", "createdAt":"..."}]`
- Supports `?q=` query parameter for filtering by username substring

**`PATCH /api/admin/users/{username}`**
- Body: `{"isAdmin": true}` (partial update)
- Returns: updated user object
- Emits: `admin.user.update` audit event
- Cannot remove admin from yourself (safety check)

**`DELETE /api/admin/users/{username}`**
- Removes user record and all their `user_accesses` entries
- Returns: 204
- Emits: `admin.user.delete` audit event
- Cannot delete yourself (safety check)

### 3.2 Device Management

**`GET /api/admin/devices`**
- Returns all devices (not filtered by access), each with a list of granted usernames
- Response: `[{"name":"mac", "displayName":"Mac Desktop", ..., "grants":["alice","bob"]}]`

**`POST /api/admin/devices`**
- Body: `{"name":"mac", "displayName":"Mac Desktop", "protocol":"rdp", "host":"10.0.0.1", "port":3389, "username":"admin", "password":"secret"}`
- Returns: 201 + created device
- Emits: `admin.device.add` audit event

**`PUT /api/admin/devices/{name}`**
- Body: partial or full device update (same schema as POST, minus `name`)
- Returns: updated device
- Emits: `admin.device.update` audit event

**`DELETE /api/admin/devices/{name}`**
- Removes device and all associated access grants
- Returns: 204
- Emits: `admin.device.remove` audit event

**`POST /api/admin/devices/{name}/grants`**
- Body: `{"username": "alice"}`
- Returns: 201
- Emits: `admin.device.grant` audit event

**`DELETE /api/admin/devices/{name}/grants/{username}`**
- Returns: 204
- Emits: `admin.device.revoke` audit event

### 3.3 Session Management

**`GET /api/admin/sessions`**
- Lists all running sessions across all JupyterHub users
- Calls JupyterHub `GET /users` with the admin service token, collects all servers
- Response: `[{"username":"alice", "serverName":"jupyter", "ready":true, "started":"...", "profile":"jupyter-notebook", "url":"/user/alice/jupyter/"}]`

**`DELETE /api/admin/sessions/{username}/{server}`**
- Stops any user's session via `hub.StopServer(username, server)`
- Returns: 204
- Emits: `admin.session.stop` audit event with `actor` = admin, `user` = target user

### 3.4 Device Store Extension

The existing `device.Store` interface needs additional methods for admin operations. Add to the interface or create a separate `AdminDeviceStore`:

```go
type AdminStore interface {
    Store // embeds existing interface
    ListAllDevices(ctx context.Context) ([]DeviceWithGrants, error)
    CreateDevice(ctx context.Context, d *Device) error
    UpdateDevice(ctx context.Context, name string, updates map[string]any) error
    DeleteDevice(ctx context.Context, name string) error
    GrantAccess(ctx context.Context, deviceName string, username string) error
    RevokeAccess(ctx context.Context, deviceName string, username string) error
}
```

The existing `GormStore` already has direct DB access — these methods are added to it. The `Store` interface stays unchanged so existing non-admin code is unaffected.

---

## 4. CLI Extensions

### 4.1 User Commands

```bash
tostada-cli user list                          # list all users
tostada-cli user set-admin <username> <bool>   # set admin flag
tostada-cli user delete <username>             # remove user + access grants
```

### 4.2 Audit Events from CLI

CLI device commands (`device add`, `device remove`, `device grant`, `device revoke`) emit audit events by opening the audit log file at the configured path. The CLI reads `TOSTADA_LOG_DIR` (default `/data/logs`) to find `audit.log`.

---

## 5. Router Changes

Updated `NewRouter` signature:

```go
func NewRouter(
    cfg *config.Config,
    hubClient *hub.Client,
    authProvider *auth.Auth,
    deviceStore device.AdminStore,
    userStore model.UserStore,
    auditLog *telemetry.AuditLog,
    accessLogger *telemetry.AccessLogger,
) *http.ServeMux
```

Registration:

```go
// Existing authenticated routes unchanged
authed := http.NewServeMux()
// ... existing routes ...

// Admin routes
adminMux := http.NewServeMux()
adminHandler := &adminHandler{userStore, deviceStore, hubClient, auditLog}
adminMux.HandleFunc("GET /api/admin/users", adminHandler.listUsers)
adminMux.HandleFunc("PATCH /api/admin/users/{username}", adminHandler.updateUser)
adminMux.HandleFunc("DELETE /api/admin/users/{username}", adminHandler.deleteUser)
// ... device and session routes ...

authed.Handle("/api/admin/", AdminMiddleware(userStore)(adminMux))

// Wrap with access logger
mux.Handle("/api/", accessLogger.Middleware(authProvider.Middleware(authed)))
```

---

## 6. Testing

- `internal/telemetry/` — unit tests for access logger and audit log (write to temp files, verify JSONL output)
- `internal/model/` — unit tests for user store (GORM with temp SQLite)
- `internal/api/admin_test.go` — HTTP handler tests for all admin endpoints (mock stores, verify responses and audit events)
- Admin middleware test — verify 403 for non-admin, pass-through for admin
- Integration: existing tests must continue to pass (no behavior changes to non-admin endpoints)

---

## 7. Not in Scope

- **Admin UI** — follow-up spec; this spec delivers the API
- **Log rotation** — external concern (logrotate, shipper config)
- **Prometheus / OpenTelemetry** — JSONL covers current needs
- **RBAC / fine-grained permissions** — boolean admin is sufficient
- **Rate limiting** — not needed at current scale
- **Normalizing `user_accesses` FK** — deferred to avoid migration complexity
