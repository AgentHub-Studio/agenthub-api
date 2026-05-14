package agentic

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

// CTX-009 — Content references.
//
// PDF arXiv:2604.14228v1 §7.7 (large content like KB chunks, tool
// outputs, file blobs is replaced in the prompt with stable @ref tokens
// the model treats as opaque; runtime resolves on demand).
//
// Distinct from existing plumbing:
//   - toolresultstorage.go = ContentReplacementState — REPLACES large
//     tool results with summaries for cache stability (one-shot).
//   - tool_result_budget.go (CTX-008) = ENFORCES size budgets.
//   - content_reference.go (this file) = REGISTRY of opaque references:
//     each Register(content) returns a stable @ref{kind:hash}, callers
//     embed @ref tokens in prompts, downstream Resolve(token) returns
//     the original content. Hash-keyed = de-duplication + tamper-detection.
//
// Supports multi-tenant isolation, per-kind buckets, and bounded TTL
// (references can be GC'd after a configured idle window — backed up
// by RecordHit which extends the live timer).

// ContentReferenceKind bounded enum identifies what the reference holds.
type ContentReferenceKind string

const (
	// ContentRefKindKBChunk — knowledge base chunk text.
	ContentRefKindKBChunk ContentReferenceKind = "kb_chunk"
	// ContentRefKindToolResult — tool execution output.
	ContentRefKindToolResult ContentReferenceKind = "tool_result"
	// ContentRefKindFileBlob — file content (uploaded doc, etc.).
	ContentRefKindFileBlob ContentReferenceKind = "file_blob"
	// ContentRefKindWebFetch — web fetch result.
	ContentRefKindWebFetch ContentReferenceKind = "web_fetch"
	// ContentRefKindMemorySnapshot — memory snapshot pinned for the run.
	ContentRefKindMemorySnapshot ContentReferenceKind = "memory_snapshot"
)

var allContentReferenceKinds = []ContentReferenceKind{
	ContentRefKindKBChunk, ContentRefKindToolResult, ContentRefKindFileBlob,
	ContentRefKindWebFetch, ContentRefKindMemorySnapshot,
}

// IsValidContentReferenceKind returns true for the bounded set.
func IsValidContentReferenceKind(k ContentReferenceKind) bool {
	for _, v := range allContentReferenceKinds {
		if k == v {
			return true
		}
	}
	return false
}

// AllContentReferenceKinds returns a copy.
func AllContentReferenceKinds() []ContentReferenceKind {
	out := make([]ContentReferenceKind, len(allContentReferenceKinds))
	copy(out, allContentReferenceKinds)
	return out
}

// refTokenPattern matches the canonical token form: @ref{kind:hash16}.
// hash16 is 16 lowercase hex chars (first 16 of sha256).
var refTokenPattern = regexp.MustCompile(`@ref\{([a-z_]+):([0-9a-f]{16})\}`)

// ContentReference is one registered piece of content.
type ContentReference struct {
	Token       string                 `json:"token"`
	TenantID    string                 `json:"tenantId"`
	Kind        ContentReferenceKind   `json:"kind"`
	// Hash16 is the first 16 hex chars of sha256(content) — fingerprint
	// + de-duplication key.
	Hash16      string                 `json:"hash16"`
	Content     string                 `json:"content"`
	// Bytes is the original content size in bytes.
	Bytes       int                    `json:"bytes"`
	Source      string                 `json:"source,omitempty"` // human-readable origin
	CreatedAt   time.Time              `json:"createdAt"`
	LastHitAt   time.Time              `json:"lastHitAt"`
	HitCount    int                    `json:"hitCount"`
	Metadata    map[string]string      `json:"metadata,omitempty"`
}

// Sentinels.
var (
	ErrContentReferenceInvalidKind = errors.New("content reference: invalid kind")
	ErrContentReferenceTenantReq   = errors.New("content reference: tenant_id required")
	ErrContentReferenceContentReq  = errors.New("content reference: content required")
	ErrContentReferenceNotFound    = errors.New("content reference: token not found")
	ErrContentReferenceMalformed   = errors.New("content reference: token malformed")
	ErrContentReferenceTampered    = errors.New("content reference: stored content hash mismatch (tampering detected)")
)

