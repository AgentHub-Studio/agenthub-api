package skilleval

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"strings"
)

// GradeResult holds the outcome of a single grader evaluation.
type GradeResult struct {
	Passed bool
	Score  float64 // 0.0–1.0; 1.0 means full pass
	Reason string
}

// Grader evaluates an actual output against an expected value.
type Grader interface {
	Grade(ctx context.Context, expected, actual string) (GradeResult, error)
}

// LLMClient is the narrow interface used by LLMJudgeGrader.
// Implemented by any AI chat client; the caller injects a concrete implementation.
type LLMClient interface {
	// Complete sends a prompt and returns the model's text response.
	Complete(ctx context.Context, prompt string) (string, error)
}

// EmbeddingClient computes text embeddings used by the semantic_similarity grader.
type EmbeddingClient interface {
	// Embed returns a dense vector representation of the given text.
	Embed(ctx context.Context, text string) ([]float64, error)
}

// NewGrader returns the Grader implementation for the given GraderType and config.
func NewGrader(t GraderType, config json.RawMessage, llm LLMClient) (Grader, error) {
	return NewGraderWithEmbedder(t, config, llm, nil)
}

// NewGraderWithEmbedder creates a Grader, allowing an optional EmbeddingClient for
// the semantic_similarity grader type.
func NewGraderWithEmbedder(t GraderType, config json.RawMessage, llm LLMClient, embedder EmbeddingClient) (Grader, error) {
	switch t {
	case GraderExactMatch:
		return &exactMatchGrader{}, nil
	case GraderContains:
		return &containsGrader{}, nil
	case GraderRegex:
		return &regexGrader{}, nil
	case GraderLLMJudge:
		var cfg llmJudgeConfig
		if len(config) > 2 {
			if err := json.Unmarshal(config, &cfg); err != nil {
				return nil, fmt.Errorf("skilleval: llm_judge config: %w", err)
			}
		}
		if cfg.PromptTemplate == "" {
			cfg.PromptTemplate = defaultLLMJudgePrompt
		}
		return &llmJudgeGrader{cfg: cfg, llm: llm}, nil
	case GraderSemanticSimilarity:
		var cfg semanticSimilarityConfig
		if len(config) > 2 {
			if err := json.Unmarshal(config, &cfg); err != nil {
				return nil, fmt.Errorf("skilleval: semantic_similarity config: %w", err)
			}
		}
		if cfg.Threshold == 0 {
			cfg.Threshold = 0.8
		}
		return &semanticSimilarityGrader{cfg: cfg, embedder: embedder}, nil
	default:
		return nil, fmt.Errorf("skilleval: unknown grader type %q", t)
	}
}

// --- exact_match ---

type exactMatchGrader struct{}

func (g *exactMatchGrader) Grade(_ context.Context, expected, actual string) (GradeResult, error) {
	passed := actual == expected
	score := 0.0
	if passed {
		score = 1.0
	}
	return GradeResult{Passed: passed, Score: score, Reason: fmt.Sprintf("expected %q, got %q", expected, actual)}, nil
}

// --- contains ---

type containsGrader struct{}

func (g *containsGrader) Grade(_ context.Context, expected, actual string) (GradeResult, error) {
	passed := strings.Contains(actual, expected)
	score := 0.0
	if passed {
		score = 1.0
	}
	return GradeResult{Passed: passed, Score: score, Reason: fmt.Sprintf("expected to contain %q", expected)}, nil
}

// --- regex ---

type regexGrader struct{}

func (g *regexGrader) Grade(_ context.Context, pattern, actual string) (GradeResult, error) {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return GradeResult{}, fmt.Errorf("skilleval: regex compile: %w", err)
	}
	passed := re.MatchString(actual)
	score := 0.0
	if passed {
		score = 1.0
	}
	return GradeResult{Passed: passed, Score: score, Reason: fmt.Sprintf("regex %q %s", pattern, matchWord(passed))}, nil
}

