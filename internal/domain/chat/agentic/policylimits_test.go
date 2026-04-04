package agentic_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/chat/agentic"
)

// --- Constants ---

func TestDefaultPolicyTTL(t *testing.T) {
	assert.Equal(t, time.Hour, agentic.DefaultPolicyTTL)
}

// --- NewPolicyLimitsCache ---

func TestNewPolicyLimitsCache(t *testing.T) {
	c := agentic.NewPolicyLimitsCache(0)
	assert.False(t, c.HasLimits())
	assert.Equal(t, 0, c.RestrictionCount())
}

// --- Update ---

func TestPolicyLimitsCache_Update(t *testing.T) {
	c := agentic.NewPolicyLimitsCache(time.Hour)
	c.Update(agentic.PolicyLimits{
		Restrictions: map[string]agentic.PolicyRestriction{
			"bash_tool": {Allowed: false},
		},
		ETag: "etag-1",
	})

	assert.True(t, c.HasLimits())
	assert.Equal(t, 1, c.RestrictionCount())
	assert.Equal(t, "etag-1", c.ETag())
}

// --- IsAllowed ---

func TestPolicyLimitsCache_IsAllowed_NoLimits(t *testing.T) {
	c := agentic.NewPolicyLimitsCache(time.Hour)
	assert.True(t, c.IsAllowed("anything"))
}

func TestPolicyLimitsCache_IsAllowed_AbsenceIsAllowed(t *testing.T) {
	c := agentic.NewPolicyLimitsCache(time.Hour)
	c.Update(agentic.PolicyLimits{
		Restrictions: map[string]agentic.PolicyRestriction{
			"bash_tool": {Allowed: false},
		},
	})
	assert.True(t, c.IsAllowed("read_tool"), "absent policy should be allowed")
}

func TestPolicyLimitsCache_IsAllowed_Blocked(t *testing.T) {
	c := agentic.NewPolicyLimitsCache(time.Hour)
	c.Update(agentic.PolicyLimits{
		Restrictions: map[string]agentic.PolicyRestriction{
			"bash_tool": {Allowed: false},
		},
	})
	assert.False(t, c.IsAllowed("bash_tool"))
}

func TestPolicyLimitsCache_IsAllowed_ExplicitAllow(t *testing.T) {
	c := agentic.NewPolicyLimitsCache(time.Hour)
	c.Update(agentic.PolicyLimits{
		Restrictions: map[string]agentic.PolicyRestriction{
			"safe_tool": {Allowed: true},
		},
	})
	assert.True(t, c.IsAllowed("safe_tool"))
}

// --- GetBlockedPolicies ---

func TestPolicyLimitsCache_GetBlockedPolicies(t *testing.T) {
	c := agentic.NewPolicyLimitsCache(time.Hour)
	c.Update(agentic.PolicyLimits{
		Restrictions: map[string]agentic.PolicyRestriction{
			"bash":  {Allowed: false},
			"write": {Allowed: false},
			"read":  {Allowed: true},
		},
	})

	blocked := c.GetBlockedPolicies()
	assert.Len(t, blocked, 2)
	assert.Contains(t, blocked, "bash")
	assert.Contains(t, blocked, "write")
}

func TestPolicyLimitsCache_GetBlockedPolicies_NoLimits(t *testing.T) {
	c := agentic.NewPolicyLimitsCache(time.Hour)
	assert.Nil(t, c.GetBlockedPolicies())
}

func TestPolicyLimitsCache_GetBlockedPolicies_NoneBlocked(t *testing.T) {
	c := agentic.NewPolicyLimitsCache(time.Hour)
	c.Update(agentic.PolicyLimits{
		Restrictions: map[string]agentic.PolicyRestriction{
			"read": {Allowed: true},
		},
	})
	assert.Empty(t, c.GetBlockedPolicies())
}

// --- IsStale ---

func TestPolicyLimitsCache_IsStale_NoData(t *testing.T) {
	c := agentic.NewPolicyLimitsCache(time.Hour)
	assert.True(t, c.IsStale())
}

func TestPolicyLimitsCache_IsStale_Fresh(t *testing.T) {
	c := agentic.NewPolicyLimitsCache(time.Hour)
	c.Update(agentic.PolicyLimits{Restrictions: map[string]agentic.PolicyRestriction{}})
	assert.False(t, c.IsStale())
}

func TestPolicyLimitsCache_IsStale_Expired(t *testing.T) {
	c := agentic.NewPolicyLimitsCache(50 * time.Millisecond)
	c.Update(agentic.PolicyLimits{Restrictions: map[string]agentic.PolicyRestriction{}})

	time.Sleep(100 * time.Millisecond)
	assert.True(t, c.IsStale())
}

// --- ETag ---

func TestPolicyLimitsCache_ETag_Empty(t *testing.T) {
	c := agentic.NewPolicyLimitsCache(time.Hour)
	assert.Empty(t, c.ETag())
}

// --- Clear ---

func TestPolicyLimitsCache_Clear(t *testing.T) {
	c := agentic.NewPolicyLimitsCache(time.Hour)
	c.Update(agentic.PolicyLimits{
		Restrictions: map[string]agentic.PolicyRestriction{"x": {Allowed: false}},
		ETag:         "tag",
	})

	c.Clear()
	assert.False(t, c.HasLimits())
	assert.Equal(t, 0, c.RestrictionCount())
	assert.Empty(t, c.ETag())
}

// --- PolicyFetchResult ---

func TestPolicyFetchResult_Fields(t *testing.T) {
	result := agentic.PolicyFetchResult{
		NotModified: true,
		SkipRetry:   false,
	}
	assert.True(t, result.NotModified)
	assert.False(t, result.SkipRetry)
}

// --- PolicyLimits ---

func TestPolicyLimits_Fields(t *testing.T) {
	limits := agentic.PolicyLimits{
		Restrictions: map[string]agentic.PolicyRestriction{
			"tool_a": {Allowed: true},
			"tool_b": {Allowed: false},
		},
		ETag:      "etag-abc",
		FetchedAt: time.Now(),
	}
	require.Len(t, limits.Restrictions, 2)
	assert.True(t, limits.Restrictions["tool_a"].Allowed)
	assert.False(t, limits.Restrictions["tool_b"].Allowed)
}
