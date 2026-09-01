//go:build e2e

package api

import (
	"os"
	"testing"

	"github.com/rophy/tostada/e2e/helpers"
)

func TestMain(m *testing.M) {
	// Quick connectivity check — skip all tests if the cluster isn't up.
	resp, err := helpers.NewUnauthenticatedClient().Get(helpers.BaseURL() + "/api/auth/login")
	if err != nil {
		os.Stderr.WriteString("e2e: cluster not available, skipping\n")
		os.Exit(0)
	}
	resp.Body.Close()

	os.Exit(m.Run())
}
