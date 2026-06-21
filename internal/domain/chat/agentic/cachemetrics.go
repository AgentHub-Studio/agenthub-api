package agentic

import "github.com/AgentHub-Studio/agenthub-go-commons/ai"

func PromptCacheHitRatio(usage ai.Usage) float64 {
	cacheRead := usage.CacheReadTokens
	if cacheRead <= 0 {
		return 0
	}
	totalInput := usage.PromptTokens
	if totalInput < cacheRead {
		totalInput = cacheRead
	}
	if totalInput == 0 {
		return 0
	}
	return float64(cacheRead) / float64(totalInput)
}
