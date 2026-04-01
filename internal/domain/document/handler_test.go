package document_test

import (
	"bytes"
	"context"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/document"
	"github.com/AgentHub-Studio/agenthub-api/internal/pagination"
)

// mockDocumentSvc satisfies the private documentService interface in document.Handler.
type mockDocumentSvc struct {
	docs map[uuid.UUID]document.DocumentResponse
}

func newMockDocumentSvc() *mockDocumentSvc {
	return &mockDocumentSvc{docs: make(map[uuid.UUID]document.DocumentResponse)}
}

func (m *mockDocumentSvc) ListByKnowledgeBase(_ context.Context, kbID uuid.UUID, req pagination.PageRequest) (pagination.Page[document.DocumentResponse], error) {
	var items []document.DocumentResponse
	for _, d := range m.docs {
		if d.KnowledgeBaseID == kbID {
			items = append(items, d)
		}
	}
	return pagination.NewPage(items, int64(len(items)), req), nil
}

func (m *mockDocumentSvc) Upload(_ context.Context, req document.UploadRequest) (document.DocumentResponse, error) {
	id := uuid.New()
	resp := document.DocumentResponse{
		ID:              id,
		KnowledgeBaseID: req.KnowledgeBaseID,
		FileName:        req.FileName,
		Status:          document.StatusPending,
	}
	m.docs[id] = resp
	return resp, nil
}

func (m *mockDocumentSvc) GetByID(_ context.Context, id uuid.UUID) (document.DocumentResponse, error) {
	d, ok := m.docs[id]
	if !ok {
		return document.DocumentResponse{}, document.ErrNotFound
	}
	return d, nil
}

func (m *mockDocumentSvc) Delete(_ context.Context, id uuid.UUID) error {
	if _, ok := m.docs[id]; !ok {
		return document.ErrNotFound
	}
	delete(m.docs, id)
	return nil
}

func setupDocument() (*chi.Mux, *mockDocumentSvc) {
	svc := newMockDocumentSvc()
	h := document.NewHandler(svc)
	r := chi.NewRouter()
	h.RegisterRoutes(r)
	return r, svc
}

func TestDocumentHandler_List_Success(t *testing.T) {
	r, svc := setupDocument()
	kbID := uuid.New()
	docID := uuid.New()
	svc.docs[docID] = document.DocumentResponse{ID: docID, KnowledgeBaseID: kbID, FileName: "report.pdf"}

	req := httptest.NewRequest(http.MethodGet, "/api/knowledge-bases/"+kbID.String()+"/documents", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var page pagination.Page[document.DocumentResponse]
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &page))
	assert.Equal(t, int64(1), page.TotalElements)
}

func TestDocumentHandler_Upload_Success(t *testing.T) {
	r, _ := setupDocument()
	kbID := uuid.New()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", "report.pdf")
	require.NoError(t, err)
	_, err = part.Write([]byte("fake pdf content"))
	require.NoError(t, err)
	require.NoError(t, writer.Close())

	req := httptest.NewRequest(http.MethodPost, "/api/knowledge-bases/"+kbID.String()+"/documents", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
	var resp document.DocumentResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "report.pdf", resp.FileName)
}

func TestDocumentHandler_Upload_InvalidBody(t *testing.T) {
	r, _ := setupDocument()
	kbID := uuid.New()
	// Sending JSON instead of multipart/form-data should return 400.
	req := httptest.NewRequest(http.MethodPost, "/api/knowledge-bases/"+kbID.String()+"/documents", bytes.NewReader([]byte("not-multipart")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestDocumentHandler_GetByID_NotFound(t *testing.T) {
	r, _ := setupDocument()
	kbID := uuid.New()
	req := httptest.NewRequest(http.MethodGet, "/api/knowledge-bases/"+kbID.String()+"/documents/"+uuid.New().String(), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestDocumentHandler_Delete_Success(t *testing.T) {
	r, svc := setupDocument()
	kbID := uuid.New()
	docID := uuid.New()
	svc.docs[docID] = document.DocumentResponse{ID: docID, KnowledgeBaseID: kbID, FileName: "to-delete.pdf"}

	req := httptest.NewRequest(http.MethodDelete, "/api/knowledge-bases/"+kbID.String()+"/documents/"+docID.String(), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNoContent, w.Code)
}

func TestDocumentHandler_Delete_NotFound(t *testing.T) {
	r, _ := setupDocument()
	kbID := uuid.New()
	req := httptest.NewRequest(http.MethodDelete, "/api/knowledge-bases/"+kbID.String()+"/documents/"+uuid.New().String(), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}
