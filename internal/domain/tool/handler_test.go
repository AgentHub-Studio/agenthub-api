package tool_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/tool"
	"github.com/AgentHub-Studio/agenthub-api/internal/pagination"
)

// mockToolSvc satisfies the private toolService interface in tool.Handler.
type mockToolSvc struct {
	tools    map[uuid.UUID]tool.Response
	bindings map[uuid.UUID][]tool.SkillToolResponse
}

func newMockToolSvc() *mockToolSvc {
	return &mockToolSvc{
		tools:    make(map[uuid.UUID]tool.Response),
		bindings: make(map[uuid.UUID][]tool.SkillToolResponse),
	}
}

func (m *mockToolSvc) List(_ context.Context, req pagination.PageRequest, _ string) (pagination.Page[tool.Response], error) {
	items := make([]tool.Response, 0, len(m.tools))
	for _, t := range m.tools {
		items = append(items, t)
	}
	return pagination.NewPage(items, int64(len(items)), req), nil
}

func (m *mockToolSvc) Create(_ context.Context, req tool.CreateRequest) (tool.Response, error) {
	id := uuid.New()
	resp := tool.Response{ID: id, Name: req.Name, Type: req.Type, Description: req.Description}
	m.tools[id] = resp
	return resp, nil
}

func (m *mockToolSvc) GetByID(_ context.Context, id uuid.UUID) (tool.Response, error) {
	t, ok := m.tools[id]
	if !ok {
		return tool.Response{}, tool.ErrNotFound
	}
	return t, nil
}

func (m *mockToolSvc) Update(_ context.Context, id uuid.UUID, req tool.UpdateRequest) (tool.Response, error) {
	t, ok := m.tools[id]
	if !ok {
		return tool.Response{}, tool.ErrNotFound
	}
	t.Name = req.Name
	m.tools[id] = t
	return t, nil
}

func (m *mockToolSvc) Delete(_ context.Context, id uuid.UUID) error {
	if _, ok := m.tools[id]; !ok {
		return tool.ErrNotFound
	}
	delete(m.tools, id)
	return nil
}

func (m *mockToolSvc) BindToSkill(_ context.Context, skillID uuid.UUID, req tool.BindRequest) (tool.SkillToolResponse, error) {
	t, ok := m.tools[req.ToolID]
	if !ok {
		return tool.SkillToolResponse{}, tool.ErrNotFound
	}
	binding := tool.SkillToolResponse{
		ID:      uuid.New(),
		SkillID: skillID,
		Tool:    t,
	}
	m.bindings[skillID] = append(m.bindings[skillID], binding)
	return binding, nil
}

func (m *mockToolSvc) UnbindFromSkill(_ context.Context, skillID, toolID uuid.UUID) error {
	if _, ok := m.bindings[skillID]; !ok {
		return tool.ErrNotFound
	}
	return nil
}

func (m *mockToolSvc) ListBySkill(_ context.Context, skillID uuid.UUID) ([]tool.SkillToolResponse, error) {
	return m.bindings[skillID], nil
}

func (m *mockToolSvc) ListLabels(_ context.Context) ([]string, error) {
	return []string{}, nil
}

func (m *mockToolSvc) TestTool(_ context.Context, _ uuid.UUID, _ map[string]any) (string, error) {
	return "test result", nil
}

func (m *mockToolSvc) GenerateCode(_ context.Context, _, _ string) (string, error) {
	return "// generated code", nil
}

func (m *mockToolSvc) GenerateBlockly(_ context.Context, _ string) (any, error) {
	return map[string]any{}, nil
}

func (m *mockToolSvc) GetDatabaseSchema(_ context.Context, _ string) (tool.DatabaseSchema, error) {
	return tool.DatabaseSchema{Tables: []tool.TableSchema{}}, nil
}

func setupTool() (*chi.Mux, *mockToolSvc) {
	svc := newMockToolSvc()
	h := tool.NewHandler(svc)
	r := chi.NewRouter()
	h.RegisterRoutes(r)
	return r, svc
}

