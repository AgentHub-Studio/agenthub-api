package graph

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/AgentHub-Studio/agenthub-api/internal/database"
	"github.com/AgentHub-Studio/agenthub-api/internal/tenant"
)

// EntityRecord is a persisted graph node.
type EntityRecord struct {
	ID              uuid.UUID `json:"id"`
	DocumentID      uuid.UUID `json:"documentId"`
	KnowledgeBaseID uuid.UUID `json:"knowledgeBaseId"`
	Name            string    `json:"name"`
	Type            string    `json:"type"`
}

// EdgeRecord is a persisted graph edge.
type EdgeRecord struct {
	ID              uuid.UUID `json:"id"`
	KnowledgeBaseID uuid.UUID `json:"knowledgeBaseId"`
	SourceID        uuid.UUID `json:"sourceId"`
	TargetID        uuid.UUID `json:"targetId"`
	Source          string    `json:"source"`
	Target          string    `json:"target"`
	Relation        string    `json:"relation"`
	Evidence        string    `json:"evidence"`
}

// PersistedGraph is the graph snapshot returned by the API.
type PersistedGraph struct {
	Entities []EntityRecord `json:"entities"`
	Edges    []EdgeRecord   `json:"edges"`
}

// SearchResult is a graph traversal result.
type SearchResult struct {
	ID              uuid.UUID
	DocumentID      uuid.UUID
	KnowledgeBaseID uuid.UUID
	Content         string
	Score           float64
	DocumentName    string
	Metadata        SearchMetadata
}

// SearchMetadata is serialized under the graph key in search responses.
type SearchMetadata struct {
	Chain []string     `json:"chain"`
	Edges []EdgeRecord `json:"edges"`
}

// Repository reads graph data from the tenant schema.
type Repository interface {
	List(ctx context.Context, kbID uuid.UUID) (PersistedGraph, error)
	SearchReportsTo(ctx context.Context, kbID uuid.UUID, query string, limit int) (SearchResult, bool, error)
}

type postgresRepository struct {
	pool *pgxpool.Pool
}

// NewRepository creates a PostgreSQL-backed graph repository.
func NewRepository(pool *pgxpool.Pool) Repository {
	return &postgresRepository{pool: pool}
}

func (r *postgresRepository) List(ctx context.Context, kbID uuid.UUID) (PersistedGraph, error) {
	conn, release, err := database.AcquireWithTenant(ctx, r.pool, tenant.FromContext(ctx))
	if err != nil {
		return PersistedGraph{}, err
	}
	defer release()

	entityRows, err := conn.Query(ctx, `
SELECT id, document_id, knowledge_base_id, name, type
FROM document_entity
WHERE knowledge_base_id = $1
ORDER BY lower(name), id`, kbID)
	if err != nil {
		return PersistedGraph{}, fmt.Errorf("graph: list entities: %w", err)
	}
	defer entityRows.Close()

	out := PersistedGraph{
		Entities: []EntityRecord{},
		Edges:    []EdgeRecord{},
	}
	for entityRows.Next() {
		var entity EntityRecord
		if err := entityRows.Scan(&entity.ID, &entity.DocumentID, &entity.KnowledgeBaseID, &entity.Name, &entity.Type); err != nil {
			return PersistedGraph{}, fmt.Errorf("graph: scan entity: %w", err)
		}
		out.Entities = append(out.Entities, entity)
	}
	if err := entityRows.Err(); err != nil {
		return PersistedGraph{}, fmt.Errorf("graph: iterate entities: %w", err)
	}

	edgeRows, err := conn.Query(ctx, `
SELECT e.id, e.knowledge_base_id, e.source_entity_id, e.target_entity_id,
       src.name, dst.name, e.relation, e.evidence
FROM document_entity_edge e
JOIN document_entity src ON src.id = e.source_entity_id
JOIN document_entity dst ON dst.id = e.target_entity_id
WHERE e.knowledge_base_id = $1
ORDER BY lower(src.name), lower(dst.name), e.id`, kbID)
	if err != nil {
		return PersistedGraph{}, fmt.Errorf("graph: list edges: %w", err)
	}
	defer edgeRows.Close()

	for edgeRows.Next() {
		var edge EdgeRecord
		if err := edgeRows.Scan(
			&edge.ID,
			&edge.KnowledgeBaseID,
			&edge.SourceID,
			&edge.TargetID,
			&edge.Source,
			&edge.Target,
			&edge.Relation,
			&edge.Evidence,
		); err != nil {
			return PersistedGraph{}, fmt.Errorf("graph: scan edge: %w", err)
		}
		out.Edges = append(out.Edges, edge)
	}
	if err := edgeRows.Err(); err != nil {
		return PersistedGraph{}, fmt.Errorf("graph: iterate edges: %w", err)
	}

	return out, nil
}

func (r *postgresRepository) SearchReportsTo(ctx context.Context, kbID uuid.UUID, query string, limit int) (SearchResult, bool, error) {
	graph, err := r.List(ctx, kbID)
	if err != nil {
		return SearchResult{}, false, err
	}
	if len(graph.Edges) == 0 {
		return SearchResult{}, false, nil
	}
	if limit <= 0 {
		limit = 5
	}

	start := findSourceEntity(query, graph.Edges)
	if start == "" {
		return SearchResult{}, false, nil
	}

	adjacency := map[string]EdgeRecord{}
	for _, edge := range graph.Edges {
		if edge.Relation != "reports_to" {
			continue
		}
		key := CanonicalName(edge.Source)
		if _, exists := adjacency[key]; !exists {
			adjacency[key] = edge
		}
	}

	chain := []string{start}
	traversed := []EdgeRecord{}
	current := start
	for len(traversed) < limit {
		edge, ok := adjacency[CanonicalName(current)]
		if !ok {
			break
		}
		traversed = append(traversed, edge)
		chain = append(chain, edge.Target)
		current = edge.Target
	}
	if len(traversed) == 0 {
		return SearchResult{}, false, nil
	}

	first := traversed[0]
	content := fmt.Sprintf("%s reports_to %s. Chain: %s", first.Source, first.Target, strings.Join(chain, " -> "))
	return SearchResult{
		ID:              first.ID,
		DocumentID:      documentIDForSource(first.SourceID, graph.Entities),
		KnowledgeBaseID: kbID,
		Content:         content,
		Score:           1,
		DocumentName:    "knowledge graph",
		Metadata: SearchMetadata{
			Chain: chain,
			Edges: traversed,
		},
	}, true, nil
}

func findSourceEntity(query string, edges []EdgeRecord) string {
	lowerQuery := strings.ToLower(query)
	for _, edge := range edges {
		if edge.Relation != "reports_to" {
			continue
		}
		if strings.Contains(lowerQuery, CanonicalName(edge.Source)) {
			return edge.Source
		}
	}
	return ""
}

func documentIDForSource(sourceID uuid.UUID, entities []EntityRecord) uuid.UUID {
	for _, entity := range entities {
		if entity.ID == sourceID {
			return entity.DocumentID
		}
	}
	return uuid.Nil
}
