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

func (m *mockLLMRepo) FindAll(_ context.Context, tenantID string, _ pagination.PageRequest) ([]llmpreset.LLMPreset, int64, error) {
	var out []llmpreset.LLMPreset
	for _, p := range m.data {
		if p.TenantID == tenantID {
			out = append(out, p)
		}
	}
	return out, int64(len(out)), nil
}

func (m *mockLLMRepo) FindByID(_ context.Context, tenantID string, id uuid.UUID) (llmpreset.LLMPreset, error) {
	p, ok := m.data[id]
	if !ok || p.TenantID != tenantID {
		return llmpreset.LLMPreset{}, llmpreset.ErrNotFound
	}
	return p, nil
}

func (m *mockLLMRepo) FindByProvider(_ context.Context, tenantID, provider string, _ pagination.PageRequest) ([]llmpreset.LLMPreset, int64, error) {
	var out []llmpreset.LLMPreset
	for _, p := range m.data {
		if p.TenantID == tenantID && p.Provider == provider {
			out = append(out, p)
		}
	}
	return out, int64(len(out)), nil
}

func (m *mockLLMRepo) ExistsByName(_ context.Context, tenantID, name string) (bool, error) {
	for _, p := range m.data {
		if p.TenantID == tenantID && p.Name == name {
			return true, nil
		}
	}
	return false, nil
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
	existing, ok := m.data[p.ID]
	if !ok || existing.TenantID != p.TenantID {
		return llmpreset.LLMPreset{}, llmpreset.ErrNotFound
	}
	m.data[p.ID] = p
	return p, nil
}

func (m *mockLLMRepo) Delete(_ context.Context, tenantID string, id uuid.UUID) error {
	p, ok := m.data[id]
	if !ok || p.TenantID != tenantID {
		return llmpreset.ErrNotFound
	}
	delete(m.data, id)
	return nil
}

func (m *mockLLMRepo) SetDefault(_ context.Context, tenantID string, id uuid.UUID) error {
	p, ok := m.data[id]
	if !ok || p.TenantID != tenantID {
		return llmpreset.ErrNotFound
	}
	for pid, preset := range m.data {
		if preset.TenantID == tenantID {
			preset.IsDefault = pid == id
			m.data[pid] = preset
		}
	}
	m.defaultID = id
	return nil
}

const tenantA = "tenant-a"
const tenantB = "tenant-b"

func TestLLMPresetService_Create_Success(t *testing.T) {
	svc := llmpreset.NewService(newMockRepo())
	p, err := svc.Create(context.Background(), tenantA, llmpreset.CreateLLMPresetRequest{
		Name:     "GPT-4o",
		Provider: "openai",
		Model:    "gpt-4o",
	})
	require.NoError(t, err)
	assert.Equal(t, "GPT-4o", p.Name)
	assert.Equal(t, tenantA, p.TenantID)
	assert.NotEqual(t, uuid.Nil, p.ID)
}

func TestLLMPresetService_GetByID_NotFound(t *testing.T) {
	svc := llmpreset.NewService(newMockRepo())
	_, err := svc.Get(context.Background(), tenantA, uuid.New())
	require.ErrorIs(t, err, llmpreset.ErrNotFound)
}

func TestLLMPresetService_Delete_NotFound(t *testing.T) {
	svc := llmpreset.NewService(newMockRepo())
	err := svc.Delete(context.Background(), tenantA, uuid.New())
	require.ErrorIs(t, err, llmpreset.ErrNotFound)
}

func TestLLMPresetService_SetDefault(t *testing.T) {
	svc := llmpreset.NewService(newMockRepo())
	p, err := svc.Create(context.Background(), tenantA, llmpreset.CreateLLMPresetRequest{
		Name: "Claude", Provider: "anthropic", Model: "claude-sonnet-4-6",
	})
	require.NoError(t, err)
	err = svc.SetDefault(context.Background(), tenantA, p.ID)
	require.NoError(t, err)
}

