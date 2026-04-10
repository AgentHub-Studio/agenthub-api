package document

import (
	"context"
	"errors"
	"io"
	"testing"

	"github.com/minio/minio-go/v7"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockMinioAPI is a test double for the minio.Client methods used by minioStorageClient.
type mockMinioAPI struct {
	bucketExists     bool
	makeBucketCalled bool
	lastBucketName   string
	makeBucketErr    error
	bucketExistsErr  error
	putObjectErr     error
}

func (m *mockMinioAPI) BucketExists(_ context.Context, name string) (bool, error) {
	if m.bucketExistsErr != nil {
		return false, m.bucketExistsErr
	}
	return m.bucketExists, nil
}

func (m *mockMinioAPI) MakeBucket(_ context.Context, name string, _ minio.MakeBucketOptions) error {
	m.makeBucketCalled = true
	m.lastBucketName = name
	return m.makeBucketErr
}

func (m *mockMinioAPI) PutObject(_ context.Context, _, _ string, _ io.Reader, _ int64, _ minio.PutObjectOptions) (minio.UploadInfo, error) {
	return minio.UploadInfo{}, m.putObjectErr
}

// newClientWithMock bypasses the real MinIO connection for unit tests.
func newClientWithMock(mock *mockMinioAPI, bucket string) (StorageClient, error) {
	return NewMinIOStorageClientWithAPI(mock, bucket, "us-east-1")
}

func TestEnsureBucket_BucketNotExists_CreatesBucket(t *testing.T) {
	mock := &mockMinioAPI{bucketExists: false}
	_, err := newClientWithMock(mock, "agenthub-documents")

	require.NoError(t, err)
	assert.True(t, mock.makeBucketCalled, "MakeBucket must be called when bucket does not exist")
	assert.Equal(t, "agenthub-documents", mock.lastBucketName)
}

func TestEnsureBucket_BucketAlreadyExists_NoBucketCreate(t *testing.T) {
	mock := &mockMinioAPI{bucketExists: true}
	_, err := newClientWithMock(mock, "agenthub-documents")

	require.NoError(t, err)
	assert.False(t, mock.makeBucketCalled, "MakeBucket must NOT be called when bucket already exists")
}

func TestEnsureBucket_ConcurrentCreation_NoError(t *testing.T) {
	// Simulates a race where another instance creates the bucket first.
	minioErr := minio.ErrorResponse{Code: "BucketAlreadyOwnedByYou"}
	mock := &mockMinioAPI{bucketExists: false, makeBucketErr: minioErr}
	_, err := newClientWithMock(mock, "concurrent-bucket")

	assert.NoError(t, err, "BucketAlreadyOwnedByYou must not be treated as an error")
}

func TestEnsureBucket_BucketAlreadyExists_S3Code_NoError(t *testing.T) {
	minioErr := minio.ErrorResponse{Code: "BucketAlreadyExists"}
	mock := &mockMinioAPI{bucketExists: false, makeBucketErr: minioErr}
	_, err := newClientWithMock(mock, "existing-bucket")

	assert.NoError(t, err)
}

func TestEnsureBucket_MakeBucketFails_ReturnsError(t *testing.T) {
	mock := &mockMinioAPI{bucketExists: false, makeBucketErr: errors.New("network error")}
	_, err := newClientWithMock(mock, "agenthub-documents")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "ensure bucket")
}

func TestEnsureBucket_BucketExistsCheckFails_ReturnsError(t *testing.T) {
	mock := &mockMinioAPI{bucketExistsErr: errors.New("connection refused")}
	_, err := newClientWithMock(mock, "agenthub-documents")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "ensure bucket")
}