func matchWord(matched bool) string {
	if matched {
		return "matched"
	}
	return "did not match"
}

// --- llm_judge ---

// llmJudgeConfig holds configuration for the LLM judge grader.
type llmJudgeConfig struct {
	// PromptTemplate is a Go text/template with {{.Input}}, {{.Expected}}, {{.Actual}} variables.
	PromptTemplate string `json:"promptTemplate"`
}

const defaultLLMJudgePrompt = `You are an impartial evaluator. Assess whether the actual output satisfies the expected criteria.

Expected criteria: {{.Expected}}
Actual output: {{.Actual}}

Reply with exactly one word: PASS or FAIL.`

type llmJudgeGrader struct {
	cfg llmJudgeConfig
	llm LLMClient
}

func (g *llmJudgeGrader) Grade(ctx context.Context, expected, actual string) (GradeResult, error) {
	if g.llm == nil {
		return GradeResult{}, fmt.Errorf("skilleval: llm_judge: no LLM client configured")
	}

	prompt := strings.NewReplacer(
		"{{.Expected}}", expected,
		"{{.Actual}}", actual,
	).Replace(g.cfg.PromptTemplate)

	response, err := g.llm.Complete(ctx, prompt)
	if err != nil {
		return GradeResult{}, fmt.Errorf("skilleval: llm_judge: %w", err)
	}

	verdict := strings.TrimSpace(strings.ToUpper(response))
	passed := strings.HasPrefix(verdict, "PASS")
	score := 0.0
	if passed {
		score = 1.0
	}
	return GradeResult{Passed: passed, Score: score, Reason: "LLM verdict: " + strings.TrimSpace(response)}, nil
}

// --- semantic_similarity ---

// semanticSimilarityConfig configures the semantic similarity grader.
type semanticSimilarityConfig struct {
	// Threshold is the minimum cosine similarity required to pass (0.0–1.0, default 0.8).
	Threshold float64 `json:"threshold"`
}

type semanticSimilarityGrader struct {
	cfg      semanticSimilarityConfig
	embedder EmbeddingClient
}

func (g *semanticSimilarityGrader) Grade(ctx context.Context, expected, actual string) (GradeResult, error) {
	if g.embedder == nil {
		return GradeResult{}, fmt.Errorf("skilleval: semantic_similarity: no embedding client configured")
	}

	expectedVec, err := g.embedder.Embed(ctx, expected)
	if err != nil {
		return GradeResult{}, fmt.Errorf("skilleval: semantic_similarity: embed expected: %w", err)
	}
	actualVec, err := g.embedder.Embed(ctx, actual)
	if err != nil {
		return GradeResult{}, fmt.Errorf("skilleval: semantic_similarity: embed actual: %w", err)
	}

	sim, err := cosineSimilarity(expectedVec, actualVec)
	if err != nil {
		return GradeResult{}, fmt.Errorf("skilleval: semantic_similarity: %w", err)
	}

	passed := sim >= g.cfg.Threshold
	return GradeResult{
		Passed: passed,
		Score:  sim,
		Reason: fmt.Sprintf("cosine similarity %.4f (threshold %.2f)", sim, g.cfg.Threshold),
	}, nil
}

// cosineSimilarity computes the cosine similarity between two equal-length vectors.
func cosineSimilarity(a, b []float64) (float64, error) {
	if len(a) != len(b) {
		return 0, fmt.Errorf("vector length mismatch: %d vs %d", len(a), len(b))
	}
	if len(a) == 0 {
		return 0, fmt.Errorf("empty vectors")
	}
	var dot, normA, normB float64
	for i := range a {
		dot += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}
	if normA == 0 || normB == 0 {
		return 0, fmt.Errorf("zero-norm vector")
	}
	return dot / (math.Sqrt(normA) * math.Sqrt(normB)), nil
}
