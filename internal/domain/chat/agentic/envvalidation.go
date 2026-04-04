package agentic

import (
	"fmt"
	"os"
	"strconv"
)

// Structured environment variable validation with bounds checking.
//
// Inspired by Claude Code's envValidation.ts — validates bounded
// numeric environment variables with clear status reporting.
// Returns diagnostic information (valid/capped/invalid) rather than
// just a value, enabling better logging and user feedback. Useful for
// runtime configuration of rate limits, batch sizes, token budgets,
// timeouts, and other bounded numeric parameters.

// EnvVarStatus indicates the validation result.
type EnvVarStatus string

const (
	EnvVarValid   EnvVarStatus = "valid"
	EnvVarCapped  EnvVarStatus = "capped"
	EnvVarInvalid EnvVarStatus = "invalid"
)

// EnvVarResult holds the validated value and its status.
type EnvVarResult struct {
	Effective int
	Status    EnvVarStatus
	Message   string // empty when valid
}

// ValidateBoundedIntEnv reads an environment variable and validates it
// as a positive integer within [1, upperLimit]. Returns defaultValue
// when the variable is missing, reports "invalid" for unparseable or
// non-positive values, and "capped" when the value exceeds upperLimit.
func ValidateBoundedIntEnv(name string, defaultValue, upperLimit int) EnvVarResult {
	return ValidateBoundedIntStr(name, os.Getenv(name), defaultValue, upperLimit)
}

// ValidateBoundedIntStr validates a string value as a bounded positive integer.
// Separated from ValidateBoundedIntEnv for testability.
func ValidateBoundedIntStr(name, value string, defaultValue, upperLimit int) EnvVarResult {
	if value == "" {
		return EnvVarResult{Effective: defaultValue, Status: EnvVarValid}
	}

	parsed, err := strconv.Atoi(value)
	if err != nil || parsed <= 0 {
		return EnvVarResult{
			Effective: defaultValue,
			Status:    EnvVarInvalid,
			Message:   fmt.Sprintf("%s: invalid value %q (using default: %d)", name, value, defaultValue),
		}
	}

	if parsed > upperLimit {
		return EnvVarResult{
			Effective: upperLimit,
			Status:    EnvVarCapped,
			Message:   fmt.Sprintf("%s: capped from %d to %d", name, parsed, upperLimit),
		}
	}

	return EnvVarResult{Effective: parsed, Status: EnvVarValid}
}

// ValidateBoundedDurationMs validates an environment variable as a
// duration in milliseconds within [1, upperLimit].
func ValidateBoundedDurationMs(name string, defaultMs, upperLimitMs int) EnvVarResult {
	return ValidateBoundedIntStr(name, os.Getenv(name), defaultMs, upperLimitMs)
}