// hashContent16 returns the first 16 lowercase hex chars of sha256.
func hashContent16(content string) string {
	h := sha256.Sum256([]byte(content))
	return hex.EncodeToString(h[:])[:16]
}

// FormatRefToken returns the canonical @ref{kind:hash16} form.
func FormatRefToken(kind ContentReferenceKind, hash16 string) string {
	return fmt.Sprintf("@ref{%s:%s}", kind, hash16)
}

// ParseRefToken extracts kind + hash from a token string.
func ParseRefToken(token string) (ContentReferenceKind, string, error) {
	matches := refTokenPattern.FindStringSubmatch(token)
	if matches == nil || len(matches) != 3 {
		return "", "", fmt.Errorf("%w: %q", ErrContentReferenceMalformed, token)
	}
	kind := ContentReferenceKind(matches[1])
	if !IsValidContentReferenceKind(kind) {
		return "", "", fmt.Errorf("%w: kind %q", ErrContentReferenceInvalidKind, kind)
	}
	return kind, matches[2], nil
}

// ContentReferenceRegistry is the persistence interface.
type ContentReferenceRegistry interface {
	Register(ctx context.Context, tenantID string, kind ContentReferenceKind, content, source string) (ContentReference, error)
	Resolve(ctx context.Context, tenantID, token string) (ContentReference, error)
	List(ctx context.Context, tenantID string) ([]ContentReference, error)
	ListByKind(ctx context.Context, tenantID string, kind ContentReferenceKind) ([]ContentReference, error)
	Delete(ctx context.Context, tenantID, token string) error
	PurgeIdle(ctx context.Context, idleSince time.Time) (int, error)
	RecordHit(ctx context.Context, tenantID, token string) error
}

// --- InMemoryContentReferenceRegistry ---

type contentRefKey struct {
	tenant string
	token  string
}

type InMemoryContentReferenceRegistry struct {
	mu   sync.Mutex
	refs map[contentRefKey]ContentReference
	now  func() time.Time // injectable for tests
}

// NewInMemoryContentReferenceRegistry returns a concurrent-safe registry.
func NewInMemoryContentReferenceRegistry() *InMemoryContentReferenceRegistry {
	return &InMemoryContentReferenceRegistry{
		refs: map[contentRefKey]ContentReference{},
		now:  time.Now,
	}
}

// SetClock allows tests to inject a deterministic clock.
func (r *InMemoryContentReferenceRegistry) SetClock(clock func() time.Time) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.now = clock
}

// Register stores content and returns the canonical reference.
// Idempotent: registering the same (tenant, kind, content) returns the
// existing reference and bumps LastHitAt + HitCount (de-duplication).
func (r *InMemoryContentReferenceRegistry) Register(ctx context.Context, tenantID string, kind ContentReferenceKind, content, source string) (ContentReference, error) {
	if err := ctx.Err(); err != nil {
		return ContentReference{}, err
	}
	if strings.TrimSpace(tenantID) == "" {
		return ContentReference{}, ErrContentReferenceTenantReq
	}
	if !IsValidContentReferenceKind(kind) {
		return ContentReference{}, fmt.Errorf("%w: %q", ErrContentReferenceInvalidKind, kind)
	}
	if content == "" {
		return ContentReference{}, ErrContentReferenceContentReq
	}
	hash16 := hashContent16(content)
	token := FormatRefToken(kind, hash16)
	key := contentRefKey{tenantID, token}

	r.mu.Lock()
	defer r.mu.Unlock()
	now := r.now()
	if existing, ok := r.refs[key]; ok {
		// De-duplication: same content already registered. Bump hit.
		existing.LastHitAt = now
		existing.HitCount++
		r.refs[key] = existing
		return existing, nil
	}
	ref := ContentReference{
		Token:     token,
		TenantID:  tenantID,
		Kind:      kind,
		Hash16:    hash16,
		Content:   content,
		Bytes:     len(content),
		Source:    source,
		CreatedAt: now,
		LastHitAt: now,
		HitCount:  1,
	}
	r.refs[key] = ref
	return ref, nil
}

