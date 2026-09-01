//go:build e2e

package api

import (
	"encoding/json"
	"testing"

	"github.com/rophy/tostada/e2e/helpers"
)

func TestListWorkspaces(t *testing.T) {
	c := helpers.NewClient(t)
	c.Login("alice")

	resp := c.Get("/api/workspaces")
	defer resp.Body.Close()
	c.ExpectStatus(resp, 200)

	var workspaces []map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&workspaces); err != nil {
		t.Fatalf("decode: %v", err)
	}

	// Workspaces should be a non-nil array (may be empty depending on config)
	if workspaces == nil {
		t.Fatal("workspaces is nil, expected array")
	}
}
