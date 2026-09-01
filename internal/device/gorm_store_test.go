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
