package agentic

import (
	"encoding/json"
	"regexp"
	"sync"

	"github.com/AgentHub-Studio/agenthub-api/internal/redact"
)

// Client-side secret/credential detection.
//
// Inspired by Claude Code's secretScanner.ts — curated high-confidence
// regex rules (from gitleaks) for detecting common API keys, tokens,
// and credentials. Near-zero false positives. Used to prevent accidental
// credential leaks in multi-tenant agentic workflows.

// SecretMatch represents a detected secret in content.
type SecretMatch struct {
	// RuleID identifies which rule matched.
	RuleID string `json:"ruleId"`
	// Label is a human-readable description of the secret type.
	Label string `json:"label"`
	// Match is the matched substring.
	Match string `json:"match"`
	// StartIndex is the byte offset of the match in the content.
	StartIndex int `json:"startIndex"`
}

// secretRule defines a single detection rule.
type secretRule struct {
	id      string
	label   string
	pattern *regexp.Regexp
}

// compiledRules is the lazily compiled set of detection rules.
var (
	rulesOnce     sync.Once
	compiledRules []secretRule
)

// getRules returns the compiled rules, lazily initializing them.
func getRules() []secretRule {
	rulesOnce.Do(func() {
		compiledRules = compileRules()
	})
	return compiledRules
}

// compileRules builds the detection rules. Based on gitleaks rules curated
// for high confidence (minimal false positives).
func compileRules() []secretRule {
	type rawRule struct {
		id, label, pattern string
	}

	raw := []rawRule{
		// AWS
		{"aws-access-key", "AWS Access Key ID",
			`(?:^|[^A-Za-z0-9/+=])(?:A3T[A-Z0-9]|AKIA|ASIA|ABIA|ACCA)[A-Z0-9]{16}(?:[^A-Za-z0-9/+=]|$)`},
		{"aws-secret-key", "AWS Secret Access Key",
			`(?i)(?:aws[_\-]?secret[_\-]?access[_\-]?key|aws[_\-]?secret)\s*[:=]\s*['"]?([A-Za-z0-9/+=]{40})['"]?`},

		// Anthropic
		{"anthropic-api-key", "Anthropic API Key",
			`sk-ant-[a-zA-Z0-9_-]{20,}`},

		// OpenAI
		{"openai-api-key", "OpenAI API Key",
			`sk-[a-zA-Z0-9]{20,}`},

		// GitHub
		{"github-pat", "GitHub Personal Access Token",
			`ghp_[a-zA-Z0-9]{36}`},
		{"github-oauth", "GitHub OAuth Access Token",
			`gho_[a-zA-Z0-9]{36}`},
		{"github-app", "GitHub App Token",
			`(?:ghu|ghs|ghr)_[a-zA-Z0-9]{36}`},
		{"github-fine-grained", "GitHub Fine-grained PAT",
			`github_pat_[a-zA-Z0-9]{22}_[a-zA-Z0-9]{59}`},

		// Google / GCP
		{"gcp-api-key", "Google API Key",
			`AIza[0-9A-Za-z_-]{35}`},
		{"gcp-service-account", "GCP Service Account Key",
			`(?i)"type"\s*:\s*"service_account"`},

		// Slack
		{"slack-bot-token", "Slack Bot Token",
			`xoxb-[0-9]{10,}-[0-9]{10,}-[a-zA-Z0-9]{20,}`},
		{"slack-user-token", "Slack User Token",
			`xoxp-[0-9]{10,}-[0-9]{10,}-[a-zA-Z0-9]{20,}`},
		{"slack-webhook", "Slack Webhook URL",
			`https://hooks\.slack\.com/services/T[A-Z0-9]{8,}/B[A-Z0-9]{8,}/[a-zA-Z0-9]{20,}`},

		// Stripe
		{"stripe-secret", "Stripe Secret Key",
			`sk_live_[a-zA-Z0-9]{20,}`},
		{"stripe-restricted", "Stripe Restricted Key",
			`rk_live_[a-zA-Z0-9]{20,}`},

		// Twilio
		{"twilio-api-key", "Twilio API Key",
			`SK[a-f0-9]{32}`},

		// SendGrid
		{"sendgrid-api-key", "SendGrid API Key",
			`SG\.[a-zA-Z0-9_-]{22}\.[a-zA-Z0-9_-]{43}`},

		// Mailgun
		{"mailgun-api-key", "Mailgun API Key",
			`key-[a-f0-9]{32}`},

		// Azure
		{"azure-subscription-key", "Azure Subscription Key",
			`(?i)(?:azure|subscription)[_\-]?key\s*[:=]\s*['"]?([a-f0-9]{32})['"]?`},

		// Private key
		{"private-key", "Private Key",
			`-----BEGIN (?:RSA |DSA |EC |OPENSSH )?PRIVATE KEY-----`},

		// JWT / Bearer tokens in headers
		{"bearer-token", "Bearer Token in Header",
			`(?i)(?:authorization|bearer)\s*[:=]\s*['"]?Bearer\s+[a-zA-Z0-9_-]{20,}[.][a-zA-Z0-9_-]+[.][a-zA-Z0-9_-]+['"]?`},
		// Authorization header values in JSON responses (e.g. from httpbin-style echo endpoints).
		// Catches any "Authorization": "Bearer <token>" or "Authorization": "<scheme> <value>"
		// that may appear in tool output when the remote server echoes back request headers.
		{"auth-header-json", "Authorization Header in JSON Response",
			`(?i)"[Aa]uthorization"\s*:\s*"(?:Bearer|Basic|Token)\s+[^"]{8,}"`},

		// Generic high-entropy secrets in config-like patterns
		{"generic-password", "Password in Config",
			`(?i)(?:password|passwd|pwd)\s*[:=]\s*['"]([^'"]{8,})['"]`},
		{"generic-api-key", "API Key in Config",
			`(?i)(?:api[_\-]?key|apikey|api[_\-]?secret)\s*[:=]\s*['"]([a-zA-Z0-9_-]{20,})['"]`},
	}

	rules := make([]secretRule, 0, len(raw))
	for _, r := range raw {
		compiled := regexp.MustCompile(r.pattern)
		rules = append(rules, secretRule{
			id:      r.id,
			label:   r.label,
			pattern: compiled,
		})
	}
	return rules
}

