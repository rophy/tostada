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
	"github.com/rophy/tostada/internal/hub"
	"github.com/rophy/tostada/internal/model"
	"github.com/rophy/tostada/internal/audit"

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

func testAuditLog(t *testing.T) *audit.AuditLog {
	t.Helper()
	path := filepath.Join(t.TempDir(), "audit.log")
	al := audit.NewAuditLog(path, 50, 5)
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
	userStore, _ := testUserStore(t)
	devStore := testDeviceStore(t)
	seedTestDevice(t, devStore)

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

func TestAdminUpdateDevice(t *testing.T) {
	userStore, _ := testUserStore(t)
	devStore := testDeviceStore(t)
	seedTestDevice(t, devStore)

	h := &adminHandler{userStore: userStore, deviceStore: devStore, auditLog: testAuditLog(t)}

	body := strings.NewReader(`{"display":"Renamed"}`)
	req := httptest.NewRequest("PUT", "/api/admin/devices/test-rdp", body)
	req.SetPathValue("name", "test-rdp")
	req = req.WithContext(auth.WithUser(req.Context(), "admin"))
	rec := httptest.NewRecorder()
	h.updateDevice(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("Status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}
}

func TestAdminUpdateDevice_InvalidBody(t *testing.T) {
	userStore, _ := testUserStore(t)
	devStore := testDeviceStore(t)
	seedTestDevice(t, devStore)

	h := &adminHandler{userStore: userStore, deviceStore: devStore, auditLog: testAuditLog(t)}

	body := strings.NewReader(`not json`)
	req := httptest.NewRequest("PUT", "/api/admin/devices/test-rdp", body)
	req.SetPathValue("name", "test-rdp")
	req = req.WithContext(auth.WithUser(req.Context(), "admin"))
	rec := httptest.NewRecorder()
	h.updateDevice(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("Status = %d, want 400", rec.Code)
	}
}

func TestAdminListSessions_NoHubClient(t *testing.T) {
	userStore, _ := testUserStore(t)
	h := &adminHandler{userStore: userStore, auditLog: testAuditLog(t)}

	req := httptest.NewRequest("GET", "/api/admin/sessions", nil)
	req = req.WithContext(auth.WithUser(req.Context(), "admin"))
	rec := httptest.NewRecorder()
	h.listSessions(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("Status = %d, want 200", rec.Code)
	}

	var sessions []any
	json.NewDecoder(rec.Body).Decode(&sessions)
	if len(sessions) != 0 {
		t.Errorf("len = %d, want 0", len(sessions))
	}
}

func TestAdminListSessions(t *testing.T) {
	hubSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`[{"name":"alice","servers":{"default":{"name":"default","ready":true,"url":"/user/alice/default/"}}}]`))
	}))
	defer hubSrv.Close()

	userStore, _ := testUserStore(t)
	h := &adminHandler{
		userStore: userStore,
		hubClient: hub.NewClient(hubSrv.URL, "test-token"),
		auditLog:  testAuditLog(t),
	}

	req := httptest.NewRequest("GET", "/api/admin/sessions", nil)
	req = req.WithContext(auth.WithUser(req.Context(), "admin"))
	rec := httptest.NewRecorder()
	h.listSessions(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("Status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}

	var sessions []struct {
		Username   string `json:"username"`
		ServerName string `json:"serverName"`
		Ready      bool   `json:"ready"`
		URL        string `json:"url"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&sessions); err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("len = %d, want 1", len(sessions))
	}
	if sessions[0].Username != "alice" || sessions[0].ServerName != "default" || !sessions[0].Ready {
		t.Errorf("unexpected session: %+v", sessions[0])
	}
}

func TestAdminListSessions_HubError(t *testing.T) {
	hubSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer hubSrv.Close()

	userStore, _ := testUserStore(t)
	h := &adminHandler{
		userStore: userStore,
		hubClient: hub.NewClient(hubSrv.URL, "test-token"),
		auditLog:  testAuditLog(t),
	}

	req := httptest.NewRequest("GET", "/api/admin/sessions", nil)
	req = req.WithContext(auth.WithUser(req.Context(), "admin"))
	rec := httptest.NewRecorder()
	h.listSessions(rec, req)

	if rec.Code != http.StatusBadGateway {
		t.Errorf("Status = %d, want 502", rec.Code)
	}
}

func TestAdminStopSession_NoHubClient(t *testing.T) {
	userStore, _ := testUserStore(t)
	h := &adminHandler{userStore: userStore, auditLog: testAuditLog(t)}

	req := httptest.NewRequest("DELETE", "/api/admin/sessions/alice/default", nil)
	req.SetPathValue("username", "alice")
	req.SetPathValue("server", "default")
	req = req.WithContext(auth.WithUser(req.Context(), "admin"))
	rec := httptest.NewRecorder()
	h.stopSession(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("Status = %d, want 503", rec.Code)
	}
}

func TestAdminStopSession(t *testing.T) {
	hubSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer hubSrv.Close()

	userStore, _ := testUserStore(t)
	h := &adminHandler{
		userStore: userStore,
		hubClient: hub.NewClient(hubSrv.URL, "test-token"),
		auditLog:  testAuditLog(t),
	}

	req := httptest.NewRequest("DELETE", "/api/admin/sessions/alice/default", nil)
	req.SetPathValue("username", "alice")
	req.SetPathValue("server", "default")
	req = req.WithContext(auth.WithUser(req.Context(), "admin"))
	rec := httptest.NewRecorder()
	h.stopSession(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("Status = %d, want 204; body: %s", rec.Code, rec.Body.String())
	}
}

func TestAdminStopSession_HubError(t *testing.T) {
	hubSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer hubSrv.Close()

	userStore, _ := testUserStore(t)
	h := &adminHandler{
		userStore: userStore,
		hubClient: hub.NewClient(hubSrv.URL, "test-token"),
		auditLog:  testAuditLog(t),
	}

	req := httptest.NewRequest("DELETE", "/api/admin/sessions/alice/default", nil)
	req.SetPathValue("username", "alice")
	req.SetPathValue("server", "default")
	req = req.WithContext(auth.WithUser(req.Context(), "admin"))
	rec := httptest.NewRecorder()
	h.stopSession(rec, req)

	if rec.Code != http.StatusBadGateway {
		t.Errorf("Status = %d, want 502", rec.Code)
	}
}

func TestAdminUpdateUser_InvalidBody(t *testing.T) {
	userStore, _ := testUserStore(t)
	ctx := context.Background()
	userStore.EnsureUser(ctx, "alice")

	h := &adminHandler{userStore: userStore, auditLog: testAuditLog(t)}

	body := strings.NewReader(`not json`)
	req := httptest.NewRequest("PATCH", "/api/admin/users/alice", body)
	req.SetPathValue("username", "alice")
	req = req.WithContext(auth.WithUser(req.Context(), "admin"))
	rec := httptest.NewRecorder()
	h.updateUser(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("Status = %d, want 400", rec.Code)
	}
}

func TestAdminCreateDevice_InvalidBody(t *testing.T) {
	userStore, _ := testUserStore(t)
	devStore := testDeviceStore(t)

	h := &adminHandler{userStore: userStore, deviceStore: devStore, auditLog: testAuditLog(t)}

	body := strings.NewReader(`not json`)
	req := httptest.NewRequest("POST", "/api/admin/devices", body)
	req = req.WithContext(auth.WithUser(req.Context(), "admin"))
	rec := httptest.NewRecorder()
	h.createDevice(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("Status = %d, want 400", rec.Code)
	}
}

func TestAdminGrantAccess_InvalidBody(t *testing.T) {
	userStore, _ := testUserStore(t)
	devStore := testDeviceStore(t)
	seedTestDevice(t, devStore)

	h := &adminHandler{userStore: userStore, deviceStore: devStore, auditLog: testAuditLog(t)}

	body := strings.NewReader(`not json`)
	req := httptest.NewRequest("POST", "/api/admin/devices/test-rdp/grants", body)
	req.SetPathValue("name", "test-rdp")
	req = req.WithContext(auth.WithUser(req.Context(), "admin"))
	rec := httptest.NewRecorder()
	h.grantAccess(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("Status = %d, want 400", rec.Code)
	}
}
