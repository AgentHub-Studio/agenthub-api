package storage_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-go-commons/storage"
)

func TestNewClient_InvalidEndpoint(t *testing.T) {
	// minio.New accepts any endpoint string and only validates on actual requests.
	// We just verify NewClient does not return an error for a syntactically valid config.
	cfg := storage.Config{
		Endpoint:        "localhost:9000",
		AccessKeyID:     "test",
		SecretAccessKey: "test1234",
		UseSSL:          false,
		Region:          "us-east-1",
	}
	client, err := storage.NewClient(cfg)
	require.NoError(t, err)
	assert.NotNil(t, client)
}

func TestNewClient_EmptyEndpoint(t *testing.T) {
	// minio returns an error when endpoint is empty.
	cfg := storage.Config{
		Endpoint:        "",
		AccessKeyID:     "test",
		SecretAccessKey: "secret",
	}
	_, err := storage.NewClient(cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "storage:")
}
