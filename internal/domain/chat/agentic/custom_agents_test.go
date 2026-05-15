package agentic

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func validCustomAgent() CustomAgentDefinition {
	return CustomAgentDefinition{
		Slug:                   "my-researcher",
		OwnerTenantSlug:        "acme-corp",
		Name:                   "ACME Researcher",
		Description:            "Investigates ACME-specific topics",
		SystemPromptTemplate:   "You are an ACME-specialized researcher...",
		ToolsetPolicySlug:      "documentation-readonly-allowlist",
		InheritanceModeSlug:    "extend-parent-rights",
		SummaryShapeSlug:       "success-with-artifacts",
		DerivedFromBuiltinSlug: "researcher-baseline",
		Visibility:             CustomAgentVisibilityPrivate,
		Version:                "1.0.0",
		IsActive:               true,
	}
}

func TestCustomAgent_IsValidVisibility(t *testing.T) {
	for _, v := range allCustomAgentVisibilities {
		assert.True(t, IsValidCustomAgentVisibility(v))
	}
	assert.False(t, IsValidCustomAgentVisibility(CustomAgentVisibility("nope")))
}

func TestCustomAgent_AllVisibilitiesReturnsCopy(t *testing.T) {
	v := AllCustomAgentVisibilities()
	require.Equal(t, 3, len(v))
	v[0] = "tampered"
	v2 := AllCustomAgentVisibilities()
	assert.Equal(t, CustomAgentVisibilityPrivate, v2[0])
}

func TestCustomAgent_ValidateBadSlug(t *testing.T) {
	d := validCustomAgent()
	d.Slug = "BAD_SLUG"
	assert.ErrorIs(t, d.Validate(), ErrCustomAgentBadSlug)
}

func TestCustomAgent_ValidateBadOwner(t *testing.T) {
	d := validCustomAgent()
	d.OwnerTenantSlug = "Bad Owner"
	assert.ErrorIs(t, d.Validate(), ErrCustomAgentBadOwner)
}

func TestCustomAgent_ValidateEmptyName(t *testing.T) {
	d := validCustomAgent()
	d.Name = ""
	assert.ErrorIs(t, d.Validate(), ErrCustomAgentEmptyName)
}

func TestCustomAgent_ValidateEmptyDescription(t *testing.T) {
	d := validCustomAgent()
	d.Description = ""
	assert.ErrorIs(t, d.Validate(), ErrCustomAgentEmptyDescription)
}

func TestCustomAgent_ValidateEmptySystemPrompt(t *testing.T) {
	d := validCustomAgent()
	d.SystemPromptTemplate = ""
	assert.ErrorIs(t, d.Validate(), ErrCustomAgentEmptySystemPrompt)
}

func TestCustomAgent_ValidateEmptyToolsetSlug(t *testing.T) {
	d := validCustomAgent()
	d.ToolsetPolicySlug = ""
	assert.ErrorIs(t, d.Validate(), ErrCustomAgentEmptyToolsetSlug)
}

func TestCustomAgent_ValidateEmptyInheritanceSlug(t *testing.T) {
	d := validCustomAgent()
	d.InheritanceModeSlug = ""
	assert.ErrorIs(t, d.Validate(), ErrCustomAgentEmptyInheritanceSlug)
}

func TestCustomAgent_ValidateEmptySummarySlug(t *testing.T) {
	d := validCustomAgent()
	d.SummaryShapeSlug = ""
	assert.ErrorIs(t, d.Validate(), ErrCustomAgentEmptySummarySlug)
}

func TestCustomAgent_ValidateBadVisibility(t *testing.T) {
	d := validCustomAgent()
	d.Visibility = "limbo"
	assert.ErrorIs(t, d.Validate(), ErrCustomAgentBadVisibility)
}

func TestCustomAgent_ValidateBadDerivedSlug(t *testing.T) {
	d := validCustomAgent()
	d.DerivedFromBuiltinSlug = "Not_Kebab"
	assert.ErrorIs(t, d.Validate(), ErrCustomAgentBadDerivedSlug)
}

func TestCustomAgent_ValidateDerivedSlugEmptyIsOK(t *testing.T) {
	d := validCustomAgent()
	d.DerivedFromBuiltinSlug = ""
	assert.NoError(t, d.Validate())
}

func TestCustomAgent_ValidateBadVersion(t *testing.T) {
	d := validCustomAgent()
	d.Version = "1.0"
	assert.ErrorIs(t, d.Validate(), ErrCustomAgentBadVersion)
}

func TestCustomAgent_ValidateVersionEmptyIsOK(t *testing.T) {
	d := validCustomAgent()
	d.Version = ""
	assert.NoError(t, d.Validate())
}

