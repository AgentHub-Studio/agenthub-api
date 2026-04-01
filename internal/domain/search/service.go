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

// Search runs parallel queries across agents, skills, tools, and knowledge_bases.
func (s *Service) Search(ctx context.Context, tenantID string, query string, limit int) (GlobalSearchResponse, error) {
	if limit <= 0 {
		limit = 5
	}

	type result struct {
		items []SearchResult
		err   error
	}

	agentCh := make(chan result, 1)
	skillCh := make(chan result, 1)
	toolCh := make(chan result, 1)
	kbCh := make(chan result, 1)

	var wg sync.WaitGroup
	wg.Add(4)

	go func() {
		defer wg.Done()
		items, err := s.repo.SearchAgents(ctx, tenantID, query, limit)
		agentCh <- result{items, err}
	}()

	go func() {
		defer wg.Done()
		items, err := s.repo.SearchSkills(ctx, tenantID, query, limit)
		skillCh <- result{items, err}
	}()

	go func() {
		defer wg.Done()
		items, err := s.repo.SearchTools(ctx, tenantID, query, limit)
		toolCh <- result{items, err}
	}()

	go func() {
		defer wg.Done()
		items, err := s.repo.SearchKnowledgeBases(ctx, tenantID, query, limit)
		kbCh <- result{items, err}
	}()

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

	return GlobalSearchResponse{
		Query:          query,
		Agents:         agentRes.items,
		Skills:         skillRes.items,
		Tools:          toolRes.items,
		KnowledgeBases: kbRes.items,
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

// searchTable performs an ILIKE search on name and description columns for the given table.
func (r *pgRepository) searchTable(ctx context.Context, tenantID, table, resourceType, query string, limit int) ([]SearchResult, error) {
	conn, release, err := database.AcquireWithTenant(ctx, r.pool, tenantID)
	if err != nil {
		return nil, err
	}
	defer release()

	pattern := "%" + query + "%"
	rows, err := conn.Query(ctx,
		fmt.Sprintf(
			`SELECT id::text, name, COALESCE(description,''), COALESCE(status::text,''), COALESCE(slug,'')
			 FROM %s
			 WHERE name ILIKE $1 OR description ILIKE $1
			 LIMIT $2`,
			table,
		),
		pattern, limit,
	)
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
