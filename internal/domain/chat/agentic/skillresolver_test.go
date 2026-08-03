package agentic

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/skill"
)

func TestSkillSetResolverInitialSearchUsesTopK(t *testing.T) {
	id1 := uuid.New()
	id2 := uuid.New()
	store := &fakeSkillVectorStore{
		results: []skill.EmbeddingSearchResult{
			{ID: id1, Slug: "one", Score: 0.91, EmbeddingSourceHash: "h1"},
			{ID: id2, Slug: "two", Score: 0.84, EmbeddingSourceHash: "h2"},
		},
	}
	resolver := NewSkillSetResolver(store, &fakeResolverEmbedder{vectors: [][]float32{{1, 0}}}, SkillSetResolverConfig{TopK: 8, DriftThreshold: 0.55})

	got, err := resolver.Resolve(context.Background(), SkillSetResolveInput{UserMessage: "plan a release"})
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if !got.Refreshed || got.Reason != "initial" {
		t.Fatalf("refresh = %v reason = %q, want initial refresh", got.Refreshed, got.Reason)
	}
	if store.lastTopK != 8 {
		t.Fatalf("topK = %d, want 8", store.lastTopK)
	}
	if len(got.SkillIDs) != 2 || got.SkillIDs[0] != id1 || got.SkillIDs[1] != id2 {
		t.Fatalf("SkillIDs = %v, want [%s %s]", got.SkillIDs, id1, id2)
	}
	if got.State.SourceHashes[id1] != "h1" || got.State.SourceHashes[id2] != "h2" {
		t.Fatalf("source hashes not preserved: %+v", got.State.SourceHashes)
	}
}

func TestSkillSetResolverKeepsStickySetWhenDriftBelowThreshold(t *testing.T) {
	id := uuid.New()
	store := &fakeSkillVectorStore{hashes: map[uuid.UUID]string{id: "same"}}
	resolver := NewSkillSetResolver(store, &fakeResolverEmbedder{vectors: [][]float32{{1, 0}}}, SkillSetResolverConfig{TopK: 8, DriftThreshold: 0.55})

	got, err := resolver.Resolve(context.Background(), SkillSetResolveInput{
		UserMessage: "same topic",
		Previous: SkillSetState{
			SkillIDs:       []uuid.UUID{id},
			SourceHashes:   map[uuid.UUID]string{id: "same"},
			QueryEmbedding: []float32{1, 0},
		},
	})
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if got.Refreshed || got.Reason != "sticky" {
		t.Fatalf("refresh = %v reason = %q, want sticky", got.Refreshed, got.Reason)
	}
	if store.searchCalls != 0 {
		t.Fatalf("search calls = %d, want 0", store.searchCalls)
	}
	if got.SkillIDs[0] != id {
		t.Fatalf("SkillIDs = %v, want sticky id %s", got.SkillIDs, id)
	}
}

func TestSkillSetResolverRefreshesWhenDriftExceedsThreshold(t *testing.T) {
	stickyID := uuid.New()
	newID := uuid.New()
	store := &fakeSkillVectorStore{
		hashes: map[uuid.UUID]string{stickyID: "same"},
		results: []skill.EmbeddingSearchResult{
			{ID: newID, Slug: "new", Score: 0.88, EmbeddingSourceHash: "new-hash"},
		},
	}
	resolver := NewSkillSetResolver(store, &fakeResolverEmbedder{vectors: [][]float32{{0, 1}}}, SkillSetResolverConfig{TopK: 8, DriftThreshold: 0.55})

	got, err := resolver.Resolve(context.Background(), SkillSetResolveInput{
		UserMessage: "different topic",
		Previous: SkillSetState{
			SkillIDs:       []uuid.UUID{stickyID},
			SourceHashes:   map[uuid.UUID]string{stickyID: "same"},
			QueryEmbedding: []float32{1, 0},
		},
	})
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if !got.Refreshed || got.Reason != "drift" {
		t.Fatalf("refresh = %v reason = %q, want drift refresh", got.Refreshed, got.Reason)
	}
	if got.DriftScore <= 0.55 {
		t.Fatalf("driftScore = %f, want > 0.55", got.DriftScore)
	}
	if got.SkillIDs[0] != newID {
		t.Fatalf("SkillIDs = %v, want refreshed id %s", got.SkillIDs, newID)
	}
}