func TestLLMPresetService_List(t *testing.T) {
	svc := llmpreset.NewService(newMockRepo())
	for _, name := range []string{"GPT-4o", "Claude", "Llama"} {
		_, err := svc.Create(context.Background(), tenantA, llmpreset.CreateLLMPresetRequest{
			Name: name, Provider: "openai", Model: "gpt-4o",
		})
		require.NoError(t, err)
	}
	page, err := svc.List(context.Background(), tenantA, pagination.PageRequest{Page: 0, Size: 20})
	require.NoError(t, err)
	assert.Equal(t, int64(3), page.TotalElements)
	assert.Len(t, page.Content, 3)
}

func TestLLMPresetService_ListByProvider(t *testing.T) {
	svc := llmpreset.NewService(newMockRepo())
	_, _ = svc.Create(context.Background(), tenantA, llmpreset.CreateLLMPresetRequest{
		Name: "GPT-4o", Provider: "openai", Model: "gpt-4o",
	})
	_, _ = svc.Create(context.Background(), tenantA, llmpreset.CreateLLMPresetRequest{
		Name: "Claude", Provider: "anthropic", Model: "claude-sonnet-4-6",
	})

	page, err := svc.ListByProvider(context.Background(), tenantA, "openai", pagination.PageRequest{Page: 0, Size: 20})
	require.NoError(t, err)
	assert.Equal(t, int64(1), page.TotalElements)
	assert.Len(t, page.Content, 1)
	assert.Equal(t, "openai", page.Content[0].Provider)
}

func TestLLMPresetService_Create_DuplicateName_ReturnsError(t *testing.T) {
	svc := llmpreset.NewService(newMockRepo())
	req := llmpreset.CreateLLMPresetRequest{Name: "GPT-4o", Provider: "openai", Model: "gpt-4o"}
	_, err := svc.Create(context.Background(), req)
	require.NoError(t, err)
	_, err = svc.Create(context.Background(), req)
	require.ErrorIs(t, err, llmpreset.ErrDuplicateName)
}

