package agentic

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func validExtensionContextCost() ExtensionContextCostPolicy {
	return ExtensionContextCostPolicy{
		ExtensionSlug:       "my-extension",
		StaticHeaderTokens:  120,
		PerTurnTokens:       50, // micro band
		PerToolCallTokens:   30,
		MaxBudgetPerSession: 5000,
		Category:            ExtensionContextCostMicro,
	}
}

func TestExtensionContextCost_IsValidCategory(t *testing.T) {
	for _, c := range allExtensionContextCostCategories {
		assert.True(t, IsValidExtensionContextCostCategory(c))
	}
	assert.False(t, IsValidExtensionContextCostCategory(ExtensionContextCostCategory("nope")))
}

func TestExtensionContextCost_AllCategoriesReturnsCopy(t *testing.T) {
	c := AllExtensionContextCostCategories()
	require.Equal(t, 5, len(c))
	c[0] = "tampered"
	c2 := AllExtensionContextCostCategories()
	assert.Equal(t, ExtensionContextCostMicro, c2[0])
}

func TestExtensionContextCost_ClassifyByPerTurnTokensBands(t *testing.T) {
	cases := []struct {
		perTurn  int
		expected ExtensionContextCostCategory
	}{
		{0, ExtensionContextCostMicro},
		{100, ExtensionContextCostMicro},
		{101, ExtensionContextCostSmall},
		{500, ExtensionContextCostSmall},
		{501, ExtensionContextCostMedium},
		{2000, ExtensionContextCostMedium},
		{2001, ExtensionContextCostLarge},
		{8000, ExtensionContextCostLarge},
		{8001, ExtensionContextCostHeavy},
		{50000, ExtensionContextCostHeavy},
	}
	for _, c := range cases {
		got := ClassifyByPerTurnTokens(c.perTurn)
		assert.Equal(t, c.expected, got, "per_turn=%d", c.perTurn)
	}
}

func TestExtensionContextCost_ValidateBadSlug(t *testing.T) {
	p := validExtensionContextCost()
	p.ExtensionSlug = "BAD"
	assert.ErrorIs(t, p.Validate(), ErrExtensionContextCostBadSlug)
}

func TestExtensionContextCost_ValidateNegativeHeader(t *testing.T) {
	p := validExtensionContextCost()
	p.StaticHeaderTokens = -1
	assert.ErrorIs(t, p.Validate(), ErrExtensionContextCostNegative)
}

func TestExtensionContextCost_ValidateNegativePerTurn(t *testing.T) {
	p := validExtensionContextCost()
	p.PerTurnTokens = -1
	assert.ErrorIs(t, p.Validate(), ErrExtensionContextCostNegative)
}

func TestExtensionContextCost_ValidateNegativePerToolCall(t *testing.T) {
	p := validExtensionContextCost()
	p.PerToolCallTokens = -1
	assert.ErrorIs(t, p.Validate(), ErrExtensionContextCostNegative)
}

func TestExtensionContextCost_ValidateNegativeMaxBudget(t *testing.T) {
	p := validExtensionContextCost()
	p.MaxBudgetPerSession = -1
	assert.ErrorIs(t, p.Validate(), ErrExtensionContextCostNegative)
}

func TestExtensionContextCost_ValidateBadCategory(t *testing.T) {
	p := validExtensionContextCost()
	p.Category = "limbo"
	assert.ErrorIs(t, p.Validate(), ErrExtensionContextCostBadCategory)
}

func TestExtensionContextCost_ValidateCategoryDrift(t *testing.T) {
	// Declared "micro" but PerTurnTokens=600 implies "medium".
	p := validExtensionContextCost()
	p.PerTurnTokens = 600
	p.Category = ExtensionContextCostMicro
	assert.ErrorIs(t, p.Validate(), ErrExtensionContextCostCategoryDrift)
}

func TestExtensionContextCost_ValidateMatchingDeclaredCategory(t *testing.T) {
	p := validExtensionContextCost()
	p.PerTurnTokens = 1500
	p.Category = ExtensionContextCostMedium
	assert.NoError(t, p.Validate())
}

func TestExtensionContextCost_EstimateForTurns(t *testing.T) {
	p := validExtensionContextCost()
	// 120 header + 50*10 turns + 30*5 tool calls = 120 + 500 + 150 = 770
	assert.Equal(t, 770, p.EstimateForTurns(10, 5))
}

func TestExtensionContextCost_EstimateZeroTurns(t *testing.T) {
	p := validExtensionContextCost()
	assert.Equal(t, p.StaticHeaderTokens, p.EstimateForTurns(0, 0))
}

func TestExtensionContextCost_RegistryRegisterAndLookup(t *testing.T) {
	r := NewExtensionContextCostRegistry()
	p := validExtensionContextCost()
	require.NoError(t, r.Register(p))
	got, ok := r.Lookup(p.ExtensionSlug)
	require.True(t, ok)
	assert.Equal(t, p.StaticHeaderTokens, got.StaticHeaderTokens)
}

func TestExtensionContextCost_RegistryLookupUnknown(t *testing.T) {
	r := NewExtensionContextCostRegistry()
	_, ok := r.Lookup("nope")
	assert.False(t, ok)
}

func TestExtensionContextCost_RegistryRejectsBadPolicy(t *testing.T) {
	r := NewExtensionContextCostRegistry()
	bad := validExtensionContextCost()
	bad.ExtensionSlug = "BAD"
	assert.ErrorIs(t, r.Register(bad), ErrExtensionContextCostBadSlug)
}

