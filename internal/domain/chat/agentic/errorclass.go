package agentic

import (
	"errors"
	"fmt"
	"runtime"
	"strings"
)

// --- Error IDs (obfuscated for production telemetry) ---
//
// Inspired by Claude Code's errorIds.ts.

const (
	ErrIDToolUseSummaryFailed  = 344
	ErrIDStreamFallbackFailed  = 345
	ErrIDCompactFailed         = 346
	ErrIDContextOverflow       = 347
	ErrIDMaxTokensExhausted    = 348
	ErrIDHookExecutionFailed   = 349
	ErrIDSkillExecutionFailed  = 350
	ErrIDPromptTooLong         = 351
	ErrIDSessionPersistFailed  = 352
	ErrIDModelFallbackFailed   = 353
	ErrIDPermissionDenied      = 354
	ErrIDTaskExecutionFailed   = 355
)

// --- Error types ---

// AgenticError is the base error type for the agentic package.
// It carries a numeric ID for telemetry and an optional telemetry-safe message.
//
// Inspired by Claude Code's ClaudeError and TelemetrySafeError.
type AgenticError struct {
	ID               int
	Message          string
	TelemetryMessage string // safe for logging (no PII, no file paths)
	Cause            error
}

func (e *AgenticError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("[%d] %s: %v", e.ID, e.Message, e.Cause)
	}
	return fmt.Sprintf("[%d] %s", e.ID, e.Message)
}

func (e *AgenticError) Unwrap() error {
	return e.Cause
}

// SafeMessage returns the telemetry-safe message, falling back to the regular message.
func (e *AgenticError) SafeMessage() string {
	if e.TelemetryMessage != "" {
		return e.TelemetryMessage
	}
	return e.Message
}

// NewAgenticError creates a new error with an ID and message.
func NewAgenticError(id int, message string, cause error) *AgenticError {
	return &AgenticError{ID: id, Message: message, Cause: cause}
}

// NewTelemetrySafeError creates an error with a separate telemetry-safe message
// that excludes PII, file paths, and code content.
func NewTelemetrySafeError(id int, message, telemetryMessage string, cause error) *AgenticError {
	return &AgenticError{ID: id, Message: message, TelemetryMessage: telemetryMessage, Cause: cause}
}

// --- Specific error types ---

// AbortError signals that an operation was intentionally cancelled.
//
// Inspired by Claude Code's AbortError.
type AbortError struct {
	Message string
}

func (e *AbortError) Error() string {
	if e.Message != "" {
		return e.Message
	}
	return "operation aborted"
}

// IsAbortError checks if an error is an AbortError or context cancellation.
// Handles wrapped errors via errors.As.
func IsAbortError(err error) bool {
	if err == nil {
		return false
	}
	var ae *AbortError
	if errors.As(err, &ae) {
		return true
	}
	// Check for context.Canceled which is the Go equivalent.
	return errors.Is(err, errors.New("context canceled")) || strings.Contains(err.Error(), "context canceled")
}

// ShellError represents a failed shell command execution.
//
// Inspired by Claude Code's ShellError.
type ShellError struct {
	Stdout      string
	Stderr      string
	ExitCode    int
	Interrupted bool
}

func (e *ShellError) Error() string {
	if e.Stderr != "" {
		return fmt.Sprintf("shell command failed (exit %d): %s", e.ExitCode, e.Stderr)
	}
	return fmt.Sprintf("shell command failed (exit %d)", e.ExitCode)
}

// ConfigParseError represents a failure to parse a configuration file.
//
// Inspired by Claude Code's ConfigParseError.
type ConfigParseError struct {
	FilePath      string
	DefaultConfig any
	Cause         error
}

func (e *ConfigParseError) Error() string {
	return fmt.Sprintf("config parse error in %s: %v", e.FilePath, e.Cause)
}

func (e *ConfigParseError) Unwrap() error {
	return e.Cause
}

// --- HTTP/API error classification ---

// APIErrorKind classifies API errors by category.
//
// Inspired by Claude Code's AxiosErrorKind.
type APIErrorKind string

const (
	APIErrorAuth    APIErrorKind = "auth"    // 401/403
	APIErrorTimeout APIErrorKind = "timeout" // request timeout
	APIErrorNetwork APIErrorKind = "network" // connection refused/not found
	APIErrorHTTP    APIErrorKind = "http"    // other HTTP errors
	APIErrorOther   APIErrorKind = "other"   // non-HTTP errors
)

// ClassifiedAPIError holds a classified API error with optional status code.
type ClassifiedAPIError struct {
	Kind    APIErrorKind
	Status  int
	Message string
}

// ClassifyAPIError categorizes an HTTP status code and error into an APIErrorKind.
func ClassifyAPIError(statusCode int, err error) ClassifiedAPIError {
	msg := ""
	if err != nil {
		msg = err.Error()
	}

	switch {
	case statusCode == 401 || statusCode == 403:
		return ClassifiedAPIError{Kind: APIErrorAuth, Status: statusCode, Message: msg}
	case statusCode == 408 || strings.Contains(msg, "timeout"):
		return ClassifiedAPIError{Kind: APIErrorTimeout, Status: statusCode, Message: msg}
	case statusCode == 0 && err != nil && (strings.Contains(msg, "connection refused") || strings.Contains(msg, "no such host")):
		return ClassifiedAPIError{Kind: APIErrorNetwork, Status: 0, Message: msg}
	case statusCode >= 400:
		return ClassifiedAPIError{Kind: APIErrorHTTP, Status: statusCode, Message: msg}
	default:
		return ClassifiedAPIError{Kind: APIErrorOther, Status: statusCode, Message: msg}
	}
}

// IsRetryable returns true if this error kind is worth retrying.
func (e ClassifiedAPIError) IsRetryable() bool {
	switch e.Kind {
	case APIErrorTimeout, APIErrorNetwork:
		return true
	case APIErrorHTTP:
		return e.Status == 429 || e.Status >= 500
	default:
		return false
	}
}

// --- Error utility functions ---

// ToError converts any value to an error.
//
// Inspired by Claude Code's toError.
func ToError(v any) error {
	if v == nil {
		return nil
	}
	if err, ok := v.(error); ok {
		return err
	}
	return fmt.Errorf("%v", v)
}

// ErrorMessage extracts a message string from any value.
//
// Inspired by Claude Code's errorMessage.
func ErrorMessage(v any) string {
	if v == nil {
		return ""
	}
	if err, ok := v.(error); ok {
		return err.Error()
	}
	return fmt.Sprintf("%v", v)
}

// ShortErrorStack returns a truncated stack trace (max N frames).
//
// Inspired by Claude Code's shortErrorStack.
func ShortErrorStack(maxFrames int) string {
	if maxFrames <= 0 {
		maxFrames = 5
	}
	pcs := make([]uintptr, maxFrames)
	n := runtime.Callers(2, pcs) // skip Callers + ShortErrorStack
	if n == 0 {
		return ""
	}

	frames := runtime.CallersFrames(pcs[:n])
	var b strings.Builder
	for i := 0; i < maxFrames; i++ {
		frame, more := frames.Next()
		fmt.Fprintf(&b, "%s\n\t%s:%d\n", frame.Function, frame.File, frame.Line)
		if !more {
			break
		}
	}
	return b.String()
}

// HasExactMessage checks if an error's message matches exactly.
func HasExactMessage(err error, msg string) bool {
	if err == nil {
		return false
	}
	return err.Error() == msg
}
