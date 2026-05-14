package agentic

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestContentReference_KindEnumIsBounded(t *testing.T) {
	for _, k := range AllContentReferenceKinds() {
		assert.True(t, IsValidContentReferenceKind(k))
	}
	assert.False(t, IsValidContentReferenceKind(ContentReferenceKind("foo")))
	assert.Equal(t, 5, len(AllContentReferenceKinds()))
}

func TestFormatRefToken_CanonicalForm(t *testing.T) {
	got := FormatRefToken(ContentRefKindKBChunk, "abcdef0123456789")
	assert.Equal(t, "@ref{kb_chunk:abcdef0123456789}", got)
}

func TestParseRefToken_RoundTripsCanonicalForm(t *testing.T) {
	token := "@ref{kb_chunk:abcdef0123456789}"
	kind, hash, err := ParseRefToken(token)
	require.NoError(t, err)
	assert.Equal(t, ContentRefKindKBChunk, kind)
	assert.Equal(t, "abcdef0123456789", hash)
}

func TestParseRefToken_RejectsMalformedTokens(t *testing.T) {
	for _, bad := range []string{
		"not-a-ref",
		"@ref{kb_chunk}",
		"@ref{kb_chunk:short}",
		"@ref{kb_chunk:UPPERCASE16HEX01}",
		"",
	} {
		_, _, err := ParseRefToken(bad)
		assert.Error(t, err, "must reject %q", bad)
	}
}

func TestParseRefToken_RejectsUnknownKind(t *testing.T) {
	_, _, err := ParseRefToken("@ref{rogue_kind:abcdef0123456789}")
	assert.True(t, errors.Is(err, ErrContentReferenceInvalidKind))
}

func TestRegister_ReturnsCanonicalToken(t *testing.T) {
	r := NewInMemoryContentReferenceRegistry()
	ref, err := r.Register(context.Background(), "t-1", ContentRefKindKBChunk, "hello world", "kb_loader")
	require.NoError(t, err)
	assert.Contains(t, ref.Token, "@ref{kb_chunk:")
	assert.Equal(t, len("hello world"), ref.Bytes)
	assert.Equal(t, 1, ref.HitCount)
}

func TestRegister_RejectsEmptyTenant(t *testing.T) {
	r := NewInMemoryContentReferenceRegistry()
	_, err := r.Register(context.Background(), "", ContentRefKindKBChunk, "x", "src")
	assert.True(t, errors.Is(err, ErrContentReferenceTenantReq))
}

func TestRegister_RejectsInvalidKind(t *testing.T) {
	r := NewInMemoryContentReferenceRegistry()
	_, err := r.Register(context.Background(), "t", ContentReferenceKind("bogus"), "x", "src")
	assert.True(t, errors.Is(err, ErrContentReferenceInvalidKind))
}

func TestRegister_RejectsEmptyContent(t *testing.T) {
	r := NewInMemoryContentReferenceRegistry()
	_, err := r.Register(context.Background(), "t", ContentRefKindKBChunk, "", "src")
	assert.True(t, errors.Is(err, ErrContentReferenceContentReq))
}

func TestRegister_DeduplicatesIdenticalContent(t *testing.T) {
	r := NewInMemoryContentReferenceRegistry()
	first, _ := r.Register(context.Background(), "t-1", ContentRefKindKBChunk, "shared", "src1")
	second, err := r.Register(context.Background(), "t-1", ContentRefKindKBChunk, "shared", "src2")
	require.NoError(t, err)
	assert.Equal(t, first.Token, second.Token, "same content → same token (de-dup)")
	assert.Equal(t, 2, second.HitCount, "second register bumps hit count")
}

func TestRegister_DifferentContentDifferentTokens(t *testing.T) {
	r := NewInMemoryContentReferenceRegistry()
	a, _ := r.Register(context.Background(), "t-1", ContentRefKindKBChunk, "alpha", "src")
	b, _ := r.Register(context.Background(), "t-1", ContentRefKindKBChunk, "beta", "src")
	assert.NotEqual(t, a.Token, b.Token)
}

func TestRegister_DifferentKindsDifferentTokens(t *testing.T) {
	r := NewInMemoryContentReferenceRegistry()
	a, _ := r.Register(context.Background(), "t-1", ContentRefKindKBChunk, "shared", "src")
	b, _ := r.Register(context.Background(), "t-1", ContentRefKindToolResult, "shared", "src")
	assert.NotEqual(t, a.Token, b.Token, "kind is part of token namespace")
}

func TestRegister_TenantIsolationProducesSameTokenButSeparateRecords(t *testing.T) {
	r := NewInMemoryContentReferenceRegistry()
	a, _ := r.Register(context.Background(), "t-a", ContentRefKindKBChunk, "shared", "src")
	b, _ := r.Register(context.Background(), "t-b", ContentRefKindKBChunk, "shared", "src")
	// Same content → same token format (token doesn't carry tenant).
	assert.Equal(t, a.Token, b.Token)
	// But records are separate per tenant.
	listA, _ := r.List(context.Background(), "t-a")
	assert.Len(t, listA, 1)
	listB, _ := r.List(context.Background(), "t-b")
	assert.Len(t, listB, 1)
}

