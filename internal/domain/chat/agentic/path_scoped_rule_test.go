package agentic

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func validPathRule() PathScopedRule {
	return PathScopedRule{
		TenantID: "t-1",
		Slug:     "no-secrets-in-go-files",
		Scope:    PathRuleScopeFile,
		PathGlob: "**/*.go",
		Content:  "Never commit literal secrets in Go files; use env vars.",
		Priority: 50,
		Enabled:  true,
	}
}

func TestPathScopedRule_ScopeEnumIsBounded(t *testing.T) {
	for _, s := range AllPathScopedRuleScopes() {
		assert.True(t, IsValidPathScopedRuleScope(s))
	}
	assert.False(t, IsValidPathScopedRuleScope(PathScopedRuleScope("planet")))
	assert.Equal(t, 5, len(AllPathScopedRuleScopes()))
}

func TestMatchPathGlob_StarMatchesEverything(t *testing.T) {
	assert.True(t, matchPathGlob("*", "anything"))
	assert.True(t, matchPathGlob("**", "any/path/at/all"))
}

func TestMatchPathGlob_RecursiveDoubleStar(t *testing.T) {
	assert.True(t, matchPathGlob("**/*.go", "internal/domain/chat/agent.go"))
	assert.True(t, matchPathGlob("tools/**", "tools/execute_sql/v1.json"))
	assert.False(t, matchPathGlob("tools/**", "agents/researcher.json"))
}

func TestMatchPathGlob_ExactPath(t *testing.T) {
	assert.True(t, matchPathGlob("config/secrets.yaml", "config/secrets.yaml"))
	assert.False(t, matchPathGlob("config/secrets.yaml", "config/other.yaml"))
}

func TestMatchPathGlob_RejectsNonMatch(t *testing.T) {
	assert.False(t, matchPathGlob("**/*.go", "internal/domain/chat/agent.py"))
}

func TestSpecificityScore_ExactPathHighestScore(t *testing.T) {
	exact := specificityScore("config/secrets.yaml")
	withStar := specificityScore("config/*.yaml")
	withDoubleStar := specificityScore("**/*.yaml")
	wildcard := specificityScore("*")

	assert.Greater(t, exact, withStar)
	assert.Greater(t, withStar, withDoubleStar)
	assert.Greater(t, withDoubleStar, wildcard)
}

func TestPathRule_Register_AssignsIDAndCreatedAt(t *testing.T) {
	r := NewInMemoryPathScopedRuleRegistry()
	saved, err := r.Register(context.Background(), validPathRule())
	require.NoError(t, err)
	assert.NotEqual(t, "", saved.ID.String())
	assert.False(t, saved.CreatedAt.IsZero())
}

func TestPathRule_Register_RejectsInvalidScope(t *testing.T) {
	r := NewInMemoryPathScopedRuleRegistry()
	rule := validPathRule()
	rule.Scope = "planet"
	_, err := r.Register(context.Background(), rule)
	assert.True(t, errors.Is(err, ErrPathScopedRuleInvalidScope))
}

func TestPathRule_Register_RejectsEmptyTenant(t *testing.T) {
	r := NewInMemoryPathScopedRuleRegistry()
	rule := validPathRule()
	rule.TenantID = ""
	_, err := r.Register(context.Background(), rule)
	assert.True(t, errors.Is(err, ErrPathScopedRuleTenantEmpty))
}

func TestPathRule_Register_RejectsEmptySlug(t *testing.T) {
	r := NewInMemoryPathScopedRuleRegistry()
	rule := validPathRule()
	rule.Slug = "  "
	_, err := r.Register(context.Background(), rule)
	assert.True(t, errors.Is(err, ErrPathScopedRuleSlugEmpty))
}

func TestPathRule_Register_RejectsEmptyGlob(t *testing.T) {
	r := NewInMemoryPathScopedRuleRegistry()
	rule := validPathRule()
	rule.PathGlob = ""
	_, err := r.Register(context.Background(), rule)
	assert.True(t, errors.Is(err, ErrPathScopedRuleGlobEmpty))
}

func TestPathRule_Register_RejectsEmptyContent(t *testing.T) {
	r := NewInMemoryPathScopedRuleRegistry()
	rule := validPathRule()
	rule.Content = ""
	_, err := r.Register(context.Background(), rule)
	assert.True(t, errors.Is(err, ErrPathScopedRuleContentEmpty))
}