func TestExtensionContextCost_RegistryRejectsDuplicate(t *testing.T) {
	r := NewExtensionContextCostRegistry()
	p := validExtensionContextCost()
	require.NoError(t, r.Register(p))
	err := r.Register(p)
	assert.ErrorIs(t, err, ErrExtensionContextCostDuplicate)
}

func TestExtensionContextCost_ListAllSortedBySlug(t *testing.T) {
	r := NewExtensionContextCostRegistry()
	p1 := validExtensionContextCost()
	p1.ExtensionSlug = "zeta-ext"
	p2 := validExtensionContextCost()
	p2.ExtensionSlug = "alpha-ext"
	require.NoError(t, r.Register(p1))
	require.NoError(t, r.Register(p2))
	list := r.ListAll()
	require.Equal(t, 2, len(list))
	assert.Equal(t, "alpha-ext", list[0].ExtensionSlug)
}

func TestExtensionContextCost_ListByCategoryFilters(t *testing.T) {
	r := NewExtensionContextCostRegistry()
	p1 := validExtensionContextCost()
	p1.ExtensionSlug = "micro-ext"
	p2 := validExtensionContextCost()
	p2.ExtensionSlug = "medium-ext"
	p2.PerTurnTokens = 1500
	p2.Category = ExtensionContextCostMedium
	require.NoError(t, r.Register(p1))
	require.NoError(t, r.Register(p2))
	micros := r.ListByCategory(ExtensionContextCostMicro)
	mediums := r.ListByCategory(ExtensionContextCostMedium)
	assert.Equal(t, 1, len(micros))
	assert.Equal(t, 1, len(mediums))
}

func TestExtensionContextCost_EstimateLoadedExtensionsSums(t *testing.T) {
	r := NewExtensionContextCostRegistry()
	p1 := validExtensionContextCost()
	p1.ExtensionSlug = "ext-a" // header 120 + 50/turn + 30/call
	p2 := validExtensionContextCost()
	p2.ExtensionSlug = "ext-b"
	p2.StaticHeaderTokens = 200
	p2.PerTurnTokens = 80
	p2.PerToolCallTokens = 40
	require.NoError(t, r.Register(p1))
	require.NoError(t, r.Register(p2))
	// 10 turns, 5 calls each:
	// p1: 120 + 500 + 150 = 770
	// p2: 200 + 800 + 200 = 1200
	// Total = 1970
	total, missing := r.EstimateLoadedExtensions([]string{"ext-a", "ext-b"}, 10, 5)
	assert.Equal(t, 1970, total)
	assert.Empty(t, missing)
}

func TestExtensionContextCost_EstimateLoadedExtensionsReportsMissing(t *testing.T) {
	r := NewExtensionContextCostRegistry()
	p := validExtensionContextCost()
	require.NoError(t, r.Register(p))
	total, missing := r.EstimateLoadedExtensions(
		[]string{p.ExtensionSlug, "ghost-a", "ghost-b"}, 1, 0)
	// Only registered ext contributes.
	assert.Greater(t, total, 0)
	assert.Equal(t, []string{"ghost-a", "ghost-b"}, missing)
}

func TestExtensionContextCost_CanAffordWithinWindow(t *testing.T) {
	r := NewExtensionContextCostRegistry()
	p := validExtensionContextCost()
	require.NoError(t, r.Register(p))
	// Window 200k, headroom 100k → budget 100k → 770 fits.
	ok, est, missing := r.CanAffordSessionWindow(
		[]string{p.ExtensionSlug}, 10, 5, 200000, 100000,
	)
	assert.True(t, ok)
	assert.Equal(t, 770, est)
	assert.Empty(t, missing)
}

func TestExtensionContextCost_CanAffordRejectsOverBudget(t *testing.T) {
	r := NewExtensionContextCostRegistry()
	p := validExtensionContextCost()
	p.PerTurnTokens = 1500
	p.Category = ExtensionContextCostMedium
	require.NoError(t, r.Register(p))
	// 1000 turns × 1500 tokens = 1.5M tokens. Won't fit.
	ok, est, _ := r.CanAffordSessionWindow(
		[]string{p.ExtensionSlug}, 1000, 0, 200000, 100000,
	)
	assert.False(t, ok)
	assert.Greater(t, est, 100000)
}

func TestExtensionContextCost_Size(t *testing.T) {
	r := NewExtensionContextCostRegistry()
	assert.Equal(t, 0, r.Size())
	_ = r.Register(validExtensionContextCost())
	assert.Equal(t, 1, r.Size())
}

func TestExtensionContextCost_ConcurrentRegisterSafe(t *testing.T) {
	r := NewExtensionContextCostRegistry()
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			p := validExtensionContextCost()
			p.ExtensionSlug = "concurrent-" + string(rune('a'+i%26)) + string(rune('0'+i/26))
			_ = r.Register(p)
		}(i)
	}
	wg.Wait()
	assert.Equal(t, 50, r.Size())
}

func TestExtensionContextCost_FiveCategoriesCoverBands(t *testing.T) {
	expected := map[ExtensionContextCostCategory]bool{
		ExtensionContextCostMicro:  true,
		ExtensionContextCostSmall:  true,
		ExtensionContextCostMedium: true,
		ExtensionContextCostLarge:  true,
		ExtensionContextCostHeavy:  true,
	}
	assert.Equal(t, 5, len(expected))
	for _, c := range allExtensionContextCostCategories {
		assert.True(t, expected[c])
	}
}
