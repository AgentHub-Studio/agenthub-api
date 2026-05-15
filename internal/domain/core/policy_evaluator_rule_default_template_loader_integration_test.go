//go:build integration

package core_test

import (
	"context"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/core"
	"github.com/AgentHub-Studio/agenthub-go-commons/testutil"
)

const perrMigration = "000068_seed_policy_evaluator_rule_default_templates.up.sql"
const perrMigrationDown = "000068_seed_policy_evaluator_rule_default_templates.down.sql"

func TestIntegration_CorePERR_LoadAll_AfterSeed(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, perrMigration)

	loader := core.NewCorePolicyEvaluatorRuleDefaultTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Equal(t, core.SeedExpectedPERRTemplateRowCount, len(got))
}

func TestIntegration_CorePERR_LoadAll_NonFatalWhenSchemaMissing(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	loader := core.NewCorePolicyEvaluatorRuleDefaultTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestIntegration_CorePERR_FindBySlug_DestructiveShellShape(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, perrMigration)

	loader := core.NewCorePolicyEvaluatorRuleDefaultTemplateLoader(pool)
	tmpl, found, err := loader.FindBySlug(context.Background(), "deny-destructive-shell")
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, "deny", tmpl.Decision)
	assert.Equal(t, "high", tmpl.Confidence)
	assert.Equal(t, 10, tmpl.Priority)
	assert.Equal(t, "destructive_io", tmpl.RiskKind)
}

func TestIntegration_CorePERR_FindBySlug_UnknownReturnsNotFound(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, perrMigration)

	loader := core.NewCorePolicyEvaluatorRuleDefaultTemplateLoader(pool)
	_, found, err := loader.FindBySlug(context.Background(), "nope")
	require.NoError(t, err)
	assert.False(t, found)
}

func TestIntegration_CorePERR_LoadByDecision_DenySubset(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, perrMigration)

	loader := core.NewCorePolicyEvaluatorRuleDefaultTemplateLoader(pool)
	denies, err := loader.LoadByDecision(context.Background(), "deny")
	require.NoError(t, err)
	got := []string{}
	for _, t2 := range denies {
		got = append(got, t2.Slug)
	}
	expected := append([]string{}, core.SeedDenyDecisionPERRTemplateSlugs...)
	sort.Strings(got)
	sort.Strings(expected)
	assert.Equal(t, expected, got)
}

func TestIntegration_CorePERR_LoadByDecision_EscalateSubset(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, perrMigration)

	loader := core.NewCorePolicyEvaluatorRuleDefaultTemplateLoader(pool)
	escalates, err := loader.LoadByDecision(context.Background(), "escalate")
	require.NoError(t, err)
	got := []string{}
	for _, t2 := range escalates {
		got = append(got, t2.Slug)
	}
	expected := append([]string{}, core.SeedEscalateDecisionPERRTemplateSlugs...)
	sort.Strings(got)
	sort.Strings(expected)
	assert.Equal(t, expected, got)
}

func TestIntegration_CorePERR_LoadByRiskKind_OneToOne(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, perrMigration)

	loader := core.NewCorePolicyEvaluatorRuleDefaultTemplateLoader(pool)
	for _, r := range core.SeedExpectedPERRTemplateRiskKinds {
		matched, err := loader.LoadByRiskKind(context.Background(), r)
		require.NoError(t, err)
		assert.Equal(t, 1, len(matched),
			"risk_kind %q must have exactly 1 rule (1:1)", r)
	}
}

func TestIntegration_CorePERR_AllRuleIDsMatchPERM007Regex(t *testing.T) {
	// Cross-feature invariant: every rule_id must satisfy PERM-007 kebab regex.
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, perrMigration)

	loader := core.NewCorePolicyEvaluatorRuleDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	for _, t2 := range all {
		assert.True(t, core.PolicyEvaluatorRuleIDRE.MatchString(t2.RuleID),
			"rule_id %q must match PERM-007 kebab regex", t2.RuleID)
	}
}

func TestIntegration_CorePERR_AllDecisionsInPERM007Enum(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, perrMigration)

	loader := core.NewCorePolicyEvaluatorRuleDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	allowed := map[string]bool{"allow": true, "deny": true, "escalate": true, "abstain": true}
	for _, t2 := range all {
		assert.True(t, allowed[t2.Decision],
			"%s decision %q outside PERM-007 enum", t2.Slug, t2.Decision)
	}
}

func TestIntegration_CorePERR_AllConfidencesInPERM007Enum(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, perrMigration)

	loader := core.NewCorePolicyEvaluatorRuleDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	allowed := map[string]bool{"low": true, "medium": true, "high": true}
	for _, t2 := range all {
		assert.True(t, allowed[t2.Confidence],
			"%s confidence %q outside PERM-007 enum", t2.Slug, t2.Confidence)
	}
}

