package agentic

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMemoryInclude_PolicyEnumIsBounded(t *testing.T) {
	for _, p := range AllMemoryIncludeMissingPolicies() {
		assert.True(t, IsValidMemoryIncludeMissingPolicy(p))
	}
	assert.False(t, IsValidMemoryIncludeMissingPolicy(MemoryIncludeMissingPolicy("ignore")))
	assert.Equal(t, 3, len(AllMemoryIncludeMissingPolicies()))
}

func TestNewMemoryIncludeResolver_RejectsInvalidPolicy(t *testing.T) {
	_, err := NewMemoryIncludeResolver(MemoryIncludeResolverConfig{
		MissingPolicy: "bogus",
	}, nil)
	assert.True(t, errors.Is(err, ErrMemoryIncludeInvalidPolicy))
}

func TestNewMemoryIncludeResolver_RejectsNegativeMaxDepth(t *testing.T) {
	_, err := NewMemoryIncludeResolver(MemoryIncludeResolverConfig{
		MaxDepth:      -1,
		MissingPolicy: MemoryIncludeLeaveAsIs,
	}, nil)
	assert.Error(t, err)
}

func TestResolve_NoIncludesReturnsTextVerbatim(t *testing.T) {
	r, _ := NewMemoryIncludeResolver(DefaultMemoryIncludeResolverConfig(), nil)
	got, trace, err := r.Resolve(context.Background(), "plain text no includes")
	require.NoError(t, err)
	assert.Equal(t, "plain text no includes", got)
	assert.Empty(t, trace.ExpandedKeys)
}

func TestResolve_SingleIncludeExpandsCorrectly(t *testing.T) {
	r, _ := NewMemoryIncludeResolver(DefaultMemoryIncludeResolverConfig(), map[string]string{
		"hello": "world",
	})
	got, trace, err := r.Resolve(context.Background(), "say @include{hello}!")
	require.NoError(t, err)
	assert.Equal(t, "say world!", got)
	assert.Contains(t, trace.ExpandedKeys, "hello")
}

func TestResolve_NestedIncludesExpandRecursively(t *testing.T) {
	r, _ := NewMemoryIncludeResolver(DefaultMemoryIncludeResolverConfig(), map[string]string{
		"outer": "outer-text including @include{inner}",
		"inner": "inner-value",
	})
	got, trace, err := r.Resolve(context.Background(), "@include{outer}")
	require.NoError(t, err)
	assert.Contains(t, got, "outer-text including inner-value")
	assert.Equal(t, 2, len(trace.ExpandedKeys))
}

func TestResolve_DetectsCycles(t *testing.T) {
	r, _ := NewMemoryIncludeResolver(DefaultMemoryIncludeResolverConfig(), map[string]string{
		"a": "calls @include{b}",
		"b": "calls @include{a}",
	})
	_, trace, err := r.Resolve(context.Background(), "@include{a}")
	assert.True(t, errors.Is(err, ErrMemoryIncludeCycle))
	assert.NotEmpty(t, trace.CycleDetected)
}

func TestResolve_DetectsSelfCycle(t *testing.T) {
	r, _ := NewMemoryIncludeResolver(DefaultMemoryIncludeResolverConfig(), map[string]string{
		"loop": "self-ref @include{loop}",
	})
	_, _, err := r.Resolve(context.Background(), "@include{loop}")
	assert.True(t, errors.Is(err, ErrMemoryIncludeCycle))
}

func TestResolve_MaxDepthGuardAborts(t *testing.T) {
	r, _ := NewMemoryIncludeResolver(MemoryIncludeResolverConfig{
		MaxDepth:      2,
		MissingPolicy: MemoryIncludeLeaveAsIs,
	}, map[string]string{
		"a": "@include{b}",
		"b": "@include{c}",
		"c": "@include{d}",
		"d": "@include{e}",
		"e": "deep",
	})
	_, _, err := r.Resolve(context.Background(), "@include{a}")
	assert.True(t, errors.Is(err, ErrMemoryIncludeMaxDepth))
}

func TestResolve_MissingPolicyLeaveAsIs(t *testing.T) {
	r, _ := NewMemoryIncludeResolver(MemoryIncludeResolverConfig{
		MaxDepth:      8,
		MissingPolicy: MemoryIncludeLeaveAsIs,
	}, map[string]string{})
	got, trace, err := r.Resolve(context.Background(), "@include{unknown}")
	require.NoError(t, err)
	assert.Equal(t, "@include{unknown}", got)
	assert.Contains(t, trace.MissingKeys, "unknown")
}

func TestResolve_MissingPolicyReplaceWithEmpty(t *testing.T) {
	r, _ := NewMemoryIncludeResolver(MemoryIncludeResolverConfig{
		MaxDepth:      8,
		MissingPolicy: MemoryIncludeReplaceWithEmpty,
	}, map[string]string{})
	got, trace, err := r.Resolve(context.Background(), "before @include{unknown} after")
	require.NoError(t, err)
	assert.Equal(t, "before  after", got)
	assert.Contains(t, trace.MissingKeys, "unknown")
}

func TestResolve_MissingPolicyError(t *testing.T) {
	r, _ := NewMemoryIncludeResolver(MemoryIncludeResolverConfig{
		MaxDepth:      8,
		MissingPolicy: MemoryIncludeErrorOnMissing,
	}, map[string]string{})
	_, _, err := r.Resolve(context.Background(), "@include{unknown}")
	assert.True(t, errors.Is(err, ErrMemoryIncludeMissing))
}

