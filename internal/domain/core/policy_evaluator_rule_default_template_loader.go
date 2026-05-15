package core

import (
	"context"
	"fmt"
	"log/slog"
	"regexp"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// CorePolicyEvaluatorRuleDefaultTemplate is a starter fallback rule
// paired with PERM-007 PolicyEvaluatorRule. Each row encodes one
// proven dangerous-pattern rule that fresh tenants inherit at boot.
type CorePolicyEvaluatorRuleDefaultTemplate struct {
	ID              uuid.UUID
	Slug            string
	RuleID          string
	ToolNamePattern string
	Description     string
	Decision        string // matches PERM-007 enum byte-for-byte
	Confidence      string // matches PERM-007 enum byte-for-byte
	Priority        int
	RiskKind        string
	IsRecommended   bool
	IsActive        bool
	SortOrder       int
}

// CorePolicyEvaluatorRuleDefaultTemplateLoader loads templates.
type CorePolicyEvaluatorRuleDefaultTemplateLoader struct {
	pool *pgxpool.Pool
}

// NewCorePolicyEvaluatorRuleDefaultTemplateLoader creates the loader.
func NewCorePolicyEvaluatorRuleDefaultTemplateLoader(pool *pgxpool.Pool) *CorePolicyEvaluatorRuleDefaultTemplateLoader {
	return &CorePolicyEvaluatorRuleDefaultTemplateLoader{pool: pool}
}

// LoadAll returns active templates ordered by priority then slug.
func (l *CorePolicyEvaluatorRuleDefaultTemplateLoader) LoadAll(ctx context.Context) ([]CorePolicyEvaluatorRuleDefaultTemplate, error) {
	conn, err := l.pool.Acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("core: acquire connection: %w", err)
	}
	defer conn.Release()

	const query = `
		SELECT id, slug, rule_id, tool_name_pattern, description,
		       decision, confidence, priority, risk_kind,
		       is_recommended, is_active, sort_order
		  FROM ah_core.policy_evaluator_rule_default_template
		 WHERE is_active = true
		 ORDER BY priority, slug`

	rows, err := conn.Query(ctx, query)
	if err != nil {
		if isUndefinedRelation(err) {
			slog.WarnContext(ctx, "core: ah_core.policy_evaluator_rule_default_template not accessible", "err", err)
			return nil, nil
		}
		return nil, fmt.Errorf("core: query policy_evaluator_rule_default_template: %w", err)
	}
	defer rows.Close()

	var out []CorePolicyEvaluatorRuleDefaultTemplate
	for rows.Next() {
		var t CorePolicyEvaluatorRuleDefaultTemplate
		if err := rows.Scan(
			&t.ID, &t.Slug, &t.RuleID, &t.ToolNamePattern, &t.Description,
			&t.Decision, &t.Confidence, &t.Priority, &t.RiskKind,
			&t.IsRecommended, &t.IsActive, &t.SortOrder,
		); err != nil {
			return nil, fmt.Errorf("core: scan policy_evaluator_rule_default_template: %w", err)
		}
		out = append(out, t)
	}
	if err := rows.Err(); err != nil {
		if isUndefinedRelation(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("core: iterate policy_evaluator_rule_default_template: %w", err)
	}
	return out, nil
}

// FindBySlug returns one template.
func (l *CorePolicyEvaluatorRuleDefaultTemplateLoader) FindBySlug(ctx context.Context, slug string) (CorePolicyEvaluatorRuleDefaultTemplate, bool, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return CorePolicyEvaluatorRuleDefaultTemplate{}, false, err
	}
	for _, t := range all {
		if t.Slug == slug {
			return t, true, nil
		}
	}
	return CorePolicyEvaluatorRuleDefaultTemplate{}, false, nil
}

// LoadByDecision filters by PERM-007 decision enum value.
func (l *CorePolicyEvaluatorRuleDefaultTemplateLoader) LoadByDecision(ctx context.Context, decision string) ([]CorePolicyEvaluatorRuleDefaultTemplate, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return nil, err
	}
	var matched []CorePolicyEvaluatorRuleDefaultTemplate
	for _, t := range all {
		if t.Decision == decision {
			matched = append(matched, t)
		}
	}
	return matched, nil
}

// LoadByRiskKind filters by risk_kind taxonomy.
func (l *CorePolicyEvaluatorRuleDefaultTemplateLoader) LoadByRiskKind(ctx context.Context, riskKind string) ([]CorePolicyEvaluatorRuleDefaultTemplate, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return nil, err
	}
	var matched []CorePolicyEvaluatorRuleDefaultTemplate
	for _, t := range all {
		if t.RiskKind == riskKind {
			matched = append(matched, t)
		}
	}
	return matched, nil
}

// SeedExpectedPERRTemplateSlugs is the closed canonical set.
var SeedExpectedPERRTemplateSlugs = []string{
	"deny-destructive-shell",
	"escalate-prod-db-writes",
	"deny-secret-path-access",
	"escalate-network-egress",
	"escalate-pii-read",
	"escalate-installation-privileges",
}

// SeedExpectedPERRTemplateDecisions — used decisions (deny/escalate;
// no allow rules in starter set since classifier fallback should err
// on the side of caution).
var SeedExpectedPERRTemplateDecisions = []string{"deny", "escalate"}

// SeedExpectedPERRTemplateConfidences — used confidence levels.
var SeedExpectedPERRTemplateConfidences = []string{"high", "medium"}

// SeedExpectedPERRTemplateRiskKinds — closed risk taxonomy.
var SeedExpectedPERRTemplateRiskKinds = []string{
	"destructive_io", "prod_data_mutation", "secret_exposure",
	"network_egress", "pii_compliance", "privilege_escalation",
}

// SeedDenyDecisionPERRTemplateSlugs — rules that hard-deny.
var SeedDenyDecisionPERRTemplateSlugs = []string{
	"deny-destructive-shell",
	"deny-secret-path-access",
}

// SeedEscalateDecisionPERRTemplateSlugs — rules that escalate to human.
var SeedEscalateDecisionPERRTemplateSlugs = []string{
	"escalate-prod-db-writes",
	"escalate-network-egress",
	"escalate-pii-read",
	"escalate-installation-privileges",
}

// SeedRecommendedPERRTemplateSlugs — all 6 are recommended.
var SeedRecommendedPERRTemplateSlugs = SeedExpectedPERRTemplateSlugs

// SeedExpectedPERRTemplateRowCount = 6.
const SeedExpectedPERRTemplateRowCount = 6

// PolicyEvaluatorRuleIDRE replicates the PERM-007 RuleID kebab constraint.
var PolicyEvaluatorRuleIDRE = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*[a-z0-9]$`)
