package document

import (
	"errors"
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

// UploadRequest is the payload for uploading a document.
type UploadRequest struct {
	KnowledgeBaseID uuid.UUID `json:"knowledgeBaseId"`
	FileName        string    `json:"fileName"`
	ContentType     string    `json:"contentType"`
	FileSize        int64     `json:"fileSize"`
	StoragePath     string    `json:"storagePath"`
}
