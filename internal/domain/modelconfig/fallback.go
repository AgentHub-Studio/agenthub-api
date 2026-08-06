// Package modelconfig contains validation and normalization shared by the
// agent write path and the agentic runtime for persisted model configuration.
package modelconfig

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// FallbackStep is the canonical representation of one configured model
// fallback candidate.
type FallbackStep struct {
	Provider   string
	Model      string
	MaxRetries int
	TriggerOn  []string
}

type fallbackStepAlias struct {
	Provider        string    `json:"provider"`
	Model           string    `json:"model"`
	MaxRetries      *int      `json:"max_retries,omitempty"`
	MaxRetriesCamel *int      `json:"maxRetries,omitempty"`
	TriggerOn       *[]string `json:"trigger_on,omitempty"`
	TriggerOnCamel  *[]string `json:"triggerOn,omitempty"`
}

// ValidateFallbackChainAliases rejects model fallback aliases that would
// otherwise select a value by field order. It accepts either naming style and
// accepts both only when they describe the same effective fallback chain.
func ValidateFallbackChainAliases(raw json.RawMessage) error {
	_, _, err := resolveFallbackChain(raw)
	return err
}

// ResolveFallbackChain returns the canonical chain in a model configuration.
// Invalid or ambiguous legacy configuration returns no chain so it cannot
// silently redirect a run to a different provider or model.
func ResolveFallbackChain(raw json.RawMessage) []FallbackStep {
	chain, _, err := resolveFallbackChain(raw)
	if err != nil {
		return nil
	}
	return chain
}

func resolveFallbackChain(raw json.RawMessage) ([]FallbackStep, bool, error) {
	var config map[string]json.RawMessage
	if err := json.Unmarshal(raw, &config); err != nil {
		return nil, false, fmt.Errorf("modelConfig must be a JSON object")
	}

	snake, snakePresent, err := decodeFallbackChain(config, "fallback_chain")
	if err != nil {
		return nil, false, err
	}
	camel, camelPresent, err := decodeFallbackChain(config, "fallbackChain")
	if err != nil {
		return nil, false, err
	}
	if snakePresent && camelPresent && !sameFallbackChain(snake, camel) {
		return nil, false, fmt.Errorf("fallback_chain and fallbackChain must describe the same fallback chain")
	}
	if snakePresent {
		return snake, true, nil
	}
	if camelPresent {
		return camel, true, nil
	}
	return nil, false, nil
}

func decodeFallbackChain(config map[string]json.RawMessage, name string) ([]FallbackStep, bool, error) {
	raw, present := config[name]
	if !present {
		return nil, false, nil
	}
	var aliases []fallbackStepAlias
	if err := json.Unmarshal(raw, &aliases); err != nil {
		return nil, false, fmt.Errorf("%s must be an array of fallback steps", name)
	}
	chain, err := normalizeFallbackChain(aliases)
	if err != nil {
		return nil, false, fmt.Errorf("%s", err)
	}
	return chain, true, nil
}

func normalizeFallbackChain(aliases []fallbackStepAlias) ([]FallbackStep, error) {
	chain := make([]FallbackStep, 0, len(aliases))
	for index, alias := range aliases {
		if alias.MaxRetries != nil && alias.MaxRetriesCamel != nil && *alias.MaxRetries != *alias.MaxRetriesCamel {
			return nil, fmt.Errorf("fallback step %d: max_retries and maxRetries must agree", index)
		}
		if alias.TriggerOn != nil && alias.TriggerOnCamel != nil && !sameTriggerSet(*alias.TriggerOn, *alias.TriggerOnCamel) {
			return nil, fmt.Errorf("fallback step %d: trigger_on and triggerOn must agree", index)
		}

		model := strings.TrimSpace(alias.Model)
		if model == "" {
			continue
		}
		maxRetries := 0
		if alias.MaxRetries != nil {
			maxRetries = *alias.MaxRetries
		} else if alias.MaxRetriesCamel != nil {
			maxRetries = *alias.MaxRetriesCamel
		}
		triggerOn := normalizeTriggerNames(nil)
		if alias.TriggerOn != nil {
			triggerOn = normalizeTriggerNames(*alias.TriggerOn)
		} else if alias.TriggerOnCamel != nil {
			triggerOn = normalizeTriggerNames(*alias.TriggerOnCamel)
		}
		chain = append(chain, FallbackStep{
			Provider:   strings.ToLower(strings.TrimSpace(alias.Provider)),
			Model:      model,
			MaxRetries: maxRetries,
			TriggerOn:  triggerOn,
		})
	}
	return chain, nil
}

func sameFallbackChain(left, right []FallbackStep) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index].Provider != right[index].Provider ||
			left[index].Model != right[index].Model ||
			left[index].MaxRetries != right[index].MaxRetries ||
			!sameTriggerSet(left[index].TriggerOn, right[index].TriggerOn) {
			return false
		}
	}
	return true
}

func sameTriggerSet(left, right []string) bool {
	leftNames := canonicalTriggerSet(left)
	rightNames := canonicalTriggerSet(right)
	if len(leftNames) != len(rightNames) {
		return false
	}
	for index := range leftNames {
		if leftNames[index] != rightNames[index] {
			return false
		}
	}
	return true
}

func canonicalTriggerSet(values []string) []string {
	set := make(map[string]struct{}, len(values))
	for _, value := range normalizeTriggerNames(values) {
		set[value] = struct{}{}
	}
	out := make([]string, 0, len(set))
	for value := range set {
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}

func normalizeTriggerNames(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		normalized := strings.ToLower(strings.TrimSpace(value))
		if normalized != "" {
			out = append(out, normalized)
		}
	}
	return out
}
