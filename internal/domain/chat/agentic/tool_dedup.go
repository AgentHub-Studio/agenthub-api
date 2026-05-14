package agentic

import (
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"sync"
)

// TOOL-004 — Tool deduplication and precedence.
//
// PDF arXiv:2604.14228v1 §3 (Tools / Loop) — when the runtime assembles
// the per-iteration pool (TOOL-003), name collisions are only the first
// problem. The deeper semantics covered here:
//
//   1. Aliases — `read_file` and `Read` may be the same tool surfaced
//      under two names; the dedup layer resolves to a canonical name
//      so the LLM does not see split-brain ghosts of one capability.
//   2. Versions — `vector_search@1.2` and `vector_search@2.0` differ in
//      contract; the platform can pin a version per tenant, keep both
//      for migration windows, or fail loudly on collision.
//   3. Per-tool policy — some tools are stateful (sessions/queues) and
//      must NOT be deduped even if the same name appears twice; some
//      are pure and either copy is fine.
//
// Distinct from TOOL-003: TOOL-003 picks a winner when two ToolSources
// claim the same NAME using a single source-rank ladder. TOOL-004
// operates on the SEMANTIC level — alias-resolve first, version-resolve
// next, then leave the residue for TOOL-003 source-rank dedup.

// ToolDedupPolicy bounded enum captures the five resolution strategies.
type ToolDedupPolicy string

const (
	// ToolDedupDenyCollision — fail Resolve if two versions appear.
	ToolDedupDenyCollision ToolDedupPolicy = "deny_collision"
	// ToolDedupPreferSourceRank — drop the loser by TOOL-003 source rank.
	ToolDedupPreferSourceRank ToolDedupPolicy = "prefer_source_rank"
	// ToolDedupPreferLatestVersion — keep the highest semver, drop rest.
	ToolDedupPreferLatestVersion ToolDedupPolicy = "prefer_latest_version"
	// ToolDedupPreferPinned — admin pinned a specific version per tenant;
	// only that version survives. Falls back to source rank if no pin.
	ToolDedupPreferPinned ToolDedupPolicy = "prefer_pinned"
	// ToolDedupKeepAllVersions — both/all versions remain visible.
	ToolDedupKeepAllVersions ToolDedupPolicy = "keep_all_versions"
)

var allToolDedupPolicies = []ToolDedupPolicy{
	ToolDedupDenyCollision,
	ToolDedupPreferSourceRank,
	ToolDedupPreferLatestVersion,
	ToolDedupPreferPinned,
	ToolDedupKeepAllVersions,
}

// IsValidToolDedupPolicy returns true for the bounded set.
func IsValidToolDedupPolicy(p ToolDedupPolicy) bool {
	for _, v := range allToolDedupPolicies {
		if p == v {
			return true
		}
	}
	return false
}

// VersionedToolName is `name` or `name@vX.Y.Z`.
type VersionedToolName struct {
	Name    string
	Version string
}

var versionedNameRE = regexp.MustCompile(`^(?P<name>[A-Za-z][A-Za-z0-9_]*)(?:@(?P<ver>[0-9]+(?:\.[0-9]+){0,2}))?$`)

// ParseVersionedToolName parses `name@version` or returns an error.
func ParseVersionedToolName(raw string) (VersionedToolName, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return VersionedToolName{}, ErrToolDedupEmptyName
	}
	m := versionedNameRE.FindStringSubmatch(raw)
	if m == nil {
		return VersionedToolName{}, fmt.Errorf("%w: %q", ErrToolDedupBadName, raw)
	}
	return VersionedToolName{Name: m[1], Version: m[2]}, nil
}

// String renders back to canonical form.
func (v VersionedToolName) String() string {
	if v.Version == "" {
		return v.Name
	}
	return v.Name + "@" + v.Version
}

// CompareSemver returns -1/0/+1. Reuses splitSemver from plugin_manifest.
func CompareSemver(a, b string) int {
	pa := splitSemver(a)
	pb := splitSemver(b)
	for i := 0; i < 3; i++ {
		if pa[i] < pb[i] {
			return -1
		}
		if pa[i] > pb[i] {
			return 1
		}
	}
	return 0
}

// ToolAliasMap resolves alias names to a canonical name.
type ToolAliasMap map[string]string

// Canonical returns the canonical name for an alias, or the input if
// unknown.
func (m ToolAliasMap) Canonical(name string) string {
	if m == nil {
		return name
	}
	if c, ok := m[name]; ok {
		return c
	}
	return name
}

// ToolVersionPin records "this tenant uses name@version".
type ToolVersionPin struct {
	Name    string
	Version string
}

