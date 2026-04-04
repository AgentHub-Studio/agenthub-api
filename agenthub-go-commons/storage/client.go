package storage

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// Config holds MinIO connection configuration.
type Config struct {
	Endpoint        string `env:"MINIO_ENDPOINT,required"`
	AccessKeyID     string `env:"MINIO_ACCESS_KEY,required"`
	SecretAccessKey string `env:"MINIO_SECRET_KEY,required"`
	UseSSL          bool   `env:"MINIO_USE_SSL" envDefault:"false"`
	Region          string `env:"MINIO_REGION"  envDefault:"us-east-1"`
}

// Client wraps minio.Client with AgentHub-specific helpers.
type Client struct {
	mc     *minio.Client
	region string
}

// NewClient creates a new storage Client.
func NewClient(cfg Config) (*Client, error) {
	mc, err := minio.New(cfg.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKeyID, cfg.SecretAccessKey, ""),
		Secure: cfg.UseSSL,
		Region: cfg.Region,
	})
	if err != nil {
		return nil, fmt.Errorf("storage: create minio client: %w", err)
	}
	return &Client{mc: mc, region: cfg.Region}, nil
}

// EnsureBucket creates the bucket if it does not exist.
func (c *Client) EnsureBucket(ctx context.Context, bucket string) error {
	exists, err := c.mc.BucketExists(ctx, bucket)
	if err != nil {
		return fmt.Errorf("storage: check bucket %q: %w", bucket, err)
	}
	if exists {
		return nil
	}
	if err := c.mc.MakeBucket(ctx, bucket, minio.MakeBucketOptions{Region: c.region}); err != nil {
		return fmt.Errorf("storage: create bucket %q: %w", bucket, err)
	}
	return nil
}

// Upload stores reader as objectKey in bucket with the given contentType.
// Returns the object path.
func (c *Client) Upload(ctx context.Context, bucket, objectKey string, reader io.Reader, size int64, contentType string) (string, error) {
	_, err := c.mc.PutObject(ctx, bucket, objectKey, reader, size, minio.PutObjectOptions{
		ContentType: contentType,
	})
	if err != nil {
		return "", fmt.Errorf("storage: upload %s/%s: %w", bucket, objectKey, err)
	}
	return objectKey, nil
}

// Download retrieves objectKey from bucket. Caller must close the returned ReadCloser.
func (c *Client) Download(ctx context.Context, bucket, objectKey string) (io.ReadCloser, error) {
	obj, err := c.mc.GetObject(ctx, bucket, objectKey, minio.GetObjectOptions{})
	if err != nil {
		return nil, fmt.Errorf("storage: download %s/%s: %w", bucket, objectKey, err)
	}
	return obj, nil
}

// PresignedURL generates a pre-signed GET URL for objectKey valid for expiry.
func (c *Client) PresignedURL(ctx context.Context, bucket, objectKey string, expiry time.Duration) (string, error) {
	u, err := c.mc.PresignedGetObject(ctx, bucket, objectKey, expiry, nil)
	if err != nil {
		return "", fmt.Errorf("storage: presigned url %s/%s: %w", bucket, objectKey, err)
	}
	return u.String(), nil
}

// Delete removes objectKey from bucket.
func (c *Client) Delete(ctx context.Context, bucket, objectKey string) error {
	if err := c.mc.RemoveObject(ctx, bucket, objectKey, minio.RemoveObjectOptions{}); err != nil {
		return fmt.Errorf("storage: delete %s/%s: %w", bucket, objectKey, err)
	}
	return nil
}