func TestPathRule_Register_RejectsDuplicateSlugSameTenant(t *testing.T) {
	r := NewInMemoryPathScopedRuleRegistry()
	_, _ = r.Register(context.Background(), validPathRule())
	_, err := r.Register(context.Background(), validPathRule())
	assert.True(t, errors.Is(err, ErrPathScopedRuleDuplicate))
}

func TestPathRule_Register_AllowsSameSlugAcrossDifferentTenants(t *testing.T) {
	r := NewInMemoryPathScopedRuleRegistry()
	_, err := r.Register(context.Background(), validPathRule())
	require.NoError(t, err)
	rule2 := validPathRule()
	rule2.TenantID = "t-2"
	_, err = r.Register(context.Background(), rule2)
	assert.NoError(t, err)
}

func TestPathRule_Find_RoundTrips(t *testing.T) {
	r := NewInMemoryPathScopedRuleRegistry()
	saved, _ := r.Register(context.Background(), validPathRule())
	got, err := r.Find(context.Background(), saved.TenantID, saved.Slug)
	require.NoError(t, err)
	assert.Equal(t, saved.ID, got.ID)
}

func TestPathRule_Find_UnknownReturnsNotFound(t *testing.T) {
	r := NewInMemoryPathScopedRuleRegistry()
	_, err := r.Find(context.Background(), "t-1", "nope")
	assert.True(t, errors.Is(err, ErrPathScopedRuleNotFound))
}

func TestPathRule_List_OrderedBySlug(t *testing.T) {
	r := NewInMemoryPathScopedRuleRegistry()
	for _, slug := range []string{"zeta", "alpha", "mid"} {
		rule := validPathRule()
		rule.Slug = slug
		_, _ = r.Register(context.Background(), rule)
	}
	got, _ := r.List(context.Background(), "t-1")
	require.Len(t, got, 3)
	for i := 1; i < len(got); i++ {
		assert.Less(t, got[i-1].Slug, got[i].Slug)
	}
}

func TestPathRule_List_TenantIsolation(t *testing.T) {
	r := NewInMemoryPathScopedRuleRegistry()
	_, _ = r.Register(context.Background(), validPathRule())
	rule2 := validPathRule()
	rule2.TenantID = "t-2"
	_, _ = r.Register(context.Background(), rule2)
	got, _ := r.List(context.Background(), "t-1")
	assert.Len(t, got, 1)
	assert.Equal(t, "t-1", got[0].TenantID)
}

func TestPathRule_ListByScope_FiltersStrictly(t *testing.T) {
	r := NewInMemoryPathScopedRuleRegistry()
	rule1 := validPathRule()
	rule1.Slug = "go-rule"
	rule1.Scope = PathRuleScopeFile
	_, _ = r.Register(context.Background(), rule1)

	rule2 := validPathRule()
	rule2.Slug = "tool-rule"
	rule2.Scope = PathRuleScopeTool
	rule2.PathGlob = "tools/**"
	_, _ = r.Register(context.Background(), rule2)

	files, _ := r.ListByScope(context.Background(), "t-1", PathRuleScopeFile)
	assert.Len(t, files, 1)
	assert.Equal(t, PathRuleScopeFile, files[0].Scope)
}

func TestMatch_ReturnsMatchingRules(t *testing.T) {
	r := NewInMemoryPathScopedRuleRegistry()
	rule := validPathRule() // **/*.go
	_, _ = r.Register(context.Background(), rule)

	matched, err := r.Match(context.Background(), "t-1", "internal/agent.go")
	require.NoError(t, err)
	require.Len(t, matched, 1)
	assert.Equal(t, "no-secrets-in-go-files", matched[0].Slug)
}

func TestMatch_OmitsNonMatchingRules(t *testing.T) {
	r := NewInMemoryPathScopedRuleRegistry()
	_, _ = r.Register(context.Background(), validPathRule())
	matched, _ := r.Match(context.Background(), "t-1", "internal/agent.py")
	assert.Empty(t, matched)
}

