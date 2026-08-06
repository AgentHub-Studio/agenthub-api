package agentic

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/AgentHub-Studio/agenthub-api/internal/randutil"
	"github.com/AgentHub-Studio/agenthub-go-commons/ai"
)

// FallbackResult is returned by retryStreamWithFallback to indicate which model was used.
type FallbackResult struct {
	Stream           <-chan ai.StreamChunk
	ModelUsed        string // the model that succeeded
	WasFallback      bool   // true if a fallback model was used
	UsedNonStreaming bool   // true if the primary model recovered through Chat
}

// retryStreamWithFallback wraps retryStream with model fallback support.
// After exhausting retries on the primary model, it tries each fallback model
// in order (1 attempt per fallback). Returns the stream plus metadata about
// which model was used.
func retryStreamWithFallback(
	ctx context.Context,
	model ai.ChatModel,
	messages []ai.Message,
	opts ai.ChatOptions,
	config RunConfig,
	onFallback func(from, to string, err error),
) (*FallbackResult, error) {
	return retryStreamWithFallbackSource(ctx, model, messages, opts, config, SourceMainLoop, onFallback)
}

// retryStreamWithFallbackSource is the internal implementation that accepts a QuerySource
// for differentiating foreground vs background retry policies.
func retryStreamWithFallbackSource(
	ctx context.Context,
	model ai.ChatModel,
	messages []ai.Message,
	opts ai.ChatOptions,
	config RunConfig,
	source QuerySource,
	onFallback func(from, to string, err error),
) (*FallbackResult, error) {
	// Try primary model with full retries.
	stream, err := retryStream(ctx, model, messages, opts, config.RetryMaxAttempts, source)
	if err == nil {
		return &FallbackResult{Stream: stream, ModelUsed: opts.Model}, nil
	}

	// A provider may reject its streaming endpoint while accepting the equivalent
	// non-streaming request. Convert that response back into StreamChunks so the
	// runner keeps its public SSE contract.
	nonStreaming, attempted, nonStreamingErr := retryNonStreamingAfterStreamFailure(
		ctx,
		model,
		messages,
		opts,
		DefaultStreamFallbackConfig(),
		err,
	)
	if attempted && nonStreamingErr == nil {
		modelUsed := opts.Model
		if nonStreaming.Model != "" {
			modelUsed = nonStreaming.Model
		}
		return &FallbackResult{
			Stream:           nonStreaming.Stream,
			ModelUsed:        modelUsed,
			UsedNonStreaming: true,
		}, nil
	}

	fallbacks := effectiveFallbackSteps(config)

	// No fallbacks configured — return the original error.
	if len(fallbacks) == 0 {
		if attempted {
			return nil, nonStreamingErr
		}
		return nil, err
	}

	// Check if the error type qualifies for fallback.
	if !shouldFallback(err, config) {
		return nil, err
	}

	primaryModel := opts.Model
	primaryErr := err

	// Try each fallback model in configured order.
	for _, fallback := range fallbacks {
		slog.Warn("falling back to alternative model",
			"from", primaryModel,
			"to", fallback.Model,
			"primaryError", primaryErr,
		)

		if onFallback != nil {
			onFallback(primaryModel, fallback.Model, primaryErr)
		}

		fallbackOpts := opts
		fallbackOpts.Model = fallback.Model

		stream, err := retryStream(ctx, model, messages, fallbackOpts, fallbackAttempts(fallback), source)
		if err == nil {
			return &FallbackResult{
				Stream:      stream,
				ModelUsed:   fallback.Model,
				WasFallback: true,
			}, nil
		}

		slog.Warn("fallback model also failed",
			"model", fallback.Model,
			"error", err,
		)
	}

	return nil, fmt.Errorf("all models failed (primary: %s, fallbacks: %v): %w",
		primaryModel, fallbackModelNames(fallbacks), primaryErr)
}

func effectiveFallbackSteps(config RunConfig) []ModelFallbackStep {
	if len(config.ModelFallbackChain) > 0 {
		return config.ModelFallbackChain
	}
	steps := make([]ModelFallbackStep, 0, len(config.ModelFallbacks))
	for _, model := range config.ModelFallbacks {
		model = strings.TrimSpace(model)
		if model == "" {
			continue
		}
		steps = append(steps, ModelFallbackStep{Model: model, MaxRetries: 1})
	}
	return steps
}

