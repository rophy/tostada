//go:build e2e

package api

import (
	"encoding/json"
	"testing"

	"github.com/rophy/tostada/e2e/helpers"
)

func TestListDevices(t *testing.T) {
	c := helpers.NewClient(t)
	c.Login("alice")

	resp := c.Get("/api/devices")
	defer resp.Body.Close()
	c.ExpectStatus(resp, 200)

	var devices []map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&devices); err != nil {
		t.Fatalf("decode: %v", err)
	}

	// Devices list depends on access grants — just verify the response is valid JSON
	if devices == nil {
		t.Fatal("devices is nil, expected array")
	}
}
