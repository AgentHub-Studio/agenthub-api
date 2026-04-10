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

// minioAPI abstracts the minio.Client methods used by minioStorageClient (for testing).
type minioAPI interface {
	BucketExists(ctx context.Context, bucketName string) (bool, error)
	MakeBucket(ctx context.Context, bucketName string, opts minio.MakeBucketOptions) error
	PutObject(ctx context.Context, bucketName, objectName string, reader io.Reader, objectSize int64, opts minio.PutObjectOptions) (minio.UploadInfo, error)
}

type minioStorageClient struct {
	mc     minioAPI
	bucket string
}

// NewMinIOStorageClient creates a StorageClient backed by MinIO.
// P-C179-2: ensures the bucket exists at startup (idempotent).
func NewMinIOStorageClient(endpoint, accessKey, secretKey string, ssl bool, region, bucket string) (StorageClient, error) {
	mc, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: ssl,
		Region: region,
	})
	if err != nil {
		return nil, fmt.Errorf("document: minio client: %w", err)
	}
	c := &minioStorageClient{mc: mc, bucket: bucket}
	if err := c.ensureBucket(context.Background(), bucket, region); err != nil {
		return nil, fmt.Errorf("document: ensure bucket %q: %w", bucket, err)
	}
	return c, nil
}

// NewMinIOStorageClientWithAPI creates a StorageClient using the provided minioAPI
// implementation. Intended for unit tests that supply a mock.
func NewMinIOStorageClientWithAPI(api minioAPI, bucket, region string) (StorageClient, error) {
	c := &minioStorageClient{mc: api, bucket: bucket}
	if err := c.ensureBucket(context.Background(), bucket, region); err != nil {
		return nil, fmt.Errorf("document: ensure bucket %q: %w", bucket, err)
	}
	return c, nil
}

// ensureBucket creates the bucket if it does not already exist.
func (m *minioStorageClient) ensureBucket(ctx context.Context, bucket, region string) error {
	exists, err := m.mc.BucketExists(ctx, bucket)
	if err != nil {
		return fmt.Errorf("check bucket existence: %w", err)
	}
	if exists {
		return nil
	}
	if err := m.mc.MakeBucket(ctx, bucket, minio.MakeBucketOptions{Region: region}); err != nil {
		errResp := minio.ToErrorResponse(err)
		if errResp.Code == "BucketAlreadyOwnedByYou" || errResp.Code == "BucketAlreadyExists" {
			return nil // concurrent creation — bucket exists, no error
		}
		return fmt.Errorf("create bucket: %w", err)
	}
	return nil
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