func fallbackAttempts(step ModelFallbackStep) int {
	if step.MaxRetries > 0 {
		return step.MaxRetries
	}
	return 1
}

func fallbackModelNames(steps []ModelFallbackStep) []string {
	names := make([]string, 0, len(steps))
	for _, step := range steps {
		names = append(names, step.Model)
	}
	return names
}

// shouldFallback checks if the error type qualifies for model fallback
// based on the configuration.
func shouldFallback(err error, config RunConfig) bool {
	if err == nil {
		return false
	}

	errType := classifyTransientError(err)
	switch errType {
	case errorTypeRateLimit:
		return config.IsFallbackOnRateLimit()
	case errorTypeOverload:
		return config.IsFallbackOnOverload()
	case errorTypeTimeout:
		return config.IsFallbackOnTimeout()
	default:
		return false
	}
}

// errorType classifies transient errors for fallback routing.
type errorType int

const (
	errorTypeUnknown   errorType = iota
	errorTypeRateLimit           // 429, 529, "rate limit"
	errorTypeOverload            // 502, 503, "overloaded"
	errorTypeTimeout             // "timeout", "connection reset"
)

// classifyTransientError determines the category of a transient error.
func classifyTransientError(err error) errorType {
	if err == nil {
		return errorTypeUnknown
	}
	msg := strings.ToLower(err.Error())

	// Rate limit patterns.
	if strings.Contains(msg, "429") || strings.Contains(msg, "529") || strings.Contains(msg, "rate limit") {
		return errorTypeRateLimit
	}

	// Overload patterns.
	if strings.Contains(msg, "502") || strings.Contains(msg, "503") || strings.Contains(msg, "overloaded") {
		return errorTypeOverload
	}

	// Timeout patterns.
	if strings.Contains(msg, "timeout") || strings.Contains(msg, "connection reset") || strings.Contains(msg, "connection refused") || strings.Contains(msg, "eof") {
		return errorTypeTimeout
	}

	return errorTypeUnknown
}

// retryStream wraps ChatModel.ChatStream with exponential backoff for transient errors.
// It retries up to maxAttempts times (total attempts, not retries). If maxAttempts <= 1
// it calls once without retry. The source parameter controls retry policy for capacity
// errors (529): foreground sources retry, background sources bail immediately.
func retryStream(
	ctx context.Context,
	model ai.ChatModel,
	messages []ai.Message,
	opts ai.ChatOptions,
	maxAttempts int,
	source QuerySource,
) (<-chan ai.StreamChunk, error) {
	if maxAttempts <= 1 {
		return model.ChatStream(ctx, messages, opts)
	}

	const max529Retries = 3 // stop retrying 529s after this many consecutive hits

	var lastErr error
	consecutive529 := 0
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		stream, err := model.ChatStream(ctx, messages, opts)
		if err == nil {
			return stream, nil
		}

		lastErr = err

		// Abort errors (context cancelled/deadline exceeded) — don't retry.
		if isAbortError(err) {
			return nil, err
		}

		// Media size errors — surface to user, don't retry.
		if isMediaSizeError(err) {
			return nil, err
		}

		// Prompt too long — needs compaction, not retry.
		if isPromptTooLong(err) {
			return nil, err
		}

		if !isTransientError(err) {
			return nil, err
		}

		// Track consecutive 529 errors — give up after max529Retries to avoid
		// prolonged stalls during major outages. Inspired by Claude Code's MAX_529_RETRIES.
		if isOverloadError(err) {
			consecutive529++
			if consecutive529 >= max529Retries {
				slog.Warn("giving up after consecutive 529 errors",
					"consecutive529", consecutive529,
					"source", source,
					"error", err,
				)
				return nil, fmt.Errorf("service overloaded after %d consecutive 529 errors: %w", consecutive529, err)
			}
		} else {
			consecutive529 = 0
		}

		// Background sources don't retry on capacity overload (529) to avoid
		// amplifying cascades during overload events.
		if isOverloadError(err) && !source.IsForegroundSource() {
			slog.Warn("skipping retry for background source during overload",
				"source", source,
				"error", err,
			)
			return nil, err
		}

		if attempt == maxAttempts {
			break
		}

		// Use Retry-After from error if available, otherwise exponential backoff with jitter.
		backoff := parseRetryAfter(err.Error())
		if backoff == 0 {
			base := math.Pow(2, float64(attempt-1))
			// Add ±25% jitter to prevent thundering herd on shared rate limits.
			jitter := base * 0.25 * (2*randutil.Float64() - 1) // [-25%, +25%]
			backoff = time.Duration((base+jitter)*1000) * time.Millisecond
		}
		slog.Warn("retrying LLM call after transient error",
			"attempt", attempt,
			"maxAttempts", maxAttempts,
			"backoff", backoff,
			"source", source,
			"error", err,
		)

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(backoff):
		}
	}

	return nil, fmt.Errorf("all %d attempts failed: %w", maxAttempts, lastErr)
}

