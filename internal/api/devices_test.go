package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/rophy/tostada/internal/auth"
	"github.com/rophy/tostada/internal/config"
	"github.com/rophy/tostada/internal/device"
)

func testDeviceStore(t *testing.T) *device.GormStore {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	store, err := device.NewGormStore(dbPath, device.WithSilentLogger())
	if err != nil {
		t.Fatalf("NewGormStore: %v", err)
	}
	return store
}

func seedTestDevice(t *testing.T, store *device.GormStore) {
	t.Helper()
	d := device.Device{Name: "test-rdp", Display: "Test RDP", Protocol: "rdp", Host: "10.0.0.1", Port: 3389, Username: "testuser", Password: "testpass"}
	if err := store.DB().Create(&d).Error; err != nil {
		t.Fatalf("create device: %v", err)
	}
	if err := store.DB().Create(&device.UserAccess{Username: "alice", DeviceID: d.ID}).Error; err != nil {
		t.Fatalf("create access: %v", err)
	}
}

func TestDevicesList(t *testing.T) {
	store := testDeviceStore(t)
	seedTestDevice(t, store)

	h := &devicesHandler{store: store}

	req := httptest.NewRequest("GET", "/api/devices", nil)
	req = req.WithContext(auth.WithUser(req.Context(), "alice"))
	rec := httptest.NewRecorder()

	h.list(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("Status = %d, want 200", rec.Code)
	}

	var devices []device.Device
	if err := json.NewDecoder(rec.Body).Decode(&devices); err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if len(devices) != 1 {
		t.Fatalf("len = %d, want 1", len(devices))
	}
	if devices[0].Name != "test-rdp" {
		t.Errorf("Name = %q, want %q", devices[0].Name, "test-rdp")
	}
	if devices[0].Host != "10.0.0.1" {
		t.Errorf("Host = %q, want %q", devices[0].Host, "10.0.0.1")
	}
	if devices[0].Username != "testuser" {
		t.Errorf("Username = %q, want %q", devices[0].Username, "testuser")
	}
	if devices[0].Password != "" {
		t.Error("Password should not be serialized in JSON")
	}
}

func TestDevicesList_NoAccess(t *testing.T) {
	store := testDeviceStore(t)
	seedTestDevice(t, store)

	h := &devicesHandler{store: store}

	req := httptest.NewRequest("GET", "/api/devices", nil)
	req = req.WithContext(auth.WithUser(req.Context(), "bob"))
	rec := httptest.NewRecorder()

	h.list(rec, req)

	var devices []device.Device
	json.NewDecoder(rec.Body).Decode(&devices)
	if len(devices) != 0 {
		t.Errorf("len = %d, want 0", len(devices))
	}
}

func TestDevicesConnect(t *testing.T) {
	guacSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/tokens" && r.Method == "POST" {
			json.NewEncoder(w).Encode(map[string]string{"authToken": "fake-auth-token"})
			return
		}
		t.Errorf("unexpected guac request: %s %s", r.Method, r.URL.Path)
		w.WriteHeader(http.StatusNotFound)
	}))
	defer guacSrv.Close()

	store := testDeviceStore(t)
	seedTestDevice(t, store)

	h := &devicesHandler{
		store: store,
		guacCfg: config.GuacamoleConfig{
			URL:           guacSrv.URL,
			JSONSecretKey: "4c0b569e4c96df157eee1b65dd0e4d41",
		},
	}

	req := httptest.NewRequest("GET", "/api/devices/test-rdp/connect", nil)
	req.SetPathValue("name", "test-rdp")
	req = req.WithContext(auth.WithUser(req.Context(), "alice"))
	rec := httptest.NewRecorder()

	h.connect(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("Status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if resp["token"] != "fake-auth-token" {
		t.Errorf("token = %q, want %q", resp["token"], "fake-auth-token")
	}
	if resp["connectionId"] != "test-rdp" {
		t.Errorf("connectionId = %q, want %q", resp["connectionId"], "test-rdp")
	}
}

func TestDevicesConnect_NotFound(t *testing.T) {
	store := testDeviceStore(t)
	seedTestDevice(t, store)

	h := &devicesHandler{store: store}

	req := httptest.NewRequest("GET", "/api/devices/nonexistent/connect", nil)
	req.SetPathValue("name", "nonexistent")
	req = req.WithContext(auth.WithUser(req.Context(), "alice"))
	rec := httptest.NewRecorder()

	h.connect(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("Status = %d, want 404", rec.Code)
	}
}

func TestDevicesConnect_Unauthorized(t *testing.T) {
	store := testDeviceStore(t)
	seedTestDevice(t, store)

	h := &devicesHandler{store: store}

	req := httptest.NewRequest("GET", "/api/devices/test-rdp/connect", nil)
	req.SetPathValue("name", "test-rdp")
	req = req.WithContext(auth.WithUser(req.Context(), "bob"))
	rec := httptest.NewRecorder()

	h.connect(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("Status = %d, want 404", rec.Code)
	}
}

type errorStore struct{}

func (e *errorStore) ListDevices(_ context.Context, _ string) ([]device.Device, error) {
	return nil, fmt.Errorf("db error")
}

func (e *errorStore) GetDevice(_ context.Context, _ string, _ string) (*device.Device, error) {
	return nil, fmt.Errorf("not found")
}

func TestDevicesList_StoreError(t *testing.T) {
	h := &devicesHandler{store: &errorStore{}}

	req := httptest.NewRequest("GET", "/api/devices", nil)
	req = req.WithContext(auth.WithUser(req.Context(), "alice"))
	rec := httptest.NewRecorder()

	h.list(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("Status = %d, want 500", rec.Code)
	}
}

func TestDevicesConnect_GetDeviceError(t *testing.T) {
	h := &devicesHandler{store: &errorStore{}}

	req := httptest.NewRequest("GET", "/api/devices/bad/connect", nil)
	req.SetPathValue("name", "bad")
	req = req.WithContext(auth.WithUser(req.Context(), "alice"))
	rec := httptest.NewRecorder()

	h.connect(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("Status = %d, want 404", rec.Code)
	}
}
