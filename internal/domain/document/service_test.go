package document_test

import (
	"context"
	"io"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/document"
	"github.com/AgentHub-Studio/agenthub-api/internal/pagination"
)

// mockDocRepo is an in-memory Repository for unit tests.
type mockDocRepo struct {
	data map[uuid.UUID]document.Document
}

func newMockRepo() *mockDocRepo {
	return &mockDocRepo{data: make(map[uuid.UUID]document.Document)}
}

func (m *mockDocRepo) FindByKnowledgeBase(_ context.Context, kbID uuid.UUID, _ pagination.PageRequest) ([]document.Document, int64, error) {
	var out []document.Document
	for _, d := range m.data {
		if d.KnowledgeBaseID == kbID {
			out = append(out, d)
		}
	}
	return out, int64(len(out)), nil
}

func (m *mockDocRepo) FindByID(_ context.Context, id uuid.UUID) (document.Document, error) {
	d, ok := m.data[id]
	if !ok {
		return document.Document{}, document.ErrNotFound
	}
	return d, nil
}

func (m *mockDocRepo) Create(_ context.Context, d document.Document) (document.Document, error) {
	if d.ID == uuid.Nil {
		d.ID = uuid.New()
	}
	d.Status = document.StatusPending
	m.data[d.ID] = d
	return d, nil
}

func (m *mockDocRepo) UpdateStatus(_ context.Context, id uuid.UUID, status document.DocumentStatus) (document.Document, error) {
	d, ok := m.data[id]
	if !ok {
		return document.Document{}, document.ErrNotFound
	}
	d.Status = status
	m.data[id] = d
	return d, nil
}

func (m *mockDocRepo) Delete(_ context.Context, id uuid.UUID) error {
	if _, ok := m.data[id]; !ok {
		return document.ErrNotFound
	}
	delete(m.data, id)
	return nil
}

// mockStorage is a StorageClient that records uploads for assertions.
type mockStorage struct {
	uploaded []string
}

func (m *mockStorage) Upload(_ context.Context, key string, _ io.Reader, _ int64, _ string) (string, error) {
	m.uploaded = append(m.uploaded, key)
	return key, nil
}

// mockPublisher is an EventPublisher that records published events for assertions.
type mockPublisher struct {
	events []document.DocumentUploadedEvent
}

func (m *mockPublisher) PublishUploaded(_ context.Context, e document.DocumentUploadedEvent) error {
	m.events = append(m.events, e)
	return nil
}

func newSvc() (*document.Service, *mockStorage, *mockPublisher) {
	storage := &mockStorage{}
	publisher := &mockPublisher{}
	svc := document.NewService(newMockRepo(), storage, publisher)
	return svc, storage, publisher
}

func TestDocumentService_Upload_Success(t *testing.T) {
	svc, storage, publisher := newSvc()
	kbID := uuid.New()
	d, err := svc.Upload(context.Background(), document.UploadRequest{
		KnowledgeBaseID: kbID,
		FileName:        "report.pdf",
		ContentType:     "application/pdf",
		FileSize:        204800,
		Content:         strings.NewReader("fake pdf content"),
	})
	require.NoError(t, err)
	assert.Equal(t, "report.pdf", d.FileName)
	assert.NotEqual(t, uuid.Nil, d.ID)
	assert.Equal(t, document.StatusPending, d.Status)
	assert.NotEmpty(t, d.StoragePath)
	// Storage path should contain the kbID and document ID.
	assert.Contains(t, d.StoragePath, kbID.String())
	assert.Contains(t, d.StoragePath, d.ID.String())
	require.Len(t, storage.uploaded, 1)
	assert.Equal(t, d.StoragePath, storage.uploaded[0])
	// Publisher should have been called exactly once with correct fields.
	require.Len(t, publisher.events, 1, "upload event must be published")
	ev := publisher.events[0]
	assert.Equal(t, d.ID, ev.DocumentID)
	assert.Equal(t, kbID, ev.KnowledgeBaseID)
	assert.Equal(t, d.StoragePath, ev.StoragePath)
	assert.Equal(t, "application/pdf", ev.ContentType)
	assert.Equal(t, "report.pdf", ev.FileName)
}

func TestDocumentService_Upload_MissingFileName(t *testing.T) {
	svc, _, _ := newSvc()
	_, err := svc.Upload(context.Background(), document.UploadRequest{
		KnowledgeBaseID: uuid.New(),
		Content:         strings.NewReader("data"),
	})
	require.Error(t, err)
}

func TestDocumentService_GetByID_NotFound(t *testing.T) {
	svc, _, _ := newSvc()
	_, err := svc.GetByID(context.Background(), uuid.New())
	require.ErrorIs(t, err, document.ErrNotFound)
}

func TestDocumentService_UpdateStatus(t *testing.T) {
	svc, _, _ := newSvc()
	kbID := uuid.New()
	created, err := svc.Upload(context.Background(), document.UploadRequest{
		KnowledgeBaseID: kbID,
		FileName:        "doc.txt",
		ContentType:     "text/plain",
		Content:         strings.NewReader("content"),
	})
	require.NoError(t, err)
	updated, err := svc.UpdateStatus(context.Background(), created.ID, document.StatusIndexed)
	require.NoError(t, err)
	assert.Equal(t, document.StatusIndexed, updated.Status)
}

func TestDocumentService_ListByKnowledgeBase(t *testing.T) {
	svc, _, _ := newSvc()
	kbID := uuid.New()
	for i := 0; i < 3; i++ {
		_, err := svc.Upload(context.Background(), document.UploadRequest{
			KnowledgeBaseID: kbID,
			FileName:        "file.pdf",
			ContentType:     "application/pdf",
			Content:         strings.NewReader("data"),
		})
		require.NoError(t, err)
	}
	// Document from a different KB — should not appear in results.
	_, err := svc.Upload(context.Background(), document.UploadRequest{
		KnowledgeBaseID: uuid.New(),
		FileName:        "other.pdf",
		ContentType:     "application/pdf",
		Content:         strings.NewReader("data"),
	})
	require.NoError(t, err)

	page, err := svc.ListByKnowledgeBase(context.Background(), kbID, pagination.PageRequest{Page: 0, Size: 20})
	require.NoError(t, err)
	assert.Equal(t, int64(3), page.TotalElements)
	assert.Len(t, page.Content, 3)
}

func TestDocumentService_Delete_NotFound(t *testing.T) {
	svc, _, _ := newSvc()
	err := svc.Delete(context.Background(), uuid.New())
	require.ErrorIs(t, err, document.ErrNotFound)
}
