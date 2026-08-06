package agentic

import (
	"context"
	"fmt"
	"math"

	"github.com/google/uuid"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/skill"
)

type SkillVectorStore interface {
	SearchByEmbedding(ctx context.Context, embedding []float32, topK int) ([]skill.EmbeddingSearchResult, error)
	EmbeddingSourceHashesByIDs(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]string, error)
}

type SkillSetResolverConfig struct {
	TopK           int
	DriftThreshold float64
}

func DefaultSkillSetResolverConfig() SkillSetResolverConfig {
	return SkillSetResolverConfig{TopK: 8, DriftThreshold: 0.55}
}

type SkillSetState struct {
	SkillIDs       []uuid.UUID
	SourceHashes   map[uuid.UUID]string
	QueryEmbedding []float32
}

type SkillSetResolveInput struct {
	UserMessage string
	Previous    SkillSetState
}

type SkillSetResolveResult struct {
	SkillIDs    []uuid.UUID
	State       SkillSetState
	Refreshed   bool
	Reason      string
	DriftScore  float64
	SearchScore map[uuid.UUID]float64
}

type SkillSetResolver struct {
	store    SkillVectorStore
	embedder Embedder
	config   SkillSetResolverConfig
}

func NewSkillSetResolver(store SkillVectorStore, embedder Embedder, cfg SkillSetResolverConfig) *SkillSetResolver {
	if cfg.TopK <= 0 {
		cfg.TopK = DefaultSkillSetResolverConfig().TopK
	}
	if cfg.DriftThreshold <= 0 {
		cfg.DriftThreshold = DefaultSkillSetResolverConfig().DriftThreshold
	}
	return &SkillSetResolver{store: store, embedder: embedder, config: cfg}
}

func (r *SkillSetResolver) Resolve(ctx context.Context, in SkillSetResolveInput) (SkillSetResolveResult, error) {
	if r == nil || r.store == nil {
		return SkillSetResolveResult{}, fmt.Errorf("skillresolver: store is required")
	}
	if r.embedder == nil {
		return SkillSetResolveResult{}, fmt.Errorf("skillresolver: embedder is required")
	}

	invalidated, err := r.hasInvalidatedSkill(ctx, in.Previous)
	if err != nil {
		return SkillSetResolveResult{}, err
	}

	queryEmbedding, err := r.embedder.Embed(ctx, in.UserMessage)
	if err != nil {
		return SkillSetResolveResult{}, fmt.Errorf("skillresolver: embed query: %w", err)
	}

	driftScore := 0.0
	if len(in.Previous.QueryEmbedding) > 0 {
		driftScore = cosineDrift(in.Previous.QueryEmbedding, queryEmbedding)
	}

	if len(in.Previous.SkillIDs) > 0 && !invalidated && driftScore <= r.config.DriftThreshold {
		return SkillSetResolveResult{
			SkillIDs:   append([]uuid.UUID(nil), in.Previous.SkillIDs...),
			State:      in.Previous,
			Refreshed:  false,
			Reason:     "sticky",
			DriftScore: driftScore,
		}, nil
	}

	reason := "initial"
	if invalidated {
		reason = "invalidated"
	} else if len(in.Previous.SkillIDs) > 0 {
		reason = "drift"
	}
	return r.retrieve(ctx, queryEmbedding, driftScore, reason)
}

func (r *SkillSetResolver) hasInvalidatedSkill(ctx context.Context, state SkillSetState) (bool, error) {
	if len(state.SkillIDs) == 0 || len(state.SourceHashes) == 0 {
		return false, nil
	}
	current, err := r.store.EmbeddingSourceHashesByIDs(ctx, state.SkillIDs)
	if err != nil {
		return false, fmt.Errorf("skillresolver: load source hashes: %w", err)
	}
	for _, id := range state.SkillIDs {
		if current[id] != state.SourceHashes[id] {
			return true, nil
		}
	}
	return false, nil
}

func (r *SkillSetResolver) retrieve(ctx context.Context, embedding []float32, driftScore float64, reason string) (SkillSetResolveResult, error) {
	candidates, err := r.store.SearchByEmbedding(ctx, embedding, r.config.TopK)
	if err != nil {
		return SkillSetResolveResult{}, fmt.Errorf("skillresolver: search skills: %w", err)
	}

	ids := make([]uuid.UUID, 0, len(candidates))
	hashes := make(map[uuid.UUID]string, len(candidates))
	scores := make(map[uuid.UUID]float64, len(candidates))
	for _, candidate := range candidates {
		ids = append(ids, candidate.ID)
		hashes[candidate.ID] = candidate.EmbeddingSourceHash
		scores[candidate.ID] = candidate.Score
	}

	return SkillSetResolveResult{
		SkillIDs: ids,
		State: SkillSetState{
			SkillIDs:       ids,
			SourceHashes:   hashes,
			QueryEmbedding: append([]float32(nil), embedding...),
		},
		Refreshed:   true,
		Reason:      reason,
		DriftScore:  driftScore,
		SearchScore: scores,
	}, nil
}

func cosineDrift(a, b []float32) float64 {
	if len(a) == 0 || len(b) == 0 || len(a) != len(b) {
		return 1
	}
	var dot, normA, normB float64
	for i := range a {
		av := float64(a[i])
		bv := float64(b[i])
		dot += av * bv
		normA += av * av
		normB += bv * bv
	}
	if normA == 0 || normB == 0 {
		return 1
	}
	similarity := dot / (math.Sqrt(normA) * math.Sqrt(normB))
	if similarity > 1 {
		similarity = 1
	}
	if similarity < -1 {
		similarity = -1
	}
	return 1 - similarity
}
