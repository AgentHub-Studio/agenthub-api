package core

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// CoreWebhookEndpointTemplate represents a platform-managed webhook
// endpoint template catalog entry.
type CoreWebhookEndpointTemplate struct {
	ID                uuid.UUID
	Slug              string
	DisplayName       string
	Description       string
	TargetKind        string // slack/teams/discord/pagerduty/opsgenie/generic_http/email/sms
	URLTemplate       string
	Method            string
	PayloadTemplate   string
	SubscribedEvents  string // comma-separated
	RequiresAuth      bool
	AuthType          string
	RetryPolicy       string // comma-separated ms
	DocumentationURL  string
	IsRecommended     bool
	IsActive          bool
	SortOrder         int
}

// SubscribedEventsList returns parsed event names.
func (t CoreWebhookEndpointTemplate) SubscribedEventsList() []string {
	return parseCommaList(t.SubscribedEvents)
}

// RetryPolicyMs returns parsed retry delays in milliseconds.
func (t CoreWebhookEndpointTemplate) RetryPolicyMs() []int {
	parts := parseCommaList(t.RetryPolicy)
	out := make([]int, 0, len(parts))
	for _, p := range parts {
		if n, err := strconv.Atoi(strings.TrimSpace(p)); err == nil {
			out = append(out, n)
		}
	}
	return out
}

// CoreWebhookEndpointTemplateLoader loads webhook endpoint templates.
type CoreWebhookEndpointTemplateLoader struct {
	pool *pgxpool.Pool
}

// NewCoreWebhookEndpointTemplateLoader creates a CoreWebhookEndpointTemplateLoader.
func NewCoreWebhookEndpointTemplateLoader(pool *pgxpool.Pool) *CoreWebhookEndpointTemplateLoader {
	return &CoreWebhookEndpointTemplateLoader{pool: pool}
}

// LoadAll returns all active templates.
func (l *CoreWebhookEndpointTemplateLoader) LoadAll(ctx context.Context) ([]CoreWebhookEndpointTemplate, error) {
	conn, err := l.pool.Acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("core: acquire connection: %w", err)
	}
	defer conn.Release()

	const query = `
		SELECT id, slug, display_name, description, target_kind,
		       url_template, method, payload_template,
		       subscribed_events, requires_auth, auth_type, retry_policy,
		       COALESCE(documentation_url, '') AS documentation_url,
		       is_recommended, is_active, sort_order
		  FROM ah_core.webhook_endpoint_template
		 WHERE is_active = true
		 ORDER BY sort_order, slug`

	rows, err := conn.Query(ctx, query)
	if err != nil {
		if isUndefinedRelation(err) {
			slog.WarnContext(ctx, "core: ah_core.webhook_endpoint_template not accessible", "err", err)
			return nil, nil
		}
		return nil, fmt.Errorf("core: query webhook_endpoint_template: %w", err)
	}
	defer rows.Close()

	var templates []CoreWebhookEndpointTemplate
	for rows.Next() {
		var t CoreWebhookEndpointTemplate
		if err := rows.Scan(
			&t.ID, &t.Slug, &t.DisplayName, &t.Description, &t.TargetKind,
			&t.URLTemplate, &t.Method, &t.PayloadTemplate,
			&t.SubscribedEvents, &t.RequiresAuth, &t.AuthType, &t.RetryPolicy,
			&t.DocumentationURL,
			&t.IsRecommended, &t.IsActive, &t.SortOrder,
		); err != nil {
			return nil, fmt.Errorf("core: scan webhook_endpoint_template: %w", err)
		}
		templates = append(templates, t)
	}
	if err := rows.Err(); err != nil {
		if isUndefinedRelation(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("core: iterate webhook_endpoint_template: %w", err)
	}
	return templates, nil
}

// FindBySlug returns one template by slug.
func (l *CoreWebhookEndpointTemplateLoader) FindBySlug(ctx context.Context, slug string) (CoreWebhookEndpointTemplate, bool, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return CoreWebhookEndpointTemplate{}, false, err
	}
	for _, t := range all {
		if t.Slug == slug {
			return t, true, nil
		}
	}
	return CoreWebhookEndpointTemplate{}, false, nil
}

// LoadByTargetKind returns templates for a kind.
func (l *CoreWebhookEndpointTemplateLoader) LoadByTargetKind(ctx context.Context, kind string) ([]CoreWebhookEndpointTemplate, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return nil, err
	}
	var matched []CoreWebhookEndpointTemplate
	for _, t := range all {
		if t.TargetKind == kind {
			matched = append(matched, t)
		}
	}
	return matched, nil
}

// LoadRecommended returns recommended templates.
func (l *CoreWebhookEndpointTemplateLoader) LoadRecommended(ctx context.Context) ([]CoreWebhookEndpointTemplate, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return nil, err
	}
	var matched []CoreWebhookEndpointTemplate
	for _, t := range all {
		if t.IsRecommended {
			matched = append(matched, t)
		}
	}
	return matched, nil
}

// SeedExpectedWebhookTemplateSlugs is the canonical list.
var SeedExpectedWebhookTemplateSlugs = []string{
	"slack-incoming-webhook",
	"teams-incoming-webhook",
	"discord-webhook",
	"pagerduty-events-v2",
	"opsgenie-alert-api",
	"generic-https-json",
	"email-smtp-relay",
	"twilio-sms",
}

// SeedExpectedWebhookTemplateTargetKinds is the closed set.
var SeedExpectedWebhookTemplateTargetKinds = []string{
	"slack", "teams", "discord", "pagerduty", "opsgenie",
	"generic_http", "email", "sms",
}

// SeedRecommendedWebhookTemplateSlugs is the curated subset.
var SeedRecommendedWebhookTemplateSlugs = []string{
	"slack-incoming-webhook",
	"pagerduty-events-v2",
	"generic-https-json",
}
