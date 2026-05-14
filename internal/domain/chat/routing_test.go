package chat

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTokenize(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want []string
	}{
		{"empty", "", nil},
		{"whitespace only", "   \t\n", nil},
		{"punctuation only", "!!! ... ???", nil},
		{"all short or stop tokens dropped", "a b cd of", nil},
		{"lowercases", "Hello WORLD Foo", []string{"hello", "world", "foo"}},
		{"drops sub-3-char tokens", "ab cde xy fgh", []string{"cde", "fgh"}},
		{"strips english stop words", "what can you search documents", []string{"search", "documents"}},
		{"keeps portuguese content words", "como criar relatorio", []string{"como", "criar", "relatorio"}},
		{"splits on non-letters", "foo-bar.baz_qux", []string{"foo", "bar", "baz", "qux"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tokenize(tt.in))
		})
	}
}

func TestSelectBestAgent(t *testing.T) {
	id0, id1, id2 := uuid.New(), uuid.New(), uuid.New()
	agents := []AgentRoutingInfo{
		{ID: id0, Name: "General Assistant", Description: "A helpful companion"},
		{ID: id1, Name: "Billing Specialist", Description: "Handles invoices and payments"},
		{ID: id2, Name: "Travel Planner", Description: "Books flights and hotels"},
	}

	t.Run("empty message returns first agent", func(t *testing.T) {
		assert.Equal(t, id0, selectBestAgent(agents, ""))
	})
	t.Run("matches on name", func(t *testing.T) {
		assert.Equal(t, id1, selectBestAgent(agents, "I have a billing matter"))
	})
	t.Run("matches on description", func(t *testing.T) {
		assert.Equal(t, id2, selectBestAgent(agents, "please book a hotel for my trip"))
	})
	t.Run("no keyword overlap returns first agent", func(t *testing.T) {
		assert.Equal(t, id0, selectBestAgent(agents, "xyzzy plugh quux"))
	})
	t.Run("tie breaks to first matching agent", func(t *testing.T) {
		tieAgents := []AgentRoutingInfo{
			{ID: id1, Name: "Reports", Description: "data"},
			{ID: id2, Name: "Reports", Description: "data"},
		}
		assert.Equal(t, id1, selectBestAgent(tieAgents, "reports data"))
	})
}

type fakeAgentRouter struct {
	agents []AgentRoutingInfo
	err    error
}

func (f fakeAgentRouter) FindAgentsForRouting(context.Context) ([]AgentRoutingInfo, error) {
	return f.agents, f.err
}

func TestRouteAgentID(t *testing.T) {
	id1, id2 := uuid.New(), uuid.New()

	t.Run("zero agents returns nil", func(t *testing.T) {
		got, err := routeAgentID(context.Background(), fakeAgentRouter{}, "hello")
		require.NoError(t, err)
		assert.Nil(t, got)
	})
	t.Run("single agent returned directly", func(t *testing.T) {
		repo := fakeAgentRouter{agents: []AgentRoutingInfo{{ID: id1, Name: "Solo"}}}
		got, err := routeAgentID(context.Background(), repo, "anything at all")
		require.NoError(t, err)
		require.NotNil(t, got)
		assert.Equal(t, id1, *got)
	})
	t.Run("multiple agents picks best by keyword", func(t *testing.T) {
		repo := fakeAgentRouter{agents: []AgentRoutingInfo{
			{ID: id1, Name: "General", Description: "helpful companion"},
			{ID: id2, Name: "Billing Specialist", Description: "handles invoices"},
		}}
		got, err := routeAgentID(context.Background(), repo, "billing question")
		require.NoError(t, err)
		require.NotNil(t, got)
		assert.Equal(t, id2, *got)
	})
	t.Run("repo error propagates", func(t *testing.T) {
		_, err := routeAgentID(context.Background(), fakeAgentRouter{err: errors.New("db down")}, "hello")
		require.Error(t, err)
	})
}