// retryAfterRe matches "retry-after: N", "retry_after: N", or "retry after Ns" patterns.
var retryAfterRe = regexp.MustCompile(`(?i)retry[\s_-]after[:\s]+(\d+\.?\d*)`)

// tryAgainInRe matches OpenAI's "try again in 24.084s" / "Please try again in Ns" format.
var tryAgainInRe = regexp.MustCompile(`(?i)try again in\s+(\d+\.?\d*)`)

// parseRetryAfter extracts a Retry-After duration from an error message.
// Supports both "retry-after: N" (standard header) and "try again in Ns" (OpenAI).
// Caps at 60s to prevent excessive waits from malformed responses.
func parseRetryAfter(errMsg string) time.Duration {
	matches := retryAfterRe.FindStringSubmatch(errMsg)
	if len(matches) < 2 {
		matches = tryAgainInRe.FindStringSubmatch(errMsg)
	}
	if len(matches) < 2 {
		return 0
	}
	seconds, err := strconv.ParseFloat(matches[1], 64)
	if err != nil || seconds <= 0 {
		return 0
	}
	if seconds > 60 {
		seconds = 60
	}
	return time.Duration(seconds * float64(time.Second))
}

// isTransientError returns true for errors that are likely transient and worth retrying.
func isTransientError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()

	// Rate limit and overload status codes.
	for _, code := range []string{"429", "529", "503", "502"} {
		if strings.Contains(msg, code) {
			return true
		}
	}

	// Common transient error patterns.
	transientPatterns := []string{
		"rate limit",
		"overloaded",
		"timeout",
		"connection reset",
		"connection refused",
		"eof",
		"temporary",
	}
	lower := strings.ToLower(msg)
	for _, pat := range transientPatterns {
		if strings.Contains(lower, pat) {
			return true
		}
	}

	return false
}

// isOverloadError returns true if the error indicates a capacity overload (529/503).
// These errors benefit from differentiated retry: foreground sources retry,
// background sources bail to avoid amplifying cascades.
func isOverloadError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "529") || strings.Contains(msg, "overloaded")
}

// contextOverflowRe matches "input length and max_tokens exceed context limit: X + Y > Z"
// from Anthropic API error messages. Distinct from prompt_too_long — this means the
// prompt fits, but prompt + max_tokens would exceed the limit. The fix is to reduce
// max_tokens, not compact the context.
// Inspired by Claude Code's withRetry.ts context overflow handling.
var contextOverflowRe = regexp.MustCompile(`(?i)input.+max_tokens.+exceed.+context.+:\s*(\d+)\s*\+\s*(\d+)\s*>\s*(\d+)`)

// ContextOverflowCounts holds the parsed input, max_tokens, and context limit
// from a context overflow error.
type ContextOverflowCounts struct {
	InputTokens  int
	MaxTokens    int
	ContextLimit int
}

// parseContextOverflow extracts token counts from a context overflow error.
// Returns zero values if the error format is unrecognised.
func parseContextOverflow(errMsg string) ContextOverflowCounts {
	matches := contextOverflowRe.FindStringSubmatch(errMsg)
	if len(matches) < 4 {
		return ContextOverflowCounts{}
	}
	input, err1 := strconv.Atoi(matches[1])
	maxTok, err2 := strconv.Atoi(matches[2])
	contextLim, err3 := strconv.Atoi(matches[3])
	if err1 != nil || err2 != nil || err3 != nil {
		return ContextOverflowCounts{}
	}
	return ContextOverflowCounts{InputTokens: input, MaxTokens: maxTok, ContextLimit: contextLim}
}

