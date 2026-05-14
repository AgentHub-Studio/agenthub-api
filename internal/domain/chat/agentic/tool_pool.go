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

// TOOL-003 — Tool pool assembly.
//
// PDF arXiv:2604.14228v1 §3 (Tools/Loop) describes how the harness, on
// every iteration, assembles the precise pool of tools the LLM is
// allowed to see. The pool is NOT the full registry: it is a filtered,
// source-tagged, permission-screened, name-unique snapshot bound to the
// turn. The same agent can see different pools on consecutive turns
// (e.g., a deferred MCP tool came online, or a hook revoked a tool).
//
// Distinct from existing AgentHub plumbing:
//   - TOOL-002 (tool registry) = catalog of every defined tool the
//     platform knows about.
//   - TOOL-003 (this file) = per-iteration assembly that selects from
//     multiple sources, applies subagent allowlist, applies permission
//     deny pre-filter (PERM-004 precursor), de-dups by name with
//     deterministic precedence, and emits an immutable snapshot.
//   - TOOL-004 (separate feature) = deduplication semantics for when
//     two sources contribute the same tool name; this file delegates
//     that decision to a precedence ladder defined here.

// ToolSource identifies where a tool entry originated.
type ToolSource string

const (
	// ToolSourceBuiltin — tools shipped with the runtime (Read/Write/Bash/etc).
	ToolSourceBuiltin ToolSource = "builtin"
	// ToolSourceSkill — tools materialized from a skill binding.
	ToolSourceSkill ToolSource = "skill"
	// ToolSourceMCP — tools advertised by an MCP server.
	ToolSourceMCP ToolSource = "mcp"
	// ToolSourceSubagent — tools that the parent harness exposes to a
	// subagent on its behalf (proxy/delegation).
	ToolSourceSubagent ToolSource = "subagent"
	// ToolSourceExtension — tools contributed by an installed extension
	// (EXT-001 / plugin manifest §6).
	ToolSourceExtension ToolSource = "extension"
)

var allToolSources = []ToolSource{
	ToolSourceBuiltin, ToolSourceSkill, ToolSourceMCP,
	ToolSourceSubagent, ToolSourceExtension,
}

// IsValidToolSource returns true for the bounded set.
func IsValidToolSource(s ToolSource) bool {
	for _, v := range allToolSources {
		if s == v {
			return true
		}
	}
	return false
}

// toolSourceRank: lower wins when two sources contribute the same name.
// Builtin > Subagent > Extension > Skill > MCP. Builtin always wins
// because the harness contract guarantees those names; a skill that
// shadows "Read" must not silently replace it.
func toolSourceRank(s ToolSource) int {
	switch s {
	case ToolSourceBuiltin:
		return 0
	case ToolSourceSubagent:
		return 1
	case ToolSourceExtension:
		return 2
	case ToolSourceSkill:
		return 3
	case ToolSourceMCP:
		return 4
	}
	return 99
}

// ToolPoolEntry is one tool the LLM may see this iteration.
type ToolPoolEntry struct {
	Name        string
	Source      ToolSource
	ProviderID  string // skill slug / mcp server name / extension slug / "harness"
	Description string
	Effect      ToolEffect
}

// Validate enforces invariants on a single entry.
func (e ToolPoolEntry) Validate() error {
	if strings.TrimSpace(e.Name) == "" {
		return ErrToolPoolEntryNameRequired
	}
	if !IsValidToolSource(e.Source) {
		return fmt.Errorf("%w: %q", ErrToolPoolInvalidSource, e.Source)
	}
	if strings.TrimSpace(e.ProviderID) == "" {
		return ErrToolPoolEntryProviderRequired
	}
	return nil
}

// ToolPoolProvider supplies a slice of candidate entries for assembly.
// Implementations must be safe for concurrent use because the assembler
// fan-outs to multiple providers.
type ToolPoolProvider interface {
	Source() ToolSource
	Provide(ctx context.Context) ([]ToolPoolEntry, error)
}

// ToolPoolFilter screens entries before they reach the LLM. Returning
// false drops the entry; returning true keeps it. Used by PERM pipeline
// (deny upfront), subagent allowlists, and dynamic hook revocation.
type ToolPoolFilter func(ToolPoolEntry) bool

// ToolPoolSnapshot is the immutable per-iteration output of assembly.
type ToolPoolSnapshot struct {
	IterationID  string
	AssembledAt  time.Time
	Entries      []ToolPoolEntry
	DroppedNames []string // names removed by filters or shadowing
}

// Names returns just the tool names in the order the LLM will see them.
func (s ToolPoolSnapshot) Names() []string {
	out := make([]string, len(s.Entries))
	for i, e := range s.Entries {
		out[i] = e.Name
	}
	return out
}

// HasName returns true if the named tool is exposed in this snapshot.
func (s ToolPoolSnapshot) HasName(name string) bool {
	for _, e := range s.Entries {
		if e.Name == name {
			return true
		}
	}
	return false
}

// Sentinel errors.
var (
	ErrToolPoolEntryNameRequired     = errors.New("tool pool entry: name required")
	ErrToolPoolEntryProviderRequired = errors.New("tool pool entry: provider id required")
	ErrToolPoolInvalidSource         = errors.New("tool pool: invalid source")
	ErrToolPoolNoProviders           = errors.New("tool pool: no providers configured")
	ErrToolPoolIterationIDRequired   = errors.New("tool pool: iteration id required")
)

// ToolPoolAssembler is the per-iteration builder. It is concurrent-safe
// (callers can build multiple snapshots for parallel subagents).
type ToolPoolAssembler struct {
	mu        sync.RWMutex
	providers []ToolPoolProvider
	filters   []ToolPoolFilter
	allowlist map[string]bool // empty = no restriction; non-empty = strict allowlist
	now       func() time.Time
}

