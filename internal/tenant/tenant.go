package tenant

import (
	"context"
)

type contextKey struct{}

// FromContext returns the tenant ID stored in the context, or empty string if not set.
func FromContext(ctx context.Context) string {
	v, _ := ctx.Value(contextKey{}).(string)
	return v
}

// WithContext returns a new context with the given tenant ID.
func WithContext(ctx context.Context, tenantID string) context.Context {
	return context.WithValue(ctx, contextKey{}, tenantID)
}
