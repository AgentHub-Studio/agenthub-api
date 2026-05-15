package agentic

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func validBuiltinSubagent() BuiltinSubagentDescriptor {
	return BuiltinSubagentDescriptor{
		Role: BuiltinSubagentResearcher, Slug: "research-baseline",
		Name: "Research Baseline", Description: "investigates topics",
		SystemPromptTemplate:       "You are a thorough researcher...",
		DefaultToolsetPolicySlug:   "documentation-readonly-allowlist",
		DefaultInheritanceModeSlug: "extend-parent-rights",
		DefaultSummaryShapeSlug:    "success-with-artifacts",
	}
}

func TestBuiltinSubagent_IsValidRole(t *testing.T) {
	for _, r := range allBuiltinSubagentRoles {
		assert.True(t, IsValidBuiltinSubagentRole(r))
	}
	assert.False(t, IsValidBuiltinSubagentRole(BuiltinSubagentRole("nope")))
}

func TestBuiltinSubagent_AllRolesReturnsCopy(t *testing.T) {
	roles := AllBuiltinSubagentRoles()
	require.Equal(t, 7, len(roles))
	roles[0] = "tampered"
	roles2 := AllBuiltinSubagentRoles()
	assert.Equal(t, BuiltinSubagentResearcher, roles2[0])
}

func TestBuiltinSubagent_ValidateBadRole(t *testing.T) {
	d := validBuiltinSubagent()
	d.Role = BuiltinSubagentRole("nope")
	assert.ErrorIs(t, d.Validate(), ErrBuiltinSubagentBadRole)
}

func TestBuiltinSubagent_ValidateBadSlug(t *testing.T) {
	d := validBuiltinSubagent()
	d.Slug = "NOT_KEBAB"
	assert.ErrorIs(t, d.Validate(), ErrBuiltinSubagentBadSlug)
}

func TestBuiltinSubagent_ValidateEmptyName(t *testing.T) {
	d := validBuiltinSubagent()
	d.Name = ""
	assert.ErrorIs(t, d.Validate(), ErrBuiltinSubagentEmptyName)
}

func TestBuiltinSubagent_ValidateEmptyDescription(t *testing.T) {
	d := validBuiltinSubagent()
	d.Description = ""
	assert.ErrorIs(t, d.Validate(), ErrBuiltinSubagentEmptyDescription)
}

func TestBuiltinSubagent_ValidateEmptySystemPrompt(t *testing.T) {
	d := validBuiltinSubagent()
	d.SystemPromptTemplate = ""
	assert.ErrorIs(t, d.Validate(), ErrBuiltinSubagentEmptySystemPrompt)
}

func TestBuiltinSubagent_ValidateEmptyToolsetSlug(t *testing.T) {
	d := validBuiltinSubagent()
	d.DefaultToolsetPolicySlug = ""
	assert.ErrorIs(t, d.Validate(), ErrBuiltinSubagentEmptyToolsetSlug)
}

func TestBuiltinSubagent_ValidateEmptyInheritanceSlug(t *testing.T) {
	d := validBuiltinSubagent()
	d.DefaultInheritanceModeSlug = ""
	assert.ErrorIs(t, d.Validate(), ErrBuiltinSubagentEmptyInheritanceSlug)
}

func TestBuiltinSubagent_ValidateEmptySummarySlug(t *testing.T) {
	d := validBuiltinSubagent()
	d.DefaultSummaryShapeSlug = ""
	assert.ErrorIs(t, d.Validate(), ErrBuiltinSubagentEmptySummarySlug)
}

func TestBuiltinSubagent_RegistryRegisterAndLookup(t *testing.T) {
	r := NewBuiltinSubagentRegistry()
	d := validBuiltinSubagent()
	require.NoError(t, r.Register(d))
	got, ok := r.Lookup(d.Slug)
	require.True(t, ok)
	assert.Equal(t, d.Name, got.Name)
}

func TestBuiltinSubagent_RegistryLookupUnknown(t *testing.T) {
	r := NewBuiltinSubagentRegistry()
	_, ok := r.Lookup("nope")
	assert.False(t, ok)
}

