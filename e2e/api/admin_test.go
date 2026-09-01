//go:build e2e

package api

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/rophy/tostada/e2e/helpers"
)

func TestAdminUsers_Forbidden(t *testing.T) {
	c := helpers.NewClient(t)
	c.Login("alice")

	resp := c.Get("/api/admin/users")
	defer resp.Body.Close()

	// 403 if admin API is deployed but alice isn't admin.
	// 404 if admin API isn't deployed yet.
	if resp.StatusCode == http.StatusNotFound {
		t.Skip("admin API not deployed — redeploy with admin routes")
	}
	c.ExpectStatus(resp, 403)
}

func TestAdminUsers_AsAdmin(t *testing.T) {
	// Uses alice — must be promoted to admin via tostada-cli first.
	c := helpers.NewClient(t)
	c.Login("alice")

	resp := c.Get("/api/admin/users")
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusNotFound:
		t.Skip("admin API not deployed — redeploy with admin routes")
	case http.StatusForbidden:
		t.Skip("alice is not admin — run: tostada-cli user set-admin alice true")
	}
	c.ExpectStatus(resp, 200)

	var users []map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&users); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(users) == 0 {
		t.Fatal("expected at least one user")
	}
}