func TestResolve_RoundTripsContent(t *testing.T) {
	r := NewInMemoryContentReferenceRegistry()
	ref, _ := r.Register(context.Background(), "t-1", ContentRefKindKBChunk, "the original content", "src")
	got, err := r.Resolve(context.Background(), "t-1", ref.Token)
	require.NoError(t, err)
	assert.Equal(t, "the original content", got.Content)
}

func TestResolve_DetectsTampering(t *testing.T) {
	r := NewInMemoryContentReferenceRegistry()
	ref, _ := r.Register(context.Background(), "t-1", ContentRefKindKBChunk, "original", "src")

	// Adversary mutates the stored content (simulate compromise).
	r.mu.Lock()
	stored := r.refs[contentRefKey{"t-1", ref.Token}]
	stored.Content = "tampered content with same length"
	r.refs[contentRefKey{"t-1", ref.Token}] = stored
	r.mu.Unlock()

	_, err := r.Resolve(context.Background(), "t-1", ref.Token)
	assert.True(t, errors.Is(err, ErrContentReferenceTampered))
}

func TestResolve_UnknownReturnsNotFound(t *testing.T) {
	r := NewInMemoryContentReferenceRegistry()
	_, err := r.Resolve(context.Background(), "t-1", "@ref{kb_chunk:0000000000000000}")
	assert.True(t, errors.Is(err, ErrContentReferenceNotFound))
}

func TestResolve_RejectsMalformedToken(t *testing.T) {
	r := NewInMemoryContentReferenceRegistry()
	_, err := r.Resolve(context.Background(), "t-1", "not-a-token")
	assert.True(t, errors.Is(err, ErrContentReferenceMalformed))
}

func TestResolve_TenantIsolation(t *testing.T) {
	r := NewInMemoryContentReferenceRegistry()
	ref, _ := r.Register(context.Background(), "t-a", ContentRefKindKBChunk, "secret", "src")
	_, err := r.Resolve(context.Background(), "t-b", ref.Token)
	assert.True(t, errors.Is(err, ErrContentReferenceNotFound),
		"tenant-b cannot resolve tenant-a's ref")
}

func TestList_OrderedNewestFirst(t *testing.T) {
	r := NewInMemoryContentReferenceRegistry()
	now := time.Now()
	r.SetClock(func() time.Time { now = now.Add(time.Millisecond); return now })
	for _, c := range []string{"a", "b", "c"} {
		_, _ = r.Register(context.Background(), "t-1", ContentRefKindKBChunk, c, "src")
	}
	got, _ := r.List(context.Background(), "t-1")
	require.Len(t, got, 3)
	for i := 1; i < len(got); i++ {
		assert.True(t, got[i-1].CreatedAt.After(got[i].CreatedAt) ||
			got[i-1].CreatedAt.Equal(got[i].CreatedAt))
	}
}

func TestListByKind_FiltersStrictly(t *testing.T) {
	r := NewInMemoryContentReferenceRegistry()
	_, _ = r.Register(context.Background(), "t-1", ContentRefKindKBChunk, "a", "src")
	_, _ = r.Register(context.Background(), "t-1", ContentRefKindToolResult, "b", "src")
	got, _ := r.ListByKind(context.Background(), "t-1", ContentRefKindKBChunk)
	assert.Len(t, got, 1)
	assert.Equal(t, ContentRefKindKBChunk, got[0].Kind)
}

func TestDelete_RemovesRef(t *testing.T) {
	r := NewInMemoryContentReferenceRegistry()
	ref, _ := r.Register(context.Background(), "t-1", ContentRefKindKBChunk, "x", "src")
	require.NoError(t, r.Delete(context.Background(), "t-1", ref.Token))
	_, err := r.Resolve(context.Background(), "t-1", ref.Token)
	assert.True(t, errors.Is(err, ErrContentReferenceNotFound))
}

func TestDelete_UnknownReturnsNotFound(t *testing.T) {
	r := NewInMemoryContentReferenceRegistry()
	err := r.Delete(context.Background(), "t-1", "@ref{kb_chunk:0000000000000000}")
	assert.True(t, errors.Is(err, ErrContentReferenceNotFound))
}

func TestPurgeIdle_RemovesOlderThanCutoff(t *testing.T) {
	r := NewInMemoryContentReferenceRegistry()
	now := time.Now()
	r.SetClock(func() time.Time { return now })
	old, _ := r.Register(context.Background(), "t-1", ContentRefKindKBChunk, "old", "src")
	now = now.Add(2 * time.Hour)
	_, _ = r.Register(context.Background(), "t-1", ContentRefKindKBChunk, "fresh", "src")

	count, err := r.PurgeIdle(context.Background(), now.Add(-time.Hour))
	require.NoError(t, err)
	assert.Equal(t, 1, count)
	_, err = r.Resolve(context.Background(), "t-1", old.Token)
	assert.True(t, errors.Is(err, ErrContentReferenceNotFound))
}

