package agentic

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

type auxiliaryPromptTemplateResolverStub struct {
	templates map[string]string
}

type sessionMemoryContextKey struct{}

func (s *auxiliaryPromptTemplateResolverStub) ResolvePromptTemplate(_ context.Context, _ uuid.UUID, slug string) (string, bool, error) {
	content, ok := s.templates[slug]
	return content, ok, nil
}

func TestDetachedContextPreservesValuesWithoutCancellation(t *testing.T) {
	key := sessionMemoryContextKey{}
	parent, cancel := context.WithCancel(context.WithValue(context.Background(), key, "session-value"))
	cancel()

	detached := detachedContext(parent)

	assert.Equal(t, "session-value", detached.Value(key))
	select {
	case <-detached.Done():
		t.Fatal("detached context inherited parent cancellation")
	default:
	}
}

func TestSessionMemoryExtractor_ResolveExtractionPrompt_UsesFallback(t *testing.T) {
	extractor := NewSessionMemoryExtractor(nil, DefaultSessionMemoryConfig())

	assert.Equal(t, sessionMemoryExtractionPrompt, extractor.resolveExtractionPrompt(context.Background(), uuid.New()))
}

func TestSessionMemoryExtractor_ResolveExtractionPrompt_UsesTemplateOverride(t *testing.T) {
	extractor := NewSessionMemoryExtractor(nil, DefaultSessionMemoryConfig()).
		WithPromptTemplateResolver(&auxiliaryPromptTemplateResolverStub{
			templates: map[string]string{
				promptTemplateSlugSessionMemoryExtractionPrompt: "Custom memory extraction prompt",
			},
		})

	assert.Equal(
		t,
		"Custom memory extraction prompt",
		extractor.resolveExtractionPrompt(context.Background(), uuid.New()),
	)
}
