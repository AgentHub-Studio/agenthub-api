package memory_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/memory"
)

type key struct {
	agentID uuid.UUID
	userID  string
	key     string
}

type mockMemoryRepo struct {
	data map[key]memory.AgentMemory
}

func newMockRepo() *mockMemoryRepo {
	return &mockMemoryRepo{data: make(map[key]memory.AgentMemory)}
}

func makeKey(agentID uuid.UUID, userID *string, k string) key {
	uid := ""
	if userID != nil {
		uid = *userID
	}
	return key{agentID: agentID, userID: uid, key: k}
}

func (m *mockMemoryRepo) ListByAgent(_ context.Context, agentID uuid.UUID, userID *string) ([]memory.AgentMemory, error) {
	var out []memory.AgentMemory
	for k, v := range m.data {
		if k.agentID != agentID {
			continue
		}
		if userID != nil && k.userID != *userID {
			continue
		}
		out = append(out, v)
	}
	return out, nil
}

func (m *mockMemoryRepo) Upsert(_ context.Context, mem memory.AgentMemory) (memory.AgentMemory, error) {
	if mem.ID == uuid.Nil {
		mem.ID = uuid.New()
	}
	uid := ""
	if mem.UserID != nil {
		uid = *mem.UserID
	}
	m.data[key{agentID: mem.AgentID, userID: uid, key: mem.Key}] = mem
	return mem, nil
}

func (m *mockMemoryRepo) GetByKey(_ context.Context, agentID uuid.UUID, userID *string, k string) (memory.AgentMemory, error) {
	mk := makeKey(agentID, userID, k)
	mem, ok := m.data[mk]
	if !ok {
		return memory.AgentMemory{}, memory.ErrNotFound
	}
	return mem, nil
}

func (m *mockMemoryRepo) DeleteByKey(_ context.Context, agentID uuid.UUID, userID *string, k string) error {
	mk := makeKey(agentID, userID, k)
	if _, ok := m.data[mk]; !ok {
		return memory.ErrNotFound
	}
	delete(m.data, mk)
	return nil
}

func (m *mockMemoryRepo) ClearByAgent(_ context.Context, agentID uuid.UUID) error {
	for k := range m.data {
		if k.agentID == agentID {
			delete(m.data, k)
		}
	}
	return nil
}

func TestMemoryService_Upsert_Success(t *testing.T) {
	svc := memory.NewService(newMockRepo())
	agentID := uuid.New()
	val, _ := json.Marshal("Paris")
	mem, err := svc.Upsert(context.Background(), agentID, "last_city", memory.UpsertMemoryRequest{Value: val})
	require.NoError(t, err)
	assert.Equal(t, "last_city", mem.Key)
	assert.NotEqual(t, uuid.Nil, mem.ID)
}

func TestMemoryService_GetByKey_NotFound(t *testing.T) {
	svc := memory.NewService(newMockRepo())
	_, err := svc.GetByKey(context.Background(), uuid.New(), nil, "missing")
	require.ErrorIs(t, err, memory.ErrNotFound)
}

func TestMemoryService_DeleteByKey_Success(t *testing.T) {
	svc := memory.NewService(newMockRepo())
	agentID := uuid.New()
	val, _ := json.Marshal(42)
	_, err := svc.Upsert(context.Background(), agentID, "counter", memory.UpsertMemoryRequest{Value: val})
	require.NoError(t, err)
	err = svc.DeleteByKey(context.Background(), agentID, nil, "counter")
	require.NoError(t, err)
}

func TestMemoryService_ClearByAgent(t *testing.T) {
	svc := memory.NewService(newMockRepo())
	agentID := uuid.New()
	for _, k := range []string{"a", "b", "c"} {
		v, _ := json.Marshal(k)
		_, err := svc.Upsert(context.Background(), agentID, k, memory.UpsertMemoryRequest{Value: v})
		require.NoError(t, err)
	}
	err := svc.ClearByAgent(context.Background(), agentID)
	require.NoError(t, err)
	items, err := svc.List(context.Background(), agentID, nil)
	require.NoError(t, err)
	assert.Empty(t, items)
}
