package tool_test

import (
	"context"
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

func (m *mockToolRepo) Update(_ context.Context, id uuid.UUID, req tool.UpdateRequest) (tool.Tool, error) {
	t, ok := m.data[id]
	if !ok {
		return tool.Tool{}, tool.ErrNotFound
	}
	t.Name = req.Name
	t.Type = req.Type
	t.Description = req.Description
	t.Labels = req.Labels
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
		Name: "HTTP POST",
		Type: "HTTP",
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
	created, err := svc.Create(context.Background(), tool.CreateRequest{Name: "SQL Query", Type: "SQL"})
	require.NoError(t, err)
	skillID := uuid.New()
	binding, err := svc.BindToSkill(context.Background(), skillID, tool.BindRequest{ToolID: created.ID, Priority: 1})
	require.NoError(t, err)
	assert.Equal(t, skillID, binding.SkillID)
}

func TestToolService_BindToSkill_AlreadyBound(t *testing.T) {
	svc := tool.NewService(newMockRepo())
	created, err := svc.Create(context.Background(), tool.CreateRequest{Name: "SQL Query", Type: "SQL"})
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
	assert.Contains(t, err.Error(), "unsupported type")
}

func TestToolService_Create_MissingName(t *testing.T) {
	svc := tool.NewService(newMockRepo())
	_, err := svc.Create(context.Background(), tool.CreateRequest{Type: tool.ToolTypeHTTP})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "name")
}

func TestToolService_Create_WithLabels(t *testing.T) {
	svc := tool.NewService(newMockRepo())
	created, err := svc.Create(context.Background(), tool.CreateRequest{
		Name:   "Tagged Tool",
		Type:   tool.ToolTypeHTTP,
		Labels: []string{"prod", "external"},
	})
	require.NoError(t, err)
	assert.Equal(t, []string{"prod", "external"}, created.Labels)
}

func TestToolService_Create_ValidTypes(t *testing.T) {
	validTypes := []string{
		tool.ToolTypeHTTP,
		tool.ToolTypeSQL,
		tool.ToolTypeDocumentSearch,
		tool.ToolTypeCustom,
		tool.ToolTypeBlockly,
		tool.ToolTypeComposite,
	}
	for _, tt := range validTypes {
		t.Run(tt, func(t *testing.T) {
			svc := tool.NewService(newMockRepo())
			_, err := svc.Create(context.Background(), tool.CreateRequest{Name: "T", Type: tt})
			require.NoError(t, err)
		})
	}
}

func TestToolService_List_FilterByType(t *testing.T) {
	svc := tool.NewService(newMockRepo())
	_, err := svc.Create(context.Background(), tool.CreateRequest{Name: "HTTP", Type: "HTTP"})
	require.NoError(t, err)
	_, err = svc.Create(context.Background(), tool.CreateRequest{Name: "SQL", Type: "SQL"})
	require.NoError(t, err)
	page, err := svc.List(context.Background(), pagination.PageRequest{Page: 0, Size: 20}, "HTTP")
	require.NoError(t, err)
	assert.Equal(t, int64(1), page.TotalElements)
}