func TestResolve_MultipleIncludesInOneText(t *testing.T) {
	r, _ := NewMemoryIncludeResolver(DefaultMemoryIncludeResolverConfig(), map[string]string{
		"name":  "Alice",
		"role":  "engineer",
		"email": "alice@example.com",
	})
	got, _, err := r.Resolve(context.Background(),
		"User @include{name} (@include{role}) — @include{email}")
	require.NoError(t, err)
	assert.Equal(t, "User Alice (engineer) — alice@example.com", got)
}

func TestResolve_TraceRecordsMaxDepthReached(t *testing.T) {
	r, _ := NewMemoryIncludeResolver(DefaultMemoryIncludeResolverConfig(), map[string]string{
		"a": "@include{b}",
		"b": "@include{c}",
		"c": "deep",
	})
	_, trace, _ := r.Resolve(context.Background(), "@include{a}")
	assert.GreaterOrEqual(t, trace.MaxDepthReached, 3)
}

func TestSetLookup_ReplacesInternalTable(t *testing.T) {
	r, _ := NewMemoryIncludeResolver(DefaultMemoryIncludeResolverConfig(), map[string]string{
		"x": "old",
	})
	r.SetLookup(map[string]string{"x": "new"})
	got, _, _ := r.Resolve(context.Background(), "@include{x}")
	assert.Equal(t, "new", got)
}

func TestSetLookup_DefensiveCopy(t *testing.T) {
	external := map[string]string{"x": "value"}
	r, _ := NewMemoryIncludeResolver(DefaultMemoryIncludeResolverConfig(), external)
	external["x"] = "mutated" // external mutation must not affect resolver
	got, _, _ := r.Resolve(context.Background(), "@include{x}")
	assert.Equal(t, "value", got)
}

func TestResolve_EmptyTextReturnsEmpty(t *testing.T) {
	r, _ := NewMemoryIncludeResolver(DefaultMemoryIncludeResolverConfig(), nil)
	got, _, err := r.Resolve(context.Background(), "")
	require.NoError(t, err)
	assert.Equal(t, "", got)
}

func TestCountIncludes(t *testing.T) {
	assert.Equal(t, 0, CountIncludes("plain"))
	assert.Equal(t, 1, CountIncludes("a @include{x} b"))
	assert.Equal(t, 3, CountIncludes("@include{a} @include{b} @include{c}"))
}

func TestExtractIncludeKeys_DeduplicatesAndSorts(t *testing.T) {
	got := ExtractIncludeKeys("@include{zeta} @include{alpha} @include{zeta}")
	assert.Equal(t, []string{"alpha", "zeta"}, got)
}

func TestExtractIncludeKeys_NoMatchesReturnsEmpty(t *testing.T) {
	assert.Empty(t, ExtractIncludeKeys("plain text"))
}

func TestResolve_ConcurrentResolveIsSafe(t *testing.T) {
	r, _ := NewMemoryIncludeResolver(DefaultMemoryIncludeResolverConfig(), map[string]string{
		"x": "value",
	})
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _, _ = r.Resolve(context.Background(), "@include{x}")
		}()
	}
	wg.Wait()
}

func TestResolve_ContextCancelled(t *testing.T) {
	r, _ := NewMemoryIncludeResolver(DefaultMemoryIncludeResolverConfig(), nil)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, _, err := r.Resolve(ctx, "anything")
	assert.Error(t, err)
}

func TestResolve_AllowedKeyCharsAreAlphaNumericDotDashUnderscore(t *testing.T) {
	r, _ := NewMemoryIncludeResolver(DefaultMemoryIncludeResolverConfig(), map[string]string{
		"a.b_c-d": "value",
	})
	got, _, err := r.Resolve(context.Background(), "@include{a.b_c-d}")
	require.NoError(t, err)
	assert.Equal(t, "value", got)
}

func TestResolve_MalformedIncludePatternIgnored(t *testing.T) {
	r, _ := NewMemoryIncludeResolver(DefaultMemoryIncludeResolverConfig(), map[string]string{
		"x": "value",
	})
	got, _, err := r.Resolve(context.Background(), "@include{} @include{x with spaces}")
	require.NoError(t, err)
	// Both malformed patterns are left literal; no expansion.
	assert.Contains(t, got, "@include{}")
}

func TestResolve_LargeTextWithManyIncludes(t *testing.T) {
	lookup := map[string]string{}
	for i := 0; i < 100; i++ {
		lookup[fmt.Sprintf("key%d", i)] = fmt.Sprintf("value%d", i)
	}
	var b strings.Builder
	for i := 0; i < 100; i++ {
		fmt.Fprintf(&b, "@include{key%d} ", i)
	}
	r, _ := NewMemoryIncludeResolver(DefaultMemoryIncludeResolverConfig(), lookup)
	got, _, err := r.Resolve(context.Background(), b.String())
	require.NoError(t, err)
	assert.Contains(t, got, "value0")
	assert.Contains(t, got, "value99")
}

func TestResolve_IndirectCycleThroughThreeKeysDetected(t *testing.T) {
	r, _ := NewMemoryIncludeResolver(DefaultMemoryIncludeResolverConfig(), map[string]string{
		"a": "@include{b}",
		"b": "@include{c}",
		"c": "@include{a}",
	})
	_, _, err := r.Resolve(context.Background(), "@include{a}")
	assert.True(t, errors.Is(err, ErrMemoryIncludeCycle))
}
