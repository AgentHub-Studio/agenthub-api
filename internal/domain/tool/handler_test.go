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
	tools                map[uuid.UUID]tool.Response
	bindings             map[uuid.UUID][]tool.SkillToolResponse
	createForceDuplicate string    // if non-empty, Create returns ErrDuplicateName for this name
	updateForceDuplicate uuid.UUID // if non-zero, Update returns ErrDuplicateName for this ID
	createErr            error
	updateErr            error
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
	if m.createErr != nil {
		return tool.Response{}, m.createErr
	}
	if m.createForceDuplicate != "" && req.Name == m.createForceDuplicate {
		return tool.Response{}, tool.ErrDuplicateName
	}
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
	if m.updateErr != nil {
		return tool.Response{}, m.updateErr
	}
	if m.updateForceDuplicate != uuid.Nil && id == m.updateForceDuplicate {
		return tool.Response{}, tool.ErrDuplicateName
	}
	t, ok := m.tools[id]
	if !ok {
		return tool.Response{}, tool.ErrNotFound
	}
	if req.Name != nil {
		t.Name = *req.Name
	}
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

func setupRealTool() *chi.Mux {
	h := tool.NewHandler(tool.NewService(newMockRepo()))
	r := chi.NewRouter()
	h.RegisterRoutes(r)
	return r
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

func TestToolHandler_Create_HTTPSSRFURL_Returns422(t *testing.T) {
	r := setupRealTool()
	body, _ := json.Marshal(tool.CreateRequest{
		Name:   "SSRF Tool",
		Type:   tool.ToolTypeHTTP,
		Config: json.RawMessage(`{"url":"http://169.254.169.254/latest/meta-data/"}`),
	})
	req := httptest.NewRequest(http.MethodPost, "/api/tools", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
	assert.Contains(t, w.Body.String(), "invalid URL")
}

func TestToolHandler_Update_HTTPSSRFURL_Returns422(t *testing.T) {
	r := setupRealTool()
	createBody, _ := json.Marshal(tool.CreateRequest{
		Name:   "HTTP Tool",
		Type:   tool.ToolTypeHTTP,
		Config: json.RawMessage(`{"url":"https://api.example.com/data"}`),
	})
	createReq := httptest.NewRequest(http.MethodPost, "/api/tools", bytes.NewReader(createBody))
	createReq.Header.Set("Content-Type", "application/json")
	createW := httptest.NewRecorder()
	r.ServeHTTP(createW, createReq)
	require.Equal(t, http.StatusCreated, createW.Code)

	var created tool.Response
	require.NoError(t, json.Unmarshal(createW.Body.Bytes(), &created))
	updateBody, _ := json.Marshal(tool.UpdateRequest{
		Config: json.RawMessage(`{"url":"http://127.0.0.1/admin"}`),
	})
	updateReq := httptest.NewRequest(http.MethodPatch, "/api/tools/"+created.ID.String(), bytes.NewReader(updateBody))
	updateReq.Header.Set("Content-Type", "application/json")
	updateW := httptest.NewRecorder()
	r.ServeHTTP(updateW, updateReq)

	assert.Equal(t, http.StatusUnprocessableEntity, updateW.Code)
	assert.Contains(t, updateW.Body.String(), "invalid URL")
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

func TestToolHandler_Create_ValidationError(t *testing.T) {
	r, svc := setupTool()
	svc.createErr = tool.ErrValidation

	body, _ := json.Marshal(tool.CreateRequest{Name: "Bad Tool", Type: "HTTP"})
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

	patchedName := "Patched"
	body, _ := json.Marshal(tool.UpdateRequest{Name: &patchedName})
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
	xName := "x"
	body, _ := json.Marshal(tool.UpdateRequest{Name: &xName})
	req := httptest.NewRequest(http.MethodPatch, "/api/tools/"+uuid.New().String(), bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestToolHandler_Update_Success(t *testing.T) {
	r, svc := setupTool()
	id := uuid.New()
	svc.tools[id] = tool.Response{ID: id, Name: "Original", Type: "HTTP"}

	updatedName := "Updated"
	body, _ := json.Marshal(tool.UpdateRequest{Name: &updatedName})
	req := httptest.NewRequest(http.MethodPut, "/api/tools/"+id.String(), bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp tool.Response
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "Updated", resp.Name)
}

func TestToolHandler_Update_NotFound(t *testing.T) {
	r, _ := setupTool()
	xName := "x"
	body, _ := json.Marshal(tool.UpdateRequest{Name: &xName})
	req := httptest.NewRequest(http.MethodPut, "/api/tools/"+uuid.New().String(), bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestToolHandler_Update_ValidationError(t *testing.T) {
	r, svc := setupTool()
	id := uuid.New()
	svc.tools[id] = tool.Response{ID: id, Name: "Original", Type: "HTTP"}
	svc.updateErr = tool.ErrValidation

	updatedName := "Updated"
	body, _ := json.Marshal(tool.UpdateRequest{Name: &updatedName})
	req := httptest.NewRequest(http.MethodPut, "/api/tools/"+id.String(), bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
}

func TestToolHandler_BindToSkill_Success(t *testing.T) {
	r, svc := setupTool()
	skillID := uuid.New()
	toolID := uuid.New()
	svc.tools[toolID] = tool.Response{ID: toolID, Name: "HTTP Tool", Type: "HTTP"}

	body, _ := json.Marshal(tool.BindRequest{ToolID: toolID})
	req := httptest.NewRequest(http.MethodPost, "/api/skills/"+skillID.String()+"/tools", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
	var resp tool.SkillToolResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, skillID, resp.SkillID)
	assert.Equal(t, toolID, resp.Tool.ID)
}

func TestToolHandler_BindToSkill_ToolNotFound(t *testing.T) {
	r, _ := setupTool()
	skillID := uuid.New()

	body, _ := json.Marshal(tool.BindRequest{ToolID: uuid.New()})
	req := httptest.NewRequest(http.MethodPost, "/api/skills/"+skillID.String()+"/tools", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestToolHandler_UnbindFromSkill_Success(t *testing.T) {
	r, svc := setupTool()
	skillID := uuid.New()
	toolID := uuid.New()
	// seed a binding so UnbindFromSkill does not return ErrNotFound
	svc.bindings[skillID] = []tool.SkillToolResponse{{ID: uuid.New(), SkillID: skillID}}

	req := httptest.NewRequest(http.MethodDelete, "/api/skills/"+skillID.String()+"/tools/"+toolID.String(), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNoContent, w.Code)
}

func TestToolHandler_UnbindFromSkill_NotFound(t *testing.T) {
	r, _ := setupTool()
	req := httptest.NewRequest(http.MethodDelete, "/api/skills/"+uuid.New().String()+"/tools/"+uuid.New().String(), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

// TestToolHandler_Create_DuplicateName verifies that duplicate name returns 422
// with a user-friendly error (P-C338-1 / ACT-F3-15).
func TestToolHandler_Create_DuplicateName(t *testing.T) {
	r, svc := setupTool()
	// Pre-seed a tool with the same name so mock returns ErrDuplicateName.
	existingID := uuid.New()
	svc.tools[existingID] = tool.Response{ID: existingID, Name: "duplicate-tool", Type: "HTTP"}

	body, _ := json.Marshal(tool.CreateRequest{Name: "duplicate-tool", Type: "HTTP", Config: json.RawMessage(`{"url":"http://example.com"}`)})
	req := httptest.NewRequest(http.MethodPost, "/api/tools", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	// Override Create to return ErrDuplicateName for this name.
	svc.createForceDuplicate = "duplicate-tool"
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
	var resp map[string]string
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	assert.Contains(t, resp["error"], "already exists")
}

// TestToolHandler_Update_DuplicateName verifies that renaming to an existing tool name returns 422.
func TestToolHandler_Update_DuplicateName(t *testing.T) {
	r, svc := setupTool()
	id := uuid.New()
	svc.tools[id] = tool.Response{ID: id, Name: "original-tool", Type: "HTTP"}
	svc.updateForceDuplicate = id

	name := "duplicate-tool"
	body, _ := json.Marshal(tool.UpdateRequest{Name: &name})
	req := httptest.NewRequest(http.MethodPatch, "/api/tools/"+id.String(), bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
}