func TestMatch_OrdersBySpecificity(t *testing.T) {
	r := NewInMemoryPathScopedRuleRegistry()

	wildcard := validPathRule()
	wildcard.Slug = "global-wildcard"
	wildcard.PathGlob = "**"
	wildcard.Scope = PathRuleScopeGlobal
	_, _ = r.Register(context.Background(), wildcard)

	allGo := validPathRule()
	allGo.Slug = "all-go"
	allGo.PathGlob = "**/*.go"
	_, _ = r.Register(context.Background(), allGo)

	exact := validPathRule()
	exact.Slug = "exact-secrets"
	exact.PathGlob = "config/secrets.yaml"
	_, _ = r.Register(context.Background(), exact)

	matched, err := r.Match(context.Background(), "t-1", "config/secrets.yaml")
	require.NoError(t, err)
	require.Len(t, matched, 2, "only matching rules; **/*.go doesn't match yaml")
	// More specific (exact path) should win.
	assert.Equal(t, "exact-secrets", matched[0].Slug)
	assert.Equal(t, "global-wildcard", matched[1].Slug)
}

func TestMatch_OmitsDisabledRules(t *testing.T) {
	r := NewInMemoryPathScopedRuleRegistry()
	rule := validPathRule()
	rule.Enabled = false
	_, _ = r.Register(context.Background(), rule)

	matched, _ := r.Match(context.Background(), "t-1", "internal/agent.go")
	assert.Empty(t, matched, "disabled rules excluded from matches")
}

func TestMatch_RejectsEmptyQueryPath(t *testing.T) {
	r := NewInMemoryPathScopedRuleRegistry()
	_, err := r.Match(context.Background(), "t-1", "")
	assert.Error(t, err)
}

func TestMatch_PriorityBreaksSpecificityTies(t *testing.T) {
	r := NewInMemoryPathScopedRuleRegistry()
	low := validPathRule()
	low.Slug = "low-prio"
	low.PathGlob = "**/*.go"
	low.Priority = 10
	_, _ = r.Register(context.Background(), low)

	high := validPathRule()
	high.Slug = "high-prio"
	high.PathGlob = "**/*.go"
	high.Priority = 100
	_, _ = r.Register(context.Background(), high)

	matched, _ := r.Match(context.Background(), "t-1", "internal/agent.go")
	require.Len(t, matched, 2)
	assert.Equal(t, "high-prio", matched[0].Slug, "higher priority wins ties")
}

func TestPathRule_Delete_RemovesRule(t *testing.T) {
	r := NewInMemoryPathScopedRuleRegistry()
	saved, _ := r.Register(context.Background(), validPathRule())
	require.NoError(t, r.Delete(context.Background(), saved.TenantID, saved.Slug))
	_, err := r.Find(context.Background(), saved.TenantID, saved.Slug)
	assert.True(t, errors.Is(err, ErrPathScopedRuleNotFound))
}

func TestPathRule_Delete_UnknownReturnsNotFound(t *testing.T) {
	r := NewInMemoryPathScopedRuleRegistry()
	err := r.Delete(context.Background(), "t-1", "missing")
	assert.True(t, errors.Is(err, ErrPathScopedRuleNotFound))
}

func TestPathRule_Register_ConcurrentIsSafe(t *testing.T) {
	r := NewInMemoryPathScopedRuleRegistry()
	var wg sync.WaitGroup
	for i := 0; i < 30; i++ {
		i := i
		wg.Add(1)
		go func() {
			defer wg.Done()
			rule := validPathRule()
			rule.Slug = "rule-" + string(rune('a'+(i%26))) + "-" + string(rune('a'+(i/26)))
			_, _ = r.Register(context.Background(), rule)
		}()
	}
	wg.Wait()
	got, _ := r.List(context.Background(), "t-1")
	assert.GreaterOrEqual(t, len(got), 1)
}

func TestContext_Cancelled_PathScopedRule(t *testing.T) {
	r := NewInMemoryPathScopedRuleRegistry()
	saved, _ := r.Register(context.Background(), validPathRule())
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := r.Register(ctx, validPathRule())
	assert.Error(t, err)
	_, err = r.Find(ctx, saved.TenantID, saved.Slug)
	assert.Error(t, err)
	_, err = r.List(ctx, saved.TenantID)
	assert.Error(t, err)
	_, err = r.ListByScope(ctx, saved.TenantID, PathRuleScopeFile)
	assert.Error(t, err)
	_, err = r.Match(ctx, saved.TenantID, "x")
	assert.Error(t, err)
	err = r.Delete(ctx, saved.TenantID, saved.Slug)
	assert.Error(t, err)
}
