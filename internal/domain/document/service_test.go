package document_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/document"
	"github.com/AgentHub-Studio/agenthub-api/internal/pagination"
)

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
	d.ID = uuid.New()
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

func TestDocumentService_Upload_Success(t *testing.T) {
	svc := document.NewService(newMockRepo())
	kbID := uuid.New()
	d, err := svc.Upload(context.Background(), document.UploadRequest{
		KnowledgeBaseID: kbID,
		FileName:        "report.pdf",
		ContentType:     "application/pdf",
		FileSize:        204800,
		StoragePath:     "kb/" + kbID.String() + "/report.pdf",
	})
	require.NoError(t, err)
	assert.Equal(t, "report.pdf", d.FileName)
	assert.NotEqual(t, uuid.Nil, d.ID)
	assert.Equal(t, document.StatusPending, d.Status)
}

func TestDocumentService_GetByID_NotFound(t *testing.T) {
	svc := document.NewService(newMockRepo())
	_, err := svc.GetByID(context.Background(), uuid.New())
	require.ErrorIs(t, err, document.ErrNotFound)
}

func TestDocumentService_UpdateStatus(t *testing.T) {
	svc := document.NewService(newMockRepo())
	kbID := uuid.New()
	created, err := svc.Upload(context.Background(), document.UploadRequest{
		KnowledgeBaseID: kbID, FileName: "doc.txt", ContentType: "text/plain", StoragePath: "x",
	})
	require.NoError(t, err)
	updated, err := svc.UpdateStatus(context.Background(), created.ID, document.StatusIndexed)
	require.NoError(t, err)
	assert.Equal(t, document.StatusIndexed, updated.Status)
}

func TestDocumentService_ListByKnowledgeBase(t *testing.T) {
	svc := document.NewService(newMockRepo())
	kbID := uuid.New()
	for i := 0; i < 3; i++ {
		_, err := svc.Upload(context.Background(), document.UploadRequest{
			KnowledgeBaseID: kbID, FileName: "file.pdf", ContentType: "application/pdf", StoragePath: "x",
		})
		require.NoError(t, err)
	}
	// document from a different KB — should not appear
	_, err := svc.Upload(context.Background(), document.UploadRequest{
		KnowledgeBaseID: uuid.New(), FileName: "other.pdf", ContentType: "application/pdf", StoragePath: "y",
	})
	require.NoError(t, err)

	page, err := svc.ListByKnowledgeBase(context.Background(), kbID, pagination.PageRequest{Page: 0, Size: 20})
	require.NoError(t, err)
	assert.Equal(t, int64(3), page.TotalElements)
	assert.Len(t, page.Content, 3)
}

func TestDocumentService_Delete_NotFound(t *testing.T) {
	svc := document.NewService(newMockRepo())
	err := svc.Delete(context.Background(), uuid.New())
	require.ErrorIs(t, err, document.ErrNotFound)
}
