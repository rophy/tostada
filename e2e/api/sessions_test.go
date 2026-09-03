//go:build e2e

package api

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/rophy/tostada/e2e/helpers"
)

func cleanupStaleSessions(t *testing.T, c *helpers.Client) {
	t.Helper()
	resp := c.Get("/api/sessions")
	var servers map[string]any
	json.NewDecoder(resp.Body).Decode(&servers)
	resp.Body.Close()

	for name := range servers {
		if strings.HasPrefix(name, "e2e-") {
			t.Logf("cleaning up stale session: %s", name)
			r := c.Delete("/api/sessions/" + name)
			r.Body.Close()
		}
	}
}

func TestSessionLifecycle(t *testing.T) {
	c := helpers.NewClient(t)
	c.Login("alice")

	cleanupStaleSessions(t, c)

	serverName := fmt.Sprintf("e2e-%d", time.Now().Unix())

	t.Cleanup(func() {
		r := c.Delete("/api/sessions/" + serverName)
		r.Body.Close()
	})

	// Spawn a KasmVNC session (chrome is smaller, faster to pull in CI)
	resp := c.PostJSON("/api/sessions", fmt.Sprintf(`{"workspace":"kasmvnc-chrome","serverName":"%s"}`, serverName))
	defer resp.Body.Close()
	c.ExpectStatus(resp, 201)

	var createResp map[string]string
	json.NewDecoder(resp.Body).Decode(&createResp)
	if createResp["status"] != "spawning" {
		t.Fatalf("expected status=spawning, got %s", createResp["status"])
	}

	// Poll until ready (timeout 120s for pod startup)
	ready := false
	deadline := time.Now().Add(120 * time.Second)
	for time.Now().Before(deadline) {
		resp := c.Get("/api/sessions")
		var servers map[string]struct {
			Ready bool `json:"ready"`
		}
		json.NewDecoder(resp.Body).Decode(&servers)
		resp.Body.Close()

		if srv, ok := servers[serverName]; ok && srv.Ready {
			ready = true
			break
		}
		time.Sleep(3 * time.Second)
	}
	if !ready {
		t.Fatal("session did not become ready within 120s")
	}

	// Get connection URL
	resp = c.Get("/api/sessions/" + serverName + "/connect")
	c.ExpectStatus(resp, 200)

	var connectResp map[string]string
	json.NewDecoder(resp.Body).Decode(&connectResp)
	if connectResp["url"] == "" {
		t.Fatal("connect response has no url")
	}
	t.Logf("connect url: %s", connectResp["url"])

	// Verify the proxied page loads (not 404).
	// This catches prefix-stripping bugs: if servesFromRoot isn't working,
	// CHP forwards with the prefix and KasmVNC returns 404.
	proxyURL := connectResp["url"]
	parsed, err := url.Parse(proxyURL)
	if err != nil {
		t.Fatalf("parse connect url: %v", err)
	}
	// Use just the path+query so it goes through our gateway
	proxyResp := c.Get(parsed.Path + "?" + parsed.RawQuery)
	defer proxyResp.Body.Close()
	if proxyResp.StatusCode == 404 {
		t.Fatal("proxied session page returned 404 — prefix-stripping likely broken")
	}
	t.Logf("proxy status: %d", proxyResp.StatusCode)
}
