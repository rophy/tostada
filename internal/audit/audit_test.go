package audit

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestAuditLog_Log(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "audit.log")

	al := NewAuditLog(path, 50, 5)
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

	al := NewAuditLog(path, 50, 5)
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

	al1 := NewAuditLog(path, 50, 5)
	al1.Log("auth.login", "alice", "", nil)
	al1.Close()

	al2 := NewAuditLog(path, 50, 5)
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
