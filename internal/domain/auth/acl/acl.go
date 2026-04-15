// Package acl implements resource-level access control on top of the wildcard
// permission matcher in the auth package. A grant binds (subject, resource,
// action) — typically "user X can edit agent Y" — and is checked by a
// middleware before mutating endpoints run.
//
// Inspired by Mastra EE's IACLProvider. Storage stays in-memory for the
// initial scaffold; a pgx-backed repository ships in a follow-up.
package acl

import (
	"context"
	"sync"

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
	ID           uuid.UUID
	SubjectType  SubjectType
	SubjectID    string
	ResourceType string
	ResourceID   string
	Actions      []string
}

// Provider is the surface middleware uses. In-memory store is the default.
type Provider interface {
	CanAccess(ctx context.Context, subjectID, resourceType, resourceID, action string) (bool, error)
	AddGrant(ctx context.Context, g Grant) (Grant, error)
	RemoveGrant(ctx context.Context, grantID uuid.UUID) error
	ListGrants(ctx context.Context, subjectID string) ([]Grant, error)
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
	p.grants[g.ID] = g
	return g, nil
}

func (p *MemoryProvider) RemoveGrant(_ context.Context, id uuid.UUID) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	delete(p.grants, id)
	return nil
}

func (p *MemoryProvider) ListGrants(_ context.Context, subjectID string) ([]Grant, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	out := make([]Grant, 0)
	for _, g := range p.grants {
		if g.SubjectID == subjectID {
			out = append(out, g)
		}
	}
	return out, nil
}
