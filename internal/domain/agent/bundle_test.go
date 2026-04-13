package agent_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/agent"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/skill"
	"github.com/AgentHub-Studio/agenthub-api/internal/pagination"
)

// --- minimal stubs ---

// stubSkillCreator implements agent.SkillCreator.
type stubSkillCreator struct {
	created []skill.Response
}

func (s *stubSkillCreator) Create(_ context.Context, req skill.CreateRequest) (skill.Response, error) {
	r := skill.Response{
		ID:           uuid.New(),
		Name:         req.Name,
		Slug:         req.Slug,
		Description:  req.Description,
		Instructions: req.Instructions,
		Category:     req.Category,
		AllowedTools: req.AllowedTools,
		WhenToUse:    req.WhenToUse,
	}
	s.created = append(s.created, r)
	return r, nil
}

// stubSkillRepo is a minimal skill.SkillRepository for bundle tests.
type stubSkillRepo struct {
	skills map[uuid.UUID]skill.Skill
}

func newStubSkillRepo(skills ...skill.Skill) *stubSkillRepo {
	r := &stubSkillRepo{skills: make(map[uuid.UUID]skill.Skill)}
	for _, s := range skills {
		r.skills[s.ID] = s
	}
	return r
}

func (r *stubSkillRepo) List(_ context.Context, _ *string, _ pagination.PageRequest) ([]skill.Skill, int64, error) {
	return nil, 0, nil
}
func (r *stubSkillRepo) Create(_ context.Context, s skill.Skill) (skill.Skill, error) { return s, nil }
func (r *stubSkillRepo) GetByID(_ context.Context, id uuid.UUID) (skill.Skill, error) {
	return r.skills[id], nil
}
func (r *stubSkillRepo) Update(_ context.Context, _ uuid.UUID, _ skill.UpdateRequest) (skill.Skill, error) {
	return skill.Skill{}, nil
}
func (r *stubSkillRepo) Delete(_ context.Context, _ uuid.UUID) error                 { return nil }
func (r *stubSkillRepo) SlugExists(_ context.Context, _ string) (bool, error)        { return false, nil }
func (r *stubSkillRepo) ListByAgentID(_ context.Context, _ uuid.UUID) ([]skill.Skill, error) {
	return nil, nil
}
func (r *stubSkillRepo) ListByIDs(_ context.Context, ids []uuid.UUID) ([]skill.Skill, error) {
	var out []skill.Skill
	for _, id := range ids {
		if s, ok := r.skills[id]; ok {
			out = append(out, s)
		}
	}
	return out, nil
}
func (r *stubSkillRepo) CountAgentBindings(_ context.Context, _ uuid.UUID) (int64, error) {
	return 0, nil
}
func (r *stubSkillRepo) CountActiveToolsForSkills(_ context.Context, _ []uuid.UUID) (int, error) {
	return 0, nil
}

var _ skill.SkillRepository = (*stubSkillRepo)(nil)

// stubBindingRepo implements agent.BindingRepository.
type stubBindingRepo struct {
	skillIDs map[uuid.UUID][]uuid.UUID // agentID → skill IDs
	synced   []uuid.UUID               // last synced skill IDs
}

func newStubBindingRepo() *stubBindingRepo {
	return &stubBindingRepo{skillIDs: make(map[uuid.UUID][]uuid.UUID)}
}

func (r *stubBindingRepo) ListSkillIDs(_ context.Context, agentID uuid.UUID) ([]uuid.UUID, error) {
	return r.skillIDs[agentID], nil
}
func (r *stubBindingRepo) SyncSkills(_ context.Context, agentID uuid.UUID, ids []uuid.UUID) error {
	r.skillIDs[agentID] = ids
	r.synced = ids
	return nil
}
func (r *stubBindingRepo) ListKnowledgeBaseIDs(_ context.Context, _ uuid.UUID) ([]uuid.UUID, error) {
	return nil, nil
}
func (r *stubBindingRepo) SyncKnowledgeBases(_ context.Context, _ uuid.UUID, _ []uuid.UUID) error {
	return nil
}
func (r *stubBindingRepo) ListMCPServerIDs(_ context.Context, _ uuid.UUID) ([]uuid.UUID, error) {
	return nil, nil
}
func (r *stubBindingRepo) ListMCPServerNames(_ context.Context, _ uuid.UUID) ([]string, error) {
	return nil, nil
}
func (r *stubBindingRepo) SyncMCPServers(_ context.Context, _ uuid.UUID, _ []uuid.UUID) error {
	return nil
}

