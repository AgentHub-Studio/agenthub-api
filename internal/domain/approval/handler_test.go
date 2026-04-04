package approval_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/approval"
	"github.com/AgentHub-Studio/agenthub-api/internal/pagination"
)

// mockApprovalSvc satisfies the private approvalService interface in approval.Handler.
type mockApprovalSvc struct {
	data map[uuid.UUID]approval.PendingApproval
}

func newMockApprovalSvc() *mockApprovalSvc {
	return &mockApprovalSvc{data: make(map[uuid.UUID]approval.PendingApproval)}
}

func (m *mockApprovalSvc) Create(_ context.Context, req approval.CreateApprovalRequest) (approval.PendingApproval, error) {
	if req.ExecutionID == uuid.Nil {
		return approval.PendingApproval{}, &validationError{"executionId is required"}
	}
	a := approval.PendingApproval{
		ID:             uuid.New(),
		ExecutionID:    req.ExecutionID,
		NodeID:         req.NodeID,
		Title:          req.Title,
		Status:         approval.StatusPending,
		NotifyChannels: []string{},
		CreatedAt:      time.Now(),
	}
	m.data[a.ID] = a
	return a, nil
}

func (m *mockApprovalSvc) GetByID(_ context.Context, id uuid.UUID) (approval.PendingApproval, error) {
	a, ok := m.data[id]
	if !ok {
		return approval.PendingApproval{}, approval.ErrNotFound
	}
	return a, nil
}

func (m *mockApprovalSvc) List(_ context.Context, req pagination.PageRequest) (pagination.Page[approval.PendingApproval], error) {
	items := make([]approval.PendingApproval, 0, len(m.data))
	for _, a := range m.data {
		items = append(items, a)
	}
	return pagination.NewPage(items, int64(len(items)), req), nil
}

func (m *mockApprovalSvc) PendingCount(_ context.Context) (approval.PendingCountResponse, error) {
	var count int64
	for _, a := range m.data {
		if a.Status == approval.StatusPending {
			count++
		}
	}
	return approval.PendingCountResponse{Count: count}, nil
}

func (m *mockApprovalSvc) Delete(_ context.Context, id uuid.UUID) error {
	if _, ok := m.data[id]; !ok {
		return approval.ErrNotFound
	}
	delete(m.data, id)
	return nil
}

func (m *mockApprovalSvc) Respond(_ context.Context, id uuid.UUID, respondedBy string, req approval.RespondRequest) (approval.PendingApproval, error) {
	a, ok := m.data[id]
	if !ok {
		return approval.PendingApproval{}, approval.ErrNotFound
	}
	if a.Status != approval.StatusPending {
		return approval.PendingApproval{}, approval.ErrAlreadyResolved
	}
	if req.Approved {
		a.Status = approval.StatusApproved
	} else {
		a.Status = approval.StatusRejected
	}
	now := time.Now()
	a.RespondedBy = &respondedBy
	a.RespondedAt = &now
	m.data[id] = a
	return a, nil
}

// validationError implements the error interface for bad-request scenarios.
type validationError struct{ msg string }

func (e *validationError) Error() string { return e.msg }

// setup creates a chi router with approval routes wired.
func setupApprovalRouter() (*chi.Mux, *mockApprovalSvc) {
	svc := newMockApprovalSvc()
	h := approval.NewHandler(svc)
	r := chi.NewRouter()
	h.RegisterRoutes(r)
	return r, svc
}

func seedApproval(svc *mockApprovalSvc) approval.PendingApproval {
	a := approval.PendingApproval{
		ID:             uuid.New(),
		ExecutionID:    uuid.New(),
		NodeID:         "node-1",
		Title:          "Deploy approval",
		Status:         approval.StatusPending,
		NotifyChannels: []string{},
		CreatedAt:      time.Now(),
	}
	svc.data[a.ID] = a
	return a
}

// ------ handler tests ------

