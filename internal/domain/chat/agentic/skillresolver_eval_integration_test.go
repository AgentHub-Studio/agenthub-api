//go:build integration

package agentic

import (
	"context"
	"math"
	"sort"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/skill"
)

const (
	dsr10TenantFixtureID = "tenant-dsr10-synthetic"
	dsr10RecallAtK       = 8
	dsr10MinCaseCount    = 50
	dsr10MinRecallAt8    = 0.85
	dsr10MaxRetrieveP95  = 50 * time.Millisecond
)

func TestIntegration_DSR10SkillRetrievalEvalRecallAt8(t *testing.T) {
	fixtureSkills := dsr10SyntheticSkills()
	cases := dsr10SyntheticCases(fixtureSkills)
	if len(cases) < dsr10MinCaseCount {
		t.Fatalf("synthetic cases = %d, want at least %d", len(cases), dsr10MinCaseCount)
	}

	store := &dsr10VectorStore{skills: fixtureSkills}
	embedder := &dsr10Embedder{vectorsByQuery: map[string][]float32{}}
	for _, c := range cases {
		embedder.vectorsByQuery[c.query] = append([]float32(nil), c.embedding...)
	}
	resolver := NewSkillSetResolver(store, embedder, SkillSetResolverConfig{
		TopK:           dsr10RecallAtK,
		DriftThreshold: 0.55,
	})

	var hits int
	latencies := make([]time.Duration, 0, len(cases))
	for _, c := range cases {
		start := time.Now()
		got, err := resolver.Resolve(context.Background(), SkillSetResolveInput{
			UserMessage: c.query,
		})
		latencies = append(latencies, time.Since(start))
		if err != nil {
			t.Fatalf("Resolve(%q) error = %v", c.query, err)
		}
		if containsUUID(got.SkillIDs, c.expectedSkillID) {
			hits++
		}
	}

	recallAt8 := float64(hits) / float64(len(cases))
	p95 := percentileDuration(latencies, 0.95)
	t.Logf("DSR-10 tenant=%s cases=%d recall@8=%.3f p95=%s", dsr10TenantFixtureID, len(cases), recallAt8, p95)

	if recallAt8 < dsr10MinRecallAt8 {
		t.Fatalf("recall@8 = %.3f, want >= %.2f", recallAt8, dsr10MinRecallAt8)
	}
	if p95 >= dsr10MaxRetrieveP95 {
		t.Fatalf("retrieve p95 = %s, want < %s", p95, dsr10MaxRetrieveP95)
	}
}

type dsr10SkillFixture struct {
	id        uuid.UUID
	slug      string
	embedding []float32
	hash      string
}

type dsr10EvalCase struct {
	tenantID        string
	query           string
	expectedSkillID uuid.UUID
	embedding       []float32
}

func dsr10SyntheticSkills() []dsr10SkillFixture {
	slugs := []string{
		"billing-support",
		"customer-triage",
		"database-query",
		"oauth-setup",
		"mcp-integration",
		"document-rag",
		"usage-analytics",
		"tenant-onboarding",
		"deployment-ops",
		"security-review",
		"workflow-planning",
		"integration-builder",
	}
	out := make([]dsr10SkillFixture, 0, len(slugs))
	for i, slug := range slugs {
		out = append(out, dsr10SkillFixture{
			id:        uuid.NewSHA1(uuid.NameSpaceURL, []byte("agenthub-dsr10/"+slug)),
			slug:      slug,
			embedding: oneHotVector(i, len(slugs)),
			hash:      "hash-" + slug,
		})
	}
	return out
}