// ToolDedupConfig wires policy + aliases + pins.
type ToolDedupConfig struct {
	Policy        ToolDedupPolicy
	Aliases       ToolAliasMap
	Pins          []ToolVersionPin
	PerToolPolicy map[string]ToolDedupPolicy
	StatefulTools map[string]bool
}

// Validate ensures internal consistency.
func (c ToolDedupConfig) Validate() error {
	if !IsValidToolDedupPolicy(c.Policy) {
		return fmt.Errorf("%w: %q", ErrToolDedupBadPolicy, c.Policy)
	}
	for k, v := range c.PerToolPolicy {
		if strings.TrimSpace(k) == "" {
			return ErrToolDedupEmptyName
		}
		if !IsValidToolDedupPolicy(v) {
			return fmt.Errorf("%w: per-tool %q policy %q", ErrToolDedupBadPolicy, k, v)
		}
	}
	for _, p := range c.Pins {
		if strings.TrimSpace(p.Name) == "" || strings.TrimSpace(p.Version) == "" {
			return ErrToolDedupBadPin
		}
	}
	return nil
}

func (c ToolDedupConfig) pinFor(name string) string {
	for _, p := range c.Pins {
		if p.Name == name {
			return p.Version
		}
	}
	return ""
}

func (c ToolDedupConfig) policyFor(name string) ToolDedupPolicy {
	if p, ok := c.PerToolPolicy[name]; ok {
		return p
	}
	return c.Policy
}

// Sentinel errors.
var (
	ErrToolDedupEmptyName       = errors.New("tool dedup: empty name")
	ErrToolDedupBadName         = errors.New("tool dedup: invalid name format")
	ErrToolDedupBadPolicy       = errors.New("tool dedup: invalid policy")
	ErrToolDedupBadPin          = errors.New("tool dedup: invalid pin (name and version required)")
	ErrToolDedupVersionConflict = errors.New("tool dedup: version conflict under deny policy")
)

// DedupResult is the per-canonical-name outcome.
type DedupResult struct {
	Canonical string
	Versions  []string
	Kept      []ToolPoolEntry
	Dropped   []ToolPoolEntry
	Reason    string
}

// dedupParsed pairs an entry with its alias-resolved canonical name
// and the parsed version. Package-level so helpers can take it.
type dedupParsed struct {
	entry     ToolPoolEntry
	canonical string
	version   string
}

// ToolDedupResolver applies aliases + version policy to ToolPoolEntries.
type ToolDedupResolver struct {
	mu  sync.RWMutex
	cfg ToolDedupConfig
}

// NewToolDedupResolver builds a resolver. Validates the config eagerly.
func NewToolDedupResolver(cfg ToolDedupConfig) (*ToolDedupResolver, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return &ToolDedupResolver{cfg: cfg}, nil
}

