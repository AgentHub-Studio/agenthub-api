package search

import (
	"context"
	"fmt"
	"sync"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/AgentHub-Studio/agenthub-api/internal/database"
)

// Service implements global cross-domain search.
type Service struct {
	pool *pgxpool.Pool
}

// NewService creates a new Service.
func NewService(pool *pgxpool.Pool) *Service {
	return &Service{pool: pool}
}

// Search runs parallel ILIKE queries across agents, skills, tools, and knowledge_bases.
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
		items, err := s.searchTable(ctx, tenantID, "agent", "agent", query, limit)
		agentCh <- result{items, err}
	}()

	go func() {
		defer wg.Done()
		items, err := s.searchTable(ctx, tenantID, "skill", "skill", query, limit)
		skillCh <- result{items, err}
	}()

	go func() {
		defer wg.Done()
		items, err := s.searchTable(ctx, tenantID, "tool", "tool", query, limit)
		toolCh <- result{items, err}
	}()

	go func() {
		defer wg.Done()
		items, err := s.searchTable(ctx, tenantID, "knowledge_base", "knowledge_base", query, limit)
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

// searchTable performs an ILIKE search on name and description columns for the given table.
func (s *Service) searchTable(ctx context.Context, tenantID, table, resourceType, query string, limit int) ([]SearchResult, error) {
	conn, release, err := database.AcquireWithTenant(ctx, s.pool, tenantID)
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
		var r SearchResult
		r.Type = resourceType
		if err := rows.Scan(&r.ID, &r.Name, &r.Description, &r.Status, &r.Slug); err != nil {
			return nil, fmt.Errorf("search: scan %s: %w", table, err)
		}
		items = append(items, r)
	}
	if items == nil {
		items = []SearchResult{}
	}
	return items, rows.Err()
}
