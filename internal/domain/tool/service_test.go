package tool_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/tool"
	"github.com/AgentHub-Studio/agenthub-api/internal/pagination"
)

type mockToolRepo struct {
	data     map[uuid.UUID]tool.Tool
	bindings []tool.SkillTool
}

func newMockRepo() *mockToolRepo {
	return &mockToolRepo{data: make(map[uuid.UUID]tool.Tool)}
}

func (m *mockToolRepo) List(_ context.Context, _ pagination.PageRequest, toolType string) ([]tool.Tool, int64, error) {
	var out []tool.Tool
	for _, t := range m.data {
		if toolType == "" || t.Type == toolType {
			out = append(out, t)
		}
	}
	return out, int64(len(out)), nil
}

func (m *mockToolRepo) Create(_ context.Context, t tool.Tool) (tool.Tool, error) {
	t.ID = uuid.New()
	m.data[t.ID] = t
	return t, nil
}

func (m *mockToolRepo) GetByID(_ context.Context, id uuid.UUID) (tool.Tool, error) {
	t, ok := m.data[id]
	if !ok {
		return tool.Tool{}, tool.ErrNotFound
	}
	return t, nil
}

func (m *mockToolRepo) Update(_ context.Context, id uuid.UUID, t tool.Tool) (tool.Tool, error) {
	if _, ok := m.data[id]; !ok {
		return tool.Tool{}, tool.ErrNotFound
	}
	m.data[id] = t
	return t, nil
}

func (m *mockToolRepo) Delete(_ context.Context, id uuid.UUID) error {
	if _, ok := m.data[id]; !ok {
		return tool.ErrNotFound
	}
	delete(m.data, id)
	return nil
}

func (m *mockToolRepo) BindToSkill(_ context.Context, skillID uuid.UUID, req tool.BindRequest) (tool.SkillTool, error) {
	for _, b := range m.bindings {
		if b.SkillID == skillID && b.ToolID == req.ToolID {
			return tool.SkillTool{}, tool.ErrAlreadyBound
		}
	}
	active := true
	if req.IsActive != nil {
		active = *req.IsActive
	}
	st := tool.SkillTool{
		ID:       uuid.New(),
		SkillID:  skillID,
		ToolID:   req.ToolID,
		Priority: req.Priority,
		IsActive: active,
	}
	m.bindings = append(m.bindings, st)
	return st, nil
}

func (m *mockToolRepo) UnbindFromSkill(_ context.Context, skillID, toolID uuid.UUID) error {
	for i, b := range m.bindings {
		if b.SkillID == skillID && b.ToolID == toolID {
			m.bindings = append(m.bindings[:i], m.bindings[i+1:]...)
			return nil
		}
	}
	return tool.ErrNotFound
}

func (m *mockToolRepo) ListLabels(_ context.Context) ([]string, error) {
	seen := map[string]bool{}
	var labels []string
	for _, t := range m.data {
		for _, l := range t.Labels {
			if !seen[l] {
				seen[l] = true
				labels = append(labels, l)
			}
		}
	}
	return labels, nil
}

func (m *mockToolRepo) ListBySkill(_ context.Context, skillID uuid.UUID) ([]tool.SkillTool, []tool.Tool, error) {
	var bindings []tool.SkillTool
	var tools []tool.Tool
	for _, b := range m.bindings {
		if b.SkillID == skillID {
			bindings = append(bindings, b)
			if t, ok := m.data[b.ToolID]; ok {
				tools = append(tools, t)
			}
		}
	}
	return bindings, tools, nil
}

func TestToolService_Create_Success(t *testing.T) {
	svc := tool.NewService(newMockRepo())
	created, err := svc.Create(context.Background(), tool.CreateRequest{
		Name:   "HTTP POST",
		Type:   "HTTP",
		Config: json.RawMessage(`{"url":"https://api.example.com/endpoint"}`),
	})
	require.NoError(t, err)
	assert.Equal(t, "HTTP POST", created.Name)
	assert.NotEqual(t, uuid.Nil, created.ID)
}

func TestToolService_GetByID_NotFound(t *testing.T) {
	svc := tool.NewService(newMockRepo())
	_, err := svc.GetByID(context.Background(), uuid.New())
	require.ErrorIs(t, err, tool.ErrNotFound)
}

