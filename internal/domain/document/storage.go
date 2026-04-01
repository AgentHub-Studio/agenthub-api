package document

import (
	"context"
	"fmt"
	"io"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// StorageClient abstracts object-storage operations for document files.
type StorageClient interface {
	Upload(ctx context.Context, key string, r io.Reader, size int64, contentType string) (string, error)
}

type minioStorageClient struct {
	mc     *minio.Client
	bucket string
}

// NewMinIOStorageClient creates a StorageClient backed by MinIO.
func NewMinIOStorageClient(endpoint, accessKey, secretKey string, ssl bool, region, bucket string) (StorageClient, error) {
	mc, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: ssl,
		Region: region,
	})
	if err != nil {
		return nil, fmt.Errorf("document: minio client: %w", err)
	}
	return &minioStorageClient{mc: mc, bucket: bucket}, nil
}

func (m *minioStorageClient) Upload(ctx context.Context, key string, r io.Reader, size int64, contentType string) (string, error) {
	_, err := m.mc.PutObject(ctx, m.bucket, key, r, size, minio.PutObjectOptions{
		ContentType: contentType,
	})
	if err != nil {
		return "", fmt.Errorf("document: minio upload %s/%s: %w", m.bucket, key, err)
	}
	return key, nil
}

// NoopStorageClient silently accepts uploads — use when MinIO is not configured.
type NoopStorageClient struct{}

func (n *NoopStorageClient) Upload(_ context.Context, key string, _ io.Reader, _ int64, _ string) (string, error) {
	return key, nil
}