func dsr10SyntheticCases(skills []dsr10SkillFixture) []dsr10EvalCase {
	queries := map[string][]string{
		"billing-support": {
			"explain invoice overage and credit adjustment",
			"open a billing refund request for duplicate charge",
			"calculate seat cost after plan upgrade",
			"review subscription renewal payment failure",
			"answer customer about tax line on invoice",
		},
		"customer-triage": {
			"prioritize incoming support ticket by severity",
			"classify bug report versus usage question",
			"draft response for angry customer escalation",
			"route onboarding issue to support queue",
			"summarize unresolved customer blockers",
		},
		"database-query": {
			"run read only postgres query for active users",
			"inspect slow SQL and propose index",
			"count rows grouped by tenant id",
			"check database connection config",
			"explain failed migration lock timeout",
		},
		"oauth-setup": {
			"configure oauth client redirect uri",
			"debug token exchange invalid grant",
			"rotate oauth credential secret safely",
			"list scopes required for calendar provider",
			"connect third party app using authorization code",
		},
		"mcp-integration": {
			"register remote mcp server for github tools",
			"test mcp connection handshake failure",
			"map mcp tool schema to agent capability",
			"refresh mcp oauth session after expiry",
			"disable unavailable mcp server",
		},
		"document-rag": {
			"upload pdf and search indexed knowledge base",
			"explain why document chunks are not retrieved",
			"create rag collection for policy documents",
			"answer question from uploaded manual",
			"reindex knowledge base after document update",
		},
		"usage-analytics": {
			"show token usage by model last week",
			"calculate agent run cost trend",
			"export trace metrics for dashboard",
			"compare latency p95 across providers",
			"identify tenants with highest spend",
		},
		"tenant-onboarding": {
			"guide new tenant through first setup checklist",
			"create default assistant for fresh workspace",
			"verify onboarding banner completed state",
			"configure initial provider for a new account",
			"prepare first chat test for new tenant",
		},
		"deployment-ops": {
			"inspect kubernetes rollout status",
			"debug container image pull failure",
			"update ingress host for production deployment",
			"restart api pod after config change",
			"check registry push for latest image",
		},
		"security-review": {
			"review tool url for ssrf risk",
			"redact api keys from response payload",
			"audit permission rule for destructive action",
			"validate html input sanitization",
			"block path traversal in upload name",
		},
		"workflow-planning": {
			"break feature spec into implementation tasks",
			"plan multi agent execution workflow",
			"sequence dependency graph for release",
			"track coordinator subtasks through completion",
			"estimate milestones for roadmap epic",
		},
		"integration-builder": {
			"create http integration for crm endpoint",
			"generate tool from openapi schema",
			"configure database integration query template",
			"test external api connection with headers",
			"publish generated integration capability",
		},
	}

	bySlug := map[string]dsr10SkillFixture{}
	for _, s := range skills {
		bySlug[s.slug] = s
	}

	cases := make([]dsr10EvalCase, 0, 60)
	for _, s := range skills {
		for _, query := range queries[s.slug] {
			cases = append(cases, dsr10EvalCase{
				tenantID:        dsr10TenantFixtureID,
				query:           query,
				expectedSkillID: bySlug[s.slug].id,
				embedding:       append([]float32(nil), bySlug[s.slug].embedding...),
			})
		}
	}
	return cases
}

type dsr10VectorStore struct {
	skills []dsr10SkillFixture
}

func (s *dsr10VectorStore) SearchByEmbedding(_ context.Context, embedding []float32, topK int) ([]skill.EmbeddingSearchResult, error) {
	results := make([]skill.EmbeddingSearchResult, 0, len(s.skills))
	for _, item := range s.skills {
		results = append(results, skill.EmbeddingSearchResult{
			ID:                  item.id,
			Slug:                item.slug,
			Score:               cosineSimilarity(embedding, item.embedding),
			EmbeddingSourceHash: item.hash,
		})
	}
	sort.SliceStable(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})
	if topK > 0 && len(results) > topK {
		results = results[:topK]
	}
	return results, nil
}

func (s *dsr10VectorStore) EmbeddingSourceHashesByIDs(_ context.Context, ids []uuid.UUID) (map[uuid.UUID]string, error) {
	out := make(map[uuid.UUID]string, len(ids))
	for _, id := range ids {
		for _, item := range s.skills {
			if item.id == id {
				out[id] = item.hash
				break
			}
		}
	}
	return out, nil
}

type dsr10Embedder struct {
	vectorsByQuery map[string][]float32
}

func (e *dsr10Embedder) Embed(_ context.Context, text string) ([]float32, error) {
	return append([]float32(nil), e.vectorsByQuery[text]...), nil
}

func oneHotVector(index, length int) []float32 {
	out := make([]float32, length)
	out[index] = 1
	return out
}

func cosineSimilarity(a, b []float32) float64 {
	if len(a) == 0 || len(a) != len(b) {
		return 0
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
		return 0
	}
	return dot / (math.Sqrt(normA) * math.Sqrt(normB))
}

func percentileDuration(values []time.Duration, percentile float64) time.Duration {
	if len(values) == 0 {
		return 0
	}
	sorted := append([]time.Duration(nil), values...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
	idx := int(math.Ceil(percentile*float64(len(sorted)))) - 1
	if idx < 0 {
		idx = 0
	}
	if idx >= len(sorted) {
		idx = len(sorted) - 1
	}
	return sorted[idx]
}

func containsUUID(values []uuid.UUID, target uuid.UUID) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
