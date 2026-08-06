package document_test

import (
	"bytes"
	"context"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/document"
	"github.com/AgentHub-Studio/agenthub-api/internal/middleware"
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
		Metadata:        req.Metadata,
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

func (m *mockDocumentSvc) Reprocess(_ context.Context, id uuid.UUID) (document.DocumentResponse, error) {
	d, ok := m.docs[id]
	if !ok {
		return document.DocumentResponse{}, document.ErrNotFound
	}
	d.Status = document.StatusPending
	m.docs[id] = d
	return d, nil
}

func setupDocument() (*chi.Mux, *mockDocumentSvc) {
	return setupDocumentWithRoles("admin")
}

func setupDocumentWithRoles(roles ...string) (*chi.Mux, *mockDocumentSvc) {
	svc := newMockDocumentSvc()
	h := document.NewHandler(svc)
	r := chi.NewRouter()
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := middleware.ContextWithRoles(r.Context(), roles...)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	})
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

func TestDocumentHandler_AdministrativeRoutesRequireAdminRole(t *testing.T) {
	r, _ := setupDocumentWithRoles("user")
	kbID := uuid.NewString()
	documentID := uuid.NewString()
	cases := []struct {
		name   string
		method string
		path   string
	}{
		{name: "list", method: http.MethodGet, path: "/api/knowledge-bases/" + kbID + "/documents"},
		{name: "upload", method: http.MethodPost, path: "/api/knowledge-bases/" + kbID + "/documents"},
		{name: "get", method: http.MethodGet, path: "/api/knowledge-bases/" + kbID + "/documents/" + documentID},
		{name: "delete", method: http.MethodDelete, path: "/api/knowledge-bases/" + kbID + "/documents/" + documentID},
		{name: "reprocess", method: http.MethodPost, path: "/api/knowledge-bases/" + kbID + "/documents/" + documentID + "/reprocess"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, nil)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			assert.Equal(t, http.StatusForbidden, w.Code)
			assert.Contains(t, w.Body.String(), "missing required role")
		})
	}
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

func TestDocumentHandler_Upload_AcceptsDocumentMetadata(t *testing.T) {
	r, _ := setupDocument()
	kbID := uuid.New()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	require.NoError(t, writer.WriteField("metadata", `{"source":"manual","tags":["release","api"],"year":2026}`))
	part, err := writer.CreateFormFile("file", "report.txt")
	require.NoError(t, err)
	_, err = part.Write([]byte("metadata contract"))
	require.NoError(t, err)
	require.NoError(t, writer.Close())

	req := httptest.NewRequest(http.MethodPost, "/api/knowledge-bases/"+kbID.String()+"/documents", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	var resp document.DocumentResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.JSONEq(t, `{"source":"manual","tags":["release","api"],"year":2026}`, string(resp.Metadata))
}

func TestDocumentHandler_Upload_AcceptsSecondLevelDocumentMetadata(t *testing.T) {
	r, _ := setupDocument()
	kbID := uuid.New()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	require.NoError(t, writer.WriteField("metadata", `{"customer":{"region":"br","tier":2,"labels":["priority"]}}`))
	part, err := writer.CreateFormFile("file", "report.txt")
	require.NoError(t, err)
	_, err = part.Write([]byte("metadata contract"))
	require.NoError(t, err)
	require.NoError(t, writer.Close())

	req := httptest.NewRequest(http.MethodPost, "/api/knowledge-bases/"+kbID.String()+"/documents", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	var resp document.DocumentResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.JSONEq(t, `{"customer":{"region":"br","tier":2,"labels":["priority"]}}`, string(resp.Metadata))
}

func TestDocumentHandler_Upload_RejectsThirdLevelDocumentMetadata(t *testing.T) {
	r, svc := setupDocument()
	kbID := uuid.New()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	require.NoError(t, writer.WriteField("metadata", `{"source":{"name":{"value":"manual"}}}`))
	part, err := writer.CreateFormFile("file", "report.txt")
	require.NoError(t, err)
	_, err = part.Write([]byte("metadata contract"))
	require.NoError(t, err)
	require.NoError(t, writer.Close())

	req := httptest.NewRequest(http.MethodPost, "/api/knowledge-bases/"+kbID.String()+"/documents", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Empty(t, svc.docs)
}

func TestDocumentHandler_Upload_RejectsDuplicateDocumentMetadataWithoutCreating(t *testing.T) {
	r, svc := setupDocument()
	kbID := uuid.New()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	require.NoError(t, writer.WriteField("metadata", `{"source":"first","source":"second"}`))
	part, err := writer.CreateFormFile("file", "report.txt")
	require.NoError(t, err)
	_, err = part.Write([]byte("metadata contract"))
	require.NoError(t, err)
	require.NoError(t, writer.Close())

	req := httptest.NewRequest(http.MethodPost, "/api/knowledge-bases/"+kbID.String()+"/documents", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
	assert.Empty(t, svc.docs)
}

func TestDocumentHandler_Upload_RejectsMetadataOutsidePostgresJSONBNumericRange(t *testing.T) {
	r, svc := setupDocument()
	kbID := uuid.New()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	require.NoError(t, writer.WriteField("metadata", `{"year":1e131072}`))
	part, err := writer.CreateFormFile("file", "report.txt")
	require.NoError(t, err)
	_, err = part.Write([]byte("metadata numeric boundary"))
	require.NoError(t, err)
	require.NoError(t, writer.Close())

	req := httptest.NewRequest(http.MethodPost, "/api/knowledge-bases/"+kbID.String()+"/documents", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Empty(t, svc.docs)
}

func TestDocumentHandler_Upload_RejectsOversizeDocumentMetadata(t *testing.T) {
	r, _ := setupDocument()
	kbID := uuid.New()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	oversizeMetadata := `{"source":"` + strings.Repeat("x", 16<<10) + `"}`
	require.NoError(t, writer.WriteField("metadata", oversizeMetadata))
	part, err := writer.CreateFormFile("file", "report.txt")
	require.NoError(t, err)
	_, err = part.Write([]byte("metadata contract"))
	require.NoError(t, err)
	require.NoError(t, writer.Close())

	req := httptest.NewRequest(http.MethodPost, "/api/knowledge-bases/"+kbID.String()+"/documents", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
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

func TestDocumentHandler_Reprocess_Success(t *testing.T) {
	r, svc := setupDocument()
	kbID := uuid.New()
	docID := uuid.New()
	svc.docs[docID] = document.DocumentResponse{ID: docID, KnowledgeBaseID: kbID, FileName: "report.pdf", Status: document.StatusFailed}

	req := httptest.NewRequest(http.MethodPost, "/api/knowledge-bases/"+kbID.String()+"/documents/"+docID.String()+"/reprocess", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp document.DocumentResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, document.StatusPending, resp.Status)
}

func TestDocumentHandler_Reprocess_NotFound(t *testing.T) {
	r, _ := setupDocument()
	kbID := uuid.New()
	req := httptest.NewRequest(http.MethodPost, "/api/knowledge-bases/"+kbID.String()+"/documents/"+uuid.New().String()+"/reprocess", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}