func TestToolHandler_List_Success(t *testing.T) {
	r, svc := setupTool()
	id := uuid.New()
	svc.tools[id] = tool.Response{ID: id, Name: "HTTP Tool", Type: "HTTP"}

	req := httptest.NewRequest(http.MethodGet, "/api/tools", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var page pagination.Page[tool.Response]
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &page))
	assert.Equal(t, int64(1), page.TotalElements)
}

func TestToolHandler_Create_Success(t *testing.T) {
	r, _ := setupTool()
	body, _ := json.Marshal(tool.CreateRequest{Name: "My Tool", Type: "HTTP"})
	req := httptest.NewRequest(http.MethodPost, "/api/tools", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
	var resp tool.Response
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "My Tool", resp.Name)
}

func TestToolHandler_Create_InvalidBody(t *testing.T) {
	r, _ := setupTool()
	req := httptest.NewRequest(http.MethodPost, "/api/tools", bytes.NewReader([]byte("not-json")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestToolHandler_Create_MissingFields(t *testing.T) {
	r, _ := setupTool()
	body, _ := json.Marshal(tool.CreateRequest{Name: ""})
	req := httptest.NewRequest(http.MethodPost, "/api/tools", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
}

func TestToolHandler_GetByID_NotFound(t *testing.T) {
	r, _ := setupTool()
	req := httptest.NewRequest(http.MethodGet, "/api/tools/"+uuid.New().String(), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestToolHandler_Delete_Success(t *testing.T) {
	r, svc := setupTool()
	id := uuid.New()
	svc.tools[id] = tool.Response{ID: id, Name: "To Delete", Type: "HTTP"}

	req := httptest.NewRequest(http.MethodDelete, "/api/tools/"+id.String(), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNoContent, w.Code)
}

func TestToolHandler_Delete_NotFound(t *testing.T) {
	r, _ := setupTool()
	req := httptest.NewRequest(http.MethodDelete, "/api/tools/"+uuid.New().String(), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestToolHandler_ListBySkill_Success(t *testing.T) {
	r, _ := setupTool()
	skillID := uuid.New()
	req := httptest.NewRequest(http.MethodGet, "/api/skills/"+skillID.String()+"/tools", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestToolHandler_ListLabels_Success(t *testing.T) {
	r, _ := setupTool()
	req := httptest.NewRequest(http.MethodGet, "/api/tools/labels", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var labels []string
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &labels))
}

func TestToolHandler_TestTool_Success(t *testing.T) {
	r, svc := setupTool()
	id := uuid.New()
	svc.tools[id] = tool.Response{ID: id, Name: "HTTP Tool", Type: "HTTP"}

	body, _ := json.Marshal(map[string]any{})
	req := httptest.NewRequest(http.MethodPost, "/api/tools/"+id.String()+"/test", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Header().Get("Content-Type"), "text/plain")
}

func TestToolHandler_GenerateCode_MissingPrompt(t *testing.T) {
	r, _ := setupTool()
	body, _ := json.Marshal(map[string]any{"prompt": ""})
	req := httptest.NewRequest(http.MethodPost, "/api/tools/generate/code", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
}

func TestToolHandler_GetDatabaseSchema_MissingParam(t *testing.T) {
	r, _ := setupTool()
	req := httptest.NewRequest(http.MethodGet, "/api/tools/database-schema", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestToolHandler_Patch_Success(t *testing.T) {
	r, svc := setupTool()
	id := uuid.New()
	svc.tools[id] = tool.Response{ID: id, Name: "Original", Type: "HTTP"}

	body, _ := json.Marshal(tool.UpdateRequest{Name: "Patched"})
	req := httptest.NewRequest(http.MethodPatch, "/api/tools/"+id.String(), bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp tool.Response
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "Patched", resp.Name)
}

func TestToolHandler_Patch_NotFound(t *testing.T) {
	r, _ := setupTool()
	body, _ := json.Marshal(tool.UpdateRequest{Name: "x"})
	req := httptest.NewRequest(http.MethodPatch, "/api/tools/"+uuid.New().String(), bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}
