package tenant

import "context"

type contextKey string

const (
	tenantKey   contextKey = "tenantID"
	rawTokenKey contextKey = "rawToken"
)

// FromContext retrieves the tenant ID from the context.
func FromContext(ctx context.Context) string {
	id, _ := ctx.Value(tenantKey).(string)
	return id
}

// NewContext returns a new context with the given tenant ID.
func NewContext(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, tenantKey, id)
}

// TokenFromContext retrieves the raw Bearer JWT from the context.
func TokenFromContext(ctx context.Context) string {
	t, _ := ctx.Value(rawTokenKey).(string)
	return t
}

// NewContextWithToken returns a new context with the raw JWT stored alongside the tenant ID.
func NewContextWithToken(ctx context.Context, id, token string) context.Context {
	ctx = context.WithValue(ctx, tenantKey, id)
	ctx = context.WithValue(ctx, rawTokenKey, token)
	return ctx
}
