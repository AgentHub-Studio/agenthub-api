package core

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCoreWebhookTemplate_SlugsNoDuplicates(t *testing.T) {
	seen := map[string]bool{}
	for _, s := range SeedExpectedWebhookTemplateSlugs {
		assert.False(t, seen[s])
		seen[s] = true
	}
}

func TestCoreWebhookTemplate_SlugsCanonicalCount(t *testing.T) {
	assert.Equal(t, 8, len(SeedExpectedWebhookTemplateSlugs))
}

func TestCoreWebhookTemplate_TargetKindsCount(t *testing.T) {
	assert.Equal(t, 8, len(SeedExpectedWebhookTemplateTargetKinds))
}

func TestCoreWebhookTemplate_RecommendedAllInSeed(t *testing.T) {
	seedSet := map[string]bool{}
	for _, s := range SeedExpectedWebhookTemplateSlugs {
		seedSet[s] = true
	}
	for _, r := range SeedRecommendedWebhookTemplateSlugs {
		assert.True(t, seedSet[r])
	}
}

func TestCoreWebhookTemplate_SlackIsRecommended(t *testing.T) {
	recSet := map[string]bool{}
	for _, r := range SeedRecommendedWebhookTemplateSlugs {
		recSet[r] = true
	}
	assert.True(t, recSet["slack-incoming-webhook"], "Slack is the most-used webhook")
}

func TestCoreWebhookTemplate_RetryPolicyMs_Parses(t *testing.T) {
	tmpl := CoreWebhookEndpointTemplate{RetryPolicy: "1000,5000,30000"}
	got := tmpl.RetryPolicyMs()
	assert.Equal(t, []int{1000, 5000, 30000}, got)
}

func TestCoreWebhookTemplate_RetryPolicyMs_HandlesWhitespace(t *testing.T) {
	tmpl := CoreWebhookEndpointTemplate{RetryPolicy: " 1000 , 5000 , 30000 "}
	got := tmpl.RetryPolicyMs()
	assert.Equal(t, []int{1000, 5000, 30000}, got)
}

func TestCoreWebhookTemplate_SubscribedEventsList_Parses(t *testing.T) {
	tmpl := CoreWebhookEndpointTemplate{SubscribedEvents: "run_complete,quality_report"}
	got := tmpl.SubscribedEventsList()
	assert.Equal(t, []string{"run_complete", "quality_report"}, got)
}

func TestCoreWebhookTemplate_SlugsKebabCase(t *testing.T) {
	for _, s := range SeedExpectedWebhookTemplateSlugs {
		assert.False(t, strings.Contains(s, "_"))
	}
}
