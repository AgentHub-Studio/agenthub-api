package llmpreset_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/llmpreset"
	"github.com/AgentHub-Studio/agenthub-api/internal/pagination"
)

type mockLLMRepo struct {
	data      map[uuid.UUID]llmpreset.LLMPreset
	defaultID uuid.UUID
}

func newMockRepo() *mockLLMRepo {
	return &mockLLMRepo{data: make(map[uuid.UUID]llmpreset.LLMPreset)}
}

func (m *mockLLMRepo) FindAll(_ context.Context, _ pagination.PageRequest) ([]llmpreset.LLMPreset, int64, error) {
	out := make([]llmpreset.LLMPreset, 0, len(m.data))
	for _, p := range m.data {
		out = append(out, p)
	}
	return out, int64(len(out)), nil
}

func (m *mockLLMRepo) FindByID(_ context.Context, id uuid.UUID) (llmpreset.LLMPreset, error) {
	p, ok := m.data[id]
	if !ok {
		return llmpreset.LLMPreset{}, llmpreset.ErrNotFound
	}
	return p, nil
}

func (m *mockLLMRepo) FindByProvider(_ context.Context, provider string, _ pagination.PageRequest) ([]llmpreset.LLMPreset, int64, error) {
	var out []llmpreset.LLMPreset
	for _, p := range m.data {
		if p.Provider == provider {
			out = append(out, p)
		}
	}
	return out, int64(len(out)), nil
}

func (m *mockLLMRepo) Create(_ context.Context, p llmpreset.LLMPreset) (llmpreset.LLMPreset, error) {
	p.ID = uuid.New()
	if p.IsDefault {
		m.defaultID = p.ID
	}
	m.data[p.ID] = p
	return p, nil
}

func (m *mockLLMRepo) Update(_ context.Context, p llmpreset.LLMPreset) (llmpreset.LLMPreset, error) {
	if _, ok := m.data[p.ID]; !ok {
		return llmpreset.LLMPreset{}, llmpreset.ErrNotFound
	}
	m.data[p.ID] = p
	return p, nil
}

func (m *mockLLMRepo) Delete(_ context.Context, id uuid.UUID) error {
	if _, ok := m.data[id]; !ok {
		return llmpreset.ErrNotFound
	}
	delete(m.data, id)
	return nil
}

func (m *mockLLMRepo) SetDefault(_ context.Context, id uuid.UUID) error {
	if _, ok := m.data[id]; !ok {
		return llmpreset.ErrNotFound
	}
	for pid, p := range m.data {
		p.IsDefault = pid == id
		m.data[pid] = p
	}
	m.defaultID = id
	return nil
}

func TestLLMPresetService_Create_Success(t *testing.T) {
	svc := llmpreset.NewService(newMockRepo())
	p, err := svc.Create(context.Background(), llmpreset.CreateLLMPresetRequest{
		Name:     "GPT-4o",
		Provider: "openai",
		Model:    "gpt-4o",
	})
	require.NoError(t, err)
	assert.Equal(t, "GPT-4o", p.Name)
	assert.NotEqual(t, uuid.Nil, p.ID)
}

func TestLLMPresetService_GetByID_NotFound(t *testing.T) {
	svc := llmpreset.NewService(newMockRepo())
	_, err := svc.Get(context.Background(), uuid.New())
	require.ErrorIs(t, err, llmpreset.ErrNotFound)
}

func TestLLMPresetService_Delete_NotFound(t *testing.T) {
	svc := llmpreset.NewService(newMockRepo())
	err := svc.Delete(context.Background(), uuid.New())
	require.ErrorIs(t, err, llmpreset.ErrNotFound)
}

func TestLLMPresetService_SetDefault(t *testing.T) {
	svc := llmpreset.NewService(newMockRepo())
	p, err := svc.Create(context.Background(), llmpreset.CreateLLMPresetRequest{
		Name: "Claude", Provider: "anthropic", Model: "claude-sonnet-4-6",
	})
	require.NoError(t, err)
	err = svc.SetDefault(context.Background(), p.ID)
	require.NoError(t, err)
}

func TestLLMPresetService_List(t *testing.T) {
	svc := llmpreset.NewService(newMockRepo())
	for _, name := range []string{"GPT-4o", "Claude", "Llama"} {
		_, err := svc.Create(context.Background(), llmpreset.CreateLLMPresetRequest{
			Name: name, Provider: "openai", Model: "gpt-4o",
		})
		require.NoError(t, err)
	}
	page, err := svc.List(context.Background(), pagination.PageRequest{Page: 0, Size: 20})
	require.NoError(t, err)
	assert.Equal(t, int64(3), page.TotalElements)
	assert.Len(t, page.Content, 3)
}

func TestLLMPresetService_ListByProvider(t *testing.T) {
	svc := llmpreset.NewService(newMockRepo())
	_, _ = svc.Create(context.Background(), llmpreset.CreateLLMPresetRequest{
		Name: "GPT-4o", Provider: "openai", Model: "gpt-4o",
	})
	_, _ = svc.Create(context.Background(), llmpreset.CreateLLMPresetRequest{
		Name: "Claude", Provider: "anthropic", Model: "claude-sonnet-4-6",
	})

	page, err := svc.ListByProvider(context.Background(), "openai", pagination.PageRequest{Page: 0, Size: 20})
	require.NoError(t, err)
	assert.Equal(t, int64(1), page.TotalElements)
	assert.Len(t, page.Content, 1)
	assert.Equal(t, "openai", page.Content[0].Provider)
}

func TestLLMPresetService_ListByProvider_EmptyProvider_ReturnsError(t *testing.T) {
	svc := llmpreset.NewService(newMockRepo())
	_, err := svc.ListByProvider(context.Background(), "", pagination.PageRequest{Page: 0, Size: 20})
	require.Error(t, err)
}