func TestIntegration_CorePERR_DBCheckRejectsInvalidDecision(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, perrMigration)

	conn, err := pool.Acquire(context.Background())
	require.NoError(t, err)
	defer conn.Release()

	_, err = conn.Exec(context.Background(), `
		INSERT INTO ah_core.policy_evaluator_rule_default_template
		    (id, slug, rule_id, tool_name_pattern, description,
		     decision, confidence, priority, risk_kind, sort_order)
		VALUES
		    ('99999999-0000-0000-0000-000000000020', 'bad-decision',
		     'bad-decision', '.*', 'bad', 'limbo', 'high', 100, 'destructive_io', 999)
	`)
	require.Error(t, err, "DB CHECK must reject decision='limbo'")
}

func TestIntegration_CorePERR_DBCheckRejectsInvalidConfidence(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, perrMigration)

	conn, err := pool.Acquire(context.Background())
	require.NoError(t, err)
	defer conn.Release()

	_, err = conn.Exec(context.Background(), `
		INSERT INTO ah_core.policy_evaluator_rule_default_template
		    (id, slug, rule_id, tool_name_pattern, description,
		     decision, confidence, priority, risk_kind, sort_order)
		VALUES
		    ('99999999-0000-0000-0000-000000000021', 'bad-confidence',
		     'bad-confidence', '.*', 'bad', 'deny', 'shaky', 100, 'destructive_io', 999)
	`)
	require.Error(t, err, "DB CHECK must reject confidence='shaky'")
}

func TestIntegration_CorePERR_DBCheckRejectsNegativePriority(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, perrMigration)

	conn, err := pool.Acquire(context.Background())
	require.NoError(t, err)
	defer conn.Release()

	_, err = conn.Exec(context.Background(), `
		INSERT INTO ah_core.policy_evaluator_rule_default_template
		    (id, slug, rule_id, tool_name_pattern, description,
		     decision, confidence, priority, risk_kind, sort_order)
		VALUES
		    ('99999999-0000-0000-0000-000000000022', 'bad-priority',
		     'bad-priority', '.*', 'bad', 'deny', 'high', -1, 'destructive_io', 999)
	`)
	require.Error(t, err, "DB CHECK must reject priority < 0")
}

func TestIntegration_CorePERR_PriorityOrderingAscending(t *testing.T) {
	// Loader returns by priority ASC — matches PERM-007 priority semantics.
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, perrMigration)

	loader := core.NewCorePolicyEvaluatorRuleDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	require.NotEmpty(t, all)
	for i := 1; i < len(all); i++ {
		if all[i-1].Priority == all[i].Priority {
			assert.LessOrEqual(t, all[i-1].Slug, all[i].Slug)
		} else {
			assert.Less(t, all[i-1].Priority, all[i].Priority)
		}
	}
	assert.Equal(t, "deny-destructive-shell", all[0].Slug)
}

func TestIntegration_CorePERR_AllSlugsAndRuleIDsUniqueInDB(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, perrMigration)

	loader := core.NewCorePolicyEvaluatorRuleDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	seenSlugs := map[string]bool{}
	seenRuleIDs := map[string]bool{}
	for _, t2 := range all {
		assert.False(t, seenSlugs[t2.Slug])
		seenSlugs[t2.Slug] = true
		assert.False(t, seenRuleIDs[t2.RuleID])
		seenRuleIDs[t2.RuleID] = true
	}
}

func TestIntegration_CorePERR_MigrationIsIdempotent(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, perrMigration)
	applyMigration(t, pool, migDir, perrMigration)

	loader := core.NewCorePolicyEvaluatorRuleDefaultTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Equal(t, core.SeedExpectedPERRTemplateRowCount, len(got))
}

func TestIntegration_CorePERR_DownMigrationDropsTable(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, perrMigration)
	applyMigration(t, pool, migDir, perrMigrationDown)

	loader := core.NewCorePolicyEvaluatorRuleDefaultTemplateLoader(pool)
	got, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestIntegration_CorePERR_SeedSlugsMatchCanonicalListExactly(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	migDir := ah_coreMigrationsDir(t)
	applyMigration(t, pool, migDir, "000001_ah_core_schema.up.sql")
	applyMigration(t, pool, migDir, perrMigration)

	loader := core.NewCorePolicyEvaluatorRuleDefaultTemplateLoader(pool)
	all, err := loader.LoadAll(context.Background())
	require.NoError(t, err)
	got := []string{}
	for _, t2 := range all {
		got = append(got, t2.Slug)
	}
	expected := append([]string{}, core.SeedExpectedPERRTemplateSlugs...)
	sort.Strings(got)
	sort.Strings(expected)
	assert.Equal(t, expected, got)
}