func TestRecordHit_BumpsLastHitAtAndPreventsPurge(t *testing.T) {
	r := NewInMemoryContentReferenceRegistry()
	now := time.Now()
	r.SetClock(func() time.Time { return now })
	ref, _ := r.Register(context.Background(), "t-1", ContentRefKindKBChunk, "x", "src")

	now = now.Add(2 * time.Hour)
	require.NoError(t, r.RecordHit(context.Background(), "t-1", ref.Token))

	// PurgeIdle with cutoff between original create and new hit must NOT purge.
	count, _ := r.PurgeIdle(context.Background(), now.Add(-time.Hour))
	assert.Equal(t, 0, count)
}

func TestRecordHit_UnknownReturnsNotFound(t *testing.T) {
	r := NewInMemoryContentReferenceRegistry()
	err := r.RecordHit(context.Background(), "t-1", "@ref{kb_chunk:0000000000000000}")
	assert.True(t, errors.Is(err, ErrContentReferenceNotFound))
}

func TestExpandRefsInPrompt_ReplacesTokensWithContent(t *testing.T) {
	r := NewInMemoryContentReferenceRegistry()
	a, _ := r.Register(context.Background(), "t-1", ContentRefKindKBChunk, "ALPHA_BODY", "src")
	b, _ := r.Register(context.Background(), "t-1", ContentRefKindToolResult, "BETA_BODY", "src")
	prompt := "First: " + a.Token + " then second: " + b.Token + " end."

	expanded, refs, err := ExpandRefsInPrompt(context.Background(), r, "t-1", prompt)
	require.NoError(t, err)
	assert.Contains(t, expanded, "ALPHA_BODY")
	assert.Contains(t, expanded, "BETA_BODY")
	assert.NotContains(t, expanded, "@ref{")
	assert.Len(t, refs, 2)
}

func TestExpandRefsInPrompt_LeavesUnknownTokensIntact(t *testing.T) {
	r := NewInMemoryContentReferenceRegistry()
	prompt := "Hello @ref{kb_chunk:0000000000000000} world"
	expanded, refs, err := ExpandRefsInPrompt(context.Background(), r, "t-1", prompt)
	require.NoError(t, err)
	assert.Contains(t, expanded, "@ref{kb_chunk:0000000000000000}",
		"unknown tokens left in place — caller's logger picks up the miss")
	assert.Len(t, refs, 0)
}

func TestExpandRefsInPrompt_NoTokensReturnsPromptVerbatim(t *testing.T) {
	r := NewInMemoryContentReferenceRegistry()
	prompt := "no refs here"
	expanded, refs, err := ExpandRefsInPrompt(context.Background(), r, "t-1", prompt)
	require.NoError(t, err)
	assert.Equal(t, prompt, expanded)
	assert.Empty(t, refs)
}

func TestCountRefsInPrompt(t *testing.T) {
	prompt := "@ref{kb_chunk:0000000000000001} and @ref{tool_result:0000000000000002}"
	assert.Equal(t, 2, CountRefsInPrompt(prompt))
	assert.Equal(t, 0, CountRefsInPrompt("no refs"))
}

func TestRegister_ConcurrentDeduplicationIsSafe(t *testing.T) {
	r := NewInMemoryContentReferenceRegistry()
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = r.Register(context.Background(), "t-1", ContentRefKindKBChunk, "shared", "src")
		}()
	}
	wg.Wait()
	got, _ := r.List(context.Background(), "t-1")
	require.Len(t, got, 1, "all 50 register the same content → 1 record")
	assert.Equal(t, 50, got[0].HitCount)
}

func TestContext_Cancelled(t *testing.T) {
	r := NewInMemoryContentReferenceRegistry()
	ref, _ := r.Register(context.Background(), "t", ContentRefKindKBChunk, "x", "src")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := r.Register(ctx, "t", ContentRefKindKBChunk, "y", "src")
	assert.Error(t, err)
	_, err = r.Resolve(ctx, "t", ref.Token)
	assert.Error(t, err)
	_, err = r.List(ctx, "t")
	assert.Error(t, err)
	_, err = r.ListByKind(ctx, "t", ContentRefKindKBChunk)
	assert.Error(t, err)
	err = r.Delete(ctx, "t", ref.Token)
	assert.Error(t, err)
	_, err = r.PurgeIdle(ctx, time.Now())
	assert.Error(t, err)
	err = r.RecordHit(ctx, "t", ref.Token)
	assert.Error(t, err)
}

func TestRegister_RoundTripsLargeContent(t *testing.T) {
	r := NewInMemoryContentReferenceRegistry()
	body := strings.Repeat("xyz", 100000) // 300k chars
	ref, err := r.Register(context.Background(), "t", ContentRefKindToolResult, body, "src")
	require.NoError(t, err)
	got, err := r.Resolve(context.Background(), "t", ref.Token)
	require.NoError(t, err)
	assert.Equal(t, body, got.Content)
	assert.Equal(t, 300000, got.Bytes)
}