func TestToolService_BindToSkill_Success(t *testing.T) {
	svc := tool.NewService(newMockRepo())
	created, err := svc.Create(context.Background(), tool.CreateRequest{Name: "SQL Query", Type: "SQL", Config: json.RawMessage(`{"query":"SELECT 1","datasourceId":"00000000-0000-0000-0000-000000000001"}`)})
	require.NoError(t, err)
	skillID := uuid.New()
	binding, err := svc.BindToSkill(context.Background(), skillID, tool.BindRequest{ToolID: created.ID, Priority: 1})
	require.NoError(t, err)
	assert.Equal(t, skillID, binding.SkillID)
}

func TestToolService_BindToSkill_AlreadyBound(t *testing.T) {
	svc := tool.NewService(newMockRepo())
	created, err := svc.Create(context.Background(), tool.CreateRequest{Name: "SQL Query", Type: "SQL", Config: json.RawMessage(`{"query":"SELECT 1","datasourceId":"00000000-0000-0000-0000-000000000001"}`)})
	require.NoError(t, err)
	skillID := uuid.New()
	_, err = svc.BindToSkill(context.Background(), skillID, tool.BindRequest{ToolID: created.ID, Priority: 1})
	require.NoError(t, err)
	_, err = svc.BindToSkill(context.Background(), skillID, tool.BindRequest{ToolID: created.ID, Priority: 1})
	require.ErrorIs(t, err, tool.ErrAlreadyBound)
}

func TestToolService_Create_InvalidType(t *testing.T) {
	svc := tool.NewService(newMockRepo())
	_, err := svc.Create(context.Background(), tool.CreateRequest{
		Name: "Bad Tool",
		Type: "INVALID_TYPE",
	})
	require.Error(t, err)
	assert.ErrorIs(t, err, tool.ErrValidation)
	assert.Contains(t, err.Error(), "unsupported type")
}

func TestToolService_Create_MissingName(t *testing.T) {
	svc := tool.NewService(newMockRepo())
	_, err := svc.Create(context.Background(), tool.CreateRequest{Type: tool.ToolTypeHTTP})
	require.Error(t, err)
	assert.ErrorIs(t, err, tool.ErrValidation)
	assert.Contains(t, err.Error(), "name")
}

func TestToolService_Create_HTTPRejectsSSRFURL(t *testing.T) {
	svc := tool.NewService(newMockRepo())
	_, err := svc.Create(context.Background(), tool.CreateRequest{
		Name:   "SSRF Tool",
		Type:   tool.ToolTypeHTTP,
		Config: json.RawMessage(`{"url":"http://169.254.169.254/latest/meta-data/"}`),
	})
	require.Error(t, err)
	assert.ErrorIs(t, err, tool.ErrValidation)
	assert.Contains(t, err.Error(), "invalid URL")
}

func TestToolService_Create_WithLabels(t *testing.T) {
	svc := tool.NewService(newMockRepo())
	created, err := svc.Create(context.Background(), tool.CreateRequest{
		Name:   "Tagged Tool",
		Type:   tool.ToolTypeHTTP,
		Config: json.RawMessage(`{"url":"https://api.example.com/v1"}`),
		Labels: []string{"prod", "external"},
	})
	require.NoError(t, err)
	assert.Equal(t, []string{"prod", "external"}, created.Labels)
}

func TestToolService_Create_ValidTypes(t *testing.T) {
	httpConfig := json.RawMessage(`{"url":"https://api.example.com/endpoint"}`)
	sqlConfig := json.RawMessage(`{"query":"SELECT 1","datasourceId":"00000000-0000-0000-0000-000000000001"}`)
	docConfig := json.RawMessage(`{"kbId":"00000000-0000-0000-0000-000000000002"}`)
	validTypes := []struct {
		typ    string
		config json.RawMessage
	}{
		{tool.ToolTypeHTTP, httpConfig},
		{tool.ToolTypeSQL, sqlConfig},
		{tool.ToolTypeDocumentSearch, docConfig},
		{tool.ToolTypeCustom, nil},
		{tool.ToolTypeBlockly, nil},
		{tool.ToolTypeComposite, nil},
		{tool.ToolTypeCode, nil},
		{tool.ToolTypeDatabase, sqlConfig},
		{tool.ToolTypeDocuments, docConfig},
	}
	for _, tc := range validTypes {
		t.Run(tc.typ, func(t *testing.T) {
			svc := tool.NewService(newMockRepo())
			_, err := svc.Create(context.Background(), tool.CreateRequest{Name: "T", Type: tc.typ, Config: tc.config})
			require.NoError(t, err)
		})
	}
}