// NewToolPoolAssembler builds an empty assembler.
func NewToolPoolAssembler() *ToolPoolAssembler {
	return &ToolPoolAssembler{now: time.Now}
}

// SetClock injects a clock for deterministic tests.
func (a *ToolPoolAssembler) SetClock(c func() time.Time) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.now = c
}

// Register adds a provider. Order does not matter; precedence is fixed
// by toolSourceRank.
func (a *ToolPoolAssembler) Register(p ToolPoolProvider) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.providers = append(a.providers, p)
}

// AddFilter installs a filter that runs against every candidate.
func (a *ToolPoolAssembler) AddFilter(f ToolPoolFilter) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.filters = append(a.filters, f)
}

// SetAllowlist restricts the snapshot to the named tools. Empty/nil
// disables the allowlist (default open behaviour). Subagents typically
// call this to enforce parent-imposed allowedTools.
func (a *ToolPoolAssembler) SetAllowlist(names []string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if len(names) == 0 {
		a.allowlist = nil
		return
	}
	a.allowlist = make(map[string]bool, len(names))
	for _, n := range names {
		a.allowlist[n] = true
	}
}

// Assemble materializes a snapshot for the given iteration ID. It
// queries every provider, applies filters and the allowlist, resolves
// name collisions via toolSourceRank (lowest rank wins), and orders
// the output deterministically (rank then name).
func (a *ToolPoolAssembler) Assemble(ctx context.Context, iterationID string) (ToolPoolSnapshot, error) {
	if strings.TrimSpace(iterationID) == "" {
		return ToolPoolSnapshot{}, ErrToolPoolIterationIDRequired
	}
	a.mu.RLock()
	providers := append([]ToolPoolProvider(nil), a.providers...)
	filters := append([]ToolPoolFilter(nil), a.filters...)
	allowlist := a.allowlist
	now := a.now
	a.mu.RUnlock()
	if len(providers) == 0 {
		return ToolPoolSnapshot{}, ErrToolPoolNoProviders
	}

	type collected struct {
		entry ToolPoolEntry
	}
	var all []collected
	for _, p := range providers {
		entries, err := p.Provide(ctx)
		if err != nil {
			return ToolPoolSnapshot{}, fmt.Errorf("provider %q: %w", p.Source(), err)
		}
		for _, e := range entries {
			if e.Source == "" {
				e.Source = p.Source()
			}
			if err := e.Validate(); err != nil {
				return ToolPoolSnapshot{}, fmt.Errorf("provider %q: %w", p.Source(), err)
			}
			all = append(all, collected{entry: e})
		}
	}

	dropped := []string{}

	// Apply filters first (deny upfront — PERM pre-filter contract).
	keep := make([]collected, 0, len(all))
	for _, c := range all {
		ok := true
		for _, f := range filters {
			if !f(c.entry) {
				ok = false
				break
			}
		}
		if !ok {
			dropped = append(dropped, c.entry.Name)
			continue
		}
		keep = append(keep, c)
	}

	// Apply allowlist (subagent restriction).
	if len(allowlist) > 0 {
		filtered := make([]collected, 0, len(keep))
		for _, c := range keep {
			if allowlist[c.entry.Name] {
				filtered = append(filtered, c)
			} else {
				dropped = append(dropped, c.entry.Name)
			}
		}
		keep = filtered
	}

	// Resolve name collisions: keep the entry with the lowest source rank.
	winners := make(map[string]ToolPoolEntry, len(keep))
	for _, c := range keep {
		existing, ok := winners[c.entry.Name]
		if !ok {
			winners[c.entry.Name] = c.entry
			continue
		}
		if toolSourceRank(c.entry.Source) < toolSourceRank(existing.Source) {
			dropped = append(dropped, existing.Name)
			winners[c.entry.Name] = c.entry
		} else {
			dropped = append(dropped, c.entry.Name)
		}
	}

	out := make([]ToolPoolEntry, 0, len(winners))
	for _, w := range winners {
		out = append(out, w)
	}
	sort.SliceStable(out, func(i, j int) bool {
		ri, rj := toolSourceRank(out[i].Source), toolSourceRank(out[j].Source)
		if ri != rj {
			return ri < rj
		}
		return out[i].Name < out[j].Name
	})

	// Stable, de-dup the dropped log so callers can audit.
	sort.Strings(dropped)
	dropped = uniqueStrings(dropped)

	return ToolPoolSnapshot{
		IterationID:  iterationID,
		AssembledAt:  now(),
		Entries:      out,
		DroppedNames: dropped,
	}, nil
}

func uniqueStrings(in []string) []string {
	if len(in) == 0 {
		return in
	}
	seen := map[string]bool{}
	out := make([]string, 0, len(in))
	for _, s := range in {
		if seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
	}
	return out
}

// StaticToolPoolProvider is a convenience provider backed by a fixed
// slice. Useful for builtin tools and tests.
type StaticToolPoolProvider struct {
	source  ToolSource
	entries []ToolPoolEntry
}

// NewStaticToolPoolProvider builds a provider; entries are validated
// lazily at Provide() time.
func NewStaticToolPoolProvider(source ToolSource, entries []ToolPoolEntry) *StaticToolPoolProvider {
	cp := append([]ToolPoolEntry(nil), entries...)
	return &StaticToolPoolProvider{source: source, entries: cp}
}

// Source returns the provider source classification.
func (p *StaticToolPoolProvider) Source() ToolSource { return p.source }

// Provide returns a copy of the entries.
func (p *StaticToolPoolProvider) Provide(_ context.Context) ([]ToolPoolEntry, error) {
	return append([]ToolPoolEntry(nil), p.entries...), nil
}
