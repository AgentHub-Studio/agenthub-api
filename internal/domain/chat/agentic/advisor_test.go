package agentic_test

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/chat/agentic"
)

// --- AdvisorDecision ---

func TestAdvisorDecision_Values(t *testing.T) {
	assert.Equal(t, agentic.AdvisorDecision("approve"), agentic.AdvisorApprove)
	assert.Equal(t, agentic.AdvisorDecision("revise"), agentic.AdvisorRevise)
	assert.Equal(t, agentic.AdvisorDecision("reject"), agentic.AdvisorReject)
}

// --- SupportsAdvisor ---

func TestSupportsAdvisor(t *testing.T) {
	assert.True(t, agentic.SupportsAdvisor("claude-opus-4"))
	assert.True(t, agentic.SupportsAdvisor("claude-sonnet-4"))
	assert.True(t, agentic.SupportsAdvisor("gpt-4-turbo"))
	assert.True(t, agentic.SupportsAdvisor("Claude-3-Opus"))
	assert.False(t, agentic.SupportsAdvisor("llama-3"))
	assert.False(t, agentic.SupportsAdvisor("mistral-7b"))
}

// --- NewAdvisorService ---

func TestNewAdvisorService(t *testing.T) {
	svc := agentic.NewAdvisorService(agentic.AdvisorConfig{Enabled: true}, nil)
	assert.True(t, svc.IsEnabled())
	assert.Equal(t, 0, svc.ConsultationCount())
}

func TestNewAdvisorService_DisabledByDefault(t *testing.T) {
	svc := agentic.NewAdvisorService(agentic.AdvisorConfig{}, nil)
	assert.False(t, svc.IsEnabled())
}

// --- SetEnabled ---

func TestAdvisorService_SetEnabled(t *testing.T) {
	svc := agentic.NewAdvisorService(agentic.AdvisorConfig{}, nil)
	svc.SetEnabled(true)
	assert.True(t, svc.IsEnabled())
	svc.SetEnabled(false)
	assert.False(t, svc.IsEnabled())
}

// --- Consult ---

func TestAdvisorService_Consult_Success(t *testing.T) {
	fn := func(_ context.Context, req agentic.AdvisorRequest) (agentic.AdvisorResponse, error) {
		return agentic.AdvisorResponse{
			Decision:   agentic.AdvisorApprove,
			Reasoning:  "looks good",
			Confidence: 0.95,
		}, nil
	}

	svc := agentic.NewAdvisorService(agentic.AdvisorConfig{Enabled: true, Model: "opus"}, fn)
	resp, err := svc.Consult(context.Background(), "refactoring auth", "extract middleware")

	require.NoError(t, err)
	assert.Equal(t, agentic.AdvisorApprove, resp.Decision)
	assert.Equal(t, "looks good", resp.Reasoning)
	assert.NotEmpty(t, resp.RequestID)
	assert.Greater(t, resp.Duration, time.Duration(0))
	assert.Equal(t, 1, svc.ConsultationCount())
}

func TestAdvisorService_Consult_Disabled(t *testing.T) {
	svc := agentic.NewAdvisorService(agentic.AdvisorConfig{Enabled: false}, nil)
	_, err := svc.Consult(context.Background(), "ctx", "action")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "disabled")
}

func TestAdvisorService_Consult_Error(t *testing.T) {
	fn := func(_ context.Context, _ agentic.AdvisorRequest) (agentic.AdvisorResponse, error) {
		return agentic.AdvisorResponse{}, fmt.Errorf("model timeout")
	}

	svc := agentic.NewAdvisorService(agentic.AdvisorConfig{Enabled: true}, fn)
	_, err := svc.Consult(context.Background(), "ctx", "action")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "model timeout")
}

func TestAdvisorService_Consult_RecordsHistory(t *testing.T) {
	fn := func(_ context.Context, _ agentic.AdvisorRequest) (agentic.AdvisorResponse, error) {
		return agentic.AdvisorResponse{Decision: agentic.AdvisorApprove}, nil
	}

	svc := agentic.NewAdvisorService(agentic.AdvisorConfig{Enabled: true}, fn)
	svc.Consult(context.Background(), "ctx1", "action1")
	svc.Consult(context.Background(), "ctx2", "action2")
	assert.Equal(t, 2, svc.ConsultationCount())
}

// --- ConsultUntilApproved ---

