package installation

import (
	"context"
	"fmt"
	"io"
	"time"
)

// NoopStorage is a placeholder storage backend used when MinIO is not configured.
// It returns errors for all operations and should only be used in development/testing.
type NoopStorage struct{}

// Upload is a no-op implementation that always returns an error.
func (n *NoopStorage) Upload(_ context.Context, key string, _ io.Reader, _ int64, _ string) (string, error) {
	return "", fmt.Errorf("noop storage: no storage backend configured; key=%s", key)
}

// PresignedURL is a no-op implementation that always returns an error.
func (n *NoopStorage) PresignedURL(_ context.Context, storagePath string, _ time.Duration) (string, error) {
	return "", fmt.Errorf("noop storage: no storage backend configured; path=%s", storagePath)
}