// computeAdjustedMaxTokens calculates a reduced max_tokens that fits within
// the context limit. Returns 0 if the overflow can't be determined.
// Uses a 1000-token safety margin. Minimum returned value is 3000.
// Inspired by Claude Code's withRetry.ts adjustedMaxTokens logic.
func computeAdjustedMaxTokens(errMsg string) int {
	counts := parseContextOverflow(errMsg)
	if counts.InputTokens == 0 || counts.ContextLimit == 0 {
		return 0
	}
	adjusted := counts.ContextLimit - counts.InputTokens - 1000
	if adjusted < 3000 {
		adjusted = 3000
	}
	return adjusted
}

// isContextOverflow returns true if the error indicates an input + max_tokens
// overflow. This is distinct from prompt_too_long: the prompt fits, but asking
// for max_tokens output would exceed the model's context window.
func isContextOverflow(err error) bool {
	if err == nil {
		return false
	}
	return contextOverflowRe.MatchString(err.Error())
}

// ptlTokenRe matches "prompt is too long: NNN tokens > NNN" patterns in error messages.
// This format is used by the Anthropic API. Vertex/Bedrock variants may differ.
var ptlTokenRe = regexp.MustCompile(`(?i)prompt is too long[^0-9]*(\d+)\s*tokens?\s*>\s*(\d+)`)

// PromptTooLongTokenCounts holds the parsed actual and limit token counts
// from a prompt-too-long error message.
type PromptTooLongTokenCounts struct {
	ActualTokens int
	LimitTokens  int
}

// parsePromptTooLongTokenCounts extracts actual and limit token counts from
// a prompt-too-long error message. Returns zero values if the error format
// is unrecognised. Inspired by Claude Code's parsePromptTooLongTokenCounts.
func parsePromptTooLongTokenCounts(errMsg string) PromptTooLongTokenCounts {
	matches := ptlTokenRe.FindStringSubmatch(errMsg)
	if len(matches) < 3 {
		return PromptTooLongTokenCounts{}
	}
	actual, err1 := strconv.Atoi(matches[1])
	limit, err2 := strconv.Atoi(matches[2])
	if err1 != nil || err2 != nil {
		return PromptTooLongTokenCounts{}
	}
	return PromptTooLongTokenCounts{ActualTokens: actual, LimitTokens: limit}
}

// getPromptTooLongTokenGap returns how many tokens over the limit the prompt is,
// or 0 if the gap can't be determined. This gap is used by the ContextManager
// to decide how many messages to drop for precise truncation rather than the
// fallback 20% heuristic. Inspired by Claude Code's getPromptTooLongTokenGap.
func getPromptTooLongTokenGap(errMsg string) int {
	counts := parsePromptTooLongTokenCounts(errMsg)
	if counts.ActualTokens == 0 || counts.LimitTokens == 0 {
		return 0
	}
	gap := counts.ActualTokens - counts.LimitTokens
	if gap > 0 {
		return gap
	}
	return 0
}

// APIErrorClass categorises API errors for analytics and differentiated retry.
// Inspired by Claude Code's classifyAPIError.
type APIErrorClass string

const (
	ErrorClassNone            APIErrorClass = ""
	ErrorClassRateLimit       APIErrorClass = "rate_limit"
	ErrorClassServerOverload  APIErrorClass = "server_overload"
	ErrorClassConnection      APIErrorClass = "connection_error"
	ErrorClassStaleConnection APIErrorClass = "stale_connection"
	ErrorClassTimeout         APIErrorClass = "api_timeout"
	ErrorClassAuth            APIErrorClass = "auth_error"
	ErrorClassPromptTooLong   APIErrorClass = "prompt_too_long"
	ErrorClassMediaSize       APIErrorClass = "media_size_error"
	ErrorClassAborted         APIErrorClass = "aborted"
	ErrorClassServerError     APIErrorClass = "server_error"
	ErrorClassUnknown         APIErrorClass = "unknown"
)

