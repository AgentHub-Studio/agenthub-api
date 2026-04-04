package agentic

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/AgentHub-Studio/agenthub-go-commons/ai"
)

// SideQuery executes a lightweight, off-main-loop LLM call. Unlike the main
// agentic loop or subtasks, side queries are single-shot API calls that do not
// feed back into conversation history and are not tracked against session token
// limits. They are used for internal decision-making and housekeeping tasks.
//
// Key characteristics:
//   - Single API call, no iteration loop
//   - Not tracked in session COGS (excluded from cumulative token accounting)
//   - Supports forced tool choice for structured output
//   - Supports stop sequences for early termination
//   - Supports thinking disable for fast responses
//
// Inspired by Claude Code's sideQuery() in utils/sideQuery.ts.
type SideQuery struct {
	model ai.ChatModel
	cfg   SideQueryConfig
}

// SideQueryConfig holds default configuration for side queries.
type SideQueryConfig struct {
	// DefaultModel is the model to use when not specified per-query.
	DefaultModel string
	// DefaultMaxTokens is the default token limit per side query. Default: 1024.
	DefaultMaxTokens int
	// DefaultTemperature is the default temperature. Default: 0 (deterministic).
	DefaultTemperature float64
}

// DefaultSideQueryConfig returns sensible defaults for side queries.
func DefaultSideQueryConfig() SideQueryConfig {
	return SideQueryConfig{
		DefaultModel:       "claude-haiku-4-5-20251001",
		DefaultMaxTokens:   1024,
		DefaultTemperature: 0,
	}
}

// SideQueryOptions configures a single side query invocation.
type SideQueryOptions struct {
	// Model overrides the default model. Empty uses SideQueryConfig.DefaultModel.
	Model string
	// SystemPrompt is the system message for this query.
	SystemPrompt string
	// Messages is the conversation to send.
	Messages []ai.Message
	// Tools are the tool definitions available to the model.
	Tools []ai.Tool
	// ToolChoice forces a specific tool selection. Nil means "auto".
	ToolChoice *ai.ToolChoice
	// StopSequences causes generation to stop at any matching string.
	StopSequences []string
	// MaxTokens overrides the default token limit. 0 uses default.
	MaxTokens int
	// Temperature overrides the default temperature. Negative means use default.
	Temperature *float64
	// DisableThinking disables extended thinking for fast responses.
	DisableThinking bool
	// QuerySource identifies this call for analytics attribution.
	QuerySource QuerySource
}

// SideQueryResult holds the response from a side query.
type SideQueryResult struct {
	// Content is the text response from the model.
	Content string
	// ToolCalls contains any tool calls the model made.
	ToolCalls []ai.ToolCall
	// FinishReason is the stop reason (stop, tool_calls, end_turn).
	FinishReason string
	// Usage tracks token consumption for this call (not added to session totals).
	Usage ai.Usage
	// Model is the model that actually served this query.
	Model string
}

// NewSideQuery creates a SideQuery executor with the given model and config.
func NewSideQuery(model ai.ChatModel, cfg SideQueryConfig) *SideQuery {
	if cfg.DefaultMaxTokens <= 0 {
		cfg.DefaultMaxTokens = 1024
	}
	return &SideQuery{model: model, cfg: cfg}
}

// Query executes a single side query and returns the result.
func (sq *SideQuery) Query(ctx context.Context, opts SideQueryOptions) (*SideQueryResult, error) {
	if sq.model == nil {
		return nil, fmt.Errorf("sidequery: model is nil")
	}

	model := opts.Model
	if model == "" {
		model = sq.cfg.DefaultModel
	}

	maxTokens := opts.MaxTokens
	if maxTokens <= 0 {
		maxTokens = sq.cfg.DefaultMaxTokens
	}

	temp := sq.cfg.DefaultTemperature
	if opts.Temperature != nil {
		temp = *opts.Temperature
	}

	chatOpts := ai.ChatOptions{
		Model:         model,
		MaxTokens:     maxTokens,
		Temperature:   temp,
		SystemMsg:     opts.SystemPrompt,
		Tools:         opts.Tools,
		ToolChoice:    opts.ToolChoice,
		StopSequences: opts.StopSequences,
	}

	if opts.DisableThinking {
		chatOpts.Thinking = &ai.ThinkingConfig{Type: ai.ThinkingDisabled}
	}

	resp, err := sq.model.Chat(ctx, opts.Messages, chatOpts)
	if err != nil {
		return nil, fmt.Errorf("sidequery: %w", err)
	}

	return &SideQueryResult{
		Content:      resp.Content,
		ToolCalls:    resp.ToolCalls,
		FinishReason: resp.FinishReason,
		Usage:        resp.Usage,
		Model:        resp.Model,
	}, nil
}

// QueryForText executes a side query and returns only the text content.
// Convenience wrapper for simple text-only queries.
func (sq *SideQuery) QueryForText(ctx context.Context, opts SideQueryOptions) (string, error) {
	result, err := sq.Query(ctx, opts)
	if err != nil {
		return "", err
	}
	return result.Content, nil
}

// QueryForToolCall executes a side query with forced tool choice and returns
// the first tool call's arguments as parsed JSON. This is the primary pattern
// for getting structured output from classifiers and validators.
//
// Inspired by Claude Code's sideQuery + ForcedToolChoice pattern in permissionExplainer.ts.
func (sq *SideQuery) QueryForToolCall(ctx context.Context, opts SideQueryOptions, toolName string) (json.RawMessage, error) {
	opts.ToolChoice = ai.ForcedToolChoice(toolName)

	result, err := sq.Query(ctx, opts)
	if err != nil {
		return nil, err
	}

	if len(result.ToolCalls) == 0 {
		return nil, fmt.Errorf("sidequery: expected tool call for %q but got none", toolName)
	}

	for _, tc := range result.ToolCalls {
		if tc.Function.Name == toolName {
			return json.RawMessage(tc.Function.Arguments), nil
		}
	}

	return json.RawMessage(result.ToolCalls[0].Function.Arguments), nil
}

// ClassifyWithXML executes a fast classification query using stop sequences to
// cut generation at the first XML block boundary. Returns the raw content
// (including the XML tags up to the stop sequence).
//
// Inspired by Claude Code's yoloClassifier.ts pattern with stop_sequences: ['</block>'].
func (sq *SideQuery) ClassifyWithXML(ctx context.Context, systemPrompt string, userMessage string, stopTag string) (string, error) {
	opts := SideQueryOptions{
		SystemPrompt:    systemPrompt,
		DisableThinking: true,
		Messages: []ai.Message{
			{Role: ai.RoleUser, Content: userMessage},
		},
		StopSequences: []string{stopTag},
		MaxTokens:     512,
		QuerySource:   "classifier",
	}

	return sq.QueryForText(ctx, opts)
}