// --- TR-01-TASK-19: PATCH preserves unset fields (P-C196-1) ---

func TestPatchTool_OnlyDescriptionSent_TypePreserved(t *testing.T) {
	svc := tool.NewService(newMockRepo())
	created, err := svc.Create(context.Background(), tool.CreateRequest{
		Name:        "my-tool",
		Type:        tool.ToolTypeHTTP,
		Description: "original",
		Config:      json.RawMessage(`{"url":"https://api.example.com/endpoint"}`),
	})
	require.NoError(t, err)

	newDesc := "updated description"
	updated, err := svc.Update(context.Background(), created.ID, tool.UpdateRequest{
		Description: &newDesc,
	})

	require.NoError(t, err)
	assert.Equal(t, "updated description", updated.Description)
	assert.Equal(t, tool.ToolType(tool.ToolTypeHTTP), updated.Type, "type must be preserved")
	assert.Equal(t, "my-tool", updated.Name, "name must be preserved")
}

func TestPatchTool_OnlyNameSent_TypeAndDescPreserved(t *testing.T) {
	svc := tool.NewService(newMockRepo())
	created, err := svc.Create(context.Background(), tool.CreateRequest{
		Name: "old-name", Type: tool.ToolTypeSQL, Description: "keep me",
		Config: json.RawMessage(`{"query":"SELECT 1","datasourceId":"00000000-0000-0000-0000-000000000001"}`),
	})
	require.NoError(t, err)

	newName := "new-name"
	updated, err := svc.Update(context.Background(), created.ID, tool.UpdateRequest{
		Name: &newName,
	})

	require.NoError(t, err)
	assert.Equal(t, "new-name", updated.Name)
	assert.Equal(t, tool.ToolType(tool.ToolTypeSQL), updated.Type, "type must be preserved")
	assert.Equal(t, "keep me", updated.Description, "description must be preserved")
}

func TestPatchTool_EmptyBody_NoChanges(t *testing.T) {
	svc := tool.NewService(newMockRepo())
	created, err := svc.Create(context.Background(), tool.CreateRequest{
		Name: "stable-tool", Type: tool.ToolTypeCustom, Description: "unchanged",
	})
	require.NoError(t, err)

	// Empty UpdateRequest — nothing should change.
	updated, err := svc.Update(context.Background(), created.ID, tool.UpdateRequest{})

	require.NoError(t, err)
	assert.Equal(t, "stable-tool", updated.Name)
	assert.Equal(t, "unchanged", updated.Description)
}

func TestToolService_List_FilterByType(t *testing.T) {
	svc := tool.NewService(newMockRepo())
	_, err := svc.Create(context.Background(), tool.CreateRequest{Name: "HTTP", Type: "HTTP", Config: json.RawMessage(`{"url":"https://api.example.com/v1"}`)})
	require.NoError(t, err)
	_, err = svc.Create(context.Background(), tool.CreateRequest{Name: "SQL", Type: "SQL", Config: json.RawMessage(`{"query":"SELECT 1","datasourceId":"00000000-0000-0000-0000-000000000001"}`)})
	require.NoError(t, err)
	page, err := svc.List(context.Background(), pagination.PageRequest{Page: 0, Size: 20}, "HTTP")
	require.NoError(t, err)
	assert.Equal(t, int64(1), page.TotalElements)
}

// --- TR-01-TASK-28: URL obrigatória em HTTP tools (P-C254-1) ---

