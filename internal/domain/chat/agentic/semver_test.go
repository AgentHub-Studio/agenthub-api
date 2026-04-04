package agentic_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/chat/agentic"
)

// --- ParseSemVer ---

func TestParseSemVer_Full(t *testing.T) {
	v := agentic.ParseSemVer("1.2.3")
	assert.NotNil(t, v)
	assert.Equal(t, 1, v.Major)
	assert.Equal(t, 2, v.Minor)
	assert.Equal(t, 3, v.Patch)
	assert.Empty(t, v.PreRelease)
}

func TestParseSemVer_WithV(t *testing.T) {
	v := agentic.ParseSemVer("v2.0.1")
	assert.NotNil(t, v)
	assert.Equal(t, 2, v.Major)
}

func TestParseSemVer_PreRelease(t *testing.T) {
	v := agentic.ParseSemVer("1.0.0-alpha.1")
	assert.NotNil(t, v)
	assert.Equal(t, "alpha.1", v.PreRelease)
}

func TestParseSemVer_BuildMetadata(t *testing.T) {
	v := agentic.ParseSemVer("1.0.0+build.123")
	assert.NotNil(t, v)
	assert.Empty(t, v.PreRelease)
}

func TestParseSemVer_PreReleaseWithBuild(t *testing.T) {
	v := agentic.ParseSemVer("1.0.0-rc.1+build")
	assert.NotNil(t, v)
	assert.Equal(t, "rc.1", v.PreRelease)
}

func TestParseSemVer_MissingPatch(t *testing.T) {
	v := agentic.ParseSemVer("1.2")
	assert.NotNil(t, v)
	assert.Equal(t, 0, v.Patch)
}

func TestParseSemVer_MajorOnly(t *testing.T) {
	v := agentic.ParseSemVer("3")
	assert.NotNil(t, v)
	assert.Equal(t, 3, v.Major)
}

func TestParseSemVer_Empty(t *testing.T) {
	assert.Nil(t, agentic.ParseSemVer(""))
}

func TestParseSemVer_Invalid(t *testing.T) {
	assert.Nil(t, agentic.ParseSemVer("abc"))
}

func TestParseSemVer_Negative(t *testing.T) {
	assert.Nil(t, agentic.ParseSemVer("-1.0.0"))
}

// --- CompareSemVer ---

func TestCompareSemVer_Equal(t *testing.T) {
	assert.Equal(t, 0, agentic.CompareSemVer("1.2.3", "1.2.3"))
}

func TestCompareSemVer_MajorGreater(t *testing.T) {
	assert.Equal(t, 1, agentic.CompareSemVer("2.0.0", "1.9.9"))
}

func TestCompareSemVer_MinorGreater(t *testing.T) {
	assert.Equal(t, 1, agentic.CompareSemVer("1.3.0", "1.2.9"))
}

func TestCompareSemVer_PatchGreater(t *testing.T) {
	assert.Equal(t, 1, agentic.CompareSemVer("1.2.4", "1.2.3"))
}

func TestCompareSemVer_PreReleaseIsLower(t *testing.T) {
	// 1.0.0-alpha < 1.0.0
	assert.Equal(t, -1, agentic.CompareSemVer("1.0.0-alpha", "1.0.0"))
}

func TestCompareSemVer_PreReleaseOrdering(t *testing.T) {
	// alpha < beta
	assert.Equal(t, -1, agentic.CompareSemVer("1.0.0-alpha", "1.0.0-beta"))
}

func TestCompareSemVer_NumericPreRelease(t *testing.T) {
	// 1.0.0-1 < 1.0.0-2
	assert.Equal(t, -1, agentic.CompareSemVer("1.0.0-1", "1.0.0-2"))
}

func TestCompareSemVer_NumericVsString(t *testing.T) {
	// Numeric < string per semver spec
	assert.Equal(t, -1, agentic.CompareSemVer("1.0.0-1", "1.0.0-alpha"))
}

func TestCompareSemVer_FewerFields(t *testing.T) {
	// alpha < alpha.1 (fewer fields = lower)
	assert.Equal(t, -1, agentic.CompareSemVer("1.0.0-alpha", "1.0.0-alpha.1"))
}

func TestCompareSemVer_Invalid(t *testing.T) {
	assert.Equal(t, 0, agentic.CompareSemVer("invalid", "1.0.0"))
}

// --- Convenience functions ---

func TestSemVerGT(t *testing.T) {
	assert.True(t, agentic.SemVerGT("2.0.0", "1.0.0"))
	assert.False(t, agentic.SemVerGT("1.0.0", "2.0.0"))
	assert.False(t, agentic.SemVerGT("1.0.0", "1.0.0"))
}

func TestSemVerGTE(t *testing.T) {
	assert.True(t, agentic.SemVerGTE("2.0.0", "1.0.0"))
	assert.True(t, agentic.SemVerGTE("1.0.0", "1.0.0"))
	assert.False(t, agentic.SemVerGTE("1.0.0", "2.0.0"))
}

func TestSemVerLT(t *testing.T) {
	assert.True(t, agentic.SemVerLT("1.0.0", "2.0.0"))
	assert.False(t, agentic.SemVerLT("2.0.0", "1.0.0"))
}

func TestSemVerLTE(t *testing.T) {
	assert.True(t, agentic.SemVerLTE("1.0.0", "1.0.0"))
	assert.True(t, agentic.SemVerLTE("1.0.0", "2.0.0"))
}

func TestSemVerEQ(t *testing.T) {
	assert.True(t, agentic.SemVerEQ("1.2.3", "1.2.3"))
	assert.False(t, agentic.SemVerEQ("1.2.3", "1.2.4"))
}

// --- String ---

func TestSemVer_String(t *testing.T) {
	v := agentic.ParseSemVer("1.2.3-beta.1")
	assert.Equal(t, "1.2.3-beta.1", v.String())
}

func TestSemVer_StringNoPreRelease(t *testing.T) {
	v := agentic.ParseSemVer("1.0.0")
	assert.Equal(t, "1.0.0", v.String())
}

// --- V prefix handling ---

func TestCompareSemVer_VPrefix(t *testing.T) {
	assert.Equal(t, 0, agentic.CompareSemVer("v1.2.3", "1.2.3"))
}
