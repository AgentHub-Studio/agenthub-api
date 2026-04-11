package agenttemplate_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/agent"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/agenttemplate"
)

// --- in-memory repository stub ---

type memRepo struct {
	data map[string]agenttemplate.AgentTemplate
}

func newMemRepo() *memRepo {
	return &memRepo{data: make(map[string]agenttemplate.AgentTemplate)}
}

func (r *memRepo) ListAll(_ context.Context) ([]agenttemplate.AgentTemplate, error) {
	out := make([]agenttemplate.AgentTemplate, 0, len(r.data))
	for _, t := range r.data {
		out = append(out, t)
	}
	return out, nil
}

func (r *memRepo) ListByCategory(_ context.Context, cat string) ([]agenttemplate.AgentTemplate, error) {
	var out []agenttemplate.AgentTemplate
	for _, t := range r.data {
		if t.Category == cat {
			out = append(out, t)
		}
	}
	return out, nil
}

func (r *memRepo) GetBySlug(_ context.Context, slug string) (agenttemplate.AgentTemplate, error) {
	t, ok := r.data[slug]
	if !ok {
		return agenttemplate.AgentTemplate{}, agenttemplate.ErrNotFound
	}
	return t, nil
}

func (r *memRepo) Create(_ context.Context, t agenttemplate.AgentTemplate) (agenttemplate.AgentTemplate, error) {
	if _, exists := r.data[t.Slug]; exists {
		return agenttemplate.AgentTemplate{}, agenttemplate.ErrSlugConflict
	}
	t.ID = uuid.New()
	r.data[t.Slug] = t
	return t, nil
}

// --- agent creator stub ---

type stubAgentCreator struct {
	created []agent.CreateAgentRequest
}

func (c *stubAgentCreator) Create(_ context.Context, req agent.CreateAgentRequest) (agent.AgentResponse, error) {
	c.created = append(c.created, req)
	return agent.AgentResponse{
		ID:   uuid.New(),
		Name: req.Name,
		Slug: "created-agent-slug",
	}, nil
}

// --- helpers ---

func seedTemplate(repo *memRepo, slug, category string) agenttemplate.AgentTemplate {
	def := agenttemplate.Definition{
		SystemPrompt: "You are a test agent.",
		Skills:       []string{"document-search"},
	}
	defJSON, _ := json.Marshal(def)
	t := agenttemplate.AgentTemplate{
		ID:             uuid.New(),
		Name:           "Test Template",
		Slug:           slug,
		Description:    "A test template",
		Category:       category,
		IsBuiltin:      true,
		DefinitionJSON: defJSON,
	}
	repo.data[slug] = t
	return t
}

// --- tests ---

func TestService_ListAll_ReturnsAllTemplates(t *testing.T) {
	repo := newMemRepo()
	seedTemplate(repo, "rag-assistant", "rag")
	seedTemplate(repo, "sql-analyst", "data")

	svc := agenttemplate.NewService(repo)
	templates, err := svc.ListAll(context.Background(), "")

	require.NoError(t, err)
	assert.Len(t, templates, 2)
}

func TestService_ListAll_FiltersByCategory(t *testing.T) {
	repo := newMemRepo()
	seedTemplate(repo, "rag-assistant", "rag")
	seedTemplate(repo, "sql-analyst", "data")

	svc := agenttemplate.NewService(repo)
	templates, err := svc.ListAll(context.Background(), "rag")

	require.NoError(t, err)
	assert.Len(t, templates, 1)
	assert.Equal(t, "rag-assistant", templates[0].Slug)
}

func TestService_GetBySlug_Found(t *testing.T) {
	repo := newMemRepo()
	seedTemplate(repo, "rag-assistant", "rag")

	svc := agenttemplate.NewService(repo)
	resp, err := svc.GetBySlug(context.Background(), "rag-assistant")

	require.NoError(t, err)
	assert.Equal(t, "rag-assistant", resp.Slug)
}

func TestService_GetBySlug_NotFound(t *testing.T) {
	svc := agenttemplate.NewService(newMemRepo())
	_, err := svc.GetBySlug(context.Background(), "nonexistent")

	assert.ErrorIs(t, err, agenttemplate.ErrNotFound)
}

func TestService_Create_TenantTemplate(t *testing.T) {
	repo := newMemRepo()
	svc := agenttemplate.NewService(repo)

	req := agenttemplate.CreateTemplateRequest{
		Name:        "My Custom Template",
		Description: "Custom",
		Category:    "custom",
		Definition:  json.RawMessage(`{"systemPrompt":"hello","skills":[]}`),
	}
	resp, err := svc.Create(context.Background(), req)

	require.NoError(t, err)
	assert.Equal(t, "my-custom-template", resp.Slug)
	assert.False(t, resp.IsBuiltin)
}

func TestService_Create_SlugConflict(t *testing.T) {
	repo := newMemRepo()
	seedTemplate(repo, "my-template", "rag")
	svc := agenttemplate.NewService(repo)

	req := agenttemplate.CreateTemplateRequest{
		Name: "My Template",
		Slug: "my-template",
	}
	_, err := svc.Create(context.Background(), req)

	assert.ErrorIs(t, err, agenttemplate.ErrSlugConflict)
}

func TestService_Instantiate_CreatesAgent(t *testing.T) {
	repo := newMemRepo()
	seedTemplate(repo, "rag-assistant", "rag")
	creator := &stubAgentCreator{}
	svc := agenttemplate.NewService(repo).WithAgentCreator(creator)

	resp, err := svc.Instantiate(context.Background(), "rag-assistant", agenttemplate.InstantiateRequest{})

	require.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, resp.AgentID)
	assert.Equal(t, "Test Template", resp.Name)

	require.Len(t, creator.created, 1)
	assert.Equal(t, "Test Template", creator.created[0].Name)
	require.NotNil(t, creator.created[0].SystemPrompt)
	assert.Equal(t, "You are a test agent.", *creator.created[0].SystemPrompt)
}

func TestService_Instantiate_NameOverride(t *testing.T) {
	repo := newMemRepo()
	seedTemplate(repo, "rag-assistant", "rag")
	creator := &stubAgentCreator{}
	svc := agenttemplate.NewService(repo).WithAgentCreator(creator)

	resp, err := svc.Instantiate(context.Background(), "rag-assistant", agenttemplate.InstantiateRequest{
		Name: "My Overridden Agent",
	})

	require.NoError(t, err)
	assert.Equal(t, "My Overridden Agent", resp.Name)
	assert.Equal(t, "My Overridden Agent", creator.created[0].Name)
}

func TestService_Instantiate_TemplateNotFound(t *testing.T) {
	creator := &stubAgentCreator{}
	svc := agenttemplate.NewService(newMemRepo()).WithAgentCreator(creator)

	_, err := svc.Instantiate(context.Background(), "nonexistent", agenttemplate.InstantiateRequest{})

	assert.ErrorIs(t, err, agenttemplate.ErrNotFound)
}