func TestBuiltinSubagent_RegistryRejectsBadDescriptor(t *testing.T) {
	r := NewBuiltinSubagentRegistry()
	bad := validBuiltinSubagent()
	bad.Slug = "BAD"
	assert.ErrorIs(t, r.Register(bad), ErrBuiltinSubagentBadSlug)
}

func TestBuiltinSubagent_RegistryRejectsDuplicate(t *testing.T) {
	r := NewBuiltinSubagentRegistry()
	d := validBuiltinSubagent()
	require.NoError(t, r.Register(d))
	err := r.Register(d)
	assert.ErrorIs(t, err, ErrBuiltinSubagentDuplicateSlug)
}

func TestBuiltinSubagent_ListAllSortedBySlug(t *testing.T) {
	r := NewBuiltinSubagentRegistry()
	d1 := validBuiltinSubagent()
	d1.Slug = "zeta-agent"
	d2 := validBuiltinSubagent()
	d2.Slug = "alpha-agent"
	require.NoError(t, r.Register(d1))
	require.NoError(t, r.Register(d2))
	list := r.ListAll()
	require.Equal(t, 2, len(list))
	assert.Equal(t, "alpha-agent", list[0].Slug)
	assert.Equal(t, "zeta-agent", list[1].Slug)
}

func TestBuiltinSubagent_ListByRoleFilters(t *testing.T) {
	r := NewBuiltinSubagentRegistry()
	d1 := validBuiltinSubagent()
	d1.Slug = "researcher-1"
	d1.Role = BuiltinSubagentResearcher
	d2 := validBuiltinSubagent()
	d2.Slug = "coder-1"
	d2.Role = BuiltinSubagentCoder
	require.NoError(t, r.Register(d1))
	require.NoError(t, r.Register(d2))
	researchers := r.ListByRole(BuiltinSubagentResearcher)
	assert.Equal(t, 1, len(researchers))
	assert.Equal(t, "researcher-1", researchers[0].Slug)
	coders := r.ListByRole(BuiltinSubagentCoder)
	assert.Equal(t, 1, len(coders))
}

func TestBuiltinSubagent_ListByRoleEmptyWhenNoMatch(t *testing.T) {
	r := NewBuiltinSubagentRegistry()
	d := validBuiltinSubagent()
	require.NoError(t, r.Register(d))
	got := r.ListByRole(BuiltinSubagentReviewer)
	assert.Empty(t, got)
}

func TestBuiltinSubagent_RegistrySize(t *testing.T) {
	r := NewBuiltinSubagentRegistry()
	assert.Equal(t, 0, r.Size())
	d := validBuiltinSubagent()
	_ = r.Register(d)
	assert.Equal(t, 1, r.Size())
}

func TestBuiltinSubagent_RegistryConcurrentRegisterSafe(t *testing.T) {
	r := NewBuiltinSubagentRegistry()
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			d := validBuiltinSubagent()
			d.Slug = "concurrent-" + string(rune('a'+idx%26)) + string(rune('0'+idx/26))
			_ = r.Register(d)
		}(i)
	}
	wg.Wait()
	assert.Equal(t, 50, r.Size())
}

func TestBuiltinSubagent_SlugMustBeKebabCase(t *testing.T) {
	cases := []string{
		"snake_case", "PascalCase", "with space", "-leading", "trailing-",
		"", "a",
	}
	for _, slug := range cases {
		d := validBuiltinSubagent()
		d.Slug = slug
		err := d.Validate()
		assert.Error(t, err, "slug %q must be rejected", slug)
	}
}

func TestBuiltinSubagent_AllSevenRolesValid(t *testing.T) {
	expected := map[BuiltinSubagentRole]bool{
		BuiltinSubagentResearcher: true, BuiltinSubagentCoder: true,
		BuiltinSubagentReviewer: true, BuiltinSubagentExplorer: true,
		BuiltinSubagentPlanner: true, BuiltinSubagentCurator: true,
		BuiltinSubagentDocumenter: true,
	}
	assert.Equal(t, 7, len(expected))
	for _, r := range allBuiltinSubagentRoles {
		assert.True(t, expected[r])
	}
}
