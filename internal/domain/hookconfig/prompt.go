// Package hookconfig contains configuration contracts shared by hook writers
// and hook executors.
package hookconfig

import (
	"encoding/json"
	"errors"
	"fmt"
)

// ErrConflictingPromptAliases is returned when the two supported prompt hook
// aliases would select different text at runtime.
var ErrConflictingPromptAliases = errors.New("conflicting prompt hook config aliases template and inject")

// PromptConfig is the config shape for a prompt hook.
type PromptConfig struct {
	Template string `json:"template"`
	Inject   string `json:"inject"`
}

// ValidatePromptAliases rejects prompt configs whose non-empty aliases have
// different values. Configs that cannot be decoded retain the legacy runtime
// validation behavior.
func ValidatePromptAliases(raw json.RawMessage) error {
	config, err := parsePromptConfig(raw)
	if err != nil {
		return nil
	}
	return validatePromptAliases(config)
}

// ResolvePromptConfig returns the text selected by a valid prompt config.
func ResolvePromptConfig(raw json.RawMessage) (string, error) {
	config, err := parsePromptConfig(raw)
	if err != nil {
		return "", fmt.Errorf("invalid prompt hook config: %w", err)
	}
	if err := validatePromptAliases(config); err != nil {
		return "", err
	}
	if config.Template != "" {
		return config.Template, nil
	}
	return config.Inject, nil
}

func parsePromptConfig(raw json.RawMessage) (PromptConfig, error) {
	var config PromptConfig
	if err := json.Unmarshal(raw, &config); err != nil {
		return PromptConfig{}, err
	}
	return config, nil
}

func validatePromptAliases(config PromptConfig) error {
	if config.Template != "" && config.Inject != "" && config.Template != config.Inject {
		return ErrConflictingPromptAliases
	}
	return nil
}
