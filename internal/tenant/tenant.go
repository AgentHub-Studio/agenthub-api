// Package tenant provides context helpers for multi-tenant request handling.
package tenant

import "context"

type contextKey struct{}

// FromContext returns the tenant ID stored in the context.
// Returns an empty string if not set.
func FromContext(ctx context.Context) string {
	v, _ := ctx.Value(contextKey{}).(string)
	return v
}

// NewContext returns a new context with the given tenant ID.
func NewContext(ctx context.Context, tenantID string) context.Context {
	return context.WithValue(ctx, contextKey{}, tenantID)
}