func TestSkillSetResolverRefreshesWhenStickySkillHashChanges(t *testing.T) {
	stickyID := uuid.New()
	newID := uuid.New()
	store := &fakeSkillVectorStore{
		hashes: map[uuid.UUID]string{stickyID: "changed"},
		results: []skill.EmbeddingSearchResult{
			{ID: newID, Slug: "new", Score: 0.90, EmbeddingSourceHash: "new-hash"},
		},
	}
	resolver := NewSkillSetResolver(store, &fakeResolverEmbedder{vectors: [][]float32{{1, 0}}}, SkillSetResolverConfig{TopK: 8, DriftThreshold: 0.55})

	got, err := resolver.Resolve(context.Background(), SkillSetResolveInput{
		UserMessage: "same topic",
		Previous: SkillSetState{
			SkillIDs:       []uuid.UUID{stickyID},
			SourceHashes:   map[uuid.UUID]string{stickyID: "old"},
			QueryEmbedding: []float32{1, 0},
		},
	})
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if !got.Refreshed || got.Reason != "invalidated" {
		t.Fatalf("refresh = %v reason = %q, want invalidated refresh", got.Refreshed, got.Reason)
	}
	if got.SkillIDs[0] != newID {
		t.Fatalf("SkillIDs = %v, want refreshed id %s", got.SkillIDs, newID)
	}
}

func TestSkillSetResolverHonorsTenantScoreAndSizeLimits(t *testing.T) {
	id1 := uuid.New()
	id2 := uuid.New()
	id3 := uuid.New()
	store := &fakeSkillVectorStore{results: []skill.EmbeddingSearchResult{
		{ID: id1, Score: 0.93, EmbeddingSourceHash: "h1"},
		{ID: id2, Score: 0.81, EmbeddingSourceHash: "h2"},
		{ID: id3, Score: 0.12, EmbeddingSourceHash: "h3"},
	}}
	resolver := NewSkillSetResolver(store, &fakeResolverEmbedder{vectors: [][]float32{{1, 0}}}, DefaultSkillSetResolverConfig())

	got, err := resolver.ResolveWithConfig(context.Background(), SkillSetResolveInput{UserMessage: "search"}, SkillSetResolverConfig{
		TopK:           1,
		MaxStickySize:  1,
		MinScore:       0.50,
		AllowDrift:     true,
		RefreshPolicy:  "drift_or_invalidation",
		DriftThreshold: 0.55,
	})
	if err != nil {
		t.Fatalf("ResolveWithConfig() error = %v", err)
	}
	if len(got.SkillIDs) != 1 || got.SkillIDs[0] != id1 {
		t.Fatalf("SkillIDs = %v, want only highest-scoring permitted skill %s", got.SkillIDs, id1)
	}
}

type fakeSkillVectorStore struct {
	results     []skill.EmbeddingSearchResult
	hashes      map[uuid.UUID]string
	lastTopK    int
	searchCalls int
}

func (s *fakeSkillVectorStore) SearchByEmbedding(_ context.Context, _ []float32, topK int) ([]skill.EmbeddingSearchResult, error) {
	s.searchCalls++
	s.lastTopK = topK
	return s.results, nil
}

func (s *fakeSkillVectorStore) EmbeddingSourceHashesByIDs(_ context.Context, ids []uuid.UUID) (map[uuid.UUID]string, error) {
	out := make(map[uuid.UUID]string, len(ids))
	for _, id := range ids {
		out[id] = s.hashes[id]
	}
	return out, nil
}

type fakeResolverEmbedder struct {
	vectors [][]float32
	calls   int
}

func (e *fakeResolverEmbedder) Embed(context.Context, string) ([]float32, error) {
	if e.calls >= len(e.vectors) {
		return nil, nil
	}
	vec := e.vectors[e.calls]
	e.calls++
	return vec, nil
}
