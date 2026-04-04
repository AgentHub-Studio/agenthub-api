package agentic

import (
	"context"
	"strings"

	"github.com/google/uuid"
)

const (
	// Prompt template slugs for auxiliary agentic prompts.
	promptTemplateSlugToolUseSummarySystemPrompt    = "agentic-tool-use-summary-system-prompt"
	promptTemplateSlugSessionMemoryExtractionPrompt = "agentic-session-memory-extraction-prompt"
)

func resolvePromptTemplateOrFallback(
	ctx context.Context,
	resolver PromptTemplateResolver,
	agentID uuid.UUID,
	slug string,
	fallback string,
) string {
	if resolver == nil {
		return fallback
	}

	content, found, err := resolver.ResolvePromptTemplate(ctx, agentID, slug)
	if err != nil || !found {
		return fallback
	}
	if strings.TrimSpace(content) == "" {
		return fallback
	}
	return content
}