func TestToolService_Create_HTTPMissingURL_Rejected(t *testing.T) {
	svc := tool.NewService(newMockRepo())
	_, err := svc.Create(context.Background(), tool.CreateRequest{
		Name: "No URL",
		Type: tool.ToolTypeHTTP,
		// Config has no "url" field.
		Config: json.RawMessage(`{"method":"GET"}`),
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "url")
	assert.Contains(t, err.Error(), "required")
}

func TestToolService_Create_HTTPEmptyConfig_Rejected(t *testing.T) {
	svc := tool.NewService(newMockRepo())
	_, err := svc.Create(context.Background(), tool.CreateRequest{
		Name: "No Config",
		Type: tool.ToolTypeHTTP,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "url")
}

func TestToolService_Update_HTTPRemoveURL_Rejected(t *testing.T) {
	svc := tool.NewService(newMockRepo())
	created, err := svc.Create(context.Background(), tool.CreateRequest{
		Name:   "With URL",
		Type:   tool.ToolTypeHTTP,
		Config: json.RawMessage(`{"url":"https://api.example.com/v1"}`),
	})
	require.NoError(t, err)

	// Update to remove the URL from config.
	_, err = svc.Update(context.Background(), created.ID, tool.UpdateRequest{
		Config: json.RawMessage(`{"method":"POST"}`),
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "url")
}

func TestToolService_NonHTTP_NoURLRequired(t *testing.T) {
	svc := tool.NewService(newMockRepo())
	_, err := svc.Create(context.Background(), tool.CreateRequest{
		Name:   "SQL Tool",
		Type:   tool.ToolTypeSQL,
		Config: json.RawMessage(`{"query":"SELECT 1","datasourceId":"00000000-0000-0000-0000-000000000001"}`),
	})
	require.NoError(t, err)
}

// --- TR-01-TASK-30: normalizar datasourceId → datasource_id (P-C249-1) ---

// configKeys returns the top-level keys of a tool's Config (which is type any).
func configKeys(t *testing.T, cfg any) map[string]bool {
	t.Helper()
	b, err := json.Marshal(cfg)
	require.NoError(t, err)
	var m map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(b, &m))
	keys := make(map[string]bool, len(m))
	for k := range m {
		keys[k] = true
	}
	return keys
}

func TestNormalizeDataSourceID_CamelCaseConvertedToSnakeCase(t *testing.T) {
	svc := tool.NewService(newMockRepo())
	created, err := svc.Create(context.Background(), tool.CreateRequest{
		Name:   "SQL Query",
		Type:   tool.ToolTypeSQL,
		Config: json.RawMessage(`{"datasourceId":"00000000-0000-0000-0000-000000000001","query":"SELECT 1"}`),
	})
	require.NoError(t, err)

	keys := configKeys(t, created.Config)
	assert.True(t, keys["datasource_id"], "snake_case key should be present")
	assert.False(t, keys["datasourceId"], "camelCase key should be removed")
}

func TestNormalizeDataSourceID_AlreadySnakeCase_Unchanged(t *testing.T) {
	svc := tool.NewService(newMockRepo())
	created, err := svc.Create(context.Background(), tool.CreateRequest{
		Name:   "SQL Query 2",
		Type:   tool.ToolTypeSQL,
		Config: json.RawMessage(`{"datasource_id":"00000000-0000-0000-0000-000000000002","query":"SELECT 1"}`),
	})
	require.NoError(t, err)

	keys := configKeys(t, created.Config)
	assert.True(t, keys["datasource_id"])
}

func TestNormalizeDataSourceID_NonSQLToolNotTouched(t *testing.T) {
	svc := tool.NewService(newMockRepo())
	created, err := svc.Create(context.Background(), tool.CreateRequest{
		Name:   "Custom Tool",
		Type:   tool.ToolTypeCustom,
		Config: json.RawMessage(`{"datasourceId":"abc-123"}`),
	})
	require.NoError(t, err)

	// Non-SQL tools should NOT be normalised — config returned as-is.
	keys := configKeys(t, created.Config)
	assert.True(t, keys["datasourceId"], "non-SQL tool config should be untouched")
}

// --- TR-QA-LOOP: skillId auto-bind on create (Cenário A) ---

func TestToolService_Create_WithSkillID_AutoBinds(t *testing.T) {
	repo := newMockRepo()
	svc := tool.NewService(repo)

	skillID := uuid.New()
	created, err := svc.Create(context.Background(), tool.CreateRequest{
		Name:    "auto-bind-tool",
		Type:    tool.ToolTypeCustom,
		SkillID: &skillID,
	})
	require.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, created.ID)

	// Verify the tool was auto-bound to the skill.
	bindings, _, err := repo.ListBySkill(context.Background(), skillID)
	require.NoError(t, err)
	require.Len(t, bindings, 1, "tool should have been auto-bound to skill")
	assert.Equal(t, created.ID, bindings[0].ToolID)
}

func TestToolService_Create_WithNilSkillID_DoesNotBind(t *testing.T) {
	repo := newMockRepo()
	svc := tool.NewService(repo)

	skillID := uuid.New()
	_, err := svc.Create(context.Background(), tool.CreateRequest{
		Name:    "no-auto-bind-tool",
		Type:    tool.ToolTypeCustom,
		SkillID: nil,
	})
	require.NoError(t, err)

	// No binding should have been created.
	bindings, _, err := repo.ListBySkill(context.Background(), skillID)
	require.NoError(t, err)
	assert.Empty(t, bindings, "no binding should be created when SkillID is nil")
}
