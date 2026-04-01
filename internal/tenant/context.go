package tenant

import "context"

type contextKey string

const tenantKey contextKey = "tenantID"

// FromContext retrieves the tenant ID from the context.
func FromContext(ctx context.Context) string {
	id, _ := ctx.Value(tenantKey).(string)
	return id
}

// NewContext returns a new context with the given tenant ID.
func NewContext(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, tenantKey, id)
}