func TestLLMPresetService_Create_ValidationErrors(t *testing.T) {
	svc := llmpreset.NewService(newMockRepo())

	_, err := svc.Create(context.Background(), llmpreset.CreateLLMPresetRequest{Provider: "openai", Model: "gpt-4o"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "name")

	_, err = svc.Create(context.Background(), llmpreset.CreateLLMPresetRequest{Name: "X", Model: "gpt-4o"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "provider")

	_, err = svc.Create(context.Background(), llmpreset.CreateLLMPresetRequest{Name: "Y", Provider: "openai"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "model")
}

func TestLLMPresetService_Update_PartialFields(t *testing.T) {
	svc := llmpreset.NewService(newMockRepo())
	created, err := svc.Create(context.Background(), llmpreset.CreateLLMPresetRequest{
		Name: "Original", Provider: "openai", Model: "gpt-4o",
	})
	require.NoError(t, err)

	newName := "Updated"
	updated, err := svc.Update(context.Background(), created.ID, llmpreset.UpdateLLMPresetRequest{Name: &newName})
	require.NoError(t, err)
	assert.Equal(t, "Updated", updated.Name)
	assert.Equal(t, "openai", updated.Provider, "provider should remain unchanged")
}

func TestLLMPresetService_ListByProvider_EmptyProvider_ReturnsError(t *testing.T) {
	svc := llmpreset.NewService(newMockRepo())
	_, err := svc.ListByProvider(context.Background(), tenantA, "", pagination.PageRequest{Page: 0, Size: 20})
	require.Error(t, err)
}

// --- Tenant isolation tests ---

func TestLLMPresetService_TenantIsolation_ListOnlySeesTenantPresets(t *testing.T) {
	svc := llmpreset.NewService(newMockRepo())
	_, _ = svc.Create(context.Background(), tenantA, llmpreset.CreateLLMPresetRequest{
		Name: "GPT-4o", Provider: "openai", Model: "gpt-4o",
	})
	_, _ = svc.Create(context.Background(), tenantB, llmpreset.CreateLLMPresetRequest{
		Name: "Claude", Provider: "anthropic", Model: "claude-sonnet-4-6",
	})

	pageA, err := svc.List(context.Background(), tenantA, pagination.PageRequest{Page: 0, Size: 20})
	require.NoError(t, err)
	assert.Equal(t, int64(1), pageA.TotalElements, "tenantA should only see its own presets")
	assert.Equal(t, tenantA, pageA.Content[0].TenantID)

	pageB, err := svc.List(context.Background(), tenantB, pagination.PageRequest{Page: 0, Size: 20})
	require.NoError(t, err)
	assert.Equal(t, int64(1), pageB.TotalElements, "tenantB should only see its own presets")
}

func TestLLMPresetService_TenantIsolation_GetByIDCrossTenantsReturnsNotFound(t *testing.T) {
	svc := llmpreset.NewService(newMockRepo())
	p, err := svc.Create(context.Background(), tenantA, llmpreset.CreateLLMPresetRequest{
		Name: "GPT-4o", Provider: "openai", Model: "gpt-4o",
	})
	require.NoError(t, err)

	// tenantB tries to get tenantA's preset
	_, err = svc.Get(context.Background(), tenantB, p.ID)
	require.ErrorIs(t, err, llmpreset.ErrNotFound)
}

// --- Name uniqueness per tenant ---

func TestLLMPresetService_Create_DuplicateName_SameTenant_ReturnsError(t *testing.T) {
	svc := llmpreset.NewService(newMockRepo())
	req := llmpreset.CreateLLMPresetRequest{Name: "GPT-4o", Provider: "openai", Model: "gpt-4o"}
	_, err := svc.Create(context.Background(), tenantA, req)
	require.NoError(t, err)

	// Same tenant, same name → error
	_, err = svc.Create(context.Background(), tenantA, req)
	require.ErrorIs(t, err, llmpreset.ErrDuplicateName)
}

func TestLLMPresetService_Create_DuplicateName_DifferentTenants_Allowed(t *testing.T) {
	svc := llmpreset.NewService(newMockRepo())
	req := llmpreset.CreateLLMPresetRequest{Name: "GPT-4o", Provider: "openai", Model: "gpt-4o"}
	_, err := svc.Create(context.Background(), tenantA, req)
	require.NoError(t, err)

	// Different tenant, same name → OK
	_, err = svc.Create(context.Background(), tenantB, req)
	require.NoError(t, err, "same name is allowed for different tenants")
}

func TestLLMPresetService_Create_ValidationErrors(t *testing.T) {
	svc := llmpreset.NewService(newMockRepo())

	_, err := svc.Create(context.Background(), tenantA, llmpreset.CreateLLMPresetRequest{Provider: "openai", Model: "gpt-4o"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "name")

	_, err = svc.Create(context.Background(), tenantA, llmpreset.CreateLLMPresetRequest{Name: "X", Model: "gpt-4o"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "provider")

	_, err = svc.Create(context.Background(), tenantA, llmpreset.CreateLLMPresetRequest{Name: "Y", Provider: "openai"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "model")
}

func TestLLMPresetService_Update_PartialFields(t *testing.T) {
	svc := llmpreset.NewService(newMockRepo())
	created, err := svc.Create(context.Background(), tenantA, llmpreset.CreateLLMPresetRequest{
		Name: "Original", Provider: "openai", Model: "gpt-4o",
	})
	require.NoError(t, err)

	newName := "Updated"
	updated, err := svc.Update(context.Background(), tenantA, created.ID, llmpreset.UpdateLLMPresetRequest{Name: &newName})
	require.NoError(t, err)
	assert.Equal(t, "Updated", updated.Name)
	assert.Equal(t, "openai", updated.Provider, "provider should remain unchanged")
}
