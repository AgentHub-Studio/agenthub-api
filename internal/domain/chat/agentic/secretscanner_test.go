package agentic_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/chat/agentic"
)

// --- ScanForSecrets ---

func TestScanForSecrets_Empty(t *testing.T) {
	assert.Nil(t, agentic.ScanForSecrets(""))
}

func TestScanForSecrets_NoSecrets(t *testing.T) {
	matches := agentic.ScanForSecrets("just a normal message about code")
	assert.Empty(t, matches)
}

// --- AWS ---

func TestScanForSecrets_AWSAccessKey(t *testing.T) {
	content := "config: AKIAIOSFODNN7EXAMPLE"
	matches := agentic.ScanForSecrets(content)
	require.NotEmpty(t, matches)
	assert.Equal(t, "aws-access-key", matches[0].RuleID)
}

func TestScanForSecrets_AWSSecretKey(t *testing.T) {
	content := `aws_secret_access_key = "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY"`
	matches := agentic.ScanForSecrets(content)
	require.NotEmpty(t, matches)
	assert.Equal(t, "aws-secret-key", matches[0].RuleID)
}

// --- Anthropic ---

func TestScanForSecrets_AnthropicKey(t *testing.T) {
	content := "ANTHROPIC_API_KEY=sk-ant-abcdefghijklmnopqrstuvwxyz"
	matches := agentic.ScanForSecrets(content)
	require.NotEmpty(t, matches)
	assert.Equal(t, "anthropic-api-key", matches[0].RuleID)
}

// --- OpenAI ---

func TestScanForSecrets_OpenAIKey(t *testing.T) {
	content := "OPENAI_API_KEY=sk-abcdefghijklmnopqrstuvwxyz1234"
	matches := agentic.ScanForSecrets(content)
	require.NotEmpty(t, matches)
	// Could match openai-api-key or anthropic pattern — check it matches something.
	found := false
	for _, m := range matches {
		if m.RuleID == "openai-api-key" {
			found = true
			break
		}
	}
	assert.True(t, found, "should detect OpenAI key")
}

// --- GitHub ---

func TestScanForSecrets_GitHubPAT(t *testing.T) {
	content := "token: ghp_ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefgh12"
	matches := agentic.ScanForSecrets(content)
	require.NotEmpty(t, matches)
	assert.Equal(t, "github-pat", matches[0].RuleID)
}

func TestScanForSecrets_GitHubOAuth(t *testing.T) {
	content := "gho_ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefgh12"
	matches := agentic.ScanForSecrets(content)
	require.NotEmpty(t, matches)
	assert.Equal(t, "github-oauth", matches[0].RuleID)
}

func TestScanForSecrets_GitHubApp(t *testing.T) {
	content := "ghs_ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefgh12"
	matches := agentic.ScanForSecrets(content)
	require.NotEmpty(t, matches)
	assert.Equal(t, "github-app", matches[0].RuleID)
}

// --- Google ---

func TestScanForSecrets_GCPAPIKey(t *testing.T) {
	content := "API_KEY=AIzaSyA0B1C2D3E4F5G6H7I8J9K0L1M2N3O4P5Q"
	matches := agentic.ScanForSecrets(content)
	require.NotEmpty(t, matches)
	assert.Equal(t, "gcp-api-key", matches[0].RuleID)
}

func TestScanForSecrets_GCPServiceAccount(t *testing.T) {
	content := `{"type": "service_account", "project_id": "my-project"}`
	matches := agentic.ScanForSecrets(content)
	require.NotEmpty(t, matches)
	assert.Equal(t, "gcp-service-account", matches[0].RuleID)
}

// --- Slack ---

func TestScanForSecrets_SlackBotToken(t *testing.T) {
	// Fake token — intentionally split to avoid secret scanning triggers in CI
	content := "SLACK_TOKEN=" + "xoxb" + "-1234567890-1234567890-ABCDEFGHIJKLMNOPabcdef"
	matches := agentic.ScanForSecrets(content)
	require.NotEmpty(t, matches)
	assert.Equal(t, "slack-bot-token", matches[0].RuleID)
}

func TestScanForSecrets_SlackWebhook(t *testing.T) {
	// Fake URL — intentionally split to avoid secret scanning triggers in CI
	content := "https://hooks.slack.com" + "/services/T12345678/B12345678/abcdefghijklmnopqrstuvwx"
	matches := agentic.ScanForSecrets(content)
	require.NotEmpty(t, matches)
	assert.Equal(t, "slack-webhook", matches[0].RuleID)
}

// --- Stripe ---

func TestScanForSecrets_StripeSecret(t *testing.T) {
	// Fake key — intentionally split to avoid secret scanning triggers in CI
	content := "STRIPE_KEY=" + "sk_live" + "_abcdefghijklmnopqrstuvwxyz"
	matches := agentic.ScanForSecrets(content)
	require.NotEmpty(t, matches)
	assert.Equal(t, "stripe-secret", matches[0].RuleID)
}

// --- SendGrid ---