// ScanForSecrets checks content for known secret patterns.
// Returns all matches found.
func ScanForSecrets(content string) []SecretMatch {
	if content == "" {
		return nil
	}

	rules := getRules()
	var matches []SecretMatch

	for _, rule := range rules {
		locs := rule.pattern.FindAllStringIndex(content, -1)
		for _, loc := range locs {
			matches = append(matches, SecretMatch{
				RuleID:     rule.id,
				Label:      rule.label,
				Match:      content[loc[0]:loc[1]],
				StartIndex: loc[0],
			})
		}
	}

	return matches
}

// HasSecrets returns true if content contains any detected secrets.
func HasSecrets(content string) bool {
	if content == "" {
		return false
	}

	rules := getRules()
	for _, rule := range rules {
		if rule.pattern.MatchString(content) {
			return true
		}
	}
	return false
}

// RedactSecrets replaces detected secrets in content with a redaction marker.
func RedactSecrets(content string, marker string) string {
	if content == "" || marker == "" {
		return content
	}

	rules := getRules()
	result := content
	for _, rule := range rules {
		result = rule.pattern.ReplaceAllString(result, marker)
	}
	return result
}

// RedactSensitiveFields redacts secret-like JSON fields and high-confidence
// credential patterns from a tool result payload.
func RedactSensitiveFields(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return raw
	}
	redacted := redact.RedactSensitiveJSONFields(raw, "[REDACTED]")
	return json.RawMessage(RedactSecrets(string(redacted), "[REDACTED]"))
}

// SecretRuleCount returns the number of compiled detection rules.
func SecretRuleCount() int {
	return len(getRules())
}
