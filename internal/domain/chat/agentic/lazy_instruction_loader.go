package agentic

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

// CTX-005 — Lazy loading de instruções.
//
// PDF arXiv:2604.14228v1 §7.9 (instruction sources are loaded on
// demand, not eagerly — first reference triggers load, subsequent
// references hit cache; cache TTL bounded for freshness).
//
// Distinct from existing AgentHub plumbing:
//   - context.go ContextManager = compaction lifecycle.
//   - prompt.go PromptBuilder = chat-coupled assembly.
//   - lazy_instruction_loader.go (this file) = LAZY DEMAND LOADER:
//     register descriptors with load_fn callbacks; first Load() invokes
//     fn (cache miss); subsequent calls hit cache; TTL expiry triggers
//     re-invoke. Saves cold-start cost of loading instructions that
//     won't be used in this run.

// LazyInstructionStatus bounded enum tracks lifecycle.
type LazyInstructionStatus string

const (
	// LazyInstructionStatusRegistered — descriptor known but never loaded.
	LazyInstructionStatusRegistered LazyInstructionStatus = "registered"
	// LazyInstructionStatusLoaded — content cached and live.
	LazyInstructionStatusLoaded LazyInstructionStatus = "loaded"
	// LazyInstructionStatusExpired — cached but TTL elapsed; next Load reloads.
	LazyInstructionStatusExpired LazyInstructionStatus = "expired"
	// LazyInstructionStatusFailed — last load attempt failed; cached error.
	LazyInstructionStatusFailed LazyInstructionStatus = "failed"
)

var allLazyInstructionStatuses = []LazyInstructionStatus{
	LazyInstructionStatusRegistered, LazyInstructionStatusLoaded,
	LazyInstructionStatusExpired, LazyInstructionStatusFailed,
}

// IsValidLazyInstructionStatus returns true for the bounded set.
func IsValidLazyInstructionStatus(s LazyInstructionStatus) bool {
	for _, v := range allLazyInstructionStatuses {
		if s == v {
			return true
		}
	}
	return false
}

// AllLazyInstructionStatuses returns a copy.
func AllLazyInstructionStatuses() []LazyInstructionStatus {
	out := make([]LazyInstructionStatus, len(allLazyInstructionStatuses))
	copy(out, allLazyInstructionStatuses)
	return out
}

// LazyInstructionLoadFn is the callback invoked on cache miss / expiry.
type LazyInstructionLoadFn func(ctx context.Context) (string, error)

// LazyInstruction is the registered descriptor + cache state.
type LazyInstruction struct {
	Slug         string                `json:"slug"`
	Description  string                `json:"description,omitempty"`
	TTL          time.Duration         `json:"ttl"`
	Loader       LazyInstructionLoadFn `json:"-"`
	Status       LazyInstructionStatus `json:"status"`
	Content      string                `json:"content,omitempty"`
	LoadedAt     time.Time             `json:"loadedAt,omitempty"`
	LoadCount    int                   `json:"loadCount"`
	LastError    string                `json:"lastError,omitempty"`
	LastAccessAt time.Time             `json:"lastAccessAt,omitempty"`
}

// IsExpired returns true if Status is loaded and time since LoadedAt > TTL.
func (i LazyInstruction) IsExpired(now time.Time) bool {
	if i.Status != LazyInstructionStatusLoaded {
		return false
	}
	if i.TTL <= 0 {
		return false // never expires
	}
	return now.Sub(i.LoadedAt) > i.TTL
}

// Sentinels.
var (
	ErrLazyInstructionInvalidStatus = errors.New("lazy instruction: invalid status")
	ErrLazyInstructionSlugEmpty     = errors.New("lazy instruction: slug required")
	ErrLazyInstructionLoaderNil     = errors.New("lazy instruction: loader fn required")
	ErrLazyInstructionNotFound      = errors.New("lazy instruction: not found")
	ErrLazyInstructionDuplicate     = errors.New("lazy instruction: slug already registered")
	ErrLazyInstructionLastFailed    = errors.New("lazy instruction: last load failed (cached error)")
)

// LazyInstructionLoader is the persistence + execution interface.
type LazyInstructionLoader interface {
	Register(ctx context.Context, slug, description string, ttl time.Duration, loader LazyInstructionLoadFn) error
	Load(ctx context.Context, slug string) (string, error)
	Find(ctx context.Context, slug string) (LazyInstruction, error)
	Invalidate(ctx context.Context, slug string) error
	InvalidateAll(ctx context.Context) error
	List(ctx context.Context) ([]LazyInstruction, error)
	ListByStatus(ctx context.Context, status LazyInstructionStatus) ([]LazyInstruction, error)
	Stats(ctx context.Context) (LazyInstructionLoaderStats, error)
}

// LazyInstructionLoaderStats summarizes cache performance.
type LazyInstructionLoaderStats struct {
	TotalRegistered int `json:"totalRegistered"`
	TotalLoaded     int `json:"totalLoaded"`
	TotalExpired    int `json:"totalExpired"`
	TotalFailed     int `json:"totalFailed"`
	CumulativeLoads int `json:"cumulativeLoads"` // sum of LoadCount across all instructions
}

// --- InMemoryLazyInstructionLoader ---

type InMemoryLazyInstructionLoader struct {
	mu           sync.Mutex
	instructions map[string]*LazyInstruction
	now          func() time.Time
}

