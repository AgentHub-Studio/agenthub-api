package installation

import (
	"context"
	"io"
	"time"

	"github.com/AgentHub-Studio/agenthub-go-commons/storage"
)

// minioStorage adapts storage.Client to the StorageBackend interface.
// It fixes the bucket at construction time so callers do not need to specify it.
type minioStorage struct {
	client *storage.Client
	bucket string
}

// NewMinIOStorage creates a StorageBackend backed by MinIO.
func NewMinIOStorage(client *storage.Client, bucket string) StorageBackend {
	return &minioStorage{client: client, bucket: bucket}
}

// Upload stores reader at key within the configured bucket and returns the storage path.
func (m *minioStorage) Upload(ctx context.Context, key string, r io.Reader, size int64, contentType string) (string, error) {
	return m.client.Upload(ctx, m.bucket, key, r, size, contentType)
}

// PresignedURL returns a time-limited download URL for the given storage path.
func (m *minioStorage) PresignedURL(ctx context.Context, storagePath string, expires time.Duration) (string, error) {
	return m.client.PresignedURL(ctx, m.bucket, storagePath, expires)
}
