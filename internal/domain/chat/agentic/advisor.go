package agentic

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"
)

// Advisor pattern for collaborative decision-making.
//
// Inspired by Claude Code's advisor.ts — wraps a stronger review model
// as a consultation step. The advisor sees the conversation context and
// provides recommendations before substantive work. Emphasizes
// reconciliation when evidence conflicts with advisor recommendations.

// AdvisorDecision represents the advisor's recommendation.
type AdvisorDecision string

const (
	// AdvisorApprove signals the advisor approves the current approach.
	AdvisorApprove AdvisorDecision = "approve"
	// AdvisorRevise signals the advisor suggests revisions.
	AdvisorRevise AdvisorDecision = "revise"
	// AdvisorReject signals the advisor recommends a different approach.
	AdvisorReject AdvisorDecision = "reject"
)

// AdvisorRequest represents a consultation request to the advisor.
type AdvisorRequest struct {
	// ID uniquely identifies this advisor consultation.
	ID string `json:"id"`
	// Context is the conversation or task context for the advisor.
	Context string `json:"context"`
	// ProposedAction describes what the agent plans to do.
	ProposedAction string `json:"proposedAction"`
	// Model is the advisor model to use (e.g., a stronger model).
	Model string `json:"model,omitempty"`
	// CreatedAt is when the request was created.
	CreatedAt time.Time `json:"createdAt"`
}

// AdvisorResponse holds the advisor's feedback.
type AdvisorResponse struct {
	// RequestID links back to the request.
	RequestID string `json:"requestId"`
	// Decision is the advisor's recommendation.
	Decision AdvisorDecision `json:"decision"`
	// Reasoning explains the decision.
	Reasoning string `json:"reasoning"`
	// Suggestions are specific improvements (for revise/reject).
	Suggestions []string `json:"suggestions,omitempty"`
	// Confidence is a 0-1 score of the advisor's confidence.
	Confidence float64 `json:"confidence"`
	// Duration is how long the consultation took.
	Duration time.Duration `json:"duration"`
}

// AdvisorFunc is the function signature for an advisor implementation.
// It receives a request and returns a response.
type AdvisorFunc func(ctx context.Context, req AdvisorRequest) (AdvisorResponse, error)

// AdvisorConfig configures the advisor service.
type AdvisorConfig struct {
	// Enabled controls whether the advisor is active.
	Enabled bool
	// Model is the default advisor model.
	Model string
	// MaxIterations limits re-consultation rounds.
	MaxIterations int
}

// AdvisorService manages advisor consultations.
type AdvisorService struct {
	mu       sync.RWMutex
	config   AdvisorConfig
	advisor  AdvisorFunc
	history  []advisorRecord
	counter  int
	enabled  bool
}

type advisorRecord struct {
	request  AdvisorRequest
	response AdvisorResponse
}

// NewAdvisorService creates an advisor service with the given configuration.
func NewAdvisorService(config AdvisorConfig, fn AdvisorFunc) *AdvisorService {
	if config.MaxIterations <= 0 {
		config.MaxIterations = 3
	}
	return &AdvisorService{
		config:  config,
		advisor: fn,
		enabled: config.Enabled,
	}
}

// IsEnabled returns whether the advisor is active.
func (s *AdvisorService) IsEnabled() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.enabled
}

// SetEnabled toggles the advisor on or off.
func (s *AdvisorService) SetEnabled(enabled bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.enabled = enabled
}

// Consult sends a consultation request to the advisor.
// Returns the response or an error if the advisor is disabled or fails.
func (s *AdvisorService) Consult(ctx context.Context, conversationContext, proposedAction string) (AdvisorResponse, error) {
	s.mu.Lock()
	if !s.enabled {
		s.mu.Unlock()
		return AdvisorResponse{}, fmt.Errorf("advisor is disabled")
	}
	s.counter++
	id := fmt.Sprintf("adv-%d-%d", time.Now().UnixMilli(), s.counter)
	s.mu.Unlock()

	req := AdvisorRequest{
		ID:             id,
		Context:        conversationContext,
		ProposedAction: proposedAction,
		Model:          s.config.Model,
		CreatedAt:      time.Now(),
	}

	start := time.Now()
	resp, err := s.advisor(ctx, req)
	if err != nil {
		return AdvisorResponse{}, fmt.Errorf("advisor consultation failed: %w", err)
	}

	resp.RequestID = id
	resp.Duration = time.Since(start)

	s.mu.Lock()
	s.history = append(s.history, advisorRecord{request: req, response: resp})
	s.mu.Unlock()

	return resp, nil
}

// ConsultUntilApproved repeatedly consults the advisor until approval
// or the max iteration limit is reached. Returns the final response.
func (s *AdvisorService) ConsultUntilApproved(
	ctx context.Context,
	conversationContext, proposedAction string,
	revise func(suggestions []string) string,
) (AdvisorResponse, error) {
	current := proposedAction

	for i := 0; i < s.config.MaxIterations; i++ {
		resp, err := s.Consult(ctx, conversationContext, current)
		if err != nil {
			return AdvisorResponse{}, err
		}

		if resp.Decision == AdvisorApprove {
			return resp, nil
		}

		if revise == nil {
			return resp, nil
		}

		current = revise(resp.Suggestions)
	}

	// Max iterations reached — return last response.
	return s.LastResponse()
}

// LastResponse returns the most recent advisor response.
func (s *AdvisorService) LastResponse() (AdvisorResponse, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if len(s.history) == 0 {
		return AdvisorResponse{}, fmt.Errorf("no advisor consultations recorded")
	}
	return s.history[len(s.history)-1].response, nil
}

// ConsultationCount returns the total number of consultations.
func (s *AdvisorService) ConsultationCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.history)
}

// SupportsAdvisor checks if a model name is valid for advisor use.
func SupportsAdvisor(model string) bool {
	lower := strings.ToLower(model)
	return strings.Contains(lower, "opus") ||
		strings.Contains(lower, "sonnet") ||
		strings.Contains(lower, "gpt-4") ||
		strings.Contains(lower, "claude")
}
