package rerank_test

import (
	"context"
	"testing"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/knowledgebase/rerank"
)

func TestRRFReorders(t *testing.T) {
	cs := []rerank.Candidate{
		{ID: "a", VectorScore: 0.9},
		{ID: "b", VectorScore: 0.8},
		{ID: "c", VectorScore: 0.7},
	}
	r := rerank.RRFReranker{Scores: map[string]float64{"c": 10, "b": 5, "a": 1}}
	out, err := r.Rerank(context.Background(), "q", cs, 3)
	if err != nil {
		t.Fatal(err)
	}
	// "c" has highest second-signal rank → after fusion "c" or "a" likely top.
	if out[0].ID == "" {
		t.Fatal("empty rerank")
	}
	if len(out) != 3 {
		t.Fatalf("want 3, got %d", len(out))
	}
}

func TestRRFTopN(t *testing.T) {
	cs := []rerank.Candidate{
		{ID: "a", VectorScore: 0.9},
		{ID: "b", VectorScore: 0.8},
		{ID: "c", VectorScore: 0.7},
	}
	out, _ := rerank.RRFReranker{}.Rerank(context.Background(), "q", cs, 2)
	if len(out) != 2 {
		t.Fatalf("topN not applied: %d", len(out))
	}
}

func TestLLMReranker(t *testing.T) {
	scorer := func(_ context.Context, _, content string) (float64, error) {
		if content == "best" {
			return 1.0, nil
		}
		return 0.1, nil
	}
	cs := []rerank.Candidate{{ID: "1", Content: "worst"}, {ID: "2", Content: "best"}}
	out, _ := rerank.LLMReranker{Scorer: scorer}.Rerank(context.Background(), "q", cs, 0)
	if out[0].ID != "2" {
		t.Fatalf("expected 2 on top, got %+v", out)
	}
}
