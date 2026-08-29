package device

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func testStore(t *testing.T) *GormStore {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	store, err := NewGormStore(dbPath)
	if err != nil {
		t.Fatalf("NewGormStore: %v", err)
	}
	return store
}

func seedDevice(t *testing.T, store *GormStore) {
	t.Helper()
	d := Device{Name: "test-device", Display: "Test Device", Protocol: "rdp", Host: "10.0.0.1", Port: 3389, Username: "testuser", Password: "testpass"}
	if err := store.db.Create(&d).Error; err != nil {
		t.Fatalf("create device: %v", err)
	}
	if err := store.db.Create(&UserAccess{Username: "alice", DeviceID: d.ID}).Error; err != nil {
		t.Fatalf("create access: %v", err)
	}
}

func TestListDevices(t *testing.T) {
	store := testStore(t)
	seedDevice(t, store)

	devices, err := store.ListDevices(context.Background(), "alice")
	if err != nil {
		t.Fatalf("ListDevices: %v", err)
	}
	if len(devices) != 1 {
		t.Fatalf("len = %d, want 1", len(devices))
	}
	if devices[0].Name != "test-device" {
		t.Errorf("Name = %q, want %q", devices[0].Name, "test-device")
	}
}

func TestListDevices_NoAccess(t *testing.T) {
	store := testStore(t)
	seedDevice(t, store)

	devices, err := store.ListDevices(context.Background(), "bob")
	if err != nil {
		t.Fatalf("ListDevices: %v", err)
	}
	if len(devices) != 0 {
		t.Errorf("len = %d, want 0", len(devices))
	}
}

func TestGetDevice(t *testing.T) {
	store := testStore(t)
	seedDevice(t, store)

	d, err := store.GetDevice(context.Background(), "alice", "test-device")
	if err != nil {
		t.Fatalf("GetDevice: %v", err)
	}
	if d.Host != "10.0.0.1" {
		t.Errorf("Host = %q, want %q", d.Host, "10.0.0.1")
	}
	if d.Username != "testuser" {
		t.Errorf("Username = %q, want %q", d.Username, "testuser")
	}
}

func TestGetDevice_Unauthorized(t *testing.T) {
	store := testStore(t)
	seedDevice(t, store)

	_, err := store.GetDevice(context.Background(), "bob", "test-device")
	if err == nil {
		t.Error("expected error for unauthorized user")
	}
}

func TestGetDevice_NotFound(t *testing.T) {
	store := testStore(t)

	_, err := store.GetDevice(context.Background(), "alice", "nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent device")
	}
}

func TestNewGormStore_CreatesDB(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "new.db")
	_, err := NewGormStore(dbPath)
	if err != nil {
		t.Fatalf("NewGormStore: %v", err)
	}
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Error("database file should exist")
	}
}