// NewInMemoryLazyInstructionLoader returns a concurrent-safe loader.
func NewInMemoryLazyInstructionLoader() *InMemoryLazyInstructionLoader {
	return &InMemoryLazyInstructionLoader{
		instructions: map[string]*LazyInstruction{},
		now:          time.Now,
	}
}

// SetClock allows tests to inject a deterministic clock.
func (l *InMemoryLazyInstructionLoader) SetClock(clock func() time.Time) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.now = clock
}

// Register stores a descriptor without invoking loader. Lazy: load_fn
// runs on first Load() call.
func (l *InMemoryLazyInstructionLoader) Register(ctx context.Context, slug, description string, ttl time.Duration, loader LazyInstructionLoadFn) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if strings.TrimSpace(slug) == "" {
		return ErrLazyInstructionSlugEmpty
	}
	if loader == nil {
		return ErrLazyInstructionLoaderNil
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if _, exists := l.instructions[slug]; exists {
		return fmt.Errorf("%w: %q", ErrLazyInstructionDuplicate, slug)
	}
	l.instructions[slug] = &LazyInstruction{
		Slug:        slug,
		Description: description,
		TTL:         ttl,
		Loader:      loader,
		Status:      LazyInstructionStatusRegistered,
	}
	return nil
}

// Load returns content. On cache miss / expiry, invokes loader fn.
// On loader error, caches the failure (returns ErrLazyInstructionLastFailed
// on subsequent calls until Invalidate).
func (l *InMemoryLazyInstructionLoader) Load(ctx context.Context, slug string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	instr, ok := l.instructions[slug]
	if !ok {
		return "", ErrLazyInstructionNotFound
	}
	now := l.now()
	instr.LastAccessAt = now

	// Mark expired if TTL elapsed.
	if instr.IsExpired(now) {
		instr.Status = LazyInstructionStatusExpired
	}

	switch instr.Status {
	case LazyInstructionStatusLoaded:
		return instr.Content, nil
	case LazyInstructionStatusFailed:
		// Cached failure — caller must Invalidate to retry.
		return "", fmt.Errorf("%w: %s", ErrLazyInstructionLastFailed, instr.LastError)
	}

	// Cache miss / expired — invoke loader.
	content, err := instr.Loader(ctx)
	if err != nil {
		instr.Status = LazyInstructionStatusFailed
		instr.LastError = err.Error()
		instr.LoadCount++
		return "", fmt.Errorf("%w: %s", ErrLazyInstructionLastFailed, err.Error())
	}
	instr.Status = LazyInstructionStatusLoaded
	instr.Content = content
	instr.LoadedAt = now
	instr.LoadCount++
	instr.LastError = ""
	return content, nil
}

// Find returns the descriptor (without invoking loader).
func (l *InMemoryLazyInstructionLoader) Find(ctx context.Context, slug string) (LazyInstruction, error) {
	if err := ctx.Err(); err != nil {
		return LazyInstruction{}, err
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	instr, ok := l.instructions[slug]
	if !ok {
		return LazyInstruction{}, ErrLazyInstructionNotFound
	}
	// Refresh expired status before return.
	if instr.IsExpired(l.now()) {
		instr.Status = LazyInstructionStatusExpired
	}
	return *instr, nil
}

// Invalidate clears cached content for one slug; status reverts to registered.
func (l *InMemoryLazyInstructionLoader) Invalidate(ctx context.Context, slug string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	instr, ok := l.instructions[slug]
	if !ok {
		return ErrLazyInstructionNotFound
	}
	instr.Status = LazyInstructionStatusRegistered
	instr.Content = ""
	instr.LastError = ""
	instr.LoadedAt = time.Time{}
	return nil
}

// InvalidateAll clears all cached content.
func (l *InMemoryLazyInstructionLoader) InvalidateAll(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	for _, instr := range l.instructions {
		instr.Status = LazyInstructionStatusRegistered
		instr.Content = ""
		instr.LastError = ""
		instr.LoadedAt = time.Time{}
	}
	return nil
}

// List returns all descriptors, sorted by slug.
func (l *InMemoryLazyInstructionLoader) List(ctx context.Context) ([]LazyInstruction, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	out := []LazyInstruction{}
	for _, instr := range l.instructions {
		if instr.IsExpired(now) {
			instr.Status = LazyInstructionStatusExpired
		}
		out = append(out, *instr)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Slug < out[j].Slug })
	return out, nil
}

// ListByStatus filters by status.
func (l *InMemoryLazyInstructionLoader) ListByStatus(ctx context.Context, status LazyInstructionStatus) ([]LazyInstruction, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	all, err := l.List(ctx)
	if err != nil {
		return nil, err
	}
	out := []LazyInstruction{}
	for _, instr := range all {
		if instr.Status == status {
			out = append(out, instr)
		}
	}
	return out, nil
}

// Stats summarizes cache performance.
func (l *InMemoryLazyInstructionLoader) Stats(ctx context.Context) (LazyInstructionLoaderStats, error) {
	if err := ctx.Err(); err != nil {
		return LazyInstructionLoaderStats{}, err
	}
	all, err := l.List(ctx)
	if err != nil {
		return LazyInstructionLoaderStats{}, err
	}
	stats := LazyInstructionLoaderStats{TotalRegistered: len(all)}
	for _, instr := range all {
		switch instr.Status {
		case LazyInstructionStatusLoaded:
			stats.TotalLoaded++
		case LazyInstructionStatusExpired:
			stats.TotalExpired++
		case LazyInstructionStatusFailed:
			stats.TotalFailed++
		}
		stats.CumulativeLoads += instr.LoadCount
	}
	return stats, nil
}