func (r *stubBindingRepo) GetSkillTokenBudgets(_ context.Context, _ uuid.UUID) (map[uuid.UUID]*int, error) {
	return map[uuid.UUID]*int{}, nil
}

var _ agent.BindingRepository = (*stubBindingRepo)(nil)

// --- tests ---

func TestExporter_Export_NoSkills(t *testing.T) {
	svc := newMockSvc()
	binding := newStubBindingRepo()
	skillRepo := newStubSkillRepo()
	exporter := agent.NewExporter(svc, skillRepo, binding)

	agentID := uuid.New()
	sp := "You are helpful."
	svc.agents[agentID] = agent.AgentResponse{
		ID:           agentID,
		Name:         "Test Agent",
		Slug:         "test-agent",
		Description:  "A test agent",
		SystemPrompt: &sp,
		ModelConfig:  json.RawMessage(`{"provider":"openai"}`),
	}

	bundle, err := exporter.Export(context.Background(), agentID)
	require.NoError(t, err)
	assert.Equal(t, "1", bundle.FormatVersion)
	assert.Equal(t, "Test Agent", bundle.Agent.Name)
	assert.Empty(t, bundle.Skills)
}

func TestExporter_Export_WithSkills(t *testing.T) {
	svc := newMockSvc()
	agentID := uuid.New()
	sp := "You are an assistant."
	svc.agents[agentID] = agent.AgentResponse{
		ID:           agentID,
		Name:         "Skill Agent",
		SystemPrompt: &sp,
	}

	skillID := uuid.New()
	skillRepo := newStubSkillRepo(skill.Skill{
		ID:           skillID,
		Name:         "Search",
		Slug:         "search",
		Instructions: "Search the web.",
	})

	binding := newStubBindingRepo()
	binding.skillIDs[agentID] = []uuid.UUID{skillID}

	exporter := agent.NewExporter(svc, skillRepo, binding)
	bundle, err := exporter.Export(context.Background(), agentID)
	require.NoError(t, err)
	require.Len(t, bundle.Skills, 1)
	assert.Equal(t, "search", bundle.Skills[0].Slug)
	assert.Equal(t, skillID, bundle.Skills[0].OriginalID)
}

func TestExporter_Export_AgentNotFound(t *testing.T) {
	svc := newMockSvc()
	exporter := agent.NewExporter(svc, newStubSkillRepo(), newStubBindingRepo())

	_, err := exporter.Export(context.Background(), uuid.New())
	require.Error(t, err)
}

func TestImporter_Import_CreatesAgentAndSkills(t *testing.T) {
	svc := newMockSvc()
	skillCreator := &stubSkillCreator{}
	binding := newStubBindingRepo()
	importer := agent.NewImporter(svc, skillCreator, binding)

	sp := "Be concise."
	bundle := agent.AgentBundle{
		FormatVersion: "1",
		Agent: agent.BundleAgentDef{
			Name:         "Imported Agent",
			Slug:         "imported-agent",
			SystemPrompt: &sp,
		},
		Skills: []agent.BundleSkill{
			{OriginalID: uuid.New(), Name: "Email", Slug: "email", Instructions: "Send emails.", Category: "communication"},
		},
	}

	result, err := importer.Import(context.Background(), bundle)
	require.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, result.AgentID)
	assert.Equal(t, "Imported Agent", result.AgentName)
	assert.Equal(t, 1, result.SkillsImported)

	// Skills should be bound.
	assert.Len(t, binding.synced, 1)
}

func TestImporter_Import_EmptyBundle_AgentOnly(t *testing.T) {
	svc := newMockSvc()
	skillCreator := &stubSkillCreator{}
	binding := newStubBindingRepo()
	importer := agent.NewImporter(svc, skillCreator, binding)

	bundle := agent.AgentBundle{
		FormatVersion: "1",
		Agent:         agent.BundleAgentDef{Name: "Minimal", Slug: "minimal"},
		Skills:        nil,
	}

	result, err := importer.Import(context.Background(), bundle)
	require.NoError(t, err)
	assert.Equal(t, "Minimal", result.AgentName)
	assert.Equal(t, 0, result.SkillsImported)
	assert.Empty(t, binding.synced)
}
