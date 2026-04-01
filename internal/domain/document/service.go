package document

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/AgentHub-Studio/agenthub-api/internal/pagination"
)

// Service provides business logic for Document operations.
type Service struct {
	repo    Repository
	storage StorageClient
}

// NewService creates a new Service backed by the given Repository and StorageClient.
func NewService(repo Repository, storage StorageClient) *Service {
	return &Service{repo: repo, storage: storage}
}

// ListByKnowledgeBase returns a paginated list of documents for a knowledge base.
func (s *Service) ListByKnowledgeBase(ctx context.Context, kbID uuid.UUID, req pagination.PageRequest) (pagination.Page[DocumentResponse], error) {
	items, total, err := s.repo.FindByKnowledgeBase(ctx, kbID, req)
	if err != nil {
		return pagination.Page[DocumentResponse]{}, fmt.Errorf("document service: list: %w", err)
	}

	responses := make([]DocumentResponse, len(items))
	for i, item := range items {
		responses[i] = ResponseFrom(item)
	}

	return pagination.NewPage(responses, total, req), nil
}

// GetByID returns a single document by ID.
func (s *Service) GetByID(ctx context.Context, id uuid.UUID) (DocumentResponse, error) {
	d, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return DocumentResponse{}, err
	}
	return ResponseFrom(d), nil
}

// Upload stores the file in object storage and creates a document record with PENDING status.
func (s *Service) Upload(ctx context.Context, req UploadRequest) (DocumentResponse, error) {
	if req.FileName == "" {
		return DocumentResponse{}, fmt.Errorf("document service: file name is required")
	}

	// Derive a stable storage key before uploading so the DB record and the object share the same path.
	docID := uuid.New()
	storagePath := fmt.Sprintf("documents/%s/%s/%s", req.KnowledgeBaseID, docID, req.FileName)

	if _, err := s.storage.Upload(ctx, storagePath, req.Content, req.FileSize, req.ContentType); err != nil {
		return DocumentResponse{}, fmt.Errorf("document service: upload file: %w", err)
	}

	d := Document{
		ID:              docID,
		KnowledgeBaseID: req.KnowledgeBaseID,
		FileName:        req.FileName,
		ContentType:     req.ContentType,
		FileSize:        req.FileSize,
		StoragePath:     storagePath,
		Status:          StatusPending,
	}

	created, err := s.repo.Create(ctx, d)
	if err != nil {
		return DocumentResponse{}, fmt.Errorf("document service: upload: %w", err)
	}

	return ResponseFrom(created), nil
}

// Delete removes a document by ID.
func (s *Service) Delete(ctx context.Context, id uuid.UUID) error {
	if err := s.repo.Delete(ctx, id); err != nil {
		return fmt.Errorf("document service: delete: %w", err)
	}
	return nil
}

// UpdateStatus updates a document's processing status (internal use).
func (s *Service) UpdateStatus(ctx context.Context, id uuid.UUID, status DocumentStatus) (DocumentResponse, error) {
	d, err := s.repo.UpdateStatus(ctx, id, status)
	if err != nil {
		return DocumentResponse{}, fmt.Errorf("document service: update status: %w", err)
	}
	return ResponseFrom(d), nil
}
