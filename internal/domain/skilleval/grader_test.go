package skilleval_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/skilleval"
)

// --- stub LLM client ---

type stubLLM struct {
	response string
	err      error
}

func (l *stubLLM) Complete(_ context.Context, _ string) (string, error) {
	return l.response, l.err
}

// --- exact_match ---

func TestGraderExactMatch_pass(t *testing.T) {
	g, err := skilleval.NewGrader(skilleval.GraderExactMatch, nil, nil)
	require.NoError(t, err)
	res, err := g.Grade(context.Background(), "hello world", "hello world")
	require.NoError(t, err)
	assert.True(t, res.Passed)
	assert.Equal(t, 1.0, res.Score)
}

func TestGraderExactMatch_fail(t *testing.T) {
	g, _ := skilleval.NewGrader(skilleval.GraderExactMatch, nil, nil)
	res, err := g.Grade(context.Background(), "hello world", "Hello World")
	require.NoError(t, err)
	assert.False(t, res.Passed)
	assert.Equal(t, 0.0, res.Score)
}

func TestGraderExactMatch_emptyExpected(t *testing.T) {
	g, _ := skilleval.NewGrader(skilleval.GraderExactMatch, nil, nil)
	res, err := g.Grade(context.Background(), "", "")
	require.NoError(t, err)
	assert.True(t, res.Passed)
}

// --- contains ---

func TestGraderContains_pass(t *testing.T) {
	g, _ := skilleval.NewGrader(skilleval.GraderContains, nil, nil)
	res, err := g.Grade(context.Background(), "world", "hello world!")
	require.NoError(t, err)
	assert.True(t, res.Passed)
}

func TestGraderContains_fail(t *testing.T) {
	g, _ := skilleval.NewGrader(skilleval.GraderContains, nil, nil)
	res, err := g.Grade(context.Background(), "missing", "hello world")
	require.NoError(t, err)
	assert.False(t, res.Passed)
}

// --- regex ---

func TestGraderRegex_pass(t *testing.T) {
	g, _ := skilleval.NewGrader(skilleval.GraderRegex, nil, nil)
	res, err := g.Grade(context.Background(), `\d{3}-\d{4}`, "call 555-1234 now")
	require.NoError(t, err)
	assert.True(t, res.Passed)
}

func TestGraderRegex_fail(t *testing.T) {
	g, _ := skilleval.NewGrader(skilleval.GraderRegex, nil, nil)
	res, err := g.Grade(context.Background(), `\d{3}-\d{4}`, "no phone number here")
	require.NoError(t, err)
	assert.False(t, res.Passed)
}

func TestGraderRegex_invalidPattern(t *testing.T) {
	g, _ := skilleval.NewGrader(skilleval.GraderRegex, nil, nil)
	_, err := g.Grade(context.Background(), `[invalid`, "text")
	require.Error(t, err)
}

// --- llm_judge ---

func TestGraderLLMJudge_pass(t *testing.T) {
	llm := &stubLLM{response: "PASS"}
	g, err := skilleval.NewGrader(skilleval.GraderLLMJudge, nil, llm)
	require.NoError(t, err)
	res, err := g.Grade(context.Background(), "contains a number", "The answer is 42")
	require.NoError(t, err)
	assert.True(t, res.Passed)
	assert.Equal(t, 1.0, res.Score)
}

func TestGraderLLMJudge_fail(t *testing.T) {
	llm := &stubLLM{response: "FAIL — no number found"}
	g, _ := skilleval.NewGrader(skilleval.GraderLLMJudge, nil, llm)
	res, err := g.Grade(context.Background(), "contains a number", "no number here")
	require.NoError(t, err)
	assert.False(t, res.Passed)
}

func TestGraderLLMJudge_llmError(t *testing.T) {
	llm := &stubLLM{err: fmt.Errorf("timeout")}
	g, _ := skilleval.NewGrader(skilleval.GraderLLMJudge, nil, llm)
	_, err := g.Grade(context.Background(), "x", "y")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "timeout")
}

func TestGraderLLMJudge_noClient(t *testing.T) {
	g, _ := skilleval.NewGrader(skilleval.GraderLLMJudge, nil, nil)
	_, err := g.Grade(context.Background(), "x", "y")
	require.Error(t, err)
}

func TestNewGrader_unknownType(t *testing.T) {
	_, err := skilleval.NewGrader("unknown_type", nil, nil)
	require.Error(t, err)
}
