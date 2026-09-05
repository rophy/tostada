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
				"my-notebook": {
					Name:        "my-notebook",
					Ready:       true,
					URL:         "/user/alice/my-notebook/",
					UserOptions: map[string]string{"profile": "jupyter-notebook"},
				},
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
	if s.UserOptions["profile"] != "jupyter-notebook" {
		t.Errorf("UserOptions[profile] = %q, want %q", s.UserOptions["profile"], "jupyter-notebook")
	}
}

func TestListUsers(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/users" {
			t.Errorf("Path = %q, want /users", r.URL.Path)
		}
		json.NewEncoder(w).Encode([]User{
			{
				Name: "alice",
				Servers: map[string]Server{
					"default": {Name: "default", Ready: true, URL: "/user/alice/default/"},
				},
			},
		})
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "test-token")
	users, err := client.ListUsers()
	if err != nil {
		t.Fatalf("ListUsers() error: %v", err)
	}
	if len(users) != 1 || users[0].Name != "alice" {
		t.Fatalf("users = %+v", users)
	}
}

func TestListUsers_Error(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "test-token")
	if _, err := client.ListUsers(); err == nil {
		t.Fatal("ListUsers() expected error")
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

func TestEnsureUser_Created(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("Method = %q, want POST", r.Method)
		}
		if r.URL.Path != "/users/alice" {
			t.Errorf("Path = %q, want /users/alice", r.URL.Path)
		}
		w.WriteHeader(http.StatusCreated)
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "test-token")
	err := client.EnsureUser("alice")
	if err != nil {
		t.Fatalf("EnsureUser() error: %v", err)
	}
}

func TestEnsureUser_AlreadyExists(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusConflict)
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "test-token")
	err := client.EnsureUser("alice")
	if err != nil {
		t.Fatalf("EnsureUser() should not error on 409, got: %v", err)
	}
}

func TestEnsureUser_Error(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("internal error"))
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "test-token")
	err := client.EnsureUser("alice")
	if err == nil {
		t.Fatal("EnsureUser() should error on 500")
	}
}

func TestCreateUserToken(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("Method = %q, want POST", r.Method)
		}
		if r.URL.Path != "/users/alice/tokens" {
			t.Errorf("Path = %q, want /users/alice/tokens", r.URL.Path)
		}
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]string{"token": "abc123"})
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "test-token")
	token, err := client.CreateUserToken("alice")
	if err != nil {
		t.Fatalf("CreateUserToken() error: %v", err)
	}
	if token != "abc123" {
		t.Errorf("token = %q, want %q", token, "abc123")
	}
}

func TestCreateUserToken_Error(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("internal error"))
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "test-token")
	_, err := client.CreateUserToken("alice")
	if err == nil {
		t.Fatal("CreateUserToken() should error on 500")
	}
}

func TestGetUser_Error(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("not found"))
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "test-token")
	_, err := client.GetUser("alice")
	if err == nil {
		t.Fatal("GetUser() should error on 404")
	}
}

func TestStopServer_Error(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("internal error"))
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "test-token")
	err := client.StopServer("alice", "my-desktop")
	if err == nil {
		t.Fatal("StopServer() should error on 500")
	}
}

func TestServerProgress(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("Method = %q, want GET", r.Method)
		}
		if r.URL.Path != "/users/alice/servers/my-nb/progress" {
			t.Errorf("Path = %q", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "token test-token" {
			t.Errorf("Authorization = %q", r.Header.Get("Authorization"))
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("data: {\"progress\": 100}\n\n"))
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "test-token")
	resp, err := client.ServerProgress("alice", "my-nb")
	if err != nil {
		t.Fatalf("ServerProgress() error: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "text/event-stream" {
		t.Errorf("Content-Type = %q", ct)
	}
}

func TestServerProgress_ConnectionError(t *testing.T) {
	client := NewClient("http://127.0.0.1:1", "test-token")
	_, err := client.ServerProgress("alice", "my-nb")
	if err == nil {
		t.Fatal("ServerProgress() should error on connection refused")
	}
}

func TestSpawnServer_Error(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("internal error"))
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "test-token")
	err := client.SpawnServer("alice", "my-desktop", "ubuntu-desktop-kasmvnc")
	if err == nil {
		t.Fatal("SpawnServer() should error on 500")
	}
}
