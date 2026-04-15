// Package processors defines a pluggable pipeline of pre/post-processors that
// operate on user messages (input) before they reach the LLM and on model
// responses (output) before they are surfaced to the caller.
//
// Inspired by Mastra's inputProcessors / outputProcessors — implementation in
// Go following AgentHub's own runner conventions.
package processors

import (
	"context"
	"regexp"
	"strings"
)

// Message is the minimal shape a processor operates on. It intentionally mirrors
// only what the runner already holds per-turn so processors can plug in without
// touching deeper runner state.
type Message struct {
	Role    string
	Content string
}

// InputProcessor runs before the LLM call. It may rewrite the message list
// (e.g. mask PII) or short-circuit by returning a non-empty Reject reason.
type InputProcessor interface {
	Name() string
	ProcessInput(ctx context.Context, in []Message) (out []Message, reject string, err error)
}

// OutputProcessor runs after the LLM returns a final response. It may rewrite
// the response content or flag it.
type OutputProcessor interface {
	Name() string
	ProcessOutput(ctx context.Context, content string) (string, error)
}

// Pipeline composes processors in deterministic order.
type Pipeline struct {
	Inputs  []InputProcessor
	Outputs []OutputProcessor
}

// RunInput applies every input processor in order. Returns the first reject
// reason encountered (empty means allowed). On reject, downstream processors
// are skipped.
func (p Pipeline) RunInput(ctx context.Context, msgs []Message) ([]Message, string, error) {
	cur := msgs
	for _, ip := range p.Inputs {
		next, reject, err := ip.ProcessInput(ctx, cur)
		if err != nil {
			return nil, "", err
		}
		if reject != "" {
			return cur, reject, nil
		}
		cur = next
	}
	return cur, "", nil
}

// RunOutput applies every output processor in order.
func (p Pipeline) RunOutput(ctx context.Context, content string) (string, error) {
	cur := content
	for _, op := range p.Outputs {
		next, err := op.ProcessOutput(ctx, cur)
		if err != nil {
			return "", err
		}
		cur = next
	}
	return cur, nil
}

// PIIRedactor masks Brazilian CPF/CNPJ/email patterns in user input so the LLM
// never sees the raw values. Masked with ***.
type PIIRedactor struct{}

var (
	cpfPattern   = regexp.MustCompile(`\b\d{3}\.?\d{3}\.?\d{3}-?\d{2}\b`)
	cnpjPattern  = regexp.MustCompile(`\b\d{2}\.?\d{3}\.?\d{3}/?\d{4}-?\d{2}\b`)
	emailPattern = regexp.MustCompile(`[A-Za-z0-9._%+\-]+@[A-Za-z0-9.\-]+\.[A-Za-z]{2,}`)
)

func (PIIRedactor) Name() string { return "pii_redactor" }

func (p PIIRedactor) ProcessInput(_ context.Context, in []Message) ([]Message, string, error) {
	out := make([]Message, len(in))
	for i, m := range in {
		c := m.Content
		c = cnpjPattern.ReplaceAllString(c, "[REDACTED_CNPJ]")
		c = cpfPattern.ReplaceAllString(c, "[REDACTED_CPF]")
		c = emailPattern.ReplaceAllString(c, "[REDACTED_EMAIL]")
		out[i] = Message{Role: m.Role, Content: c}
	}
	return out, "", nil
}

// ProfanityFilter rejects messages containing any banned term (case-insensitive).
type ProfanityFilter struct {
	Banned []string
}

func (ProfanityFilter) Name() string { return "profanity_filter" }

func (f ProfanityFilter) ProcessInput(_ context.Context, in []Message) ([]Message, string, error) {
	for _, m := range in {
		lc := strings.ToLower(m.Content)
		for _, w := range f.Banned {
			if w == "" {
				continue
			}
			if strings.Contains(lc, strings.ToLower(w)) {
				return in, "message rejected by profanity filter", nil
			}
		}
	}
	return in, "", nil
}

// PromptInjectionDetector rejects messages containing well-known injection
// markers such as "ignore previous instructions". Heuristic, not exhaustive.
type PromptInjectionDetector struct{}

var injectionMarkers = []string{
	"ignore previous instructions",
	"ignore all previous instructions",
	"disregard prior instructions",
	"reveal your system prompt",
	"system prompt:",
}

func (PromptInjectionDetector) Name() string { return "prompt_injection_detector" }

func (PromptInjectionDetector) ProcessInput(_ context.Context, in []Message) ([]Message, string, error) {
	for _, m := range in {
		lc := strings.ToLower(m.Content)
		for _, marker := range injectionMarkers {
			if strings.Contains(lc, marker) {
				return in, "message rejected: prompt injection detected", nil
			}
		}
	}
	return in, "", nil
}

// ResponseCache is a trivial output processor that memoizes the last N responses
// keyed by the raw content. Intended as a scaffold — real use would key on the
// input hash and live at a higher level in the runner.
type ResponseCache struct {
	entries map[string]string
	order   []string
	max     int
}

func NewResponseCache(max int) *ResponseCache {
	if max <= 0 {
		max = 64
	}
	return &ResponseCache{entries: make(map[string]string), max: max}
}

func (*ResponseCache) Name() string { return "response_cache" }

func (c *ResponseCache) ProcessOutput(_ context.Context, content string) (string, error) {
	if v, ok := c.entries[content]; ok {
		return v, nil
	}
	c.entries[content] = content
	c.order = append(c.order, content)
	if len(c.order) > c.max {
		delete(c.entries, c.order[0])
		c.order = c.order[1:]
	}
	return content, nil
}
