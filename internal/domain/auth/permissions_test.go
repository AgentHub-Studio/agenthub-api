package auth_test

import (
	"testing"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/auth"
)

func TestMatchesPermission(t *testing.T) {
	cases := []struct {
		granted, required string
		want              bool
	}{
		{"agents:*", "agents:read", true},
		{"agents:*", "agents:write", true},
		{"agents:read", "agents:read", true},
		{"agents:read", "agents:write", false},
		{"*:read", "agents:read", true},
		{"*:read", "agents:write", false},
		{"agents:*:my-a", "agents:read:my-a", true},
		{"agents:*:my-a", "agents:read:other", false},
		{"agents:*", "agents:read:my-a", true},
		{"agents", "agents:read", true}, // shorter granted implicitly covers
	}
	for _, tc := range cases {
		if got := auth.MatchesPermission(tc.granted, tc.required); got != tc.want {
			t.Errorf("Matches(%q,%q) = %v, want %v", tc.granted, tc.required, got, tc.want)
		}
	}
}

func TestDefaultPermissionsForRoles(t *testing.T) {
	admin := auth.DefaultPermissionsForRoles([]string{"admin"})
	if !auth.HasAny(admin, "agents:write") {
		t.Fatalf("admin should be able to write agents: %v", admin)
	}
	user := auth.DefaultPermissionsForRoles([]string{"user"})
	if !auth.HasAny(user, "agents:read") {
		t.Fatalf("user should read agents: %v", user)
	}
	if auth.HasAny(user, "audit:read") {
		t.Fatalf("user must not have audit:read")
	}
	none := auth.DefaultPermissionsForRoles(nil)
	if len(none) != 0 {
		t.Fatalf("empty roles → no permissions, got %v", none)
	}
}
