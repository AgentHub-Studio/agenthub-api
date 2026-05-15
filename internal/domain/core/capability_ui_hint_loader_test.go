package core

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// Unit tests for capability UI hint seed constants (migration 000107).
// These run without a database and guard against accidental constant drift.

func TestSeedCapabilityUIHintCount_IsSix(t *testing.T) {
	assert.Equal(t, 6, SeedCapabilityUIHintCount,
		"migration 000107 seeds exactly 6 capability UI hint rows (two per capability agent)")
}

func TestSeedCapabilityUIHintSlugs_HasLengthSix(t *testing.T) {
	assert.Len(t, SeedCapabilityUIHintSlugs, 6,
		"SeedCapabilityUIHintSlugs must have exactly 6 entries — two per agent")
}

func TestSeedCapabilityUIHintSlugs_AllStartWithHint(t *testing.T) {
	for _, slug := range SeedCapabilityUIHintSlugs {
		assert.True(t, strings.HasPrefix(slug, "hint-"),
			"UI hint slug %q must start with 'hint-' (namespace contract)", slug)
	}
}

func TestSeedCapabilityUIHintSlugs_AllAreDistinct(t *testing.T) {
	unique := map[string]struct{}{}
	for _, slug := range SeedCapabilityUIHintSlugs {
		unique[slug] = struct{}{}
	}
	assert.Len(t, unique, 6,
		"all entries in SeedCapabilityUIHintSlugs must be distinct (no duplicates)")
}

func TestSeedResearcherUIHintCount_IsTwo(t *testing.T) {
	assert.Equal(t, 2, SeedResearcherUIHintCount,
		"SeedResearcherUIHintCount must be 2 — one start tip + one citation info")
}

func TestSeedAnalystUIHintCount_IsTwo(t *testing.T) {
	assert.Equal(t, 2, SeedAnalystUIHintCount,
		"SeedAnalystUIHintCount must be 2 — one doc-upload tip + one confidence tip")
}

func TestSeedPlannerUIHintCount_IsTwo(t *testing.T) {
	assert.Equal(t, 2, SeedPlannerUIHintCount,
		"SeedPlannerUIHintCount must be 2 — one task info + one breakdown tip")
}

func TestSeedPerAgentUIHintCounts_SumToTotal(t *testing.T) {
	sum := SeedResearcherUIHintCount + SeedAnalystUIHintCount + SeedPlannerUIHintCount
	assert.Equal(t, SeedCapabilityUIHintCount, sum,
		"researcher (%d) + analyst (%d) + planner (%d) must equal total count (%d)",
		SeedResearcherUIHintCount, SeedAnalystUIHintCount, SeedPlannerUIHintCount,
		SeedCapabilityUIHintCount)
}

func TestSeedUIHintTipCount_IsFour(t *testing.T) {
	assert.Equal(t, 4, SeedUIHintTipCount,
		"SeedUIHintTipCount must be 4 — four hints have hint_type='tip'")
}

func TestSeedUIHintInfoCount_IsTwo(t *testing.T) {
	assert.Equal(t, 2, SeedUIHintInfoCount,
		"SeedUIHintInfoCount must be 2 — two hints have hint_type='info'")
}

func TestSeedUIHintTypeCounts_SumToTotal(t *testing.T) {
	sum := SeedUIHintTipCount + SeedUIHintInfoCount
	assert.Equal(t, SeedCapabilityUIHintCount, sum,
		"tip (%d) + info (%d) must equal total count (%d)",
		SeedUIHintTipCount, SeedUIHintInfoCount, SeedCapabilityUIHintCount)
}

func TestSeedUIHintTriggerChatStart_Value(t *testing.T) {
	assert.Equal(t, "agent_chat_start", SeedUIHintTriggerChatStart,
		"SeedUIHintTriggerChatStart must equal \"agent_chat_start\"")
}

func TestSeedUIHintTriggerPostToolResult_Value(t *testing.T) {
	assert.Equal(t, "post_tool_result", SeedUIHintTriggerPostToolResult,
		"SeedUIHintTriggerPostToolResult must equal \"post_tool_result\"")
}

func TestSeedUIHintTriggerContexts_AreDistinct(t *testing.T) {
	assert.NotEqual(t, SeedUIHintTriggerChatStart, SeedUIHintTriggerPostToolResult,
		"SeedUIHintTriggerChatStart and SeedUIHintTriggerPostToolResult must be distinct trigger contexts")
}

func TestSeedUIHintTypeTip_AndInfo_AreDistinct(t *testing.T) {
	assert.NotEqual(t, SeedUIHintTypeTip, SeedUIHintTypeInfo,
		"SeedUIHintTypeTip and SeedUIHintTypeInfo must be distinct type values")
}

func TestSeedCapabilityUIHintCount_EqualsSlugsLength(t *testing.T) {
	assert.Equal(t, SeedCapabilityUIHintCount, len(SeedCapabilityUIHintSlugs),
		"SeedCapabilityUIHintCount must equal len(SeedCapabilityUIHintSlugs)")
}