func TestApprovalHandler_List_OK(t *testing.T) {
	r, svc := setupApprovalRouter()
	seedApproval(svc)
	seedApproval(svc)

	req := httptest.NewRequest(http.MethodGet, "/api/approvals?page=0&size=20", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var page pagination.Page[approval.PendingApproval]
	require.NoError(t, json.NewDecoder(w.Body).Decode(&page))
	assert.Equal(t, int64(2), page.TotalElements)
}

func TestApprovalHandler_Create_OK(t *testing.T) {
	r, _ := setupApprovalRouter()
	body := approval.CreateApprovalRequest{
		ExecutionID: uuid.New(),
		NodeID:      "node-1",
		Title:       "Approve PR merge",
	}
	payload, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/api/approvals", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
	var a approval.PendingApproval
	require.NoError(t, json.NewDecoder(w.Body).Decode(&a))
	assert.NotEqual(t, uuid.Nil, a.ID)
	assert.Equal(t, approval.StatusPending, a.Status)
}

func TestApprovalHandler_Create_BadRequest(t *testing.T) {
	r, _ := setupApprovalRouter()
	req := httptest.NewRequest(http.MethodPost, "/api/approvals", bytes.NewBufferString("not json"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestApprovalHandler_PendingCount(t *testing.T) {
	r, svc := setupApprovalRouter()
	seedApproval(svc)

	req := httptest.NewRequest(http.MethodGet, "/api/approvals/pending-count", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp approval.PendingCountResponse
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	assert.Equal(t, int64(1), resp.Count)
}

func TestApprovalHandler_GetByID_OK(t *testing.T) {
	r, svc := setupApprovalRouter()
	a := seedApproval(svc)

	req := httptest.NewRequest(http.MethodGet, "/api/approvals/"+a.ID.String(), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var got approval.PendingApproval
	require.NoError(t, json.NewDecoder(w.Body).Decode(&got))
	assert.Equal(t, a.ID, got.ID)
}

func TestApprovalHandler_GetByID_NotFound(t *testing.T) {
	r, _ := setupApprovalRouter()
	req := httptest.NewRequest(http.MethodGet, "/api/approvals/"+uuid.New().String(), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestApprovalHandler_GetByID_InvalidID(t *testing.T) {
	r, _ := setupApprovalRouter()
	req := httptest.NewRequest(http.MethodGet, "/api/approvals/not-a-uuid", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestApprovalHandler_Respond_Approve(t *testing.T) {
	r, svc := setupApprovalRouter()
	a := seedApproval(svc)

	body := approval.RespondRequest{Approved: true}
	payload, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/api/approvals/"+a.ID.String()+"/respond", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	// Provide a dummy JWT with sub claim
	req.Header.Set("Authorization", "Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiJ1c2VyQGV4YW1wbGUuY29tIn0.signature")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var got approval.PendingApproval
	require.NoError(t, json.NewDecoder(w.Body).Decode(&got))
	assert.Equal(t, approval.StatusApproved, got.Status)
}

func TestApprovalHandler_Respond_NotFound(t *testing.T) {
	r, _ := setupApprovalRouter()
	body := approval.RespondRequest{Approved: true}
	payload, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/api/approvals/"+uuid.New().String()+"/respond", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestApprovalHandler_Respond_AlreadyResolved(t *testing.T) {
	r, svc := setupApprovalRouter()
	a := seedApproval(svc)
	// Mark as already approved
	a.Status = approval.StatusApproved
	svc.data[a.ID] = a

	body := approval.RespondRequest{Approved: false}
	payload, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/api/approvals/"+a.ID.String()+"/respond", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusConflict, w.Code)
}

func TestApprovalHandler_Respond_InvalidID(t *testing.T) {
	r, _ := setupApprovalRouter()
	body := approval.RespondRequest{Approved: true}
	payload, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/api/approvals/not-a-uuid/respond", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestApprovalHandler_Delete_Success(t *testing.T) {
	r, svc := setupApprovalRouter()
	a := seedApproval(svc)

	req := httptest.NewRequest(http.MethodDelete, "/api/approvals/"+a.ID.String(), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNoContent, w.Code)
	assert.Empty(t, svc.data)
}

func TestApprovalHandler_Delete_NotFound(t *testing.T) {
	r, _ := setupApprovalRouter()
	req := httptest.NewRequest(http.MethodDelete, "/api/approvals/"+uuid.New().String(), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestApprovalHandler_Delete_InvalidID(t *testing.T) {
	r, _ := setupApprovalRouter()
	req := httptest.NewRequest(http.MethodDelete, "/api/approvals/not-a-uuid", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestApprovalHandler_Stream_SSEHeaders(t *testing.T) {
	r, _ := setupApprovalRouter()

	// Cancel the context quickly so the handler exits without hanging the test.
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	req := httptest.NewRequest(http.MethodGet, "/api/approvals/stream", nil).WithContext(ctx)
	req.Header.Set("Accept", "text/event-stream")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Header().Get("Content-Type"), "text/event-stream")
	assert.Equal(t, "no-cache", w.Header().Get("Cache-Control"))
}
