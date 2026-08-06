package acl_test

import (
	"context"
	"testing"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/auth/acl"
)

func TestGrantAndCheck(t *testing.T) {
	p := acl.NewMemoryProvider()
	ctx := context.Background()
	_, err := p.AddGrant(ctx, acl.Grant{
		SubjectType:  acl.SubjectUser,
		SubjectID:    "u1",
		ResourceType: "agent",
		ResourceID:   "a1",
		Actions:      []string{"read", "write"},
	})
	if err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		subject, res, id, action string
		want                     bool
	}{
		{"u1", "agent", "a1", "read", true},
		{"u1", "agent", "a1", "write", true},
		{"u1", "agent", "a1", "delete", false},
		{"u1", "agent", "a2", "read", false},
		{"u2", "agent", "a1", "read", false},
	}
	for _, c := range cases {
		got, _ := p.CanAccess(ctx, c.subject, c.res, c.id, c.action)
		if got != c.want {
			t.Errorf("%+v got %v want %v", c, got, c.want)
		}
	}
}

func TestWildcardGrant(t *testing.T) {
	p := acl.NewMemoryProvider()
	ctx := context.Background()
	_, _ = p.AddGrant(ctx, acl.Grant{
		SubjectType: acl.SubjectUser, SubjectID: "admin",
		ResourceType: "agent", ResourceID: "*",
		Actions: []string{"*"},
	})
	ok, _ := p.CanAccess(ctx, "admin", "agent", "any-id", "delete")
	if !ok {
		t.Fatal("wildcard should allow")
	}
}

func TestRemoveGrant(t *testing.T) {
	p := acl.NewMemoryProvider()
	ctx := context.Background()
	g, _ := p.AddGrant(ctx, acl.Grant{SubjectID: "u", ResourceType: "r", ResourceID: "1", Actions: []string{"read"}})
	_ = p.RemoveGrant(ctx, g.ID)
	ok, _ := p.CanAccess(ctx, "u", "r", "1", "read")
	if ok {
		t.Fatal("grant not removed")
	}
}

func TestListGrantsFiltersByResource(t *testing.T) {
	p := acl.NewMemoryProvider()
	ctx := context.Background()
	_, _ = p.AddGrant(ctx, acl.Grant{SubjectID: "u1", ResourceType: "agents", ResourceID: "a1", Actions: []string{"read"}})
	_, _ = p.AddGrant(ctx, acl.Grant{SubjectID: "u2", ResourceType: "agents", ResourceID: "a2", Actions: []string{"read"}})
	_, _ = p.AddGrant(ctx, acl.Grant{SubjectID: "u3", ResourceType: "tools", ResourceID: "a1", Actions: []string{"read"}})

	grants, err := p.ListGrants(ctx, acl.GrantFilter{ResourceType: "agents", ResourceID: "a1"})
	if err != nil {
		t.Fatal(err)
	}
	if len(grants) != 1 {
		t.Fatalf("len(grants) = %d, want 1", len(grants))
	}
	if grants[0].SubjectID != "u1" {
		t.Fatalf("subject = %q, want u1", grants[0].SubjectID)
	}
}
