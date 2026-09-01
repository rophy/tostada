//go:build e2e

package api

import (
	"testing"

	"github.com/rophy/tostada/e2e/helpers"
)

func TestLogin(t *testing.T) {
	c := helpers.NewClient(t)
	c.Login("alice")

	// Verify authenticated access works
	resp := c.Get("/api/auth/me")
	defer resp.Body.Close()
	c.ExpectStatus(resp, 200)
}

func TestUnauthenticatedAccess(t *testing.T) {
	c := helpers.NewClient(t)

	// Without login, API should return 401
	resp := c.Get("/api/workspaces")
	defer resp.Body.Close()
	c.ExpectStatus(resp, 401)
}
