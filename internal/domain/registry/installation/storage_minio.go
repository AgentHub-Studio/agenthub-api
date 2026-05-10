package installation

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"strings"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// MinIOConfig holds the configuration needed to create a MinIO-backed StorageBackend.
type MinIOConfig struct {
	Endpoint        string
	AccessKeyID     string
	SecretAccessKey string
	UseSSL          bool
	Region          string
	Bucket          string
}

// minioStorage implements StorageBackend using MinIO (minio-go/v7).
// The bucket is fixed at construction time so callers do not need to specify it.
type minioStorage struct {
	mc     *minio.Client
	bucket string
}

// NewMinIOStorageFromConfig creates a StorageBackend backed by MinIO using the provided config.
func NewMinIOStorageFromConfig(cfg MinIOConfig) (StorageBackend, error) {
	mc, err := minio.New(cfg.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKeyID, cfg.SecretAccessKey, ""),
		Secure: cfg.UseSSL,
		Region: cfg.Region,
	})
	if err != nil {
		return nil, fmt.Errorf("minio: create client: %w", err)
	}
	return &minioStorage{mc: mc, bucket: cfg.Bucket}, nil
}

// Upload stores reader at key within the configured bucket and returns the storage path.
func (m *minioStorage) Upload(ctx context.Context, key string, r io.Reader, size int64, contentType string) (string, error) {
	_, err := m.mc.PutObject(ctx, m.bucket, key, r, size, minio.PutObjectOptions{
		ContentType: contentType,
	})
	if err != nil {
		return "", fmt.Errorf("minio: upload %s/%s: %w", m.bucket, key, err)
	}
	return key, nil
}

// PresignedURL returns a time-limited download URL for the given storage path.
// Bug 275: força Content-Disposition: attachment + Content-Type: application/octet-stream
// no presigned response para impedir o browser de renderizar inline assets que sejam
// HTML/SVG/JS/etc. (XSS via package asset upload). filename é sanitizado pelo caller
// (asset.Filename já passou por validação no upload).
func (m *minioStorage) PresignedURL(ctx context.Context, storagePath string, filename string, expires time.Duration) (string, error) {
	reqParams := make(url.Values)
	reqParams.Set("response-content-type", "application/octet-stream")
	reqParams.Set("response-content-disposition", buildContentDisposition(filename))
	u, err := m.mc.PresignedGetObject(ctx, m.bucket, storagePath, expires, reqParams)
	if err != nil {
		return "", fmt.Errorf("minio: presigned url %s/%s: %w", m.bucket, storagePath, err)
	}
	return u.String(), nil
}

// buildContentDisposition gera "attachment; filename=\"<safe>\"" com o filename quoted.
// Aspas duplas e backslash são escapados; characters não-ASCII são truncados para
// evitar header injection. Se filename vazio, usa "attachment" puro.
func buildContentDisposition(filename string) string {
	if filename == "" {
		return "attachment"
	}
	var b strings.Builder
	for _, r := range filename {
		if r < 0x20 || r > 0x7e || r == '"' || r == '\\' {
			continue
		}
		b.WriteRune(r)
	}
	safe := b.String()
	if safe == "" {
		return "attachment"
	}
	return fmt.Sprintf(`attachment; filename="%s"`, safe)
}