func TestAdvisorService_ConsultUntilApproved_ImmediateApproval(t *testing.T) {
	fn := func(_ context.Context, _ agentic.AdvisorRequest) (agentic.AdvisorResponse, error) {
		return agentic.AdvisorResponse{Decision: agentic.AdvisorApprove}, nil
	}

	svc := agentic.NewAdvisorService(agentic.AdvisorConfig{Enabled: true, MaxIterations: 5}, fn)
	resp, err := svc.ConsultUntilApproved(context.Background(), "ctx", "action", nil)
	require.NoError(t, err)
	assert.Equal(t, agentic.AdvisorApprove, resp.Decision)
	assert.Equal(t, 1, svc.ConsultationCount())
}

func TestAdvisorService_ConsultUntilApproved_ReviseThenApprove(t *testing.T) {
	var callCount int32

	fn := func(_ context.Context, req agentic.AdvisorRequest) (agentic.AdvisorResponse, error) {
		n := atomic.AddInt32(&callCount, 1)
		if n < 3 {
			return agentic.AdvisorResponse{
				Decision:    agentic.AdvisorRevise,
				Suggestions: []string{"add error handling"},
			}, nil
		}
		return agentic.AdvisorResponse{Decision: agentic.AdvisorApprove}, nil
	}

	revise := func(suggestions []string) string {
		return "revised action with: " + suggestions[0]
	}

	svc := agentic.NewAdvisorService(agentic.AdvisorConfig{Enabled: true, MaxIterations: 5}, fn)
	resp, err := svc.ConsultUntilApproved(context.Background(), "ctx", "action", revise)
	require.NoError(t, err)
	assert.Equal(t, agentic.AdvisorApprove, resp.Decision)
	assert.Equal(t, 3, svc.ConsultationCount())
}

func TestAdvisorService_ConsultUntilApproved_MaxIterations(t *testing.T) {
	fn := func(_ context.Context, _ agentic.AdvisorRequest) (agentic.AdvisorResponse, error) {
		return agentic.AdvisorResponse{Decision: agentic.AdvisorRevise, Reasoning: "not yet"}, nil
	}

	revise := func(suggestions []string) string { return "try again" }

	svc := agentic.NewAdvisorService(agentic.AdvisorConfig{Enabled: true, MaxIterations: 2}, fn)
	resp, err := svc.ConsultUntilApproved(context.Background(), "ctx", "action", revise)
	require.NoError(t, err)
	assert.Equal(t, agentic.AdvisorRevise, resp.Decision)
	assert.Equal(t, 2, svc.ConsultationCount())
}

func TestAdvisorService_ConsultUntilApproved_NilRevise(t *testing.T) {
	fn := func(_ context.Context, _ agentic.AdvisorRequest) (agentic.AdvisorResponse, error) {
		return agentic.AdvisorResponse{Decision: agentic.AdvisorReject, Reasoning: "bad approach"}, nil
	}

	svc := agentic.NewAdvisorService(agentic.AdvisorConfig{Enabled: true}, fn)
	resp, err := svc.ConsultUntilApproved(context.Background(), "ctx", "action", nil)
	require.NoError(t, err)
	assert.Equal(t, agentic.AdvisorReject, resp.Decision)
	assert.Equal(t, 1, svc.ConsultationCount(), "should stop after first non-approve with nil revise")
}

// --- LastResponse ---

func TestAdvisorService_LastResponse_NoHistory(t *testing.T) {
	svc := agentic.NewAdvisorService(agentic.AdvisorConfig{Enabled: true}, nil)
	_, err := svc.LastResponse()
	assert.Error(t, err)
}

func TestAdvisorService_LastResponse(t *testing.T) {
	var callNum int32
	fn := func(_ context.Context, _ agentic.AdvisorRequest) (agentic.AdvisorResponse, error) {
		n := atomic.AddInt32(&callNum, 1)
		return agentic.AdvisorResponse{
			Decision:  agentic.AdvisorApprove,
			Reasoning: fmt.Sprintf("call-%d", n),
		}, nil
	}

	svc := agentic.NewAdvisorService(agentic.AdvisorConfig{Enabled: true}, fn)
	svc.Consult(context.Background(), "ctx1", "a1")
	svc.Consult(context.Background(), "ctx2", "a2")

	resp, err := svc.LastResponse()
	require.NoError(t, err)
	assert.Equal(t, "call-2", resp.Reasoning)
}

// --- AdvisorRequest fields ---

func TestAdvisorRequest_Fields(t *testing.T) {
	req := agentic.AdvisorRequest{
		ID:             "adv-1",
		Context:        "refactoring",
		ProposedAction: "extract service",
		Model:          "opus",
		CreatedAt:      time.Now(),
	}
	assert.Equal(t, "adv-1", req.ID)
	assert.Equal(t, "opus", req.Model)
}
