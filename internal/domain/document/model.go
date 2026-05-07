package document

import (
	"context"
	"errors"
	"io"
	"time"

	"github.com/google/uuid"
)

// ErrNotFound is returned when a document cannot be found.
var ErrNotFound = errors.New("document: not found")

// DocumentStatus represents the processing lifecycle of a Document.
type DocumentStatus string

const (
	StatusPending    DocumentStatus = "PENDING"
	StatusExtracting DocumentStatus = "EXTRACTING"
	StatusChunking   DocumentStatus = "CHUNKING"
	StatusEmbedding  DocumentStatus = "EMBEDDING"
	StatusIndexed    DocumentStatus = "INDEXED"
	StatusFailed     DocumentStatus = "FAILED"
)

// Document is the domain entity.
type Document struct {
	ID              uuid.UUID      `db:"id"`
	KnowledgeBaseID uuid.UUID      `db:"knowledge_base_id"`
	FileName        string         `db:"file_name"`
	ContentType     string         `db:"content_type"`
	Status          DocumentStatus `db:"status"`
	StoragePath     string         `db:"storage_path"`
	FileSize        int64          `db:"file_size"`
	CreatedAt       time.Time      `db:"created_at"`
	UpdatedAt       time.Time      `db:"updated_at"`
}

// DocumentResponse is the DTO returned by the API.
type DocumentResponse struct {
	ID              uuid.UUID      `json:"id"`
	KnowledgeBaseID uuid.UUID      `json:"knowledgeBaseId"`
	FileName        string         `json:"fileName"`
	ContentType     string         `json:"contentType"`
	Status          DocumentStatus `json:"status"`
	StoragePath     string         `json:"storagePath"`
	FileSize        int64          `json:"fileSize"`
	CreatedAt       time.Time      `json:"createdAt"`
	UpdatedAt       time.Time      `json:"updatedAt"`
}

// ResponseFrom maps a Document entity to a DocumentResponse DTO.
func ResponseFrom(d Document) DocumentResponse { return DocumentResponse(d) }

// UploadRequest carries the metadata and file content for a document upload.
// Content and FileSize are populated by the handler from the multipart form;
// StoragePath is derived by the service after uploading to object storage.
type UploadRequest struct {
	KnowledgeBaseID uuid.UUID
	FileName        string
	ContentType     string
	FileSize        int64
	Content         io.Reader // file data from multipart form
}

// DocumentUploadedEvent is published to RabbitMQ after a successful upload so the
// extractor service can start the chunking pipeline for the new document.
//
// `bucket` and `filePath` ("{bucket}/{storagePath}") are emitted alongside the
// typed StoragePath because agenthub-extractor (Python pika consumer) parses
// bucket/key from a single path field — without them it dies with
// "invalid bucket name".
type DocumentUploadedEvent struct {
	DocumentID      uuid.UUID `json:"documentId"`
	KnowledgeBaseID uuid.UUID `json:"knowledgeBaseId"`
	StoragePath     string    `json:"storagePath"`
	Bucket          string    `json:"bucket"`
	FilePath        string    `json:"filePath"`
	ContentType     string    `json:"contentType"`
	FileName        string    `json:"fileName"`
	TenantID        string    `json:"tenantId"`
}

// EventPublisher publishes domain events for document lifecycle changes.
type EventPublisher interface {
	// PublishUploaded enqueues a DocumentUploadedEvent for the extractor pipeline.
	PublishUploaded(ctx context.Context, event DocumentUploadedEvent) error
}

// NoopEventPublisher silently discards events — used when RabbitMQ is not configured.
type NoopEventPublisher struct{}

// PublishUploaded is a no-op implementation.
func (n *NoopEventPublisher) PublishUploaded(_ context.Context, _ DocumentUploadedEvent) error {
	return nil
}