// Resolve groups entries by canonical name (alias-resolved), parses
// versions, then applies the configured policy.
func (r *ToolDedupResolver) Resolve(entries []ToolPoolEntry) ([]ToolPoolEntry, []DedupResult, error) {
	r.mu.RLock()
	cfg := r.cfg
	r.mu.RUnlock()

	groups := map[string][]dedupParsed{}
	canonicalOrder := []string{}

	for _, e := range entries {
		v, err := ParseVersionedToolName(e.Name)
		if err != nil {
			return nil, nil, fmt.Errorf("entry %q: %w", e.Name, err)
		}
		canonical := cfg.Aliases.Canonical(v.Name)
		if _, ok := groups[canonical]; !ok {
			canonicalOrder = append(canonicalOrder, canonical)
		}
		groups[canonical] = append(groups[canonical], dedupParsed{
			entry:     e,
			canonical: canonical,
			version:   v.Version,
		})
	}

	var kept []ToolPoolEntry
	var results []DedupResult

	for _, key := range canonicalOrder {
		bucket := groups[key]

		if cfg.StatefulTools[key] {
			out := make([]ToolPoolEntry, 0, len(bucket))
			for _, p := range bucket {
				out = append(out, p.entry)
			}
			kept = append(kept, out...)
			results = append(results, DedupResult{
				Canonical: key,
				Versions:  uniqueVersions(bucket),
				Kept:      out,
				Reason:    "stateful: dedup skipped",
			})
			continue
		}

		policy := cfg.policyFor(key)
		switch policy {
		case ToolDedupKeepAllVersions:
			out := make([]ToolPoolEntry, 0, len(bucket))
			for _, p := range bucket {
				out = append(out, p.entry)
			}
			versions := uniqueVersions(bucket)
			kept = append(kept, out...)
			results = append(results, DedupResult{
				Canonical: key,
				Versions:  versions,
				Kept:      out,
				Reason:    fmt.Sprintf("policy %q kept %d versions", policy, len(versions)),
			})

		case ToolDedupDenyCollision:
			if len(bucket) > 1 && hasMultipleVersions(bucket) {
				return nil, nil, fmt.Errorf("%w: %q has %d versions", ErrToolDedupVersionConflict, key, len(uniqueVersions(bucket)))
			}
			out := []ToolPoolEntry{bucket[0].entry}
			var dropped []ToolPoolEntry
			for _, p := range bucket[1:] {
				dropped = append(dropped, p.entry)
			}
			kept = append(kept, out...)
			results = append(results, DedupResult{
				Canonical: key,
				Versions:  uniqueVersions(bucket),
				Kept:      out,
				Dropped:   dropped,
				Reason:    "policy deny_collision: only one version present, first wins",
			})

		case ToolDedupPreferLatestVersion:
			winner := pickLatestSemver(bucket)
			out := []ToolPoolEntry{winner.entry}
			var dropped []ToolPoolEntry
			for _, p := range bucket {
				if !sameEntry(p.entry, winner.entry) {
					dropped = append(dropped, p.entry)
				}
			}
			kept = append(kept, out...)
			results = append(results, DedupResult{
				Canonical: key,
				Versions:  uniqueVersions(bucket),
				Kept:      out,
				Dropped:   dropped,
				Reason:    fmt.Sprintf("policy prefer_latest_version: kept %s", winner.version),
			})

		case ToolDedupPreferPinned:
			pin := cfg.pinFor(key)
			var winner dedupParsed
			matchedPin := false
			if pin != "" {
				for _, p := range bucket {
					if p.version == pin {
						winner = p
						matchedPin = true
						break
					}
				}
			}
			if !matchedPin {
				winner = pickBySourceRank(bucket)
			}
			out := []ToolPoolEntry{winner.entry}
			var dropped []ToolPoolEntry
			for _, p := range bucket {
				if !sameEntry(p.entry, winner.entry) {
					dropped = append(dropped, p.entry)
				}
			}
			kept = append(kept, out...)
			reason := "policy prefer_pinned: source rank fallback"
			if matchedPin {
				reason = fmt.Sprintf("policy prefer_pinned: matched pin %s@%s", key, pin)
			}
			results = append(results, DedupResult{
				Canonical: key,
				Versions:  uniqueVersions(bucket),
				Kept:      out,
				Dropped:   dropped,
				Reason:    reason,
			})

		case ToolDedupPreferSourceRank:
			fallthrough
		default:
			winner := pickBySourceRank(bucket)
			out := []ToolPoolEntry{winner.entry}
			var dropped []ToolPoolEntry
			for _, p := range bucket {
				if !sameEntry(p.entry, winner.entry) {
					dropped = append(dropped, p.entry)
				}
			}
			kept = append(kept, out...)
			results = append(results, DedupResult{
				Canonical: key,
				Versions:  uniqueVersions(bucket),
				Kept:      out,
				Dropped:   dropped,
				Reason:    fmt.Sprintf("policy prefer_source_rank: %s won", winner.entry.Source),
			})
		}
	}
	return kept, results, nil
}

func uniqueVersions(bucket []dedupParsed) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, p := range bucket {
		if p.version == "" {
			continue
		}
		if !seen[p.version] {
			seen[p.version] = true
			out = append(out, p.version)
		}
	}
	sort.Slice(out, func(i, j int) bool { return CompareSemver(out[i], out[j]) < 0 })
	return out
}

func hasMultipleVersions(bucket []dedupParsed) bool {
	first := bucket[0].version
	for _, p := range bucket[1:] {
		if p.version != first {
			return true
		}
	}
	return false
}

func pickLatestSemver(bucket []dedupParsed) dedupParsed {
	winner := bucket[0]
	for _, p := range bucket[1:] {
		if CompareSemver(p.version, winner.version) > 0 {
			winner = p
		}
	}
	return winner
}

func pickBySourceRank(bucket []dedupParsed) dedupParsed {
	winner := bucket[0]
	for _, p := range bucket[1:] {
		if toolSourceRank(p.entry.Source) < toolSourceRank(winner.entry.Source) {
			winner = p
		}
	}
	return winner
}

func sameEntry(a, b ToolPoolEntry) bool {
	return a.Name == b.Name && a.Source == b.Source && a.ProviderID == b.ProviderID
}