func TestScanForSecrets_SendGridKey(t *testing.T) {
	content := "SG.abcdefghijklmnopqrstuv.ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrst"
	matches := agentic.ScanForSecrets(content)
	require.NotEmpty(t, matches)
	assert.Equal(t, "sendgrid-api-key", matches[0].RuleID)
}

// --- Private Key ---

func TestScanForSecrets_PrivateKey(t *testing.T) {
	content := "-----BEGIN RSA PRIVATE KEY-----\nMIIEpAIBAAK..."
	matches := agentic.ScanForSecrets(content)
	require.NotEmpty(t, matches)
	assert.Equal(t, "private-key", matches[0].RuleID)
}

func TestScanForSecrets_ECPrivateKey(t *testing.T) {
	content := "-----BEGIN EC PRIVATE KEY-----"
	matches := agentic.ScanForSecrets(content)
	require.NotEmpty(t, matches)
	assert.Equal(t, "private-key", matches[0].RuleID)
}

// --- Generic config patterns ---

func TestScanForSecrets_PasswordInConfig(t *testing.T) {
	content := `password: "my_super_secret_password_123"`
	matches := agentic.ScanForSecrets(content)
	require.NotEmpty(t, matches)
	found := false
	for _, m := range matches {
		if m.RuleID == "generic-password" {
			found = true
		}
	}
	assert.True(t, found)
}

func TestScanForSecrets_APIKeyInConfig(t *testing.T) {
	content := `api_key: "abcdefghijklmnopqrstuvwxyz123456"`
	matches := agentic.ScanForSecrets(content)
	require.NotEmpty(t, matches)
	found := false
	for _, m := range matches {
		if m.RuleID == "generic-api-key" {
			found = true
		}
	}
	assert.True(t, found)
}

// --- Multiple secrets ---

func TestScanForSecrets_MultipleSecrets(t *testing.T) {
	content := `
ANTHROPIC_API_KEY=sk-ant-abcdefghijklmnopqrstuvwxyz
GITHUB_TOKEN=ghp_ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefgh12
`
	matches := agentic.ScanForSecrets(content)
	assert.GreaterOrEqual(t, len(matches), 2)

	ruleIDs := make(map[string]bool)
	for _, m := range matches {
		ruleIDs[m.RuleID] = true
	}
	assert.True(t, ruleIDs["anthropic-api-key"])
	assert.True(t, ruleIDs["github-pat"])
}

// --- HasSecrets ---

func TestHasSecrets_True(t *testing.T) {
	assert.True(t, agentic.HasSecrets("sk-ant-abcdefghijklmnopqrstuvwxyz"))
}

func TestHasSecrets_False(t *testing.T) {
	assert.False(t, agentic.HasSecrets("just normal text"))
}

func TestHasSecrets_Empty(t *testing.T) {
	assert.False(t, agentic.HasSecrets(""))
}

// --- RedactSecrets ---

func TestRedactSecrets_Redacts(t *testing.T) {
	content := "key: sk-ant-abcdefghijklmnopqrstuvwxyz"
	redacted := agentic.RedactSecrets(content, "[REDACTED]")
	assert.Contains(t, redacted, "[REDACTED]")
	assert.NotContains(t, redacted, "sk-ant-")
}

func TestRedactSecrets_NoSecrets(t *testing.T) {
	content := "safe text"
	assert.Equal(t, content, agentic.RedactSecrets(content, "[REDACTED]"))
}

func TestRedactSecrets_Empty(t *testing.T) {
	assert.Equal(t, "", agentic.RedactSecrets("", "[REDACTED]"))
}

func TestRedactSecrets_EmptyMarker(t *testing.T) {
	content := "sk-ant-abcdefghijklmnopqrstuvwxyz"
	assert.Equal(t, content, agentic.RedactSecrets(content, ""))
}

func TestRedactSensitiveFields_RedactsNestedCredentialKeys(t *testing.T) {
	raw := json.RawMessage(`{
		"ok": true,
		"apiKey": "short-secret",
		"nested": {"clientSecret": "nested-secret"},
		"items": [{"authorization": "Bearer short-token"}]
	}`)
	redacted := agentic.RedactSensitiveFields(raw)
	assert.JSONEq(t, `{
		"ok": true,
		"apiKey": "[REDACTED]",
		"nested": {"clientSecret": "[REDACTED]"},
		"items": [{"authorization": "[REDACTED]"}]
	}`, string(redacted))
	assert.NotContains(t, string(redacted), "short-secret")
	assert.NotContains(t, string(redacted), "nested-secret")
	assert.NotContains(t, string(redacted), "short-token")
}

// --- SecretRuleCount ---

func TestSecretRuleCount(t *testing.T) {
	count := agentic.SecretRuleCount()
	assert.GreaterOrEqual(t, count, 20, "should have at least 20 rules")
}

// --- Match StartIndex ---

func TestSecretMatch_StartIndex(t *testing.T) {
	content := "prefix sk-ant-abcdefghijklmnopqrstuvwxyz suffix"
	matches := agentic.ScanForSecrets(content)
	require.NotEmpty(t, matches)
	assert.Greater(t, matches[0].StartIndex, 0)
}
