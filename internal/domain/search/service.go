package search

import (
	"context"
	"fmt"
	"sync"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/AgentHub-Studio/agenthub-api/internal/database"
)

// Repository abstracts the underlying data source for search queries.
// Each method searches a single entity type and returns matching results.
type Repository interface {
	SearchAgents(ctx context.Context, tenantID, query string, limit int) ([]SearchResult, error)
	SearchSkills(ctx context.Context, tenantID, query string, limit int) ([]SearchResult, error)
	SearchTools(ctx context.Context, tenantID, query string, limit int) ([]SearchResult, error)
	SearchKnowledgeBases(ctx context.Context, tenantID, query string, limit int) ([]SearchResult, error)
}

// EntityType constants for the entityType filter accepted by Search.
const (
	EntityAgent         = "agent"
	EntitySkill         = "skill"
	EntityTool          = "tool"
	EntityKnowledgeBase = "knowledge_base"
)

// Service implements global cross-domain search.
type Service struct {
	repo Repository
}

// NewService creates a new Service.
func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

// NewServiceWithPool creates a Service backed by a pgxpool.Pool.
func NewServiceWithPool(pool *pgxpool.Pool) *Service {
	return NewService(&pgRepository{pool: pool})
}

// Search runs parallel full-text queries across agents, skills, tools, and knowledge_bases.
// entityType filters results to a single entity type when non-empty (e.g. "agent").
func (s *Service) Search(ctx context.Context, tenantID, query, entityType string, limit int) (GlobalSearchResponse, error) {
	if limit <= 0 {
		limit = 5
	}

	empty := []SearchResult{}

	type result struct {
		items []SearchResult
		err   error
	}

	run := func(fn func() ([]SearchResult, error)) chan result {
		ch := make(chan result, 1)
		go func() {
			items, err := fn()
			if items == nil {
				items = empty
			}
			ch <- result{items, err}
		}()
		return ch
	}

	skip := func() chan result {
		ch := make(chan result, 1)
		ch <- result{items: empty}
		return ch
	}

	include := func(t string) bool {
		return entityType == "" || entityType == t
	}

	var (
		agentCh = skip()
		skillCh = skip()
		toolCh  = skip()
		kbCh    = skip()
	)
	var wg sync.WaitGroup

	if include(EntityAgent) {
		wg.Add(1)
		agentCh = run(func() ([]SearchResult, error) {
			defer wg.Done()
			return s.repo.SearchAgents(ctx, tenantID, query, limit)
		})
	}
	if include(EntitySkill) {
		wg.Add(1)
		skillCh = run(func() ([]SearchResult, error) {
			defer wg.Done()
			return s.repo.SearchSkills(ctx, tenantID, query, limit)
		})
	}
	if include(EntityTool) {
		wg.Add(1)
		toolCh = run(func() ([]SearchResult, error) {
			defer wg.Done()
			return s.repo.SearchTools(ctx, tenantID, query, limit)
		})
	}
	if include(EntityKnowledgeBase) {
		wg.Add(1)
		kbCh = run(func() ([]SearchResult, error) {
			defer wg.Done()
			return s.repo.SearchKnowledgeBases(ctx, tenantID, query, limit)
		})
	}

	wg.Wait()

	agentRes := <-agentCh
	skillRes := <-skillCh
	toolRes := <-toolCh
	kbRes := <-kbCh

	for _, r := range []result{agentRes, skillRes, toolRes, kbRes} {
		if r.err != nil {
			return GlobalSearchResponse{}, r.err
		}
	}

	total := len(agentRes.items) + len(skillRes.items) + len(toolRes.items) + len(kbRes.items)
	return GlobalSearchResponse{
		Query:          query,
		Agents:         agentRes.items,
		Skills:         skillRes.items,
		Tools:          toolRes.items,
		KnowledgeBases: kbRes.items,
		TotalResults:   total,
	}, nil
}

// pgRepository is the PostgreSQL implementation of Repository.
type pgRepository struct {
	pool *pgxpool.Pool
}

func (r *pgRepository) SearchAgents(ctx context.Context, tenantID, query string, limit int) ([]SearchResult, error) {
	return r.searchTable(ctx, tenantID, "agent", "agent", query, limit)
}

func (r *pgRepository) SearchSkills(ctx context.Context, tenantID, query string, limit int) ([]SearchResult, error) {
	return r.searchTable(ctx, tenantID, "skill", "skill", query, limit)
}

func (r *pgRepository) SearchTools(ctx context.Context, tenantID, query string, limit int) ([]SearchResult, error) {
	return r.searchTable(ctx, tenantID, "tool", "tool", query, limit)
}

func (r *pgRepository) SearchKnowledgeBases(ctx context.Context, tenantID, query string, limit int) ([]SearchResult, error) {
	return r.searchTable(ctx, tenantID, "knowledge_base", "knowledge_base", query, limit)
}

// searchTable queries a single table using full-text search (ts_rank) for queries >= 3 chars,
// falling back to case-insensitive ILIKE for shorter queries.
// Results are ranked by relevance descending.
func (r *pgRepository) searchTable(ctx context.Context, tenantID, table, resourceType, query string, limit int) ([]SearchResult, error) {
	conn, release, err := database.AcquireWithTenant(ctx, r.pool, tenantID)
	if err != nil {
		return nil, err
	}
	defer release()

	var (
		sqlStr string
		args   []any
	)

	if len([]rune(query)) >= 3 {
		// Full-text search with relevance ranking.
		// plainto_tsquery handles arbitrary input safely (no operator injection).
		sqlStr = fmt.Sprintf(
			`SELECT id::text, name, COALESCE(description,''), COALESCE(status::text,''), COALESCE(slug,'')
			 FROM %s
			 WHERE to_tsvector('portuguese', name || ' ' || COALESCE(description,''))
			       @@ plainto_tsquery('portuguese', $1)
			 ORDER BY ts_rank(
			     to_tsvector('portuguese', name || ' ' || COALESCE(description,'')),
			     plainto_tsquery('portuguese', $1)
			 ) DESC
			 LIMIT $2`,
			table,
		)
		args = []any{query, limit}
	} else {
		// Short query fallback: ILIKE ordered alphabetically.
		pattern := "%" + query + "%"
		sqlStr = fmt.Sprintf(
			`SELECT id::text, name, COALESCE(description,''), COALESCE(status::text,''), COALESCE(slug,'')
			 FROM %s
			 WHERE name ILIKE $1 OR description ILIKE $1
			 ORDER BY name ASC
			 LIMIT $2`,
			table,
		)
		args = []any{pattern, limit}
	}

	rows, err := conn.Query(ctx, sqlStr, args...)
	if err != nil {
		return nil, fmt.Errorf("search: query %s: %w", table, err)
	}
	defer rows.Close()

	var items []SearchResult
	for rows.Next() {
		var res SearchResult
		res.Type = resourceType
		if err := rows.Scan(&res.ID, &res.Name, &res.Description, &res.Status, &res.Slug); err != nil {
			return nil, fmt.Errorf("search: scan %s: %w", table, err)
		}
		items = append(items, res)
	}
	if items == nil {
		items = []SearchResult{}
	}
	return items, rows.Err()
}
