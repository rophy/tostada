package hub

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGetUser(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "token test-token" {
			t.Errorf("Authorization = %q", r.Header.Get("Authorization"))
		}
		if r.URL.Path != "/users/alice" {
			t.Errorf("Path = %q, want /users/alice", r.URL.Path)
		}
		json.NewEncoder(w).Encode(User{
			Name: "alice",
			Servers: map[string]Server{
				"my-notebook": {Name: "my-notebook", Ready: true, URL: "/user/alice/my-notebook/"},
			},
		})
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "test-token")
	user, err := client.GetUser("alice")
	if err != nil {
		t.Fatalf("GetUser() error: %v", err)
	}
	if user.Name != "alice" {
		t.Errorf("Name = %q, want %q", user.Name, "alice")
	}
	if len(user.Servers) != 1 {
		t.Fatalf("len(Servers) = %d, want 1", len(user.Servers))
	}
	s := user.Servers["my-notebook"]
	if !s.Ready || s.URL != "/user/alice/my-notebook/" {
		t.Errorf("Server = %+v", s)
	}
}

func TestSpawnServer(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("Method = %q, want POST", r.Method)
		}
		if r.URL.Path != "/users/alice/servers/my-desktop" {
			t.Errorf("Path = %q", r.URL.Path)
		}
		var body map[string]string
		json.NewDecoder(r.Body).Decode(&body)
		if body["profile"] != "Ubuntu Desktop (KasmVNC)" {
			t.Errorf("profile = %q", body["profile"])
		}
		w.WriteHeader(http.StatusCreated)
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "test-token")
	err := client.SpawnServer("alice", "my-desktop", "Ubuntu Desktop (KasmVNC)")
	if err != nil {
		t.Fatalf("SpawnServer() error: %v", err)
	}
}

func TestStopServer(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("Method = %q, want DELETE", r.Method)
		}
		if r.URL.Path != "/users/alice/servers/my-desktop" {
			t.Errorf("Path = %q", r.URL.Path)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "test-token")
	err := client.StopServer("alice", "my-desktop")
	if err != nil {
		t.Fatalf("StopServer() error: %v", err)
	}
}
