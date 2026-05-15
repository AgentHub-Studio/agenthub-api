package agentic

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func time0() time.Time { return time.Now() }

func TestBDD_ContentReference(t *testing.T) {
	t.Run("Scenario_LargeKBChunkReplacedByOpaqueRefTokenInPrompt", func(t *testing.T) {
		// Given a 300k-char KB chunk that would dominate the prompt,
		// And the prompt-builder registers it as a content reference,
		// When the LLM sees the prompt,
		// Then it sees a stable @ref{kb_chunk:hash16} token instead of
		// raw bytes — context window stays small + prompt cache stable
		// across turns (PDF §7.7).
		r := NewInMemoryContentReferenceRegistry()
		body := strings.Repeat("kb chunk text ", 20000)
		ref, err := r.Register(context.Background(), "t-1", ContentRefKindKBChunk, body, "kb_loader")
		require.NoError(t, err)
		assert.Less(t, len(ref.Token), 50, "token is small even for 300k content")
	})

	t.Run("Scenario_DeduplicationCollapsesRepeatedContentToOneToken", func(t *testing.T) {
		// Given the same KB chunk is referenced from 3 different agent
		// turns within one run,
		// When each turn registers it,
		// Then they all get the SAME token + HitCount=3 (de-duplication
		// via content hash — no wasted storage).
		r := NewInMemoryContentReferenceRegistry()
		for i := 0; i < 3; i++ {
			_, err := r.Register(context.Background(), "t-1", ContentRefKindKBChunk, "shared chunk", "src")
			require.NoError(t, err)
		}
		got, _ := r.List(context.Background(), "t-1")
		require.Len(t, got, 1)
		assert.Equal(t, 3, got[0].HitCount)
	})

	t.Run("Scenario_ResolveDetectsTamperingForGOV001Audit", func(t *testing.T) {
		// Given an attacker compromised the registry storage and mutated
		// the content under a stable token (preserving token format),
		// When downstream resolves to use the content,
		// Then hash-mismatch raises tamper-detection — agent sees error
		// (no silent serving of wrong-content-under-stable-token attack).
		r := NewInMemoryContentReferenceRegistry()
		ref, _ := r.Register(context.Background(), "t-1", ContentRefKindKBChunk, "trusted", "src")
		// Simulate adversarial mutation.
		r.mu.Lock()
		stored := r.refs[contentRefKey{"t-1", ref.Token}]
		stored.Content = "MALICIOUS REPLACEMENT"
		r.refs[contentRefKey{"t-1", ref.Token}] = stored
		r.mu.Unlock()

		_, err := r.Resolve(context.Background(), "t-1", ref.Token)
		assert.True(t, errors.Is(err, ErrContentReferenceTampered))
	})

	t.Run("Scenario_TenantIsolationPreventsCrossTenantTokenResolution", func(t *testing.T) {
		// Given tenant-A registers a sensitive KB chunk,
		// When tenant-B somehow gets the token (logging leak / etc.),
		// Then tenant-B Resolve returns NotFound — token format is
		// public but registry storage is per-tenant (defense in depth).
		r := NewInMemoryContentReferenceRegistry()
		ref, _ := r.Register(context.Background(), "tenant-a", ContentRefKindKBChunk, "secret", "src")
		_, err := r.Resolve(context.Background(), "tenant-b", ref.Token)
		assert.True(t, errors.Is(err, ErrContentReferenceNotFound))
	})

	t.Run("Scenario_PurgeIdleSweepsUnusedReferencesForGCJob", func(t *testing.T) {
		// Given a nightly GC job clears references not hit in the last hour,
		// When PurgeIdle runs,
		// Then it returns the count removed and stale entries are gone
		// — registry stays bounded over a long-running platform.
		r := NewInMemoryContentReferenceRegistry()
		now := time0()
		r.SetClock(func() time.Time { return now })
		_, _ = r.Register(context.Background(), "t", ContentRefKindKBChunk, "old", "src")
		now = now.Add(2 * time.Hour)
		_, _ = r.Register(context.Background(), "t", ContentRefKindKBChunk, "fresh", "src")
		count, err := r.PurgeIdle(context.Background(), now.Add(-time.Hour))
		require.NoError(t, err)
		assert.Equal(t, 1, count)
	})

	t.Run("Scenario_RecordHitExtendsLiveWindowForHotReferences", func(t *testing.T) {
		// Given a hot KB chunk is referenced often but doesn't trigger
		// Resolve every time (e.g. preview),
		// When the runtime calls RecordHit,
		// Then LastHitAt bumps so PurgeIdle doesn't sweep it during
		// normal hot-path operation.
		r := NewInMemoryContentReferenceRegistry()
		now := time0()
		r.SetClock(func() time.Time { return now })
		ref, _ := r.Register(context.Background(), "t", ContentRefKindKBChunk, "hot", "src")
		now = now.Add(2 * time.Hour)
		require.NoError(t, r.RecordHit(context.Background(), "t", ref.Token))
		count, _ := r.PurgeIdle(context.Background(), now.Add(-time.Hour))
		assert.Equal(t, 0, count, "hot ref survived purge thanks to RecordHit")
	})

	t.Run("Scenario_ExpandRefsInPromptHydratesTokensInOneSweep", func(t *testing.T) {
		// Given the LLM-side renderer needs the actual content to send,
		// When ExpandRefsInPrompt processes the prompt body,
		// Then all @ref tokens are replaced with their content in a
		// single pass — no per-token round-trip from caller.
		r := NewInMemoryContentReferenceRegistry()
		a, _ := r.Register(context.Background(), "t", ContentRefKindKBChunk, "ALPHA", "src")
		b, _ := r.Register(context.Background(), "t", ContentRefKindToolResult, "BETA", "src")
		prompt := "Top: " + a.Token + " mid " + b.Token + " end"
		expanded, refs, err := ExpandRefsInPrompt(context.Background(), r, "t", prompt)
		require.NoError(t, err)
		assert.Contains(t, expanded, "ALPHA")
		assert.Contains(t, expanded, "BETA")
		assert.NotContains(t, expanded, "@ref")
		assert.Len(t, refs, 2)
	})

	t.Run("Scenario_UnknownTokensLeftIntactNotFatal", func(t *testing.T) {
		// Given a prompt may contain orphan tokens after a registry
		// PurgeIdle (e.g. transcript replay across days),
		// When ExpandRefsInPrompt encounters one,
		// Then the token is left in place AND the function does NOT
		// return an error — caller logs the miss and continues
		// (graceful degradation).
		r := NewInMemoryContentReferenceRegistry()
		prompt := "hello @ref{kb_chunk:0000000000000000} world"
		expanded, refs, err := ExpandRefsInPrompt(context.Background(), r, "t-1", prompt)
		require.NoError(t, err)
		assert.Contains(t, expanded, "@ref{kb_chunk:0000000000000000}")
		assert.Empty(t, refs)
	})

	t.Run("Scenario_FiveKindsCoverContextSourcesPlatformExposes", func(t *testing.T) {
		// Given AgentHub exposes context from KBs / tool results / file
		// blobs / web fetches / memory snapshots,
		// When the platform registers a reference,
		// Then it MUST classify into one of those 5 kinds — closed set
		// (new sources require seed update + classifier change).
		assert.Equal(t, 5, len(AllContentReferenceKinds()))
	})

	t.Run("Scenario_TokenFormatIsCacheStableAcrossRunsForReplay", func(t *testing.T) {
		// Given replay debugging needs reproducible token assignments,
		// When same content is registered in two separate runs,
		// Then token is BYTE-IDENTICAL (sha256 of content is deterministic)
		// — replay/transcript matches across runs.
		r1 := NewInMemoryContentReferenceRegistry()
		r2 := NewInMemoryContentReferenceRegistry()
		a, _ := r1.Register(context.Background(), "t", ContentRefKindKBChunk, "deterministic body", "src")
		b, _ := r2.Register(context.Background(), "t", ContentRefKindKBChunk, "deterministic body", "src")
		assert.Equal(t, a.Token, b.Token,
			"same content → same token across registries (deterministic for replay)")
	})
}