// classifyAPIError maps an error to a structured class for analytics and retry decisions.
func classifyAPIError(err error) APIErrorClass {
	if err == nil {
		return ErrorClassNone
	}
	msg := strings.ToLower(err.Error())

	// Abort — context cancelled or deadline exceeded.
	if isAbortError(err) {
		return ErrorClassAborted
	}

	// Prompt too long — needs compaction, not retry.
	if isPromptTooLong(err) {
		return ErrorClassPromptTooLong
	}

	// Media size — image/PDF too large.
	if isMediaSizeError(err) {
		return ErrorClassMediaSize
	}

	// Timeout.
	if strings.Contains(msg, "timeout") {
		return ErrorClassTimeout
	}

	// Stale connection (ECONNRESET/EPIPE) — keep-alive socket died.
	if isStaleConnectionError(err) {
		return ErrorClassStaleConnection
	}

	// Broader connection errors.
	if isConnectionError(err) {
		return ErrorClassConnection
	}

	// Rate limit (429).
	if strings.Contains(msg, "429") || strings.Contains(msg, "rate limit") {
		return ErrorClassRateLimit
	}

	// Server overload (529).
	if isOverloadError(err) {
		return ErrorClassServerOverload
	}

	// Auth errors.
	if strings.Contains(msg, "401") || strings.Contains(msg, "403") ||
		strings.Contains(msg, "authentication") || strings.Contains(msg, "unauthorized") ||
		strings.Contains(msg, "invalid api key") {
		return ErrorClassAuth
	}

	// Server errors (5xx).
	if strings.Contains(msg, "500") || strings.Contains(msg, "502") || strings.Contains(msg, "503") {
		return ErrorClassServerError
	}

	return ErrorClassUnknown
}

// connectionErrorCodes lists OS-level error codes that indicate a network/connection problem.
// Inspired by Claude Code's CONNECTION_ERROR_CODES.
var connectionErrorCodes = []string{
	"econnrefused",
	"econnreset",
	"etimedout",
	"enetunreach",
	"ehostunreach",
	"epipe",
	"connection refused",
	"connection reset",
	"broken pipe",
	"no such host",
}

// isConnectionError returns true if the error indicates a network/connection problem.
func isConnectionError(err error) bool {
	if err == nil {
		return false
	}
	lower := strings.ToLower(err.Error())
	for _, code := range connectionErrorCodes {
		if strings.Contains(lower, code) {
			return true
		}
	}
	return false
}

// isStaleConnectionError returns true if the error looks like a stale keep-alive
// socket (ECONNRESET or EPIPE). These benefit from disabling connection pooling
// and retrying with a fresh connection. Inspired by Claude Code's isStaleConnectionError.
func isStaleConnectionError(err error) bool {
	if err == nil {
		return false
	}
	lower := strings.ToLower(err.Error())
	return strings.Contains(lower, "econnreset") ||
		strings.Contains(lower, "connection reset") ||
		strings.Contains(lower, "epipe") ||
		strings.Contains(lower, "broken pipe")
}

// isPromptTooLong returns true if the error indicates the prompt exceeds the model's context window.
func isPromptTooLong(err error) bool {
	if err == nil {
		return false
	}
	lower := strings.ToLower(err.Error())
	return strings.Contains(lower, "prompt_too_long") ||
		strings.Contains(lower, "prompt is too long") ||
		strings.Contains(lower, "maximum context length") ||
		strings.Contains(lower, "context_length_exceeded")
}

// isMediaSizeError returns true if the error indicates an image or PDF exceeds
// the provider's size limits. These errors should be surfaced to the user
// rather than retried. Inspired by Claude Code's isMediaSizeError.
func isMediaSizeError(err error) bool {
	if err == nil {
		return false
	}
	lower := strings.ToLower(err.Error())
	return (strings.Contains(lower, "image exceeds") && strings.Contains(lower, "maximum")) ||
		(strings.Contains(lower, "image dimensions exceed")) ||
		strings.Contains(lower, "maximum of") && strings.Contains(lower, "pdf pages")
}

// isAbortError returns true if the error is due to context cancellation or
// deadline exceeded, as opposed to an API error. These should not be retried
// or reported as API failures. Inspired by Claude Code's abort handling.
func isAbortError(err error) bool {
	if err == nil {
		return false
	}
	lower := strings.ToLower(err.Error())
	return strings.Contains(lower, "context canceled") ||
		strings.Contains(lower, "context deadline exceeded") ||
		strings.Contains(lower, "operation was aborted")
}