// Resolve returns the content for a reference token. Verifies content
// hash matches token (tamper detection).
func (r *InMemoryContentReferenceRegistry) Resolve(ctx context.Context, tenantID, token string) (ContentReference, error) {
	if err := ctx.Err(); err != nil {
		return ContentReference{}, err
	}
	if strings.TrimSpace(tenantID) == "" {
		return ContentReference{}, ErrContentReferenceTenantReq
	}
	_, hash16, err := ParseRefToken(token)
	if err != nil {
		return ContentReference{}, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	ref, ok := r.refs[contentRefKey{tenantID, token}]
	if !ok {
		return ContentReference{}, ErrContentReferenceNotFound
	}
	// Tamper check: stored content must still hash to the token.
	if hashContent16(ref.Content) != hash16 {
		return ContentReference{}, ErrContentReferenceTampered
	}
	return ref, nil
}

// List returns all references for a tenant, sorted newest-first.
func (r *InMemoryContentReferenceRegistry) List(ctx context.Context, tenantID string) ([]ContentReference, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	out := []ContentReference{}
	for k, ref := range r.refs {
		if k.tenant == tenantID {
			out = append(out, ref)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	return out, nil
}

// ListByKind filters by kind.
func (r *InMemoryContentReferenceRegistry) ListByKind(ctx context.Context, tenantID string, kind ContentReferenceKind) ([]ContentReference, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	all, err := r.List(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	out := []ContentReference{}
	for _, ref := range all {
		if ref.Kind == kind {
			out = append(out, ref)
		}
	}
	return out, nil
}

// Delete removes a reference.
func (r *InMemoryContentReferenceRegistry) Delete(ctx context.Context, tenantID, token string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	key := contentRefKey{tenantID, token}
	if _, ok := r.refs[key]; !ok {
		return ErrContentReferenceNotFound
	}
	delete(r.refs, key)
	return nil
}

// PurgeIdle removes references whose LastHitAt is before idleSince.
// Returns the count removed.
func (r *InMemoryContentReferenceRegistry) PurgeIdle(ctx context.Context, idleSince time.Time) (int, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	removed := 0
	for k, ref := range r.refs {
		if ref.LastHitAt.Before(idleSince) {
			delete(r.refs, k)
			removed++
		}
	}
	return removed, nil
}

// RecordHit bumps LastHitAt + HitCount for a reference (extends its
// live window so PurgeIdle doesn't sweep it). Useful when caller knows
// it's about to use the reference but hasn't called Resolve yet.
func (r *InMemoryContentReferenceRegistry) RecordHit(ctx context.Context, tenantID, token string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	key := contentRefKey{tenantID, token}
	ref, ok := r.refs[key]
	if !ok {
		return ErrContentReferenceNotFound
	}
	ref.LastHitAt = r.now()
	ref.HitCount++
	r.refs[key] = ref
	return nil
}

// ExpandRefsInPrompt scans a prompt body for @ref tokens and resolves
// each one to its underlying content. Returns the expanded prompt and
// a list of references successfully expanded. Tokens not found are
// left as-is in the output (caller can log + continue).
func ExpandRefsInPrompt(ctx context.Context, registry ContentReferenceRegistry, tenantID, prompt string) (string, []ContentReference, error) {
	if err := ctx.Err(); err != nil {
		return "", nil, err
	}
	expanded := []ContentReference{}
	out := refTokenPattern.ReplaceAllStringFunc(prompt, func(token string) string {
		ref, err := registry.Resolve(ctx, tenantID, token)
		if err != nil {
			// Leave token as-is — caller's logger picks up the miss.
			return token
		}
		expanded = append(expanded, ref)
		return ref.Content
	})
	return out, expanded, nil
}

// CountRefsInPrompt returns the number of @ref tokens present (does not resolve).
func CountRefsInPrompt(prompt string) int {
	return len(refTokenPattern.FindAllString(prompt, -1))
}
