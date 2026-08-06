// Package rerank provides pluggable rerankers for RAG retrieval. The initial
// top-K from the vector search is re-scored before being passed to the LLM so
// the most relevant chunks surface first.
//
// Inspired by Mastra's rerank package — Go implementation lives in this repo
// and stays independent of any TS code.
package rerank

import (
	"context"
	"sort"
)

// Candidate is a chunk returned by the initial retriever.
type Candidate struct {
	ID            string
	Content       string
	VectorScore   float64
	RerankedScore float64
}

// Reranker re-scores an ordered slice of candidates. Implementations must not
// mutate the input slice; they return a new slice sorted descending by
// RerankedScore.
type Reranker interface {
	Name() string
	Rerank(ctx context.Context, query string, candidates []Candidate, topN int) ([]Candidate, error)
}

// RRFReranker implements Reciprocal Rank Fusion over the vector score ranking.
// When no additional signal is available, it simply re-emits top-N. When a
// second ranking is provided via the Scores map (candidate id → score), RRF
// fuses both.
type RRFReranker struct {
	// K is the RRF constant. Default 60 per the original paper.
	K int
	// Scores is an optional second-signal score per candidate id.
	Scores map[string]float64
}

func (RRFReranker) Name() string { return "rrf" }

func (r RRFReranker) Rerank(_ context.Context, _ string, cands []Candidate, topN int) ([]Candidate, error) {
	k := r.K
	if k <= 0 {
		k = 60
	}
	ranked := make([]Candidate, len(cands))
	copy(ranked, cands)

	sort.SliceStable(ranked, func(i, j int) bool { return ranked[i].VectorScore > ranked[j].VectorScore })
	vecRank := make(map[string]int, len(ranked))
	for i, c := range ranked {
		vecRank[c.ID] = i + 1
	}

	secondRank := make(map[string]int, len(r.Scores))
	if len(r.Scores) > 0 {
		type sc struct {
			id string
			s  float64
		}
		scs := make([]sc, 0, len(r.Scores))
		for id, s := range r.Scores {
			scs = append(scs, sc{id, s})
		}
		sort.Slice(scs, func(i, j int) bool { return scs[i].s > scs[j].s })
		for i, s := range scs {
			secondRank[s.id] = i + 1
		}
	}

	for i := range ranked {
		score := 1.0 / float64(k+vecRank[ranked[i].ID])
		if sr, ok := secondRank[ranked[i].ID]; ok {
			score += 1.0 / float64(k+sr)
		}
		ranked[i].RerankedScore = score
	}

	sort.SliceStable(ranked, func(i, j int) bool { return ranked[i].RerankedScore > ranked[j].RerankedScore })
	if topN > 0 && topN < len(ranked) {
		ranked = ranked[:topN]
	}
	return ranked, nil
}

// LLMReranker is a stub interface for an LLM-as-judge reranker. The actual
// scoring call is delegated to Scorer so transport stays outside this package.
type LLMReranker struct {
	Scorer func(ctx context.Context, query, content string) (float64, error)
}

func (LLMReranker) Name() string { return "llm" }

func (r LLMReranker) Rerank(ctx context.Context, query string, cands []Candidate, topN int) ([]Candidate, error) {
	out := make([]Candidate, len(cands))
	copy(out, cands)
	for i := range out {
		s, err := r.Scorer(ctx, query, out[i].Content)
		if err != nil {
			return nil, err
		}
		out[i].RerankedScore = s
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].RerankedScore > out[j].RerankedScore })
	if topN > 0 && topN < len(out) {
		out = out[:topN]
	}
	return out, nil
}

// CrossEncoderReranker delegates pair scoring to a cross-encoder-like scorer.
// The scorer receives the query and candidate content and returns a comparable
// relevance score where higher is better.
type CrossEncoderReranker struct {
	Scorer func(ctx context.Context, query, content string) (float64, error)
}

func (CrossEncoderReranker) Name() string { return "cross_encoder" }

func (r CrossEncoderReranker) Rerank(ctx context.Context, query string, cands []Candidate, topN int) ([]Candidate, error) {
	out := make([]Candidate, len(cands))
	copy(out, cands)
	for i := range out {
		s, err := r.Scorer(ctx, query, out[i].Content)
		if err != nil {
			return nil, err
		}
		out[i].RerankedScore = s
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].RerankedScore > out[j].RerankedScore })
	if topN > 0 && topN < len(out) {
		out = out[:topN]
	}
	return out, nil
}