func TestCustomAgent_RegistryRegisterAndLookup(t *testing.T) {
	r := NewCustomAgentRegistry()
	d := validCustomAgent()
	require.NoError(t, r.Register(d))
	got, ok := r.Lookup(d.OwnerTenantSlug, d.Slug)
	require.True(t, ok)
	assert.Equal(t, d.Name, got.Name)
}

func TestCustomAgent_RegistryLookupUnknown(t *testing.T) {
	r := NewCustomAgentRegistry()
	_, ok := r.Lookup("acme-corp", "nope")
	assert.False(t, ok)
}

func TestCustomAgent_RegistryRejectsBadDescriptor(t *testing.T) {
	r := NewCustomAgentRegistry()
	bad := validCustomAgent()
	bad.Slug = "BAD"
	assert.ErrorIs(t, r.Register(bad), ErrCustomAgentBadSlug)
}

func TestCustomAgent_RegistryRejectsDuplicate(t *testing.T) {
	r := NewCustomAgentRegistry()
	d := validCustomAgent()
	require.NoError(t, r.Register(d))
	err := r.Register(d)
	assert.ErrorIs(t, err, ErrCustomAgentDuplicate)
}

func TestCustomAgent_RegistrySameSlugDifferentOwners(t *testing.T) {
	r := NewCustomAgentRegistry()
	d1 := validCustomAgent()
	d1.OwnerTenantSlug = "acme-corp"
	d2 := validCustomAgent()
	d2.OwnerTenantSlug = "beta-co"
	require.NoError(t, r.Register(d1))
	require.NoError(t, r.Register(d2))
	assert.Equal(t, 2, r.Size())
}

func TestCustomAgent_ListByOwnerSortedBySlug(t *testing.T) {
	r := NewCustomAgentRegistry()
	d1 := validCustomAgent()
	d1.Slug = "zeta-agent"
	d2 := validCustomAgent()
	d2.Slug = "alpha-agent"
	require.NoError(t, r.Register(d1))
	require.NoError(t, r.Register(d2))
	list := r.ListByOwner("acme-corp")
	require.Equal(t, 2, len(list))
	assert.Equal(t, "alpha-agent", list[0].Slug)
}

func TestCustomAgent_ListByOwnerEmpty(t *testing.T) {
	r := NewCustomAgentRegistry()
	d := validCustomAgent()
	_ = r.Register(d)
	got := r.ListByOwner("nobody")
	assert.Empty(t, got)
}

func TestCustomAgent_ListByDerivedBuiltin(t *testing.T) {
	r := NewCustomAgentRegistry()
	d1 := validCustomAgent()
	d1.Slug = "fork-of-researcher"
	d1.DerivedFromBuiltinSlug = "researcher-baseline"
	d2 := validCustomAgent()
	d2.Slug = "fork-of-coder"
	d2.DerivedFromBuiltinSlug = "coder-baseline"
	require.NoError(t, r.Register(d1))
	require.NoError(t, r.Register(d2))
	matches := r.ListByDerivedBuiltin("researcher-baseline")
	require.Equal(t, 1, len(matches))
	assert.Equal(t, "fork-of-researcher", matches[0].Slug)
}

func TestCustomAgent_ListByVisibilityMarketplace(t *testing.T) {
	r := NewCustomAgentRegistry()
	d1 := validCustomAgent()
	d1.Slug = "private-1"
	d1.Visibility = CustomAgentVisibilityPrivate
	d2 := validCustomAgent()
	d2.Slug = "marketplace-1"
	d2.Visibility = CustomAgentVisibilityMarketplace
	require.NoError(t, r.Register(d1))
	require.NoError(t, r.Register(d2))
	mp := r.ListByVisibility(CustomAgentVisibilityMarketplace)
	require.Equal(t, 1, len(mp))
	assert.Equal(t, "marketplace-1", mp[0].Slug)
}

func TestCustomAgent_RegistryConcurrentRegisterSafe(t *testing.T) {
	r := NewCustomAgentRegistry()
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			d := validCustomAgent()
			d.Slug = "concurrent-" + string(rune('a'+idx%26)) + string(rune('0'+idx/26))
			_ = r.Register(d)
		}(i)
	}
	wg.Wait()
	assert.Equal(t, 50, r.Size())
}

func TestCustomAgent_SemverFormatStrict(t *testing.T) {
	cases := []string{"1", "1.0", "v1.0.0", "1.0.0-beta", ""}
	for _, v := range cases {
		d := validCustomAgent()
		d.Version = v
		if v == "" {
			assert.NoError(t, d.Validate(), "empty version OK")
		} else {
			assert.ErrorIs(t, d.Validate(), ErrCustomAgentBadVersion,
				"version %q must be rejected", v)
		}
	}
}
