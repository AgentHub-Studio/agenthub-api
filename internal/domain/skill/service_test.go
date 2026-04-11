package skill_test

import (
	"context"
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

func (m *mockSkillRepo) ListByIDs(_ context.Context, ids []uuid.UUID) ([]skill.Skill, error) {
	var out []skill.Skill
	for _, id := range ids {
		if s, ok := m.data[id]; ok {
			out = append(out, s)
		}
	}
	return out, nil
}

func (m *mockSkillRepo) CountAgentBindings(_ context.Context, _ uuid.UUID) (int64, error) {
	// Default to 0 bindings so existing delete tests pass.
	return 0, nil
}

func TestSkillService_Create_AutoSlug(t *testing.T) {
	svc := skill.NewService(newMockRepo())
	s, err := svc.Create(context.Background(), skill.CreateRequest{
		Name:         "Send Email",
		Category:     "communication",
		Instructions: "Send an email to the specified address.",
	})
	require.NoError(t, err)
	assert.Equal(t, "send-email", s.Slug)
	assert.NotEqual(t, uuid.Nil, s.ID)
}

func TestSkillService_Create_SlugKebabCase(t *testing.T) {
	svc := skill.NewService(newMockRepo())
	s, err := svc.Create(context.Background(), skill.CreateRequest{
		Name:         "Document Search",
		Category:     "rag",
		Instructions: "Search documents for the given query.",
	})
	require.NoError(t, err)
	assert.Equal(t, "document-search", s.Slug)
}

func TestSkillService_Create_InputSchemaArrayConversion(t *testing.T) {
	t.Skip("skill.CreateRequest.InputSchema not yet implemented — see TR-01-TASK-10")
}

func TestSkillService_Create_InputSchemaObjectPassthrough(t *testing.T) {
	t.Skip("skill.CreateRequest.InputSchema not yet implemented — see TR-01-TASK-10")
}

func TestSkillService_Create_CustomSlug(t *testing.T) {
	svc := skill.NewService(newMockRepo())
	s, err := svc.Create(context.Background(), skill.CreateRequest{
		Name:         "Document Search",
		Slug:         "doc-search",
		Category:     "rag",
		Instructions: "Search documents.",
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
	created, err := svc.Create(context.Background(), skill.CreateRequest{Name: "Old", Category: "misc", Instructions: "Do something."})
	require.NoError(t, err)
	updated, err := svc.Update(context.Background(), created.ID, skill.UpdateRequest{Name: "New", Category: "misc"})
	require.NoError(t, err)
	assert.Equal(t, "New", updated.Name)
}

func TestSkillService_Delete_Success(t *testing.T) {
	svc := skill.NewService(newMockRepo())
	created, err := svc.Create(context.Background(), skill.CreateRequest{Name: "Temp", Category: "misc", Instructions: "Do something."})
	require.NoError(t, err)
	err = svc.Delete(context.Background(), created.ID)
	require.NoError(t, err)
}

func TestSkillService_List(t *testing.T) {
	svc := skill.NewService(newMockRepo())
	for _, name := range []string{"Skill A", "Skill B", "Skill C"} {
		_, err := svc.Create(context.Background(), skill.CreateRequest{Name: name, Category: "misc", Instructions: "Do something."})
		require.NoError(t, err)
	}
	page, err := svc.List(context.Background(), nil, pagination.PageRequest{Page: 0, Size: 20})
	require.NoError(t, err)
	assert.Equal(t, int64(3), page.TotalElements)
}

// --- ACT-F3-04: rejeitar skill inerte (DX-01-J) ---

func TestSkillService_Create_InertSkill_Rejected(t *testing.T) {
	svc := skill.NewService(newMockRepo())
	_, err := svc.Create(context.Background(), skill.CreateRequest{
		Name:     "Empty Skill",
		Category: "misc",
		// No instructions, no AllowedTools — completely inert
	})
	require.Error(t, err)
	assert.ErrorIs(t, err, skill.ErrSkillInert)
}

func TestSkillService_Create_WithInstructions_Accepted(t *testing.T) {
	svc := skill.NewService(newMockRepo())
	_, err := svc.Create(context.Background(), skill.CreateRequest{
		Name:         "Instructed Skill",
		Category:     "misc",
		Instructions: "Guide the LLM to do something useful.",
	})
	require.NoError(t, err)
}

func TestSkillService_Create_WithAllowedTools_NoInstructions_Accepted(t *testing.T) {
	svc := skill.NewService(newMockRepo())
	_, err := svc.Create(context.Background(), skill.CreateRequest{
		Name:         "Tool Skill",
		Category:     "misc",
		AllowedTools: []string{"http_get"},
	})
	require.NoError(t, err)
}

func TestSkillService_Create_WhitespaceInstructions_TreatedAsEmpty(t *testing.T) {
	svc := skill.NewService(newMockRepo())
	_, err := svc.Create(context.Background(), skill.CreateRequest{
		Name:         "Whitespace Skill",
		Category:     "misc",
		Instructions: "   \t\n  ", // whitespace only — DX-01-H
	})
	require.Error(t, err)
	assert.ErrorIs(t, err, skill.ErrSkillInert)
}

// --- ACT-F3-05: limite de tamanho de instructions (32K) ---

func TestSkillService_Create_InstructionsTooLong(t *testing.T) {
	svc := skill.NewService(newMockRepo())
	bigInstructions := string(make([]byte, 32001))
	_, err := svc.Create(context.Background(), skill.CreateRequest{
		Name:         "Big Skill",
		Category:     "misc",
		Instructions: bigInstructions,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "exceeds maximum length")
}

func TestSkillService_Create_InstructionsAtLimit_Accepted(t *testing.T) {
	svc := skill.NewService(newMockRepo())
	// Exactly 32000 non-whitespace chars should be accepted.
	instructions := string(make([]byte, 32000))
	for i := range instructions {
		instructions = instructions[:i] + "x" + instructions[i+1:]
		break
	}
	instructions = "x" + string(make([]byte, 31999))
	// Fill with printable chars
	buf := make([]byte, 32000)
	for i := range buf {
		buf[i] = 'x'
	}
	_, err := svc.Create(context.Background(), skill.CreateRequest{
		Name:         "At Limit Skill",
		Category:     "misc",
		Instructions: string(buf),
	})
	require.NoError(t, err)
}
