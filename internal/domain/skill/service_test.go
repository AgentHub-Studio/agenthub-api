package skill_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/skill"
	"github.com/AgentHub-Studio/agenthub-api/internal/pagination"
)

type mockSkillRepo struct {
	data  map[uuid.UUID]skill.Skill
	slugs map[string]bool
}

func newMockRepo() *mockSkillRepo {
	return &mockSkillRepo{
		data:  make(map[uuid.UUID]skill.Skill),
		slugs: make(map[string]bool),
	}
}

func (m *mockSkillRepo) List(_ context.Context, _ *string, _ pagination.PageRequest) ([]skill.Skill, int64, error) {
	out := make([]skill.Skill, 0, len(m.data))
	for _, s := range m.data {
		out = append(out, s)
	}
	return out, int64(len(out)), nil
}

func (m *mockSkillRepo) Create(_ context.Context, s skill.Skill) (skill.Skill, error) {
	s.ID = uuid.New()
	m.data[s.ID] = s
	m.slugs[s.Slug] = true
	return s, nil
}

func (m *mockSkillRepo) GetByID(_ context.Context, id uuid.UUID) (skill.Skill, error) {
	s, ok := m.data[id]
	if !ok {
		return skill.Skill{}, skill.ErrNotFound
	}
	return s, nil
}

func (m *mockSkillRepo) Update(_ context.Context, id uuid.UUID, req skill.UpdateRequest) (skill.Skill, error) {
	s, ok := m.data[id]
	if !ok {
		return skill.Skill{}, skill.ErrNotFound
	}
	s.Name = req.Name
	s.Description = req.Description
	s.Category = req.Category
	m.data[id] = s
	return s, nil
}

func (m *mockSkillRepo) Delete(_ context.Context, id uuid.UUID) error {
	s, ok := m.data[id]
	if !ok {
		return skill.ErrNotFound
	}
	delete(m.slugs, s.Slug)
	delete(m.data, id)
	return nil
}

func (m *mockSkillRepo) SlugExists(_ context.Context, slug string) (bool, error) {
	return m.slugs[slug], nil
}

func (m *mockSkillRepo) ListByAgentID(_ context.Context, _ uuid.UUID) ([]skill.Skill, error) {
	out := make([]skill.Skill, 0, len(m.data))
	for _, s := range m.data {
		out = append(out, s)
	}
	return out, nil
}

func TestSkillService_Create_AutoSlug(t *testing.T) {
	svc := skill.NewService(newMockRepo())
	s, err := svc.Create(context.Background(), skill.CreateRequest{
		Name:     "Send Email",
		Category: "communication",
	})
	require.NoError(t, err)
	assert.Equal(t, "send-email", s.Slug)
	assert.NotEqual(t, uuid.Nil, s.ID)
}

func TestSkillService_Create_SlugKebabCase(t *testing.T) {
	svc := skill.NewService(newMockRepo())
	s, err := svc.Create(context.Background(), skill.CreateRequest{
		Name:     "Document Search",
		Category: "rag",
	})
	require.NoError(t, err)
	assert.Equal(t, "document-search", s.Slug)
}

func TestSkillService_Create_InputSchemaArrayConversion(t *testing.T) {
	svc := skill.NewService(newMockRepo())
	s, err := svc.Create(context.Background(), skill.CreateRequest{
		Name:        "Search Tool",
		Category:    "rag",
		InputSchema: json.RawMessage(`["query","topK"]`),
	})
	require.NoError(t, err)

	// Verify schema was converted to JSON Schema object
	resp := s.InputSchema
	schemaBytes, err := json.Marshal(resp)
	require.NoError(t, err)
	var schema map[string]any
	require.NoError(t, json.Unmarshal(schemaBytes, &schema))
	assert.Equal(t, "object", schema["type"])
	props, ok := schema["properties"].(map[string]any)
	require.True(t, ok)
	assert.Contains(t, props, "query")
	assert.Contains(t, props, "topK")
}

func TestSkillService_Create_InputSchemaObjectPassthrough(t *testing.T) {
	svc := skill.NewService(newMockRepo())
	objectSchema := json.RawMessage(`{"type":"object","properties":{"q":{"type":"string"}}}`)
	s, err := svc.Create(context.Background(), skill.CreateRequest{
		Name:        "Search V2",
		Category:    "rag",
		InputSchema: objectSchema,
	})
	require.NoError(t, err)
	schemaBytes, err := json.Marshal(s.InputSchema)
	require.NoError(t, err)
	var schema map[string]any
	require.NoError(t, json.Unmarshal(schemaBytes, &schema))
	assert.Equal(t, "object", schema["type"])
	assert.Contains(t, schema["properties"], "q")
}

func TestSkillService_Create_CustomSlug(t *testing.T) {
	svc := skill.NewService(newMockRepo())
	s, err := svc.Create(context.Background(), skill.CreateRequest{
		Name:     "Document Search",
		Slug:     "doc-search",
		Category: "rag",
	})
	require.NoError(t, err)
	assert.Equal(t, "doc-search", s.Slug)
}

func TestSkillService_GetByID_NotFound(t *testing.T) {
	svc := skill.NewService(newMockRepo())
	_, err := svc.GetByID(context.Background(), uuid.New())
	require.ErrorIs(t, err, skill.ErrNotFound)
}

func TestSkillService_Update_Success(t *testing.T) {
	svc := skill.NewService(newMockRepo())
	created, err := svc.Create(context.Background(), skill.CreateRequest{Name: "Old", Category: "misc"})
	require.NoError(t, err)
	updated, err := svc.Update(context.Background(), created.ID, skill.UpdateRequest{Name: "New", Category: "misc"})
	require.NoError(t, err)
	assert.Equal(t, "New", updated.Name)
}

func TestSkillService_Delete_Success(t *testing.T) {
	svc := skill.NewService(newMockRepo())
	created, err := svc.Create(context.Background(), skill.CreateRequest{Name: "Temp", Category: "misc"})
	require.NoError(t, err)
	err = svc.Delete(context.Background(), created.ID)
	require.NoError(t, err)
}

func TestSkillService_List(t *testing.T) {
	svc := skill.NewService(newMockRepo())
	for _, name := range []string{"Skill A", "Skill B", "Skill C"} {
		_, err := svc.Create(context.Background(), skill.CreateRequest{Name: name, Category: "misc"})
		require.NoError(t, err)
	}
	page, err := svc.List(context.Background(), nil, pagination.PageRequest{Page: 0, Size: 20})
	require.NoError(t, err)
	assert.Equal(t, int64(3), page.TotalElements)
}
