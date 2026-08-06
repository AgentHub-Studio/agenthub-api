//go:build e2e

package e2e

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-e2e/testutil"
)

// TestE2E_CoreAdminRouteRejectsUserWithoutAdminRole verifies that a valid core
// JWT without the admin realm role cannot access platform administration.
func TestE2E_CoreAdminRouteRejectsUserWithoutAdminRole(t *testing.T) {
	cfg := e2eConfig()
	kc := testutil.NewKeycloakClient(cfg.keycloakURL, cfg.keycloakAdmin, cfg.keycloakAdminPass)
	username := fmt.Sprintf("e2e-no-admin-%d", time.Now().UnixNano())

	userID, err := kc.CreateUser("core", username, cfg.e2eUserPassword)
	require.NoError(t, err)
	t.Cleanup(func() {
		if err := kc.DeleteUser("core", userID); err != nil {
			t.Errorf("cleanup non-admin core user: %v", err)
		}
	})

	token, err := kc.UserToken("core", username, cfg.e2eUserPassword)
	require.NoError(t, err)

	client := testutil.NewAPIClient(t, cfg.backendURL, token)
	status := client.Get("/api/admin/tenants", nil)
	require.Equal(t, http.StatusForbidden, status)
}
