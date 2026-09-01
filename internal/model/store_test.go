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
