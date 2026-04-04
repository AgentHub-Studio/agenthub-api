// Package storage provides a MinIO wrapper for AgentHub Go services,
// implementing object upload, download, presigned URL generation, and deletion.
//
// Usage:
//
//	client, err := storage.NewClient(storage.Config{
//	    Endpoint:  cfg.MinIOEndpoint,
//	    AccessKey: cfg.MinIOAccessKey,
//	    SecretKey: cfg.MinIOSecretKey,
//	})
//	url, err := client.UploadFile(ctx, bucket, key, reader, size, contentType)
package storage
