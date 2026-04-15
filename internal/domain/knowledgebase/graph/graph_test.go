package graph_test

import (
	"testing"

	"github.com/google/uuid"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/knowledgebase/graph"
)

func TestEdgeConnectsEntities(t *testing.T) {
	a := graph.Entity{ID: uuid.New(), Type: "person", Name: "Alice"}
	b := graph.Entity{ID: uuid.New(), Type: "team", Name: "platform"}
	e := graph.Edge{ID: uuid.New(), FromID: a.ID, ToID: b.ID, Relation: "member_of"}
	if e.FromID != a.ID || e.ToID != b.ID {
		t.Fatal("edge endpoints wrong")
	}
}
