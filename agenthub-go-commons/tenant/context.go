package tenant

import "context"

type contextKey string

const tenantKey contextKey = "tenantID"

// FromContext returns the tenantID stored in ctx. Returns "" if not set.
func FromContext(ctx context.Context) string {
	v, _ := ctx.Value(tenantKey).(string)
	return v
}

// NewContext returns a copy of ctx with the tenantID stored.
func NewContext(ctx context.Context, tenantID string) context.Context {
	return context.WithValue(ctx, tenantKey, tenantID)
}
