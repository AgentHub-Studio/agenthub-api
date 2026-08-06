// Package acl implements resource-level access control on top of the wildcard
// permission matcher in the auth package. A grant binds (subject, resource,
// action) — typically "user X can edit agent Y" — and is checked by a
// middleware before protected endpoints run.
//
// Inspired by Mastra EE's IACLProvider. Tests can use MemoryProvider, while
// production wires a pgx-backed provider over the tenant resource_grant table.
package acl

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"
)

// SubjectType is typically "user" or "group".
type SubjectType string

const (
	SubjectUser  SubjectType = "user"
	SubjectGroup SubjectType = "group"
)

// Grant authorizes Actions on ResourceType/ResourceID for SubjectID.
type Grant struct {
	ID           uuid.UUID   `json:"id"`
	SubjectType  SubjectType `json:"subjectType"`
	SubjectID    string      `json:"subjectId"`
	ResourceType string      `json:"resourceType"`
	ResourceID   string      `json:"resourceId"`
	Actions      []string    `json:"actions"`
	CreatedAt    time.Time   `json:"createdAt"`
	UpdatedAt    time.Time   `json:"updatedAt"`
}

// GrantFilter restricts list operations by subject and/or resource.
type GrantFilter struct {
	SubjectID    string
	ResourceType string
	ResourceID   string
}

// Provider is the surface middleware uses. In-memory store is the default.
type Provider interface {
	CanAccess(ctx context.Context, subjectID, resourceType, resourceID, action string) (bool, error)
	AddGrant(ctx context.Context, g Grant) (Grant, error)
	RemoveGrant(ctx context.Context, grantID uuid.UUID) error
	ListGrants(ctx context.Context, filter GrantFilter) ([]Grant, error)
}

type MemoryProvider struct {
	mu     sync.RWMutex
	grants map[uuid.UUID]Grant
}

func NewMemoryProvider() *MemoryProvider {
	return &MemoryProvider{grants: make(map[uuid.UUID]Grant)}
}

func (p *MemoryProvider) CanAccess(_ context.Context, subjectID, resourceType, resourceID, action string) (bool, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	for _, g := range p.grants {
		if g.SubjectID != subjectID {
			continue
		}
		if g.ResourceType != resourceType {
			continue
		}
		if g.ResourceID != resourceID && g.ResourceID != "*" {
			continue
		}
		for _, a := range g.Actions {
			if a == action || a == "*" {
				return true, nil
			}
		}
	}
	return false, nil
}

func (p *MemoryProvider) AddGrant(_ context.Context, g Grant) (Grant, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if g.ID == uuid.Nil {
		g.ID = uuid.New()
	}
	now := time.Now().UTC()
	if g.CreatedAt.IsZero() {
		g.CreatedAt = now
	}
	g.UpdatedAt = now
	p.grants[g.ID] = g
	return g, nil
}

func (p *MemoryProvider) RemoveGrant(_ context.Context, id uuid.UUID) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	delete(p.grants, id)
	return nil
}

func (p *MemoryProvider) ListGrants(_ context.Context, filter GrantFilter) ([]Grant, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	out := make([]Grant, 0)
	for _, g := range p.grants {
		if filter.SubjectID != "" && g.SubjectID != filter.SubjectID {
			continue
		}
		if filter.ResourceType != "" && g.ResourceType != filter.ResourceType {
			continue
		}
		if filter.ResourceID != "" && g.ResourceID != filter.ResourceID {
			continue
		}
		out = append(out, g)
	}
	return out, nil
}
