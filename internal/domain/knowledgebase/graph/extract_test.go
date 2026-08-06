package graph_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/knowledgebase/graph"
)

func TestExtractReportsToRelations(t *testing.T) {
	snapshot := graph.Extract("Alice reporta para Bob.\nBob reports to Carla.")

	require.Len(t, snapshot.Edges, 2)
	assert.Equal(t, "Alice", snapshot.Edges[0].Source)
	assert.Equal(t, "Bob", snapshot.Edges[0].Target)
	assert.Equal(t, "reports_to", snapshot.Edges[0].Relation)
	assert.Equal(t, "Bob", snapshot.Edges[1].Source)
	assert.Equal(t, "Carla", snapshot.Edges[1].Target)

	names := make([]string, 0, len(snapshot.Entities))
	for _, entity := range snapshot.Entities {
		names = append(names, entity.Name)
	}
	assert.ElementsMatch(t, []string{"Alice", "Bob", "Carla"}, names)
}

func TestExtractDeduplicatesRelationsAndIgnoresNoise(t *testing.T) {
	snapshot := graph.Extract(`
		Alice reports to Bob.
		Alice reports to Bob.
		alice reports to bob.
		some lowercase subject reports to Bob.
		Carol se reporta a Bob.
		Bob reporta para Diana.
	`)

	require.Len(t, snapshot.Edges, 3)
	assert.Equal(t, "Alice", snapshot.Edges[0].Source)
	assert.Equal(t, "Bob", snapshot.Edges[0].Target)
	assert.Equal(t, "Carol", snapshot.Edges[1].Source)
	assert.Equal(t, "Bob", snapshot.Edges[1].Target)
	assert.Equal(t, "Bob", snapshot.Edges[2].Source)
	assert.Equal(t, "Diana", snapshot.Edges[2].Target)
	assert.Len(t, snapshot.Entities, 4)
}

func TestExtractDoesNotPanicOnInvalidUTF8BeforeMixedCasePattern(t *testing.T) {
	snapshot := graph.Extract("\xbb reportA pArA 0")
	assert.Empty(t, snapshot.Entities)
	assert.Empty(t, snapshot.Edges)
}

func FuzzExtractMaintainsGraphInvariants(f *testing.F) {
	for _, seed := range []string{
		"Alice reports to Bob.",
		"Alice reporta para Bob.\nBob reports to Carla.",
		"Carol se reporta a Diana? Diana reports to Eve!",
		"noise without relation",
		"{{user.email}} reports to Tenant Admin.",
		"Álvaro de Souza reporta para Maria von Trapp.",
	} {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, text string) {
		if len(text) > 8192 {
			text = text[:8192]
		}

		snapshot := graph.Extract(text)

		entityKeys := make(map[string]struct{}, len(snapshot.Entities))
		for _, entity := range snapshot.Entities {
			if strings.TrimSpace(entity.Name) == "" {
				t.Fatalf("blank entity name in snapshot: %+v", snapshot)
			}
			if entity.Type != "person" {
				t.Fatalf("unexpected entity type %q for %q", entity.Type, entity.Name)
			}
			key := graph.CanonicalName(entity.Name)
			if key == "" {
				t.Fatalf("blank canonical entity key for %q", entity.Name)
			}
			if _, exists := entityKeys[key]; exists {
				t.Fatalf("duplicate entity %q in snapshot: %+v", entity.Name, snapshot)
			}
			entityKeys[key] = struct{}{}
		}

		edgeKeys := make(map[string]struct{}, len(snapshot.Edges))
		for _, edge := range snapshot.Edges {
			sourceKey := graph.CanonicalName(edge.Source)
			targetKey := graph.CanonicalName(edge.Target)
			if sourceKey == "" || targetKey == "" {
				t.Fatalf("edge has blank endpoint: %+v", edge)
			}
			if edge.Relation != "reports_to" {
				t.Fatalf("unexpected relation %q for edge %+v", edge.Relation, edge)
			}
			if strings.TrimSpace(edge.Evidence) == "" {
				t.Fatalf("edge has blank evidence: %+v", edge)
			}
			if !strings.Contains(text, edge.Evidence) {
				t.Fatalf("edge evidence %q is not present in source text %q", edge.Evidence, text)
			}
			if _, exists := entityKeys[sourceKey]; !exists {
				t.Fatalf("edge source %q missing from entities %+v", edge.Source, snapshot.Entities)
			}
			if _, exists := entityKeys[targetKey]; !exists {
				t.Fatalf("edge target %q missing from entities %+v", edge.Target, snapshot.Entities)
			}
			key := sourceKey + "|" + edge.Relation + "|" + targetKey
			if _, exists := edgeKeys[key]; exists {
				t.Fatalf("duplicate edge %q in snapshot: %+v", key, snapshot)
			}
			edgeKeys[key] = struct{}{}
		}
	})
}
