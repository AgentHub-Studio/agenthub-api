package agentic

// cross_file_dependency_registry.go - FEAT-049 - Appendix A.3 Cross-File Dependencies
//
// arXiv:2604.14228v1, Appendix A.3 "Cross-File Dependencies" (page 44).
//
// Appendix A.3 describes how the extracted Claude Code TypeScript package keeps
// a small number of important cross-file dependencies explicit:
//
//   - QueryEngine.ts delegates turn execution to query.ts.
//   - query.ts imports services/tools/ for tool execution and orchestration.
//   - query.ts imports services/compact/ for auto compaction.
//   - QueryEngine.ts imports memdir/ for memory and prompt assembly.
//   - types/permissions.ts was extracted to break import cycles.
//   - setCachedClaudeMdContent() in context.ts avoids a cycle through the
//     permissions/filesystem path.
//
// This registry models those edges so AgentHub can reason about package-level
// dependency boundaries without needing to inspect the original TypeScript tree.

// CrossFileDependencyKind classifies why one source unit depends on another.
type CrossFileDependencyKind string

const (
	// CrossFileDependencyKindDelegation means the source delegates runtime work
	// to the target while retaining a wrapper or orchestration role.
	CrossFileDependencyKindDelegation CrossFileDependencyKind = "delegation"

	// CrossFileDependencyKindServiceImport means the source imports a service
	// subsystem to perform concrete runtime work.
	CrossFileDependencyKindServiceImport CrossFileDependencyKind = "service_import"

	// CrossFileDependencyKindMemoryAssembly means the source imports memory or
	// prompt assembly support.
	CrossFileDependencyKindMemoryAssembly CrossFileDependencyKind = "memory_assembly"

	// CrossFileDependencyKindCycleBreaker means the target exists specifically to
	// prevent a circular import path or decouple two subsystems.
	CrossFileDependencyKindCycleBreaker CrossFileDependencyKind = "cycle_breaker"
)

// CrossFileDependencyProfile is one Appendix A.3 import/delegation edge.
type CrossFileDependencyProfile struct {
	// DependencyID is the stable snake_case identifier for the dependency edge.
	DependencyID string

	// Source is the file or directory that owns the dependency.
	Source string

	// Target is the file, directory, or helper symbol that is depended on.
	Target string

	// Kind explains the architectural reason for the dependency.
	Kind CrossFileDependencyKind

	// PDFSection is the paper section that describes this edge.
	PDFSection string

	// Responsibility describes the runtime responsibility carried by the target.
	Responsibility string

	// Evidence states the Appendix A.3 claim this edge encodes.
	Evidence string

	// AvoidedCycle is non-empty only for cycle breakers and names the cycle path
	// the paper says was avoided.
	AvoidedCycle string

	// IsRuntimeCritical is true when the edge sits on the main query execution
	// path rather than on a structural decoupling path.
	IsRuntimeCritical bool

	// IsCycleBreaker is true when this edge is a deliberate acyclic-boundary
	// mechanism rather than a normal runtime dependency.
	IsCycleBreaker bool
}

// CrossFileDependencyRegistry stores the Appendix A.3 dependency edges.
type CrossFileDependencyRegistry struct {
	profiles []CrossFileDependencyProfile
}

// NewCrossFileDependencyRegistry returns the Appendix A.3 dependency registry.
func NewCrossFileDependencyRegistry() *CrossFileDependencyRegistry {
	return &CrossFileDependencyRegistry{
		profiles: seedCrossFileDependencyProfiles(),
	}
}

// FindByID returns the dependency profile with the given ID.
func (r *CrossFileDependencyRegistry) FindByID(id string) (*CrossFileDependencyProfile, bool) {
	for i := range r.profiles {
		if r.profiles[i].DependencyID == id {
			return &r.profiles[i], true
		}
	}
	return nil, false
}

// All returns all dependency profiles in Appendix A.3 narrative order.
func (r *CrossFileDependencyRegistry) All() []CrossFileDependencyProfile {
	out := make([]CrossFileDependencyProfile, len(r.profiles))
	copy(out, r.profiles)
	return out
}

// Count returns the number of registered dependency edges.
func (r *CrossFileDependencyRegistry) Count() int {
	return len(r.profiles)
}

// IsValid returns true when id identifies a registered dependency edge.
func (r *CrossFileDependencyRegistry) IsValid(id string) bool {
	_, ok := r.FindByID(id)
	return ok
}

// DependenciesFrom returns all edges whose source equals source.
func (r *CrossFileDependencyRegistry) DependenciesFrom(source string) []CrossFileDependencyProfile {
	var out []CrossFileDependencyProfile
	for _, p := range r.profiles {
		if p.Source == source {
			out = append(out, p)
		}
	}
	return out
}

// DependenciesTo returns all edges whose target equals target.
func (r *CrossFileDependencyRegistry) DependenciesTo(target string) []CrossFileDependencyProfile {
	var out []CrossFileDependencyProfile
	for _, p := range r.profiles {
		if p.Target == target {
			out = append(out, p)
		}
	}
	return out
}

// DependenciesByKind returns all edges with the requested kind.
func (r *CrossFileDependencyRegistry) DependenciesByKind(kind CrossFileDependencyKind) []CrossFileDependencyProfile {
	var out []CrossFileDependencyProfile
	for _, p := range r.profiles {
		if p.Kind == kind {
			out = append(out, p)
		}
	}
	return out
}

// CycleBreakers returns the edges that deliberately prevent circular imports.
func (r *CrossFileDependencyRegistry) CycleBreakers() []CrossFileDependencyProfile {
	var out []CrossFileDependencyProfile
	for _, p := range r.profiles {
		if p.IsCycleBreaker {
			out = append(out, p)
		}
	}
	return out
}

// RuntimeCriticalDependencies returns edges on the main query execution path.
func (r *CrossFileDependencyRegistry) RuntimeCriticalDependencies() []CrossFileDependencyProfile {
	var out []CrossFileDependencyProfile
	for _, p := range r.profiles {
		if p.IsRuntimeCritical {
			out = append(out, p)
		}
	}
	return out
}

// CoreLoopDependencies returns all dependencies owned by query.ts or QueryEngine.ts.
func (r *CrossFileDependencyRegistry) CoreLoopDependencies() []CrossFileDependencyProfile {
	var out []CrossFileDependencyProfile
	for _, p := range r.profiles {
		if p.Source == "query.ts" || p.Source == "QueryEngine.ts" {
			out = append(out, p)
		}
	}
	return out
}

// DependenciesTouching returns all edges where source or target equals fileOrDir.
func (r *CrossFileDependencyRegistry) DependenciesTouching(fileOrDir string) []CrossFileDependencyProfile {
	var out []CrossFileDependencyProfile
	for _, p := range r.profiles {
		if p.Source == fileOrDir || p.Target == fileOrDir {
			out = append(out, p)
		}
	}
	return out
}

// HasNoSelfDependencies verifies that no dependency edge points to itself.
func (r *CrossFileDependencyRegistry) HasNoSelfDependencies() (bool, []string) {
	var violations []string
	for _, p := range r.profiles {
		if p.Source == p.Target {
			violations = append(violations, p.DependencyID)
		}
	}
	return len(violations) == 0, violations
}

// CycleBreakersHaveAvoidedCycle verifies every cycle breaker explains its cycle.
func (r *CrossFileDependencyRegistry) CycleBreakersHaveAvoidedCycle() (bool, []string) {
	var violations []string
	for _, p := range r.profiles {
		if p.IsCycleBreaker && p.AvoidedCycle == "" {
			violations = append(violations, p.DependencyID)
		}
	}
	return len(violations) == 0, violations
}

// QueryEngineDelegatesToQuery verifies the central Appendix A.3 delegation edge.
func (r *CrossFileDependencyRegistry) QueryEngineDelegatesToQuery() bool {
	p, ok := r.FindByID("query_engine_delegates_query")
	return ok &&
		p.Source == "QueryEngine.ts" &&
		p.Target == "query.ts" &&
		p.Kind == CrossFileDependencyKindDelegation
}

// AllProfilesHavePDFSection verifies every profile cites Appendix A.3.
func (r *CrossFileDependencyRegistry) AllProfilesHavePDFSection() (bool, []string) {
	var violations []string
	for _, p := range r.profiles {
		if p.PDFSection == "" {
			violations = append(violations, p.DependencyID)
		}
	}
	return len(violations) == 0, violations
}

// SeedCrossFileDependencyCount is the Appendix A.3 edge count modeled here.
const SeedCrossFileDependencyCount = 6

// SeedCrossFileDependencyIDs lists all dependency IDs in Appendix A.3 order.
var SeedCrossFileDependencyIDs = []string{
	"query_engine_delegates_query",
	"query_imports_tool_services",
	"query_imports_compact_services",
	"query_engine_imports_memdir",
	"permission_types_break_import_cycle",
	"context_cached_claudemd_breaks_cycle",
}

func seedCrossFileDependencyProfiles() []CrossFileDependencyProfile {
	return []CrossFileDependencyProfile{
		{
			DependencyID:      "query_engine_delegates_query",
			Source:            "QueryEngine.ts",
			Target:            "query.ts",
			Kind:              CrossFileDependencyKindDelegation,
			PDFSection:        "Appendix A.3",
			Responsibility:    "Turn execution is delegated to the shared agentic query loop.",
			Evidence:          "QueryEngine.ts delegates to query.ts for turn execution.",
			IsRuntimeCritical: true,
			IsCycleBreaker:    false,
		},
		{
			DependencyID:      "query_imports_tool_services",
			Source:            "query.ts",
			Target:            "services/tools/",
			Kind:              CrossFileDependencyKindServiceImport,
			PDFSection:        "Appendix A.3",
			Responsibility:    "StreamingToolExecutor, tool orchestration, and tool execution services.",
			Evidence:          "query.ts imports from services/tools/ for StreamingToolExecutor and runTools.",
			IsRuntimeCritical: true,
			IsCycleBreaker:    false,
		},
		{
			DependencyID:      "query_imports_compact_services",
			Source:            "query.ts",
			Target:            "services/compact/",
			Kind:              CrossFileDependencyKindServiceImport,
			PDFSection:        "Appendix A.3",
			Responsibility:    "Automatic compaction and post-compact message rebuilding.",
			Evidence:          "query.ts imports autoCompact and buildPostCompactMessages from services/compact/.",
			IsRuntimeCritical: true,
			IsCycleBreaker:    false,
		},
		{
			DependencyID:      "query_engine_imports_memdir",
			Source:            "QueryEngine.ts",
			Target:            "memdir/",
			Kind:              CrossFileDependencyKindMemoryAssembly,
			PDFSection:        "Appendix A.3",
			Responsibility:    "Memory and prompt assembly support for headless and SDK conversations.",
			Evidence:          "QueryEngine.ts imports from memdir/ for memory and prompt assembly.",
			IsRuntimeCritical: true,
			IsCycleBreaker:    false,
		},
		{
			DependencyID:      "permission_types_break_import_cycle",
			Source:            "utils/permissions/",
			Target:            "types/permissions.ts",
			Kind:              CrossFileDependencyKindCycleBreaker,
			PDFSection:        "Appendix A.3",
			Responsibility:    "Shared permission mode definitions extracted into a neutral types module.",
			Evidence:          "types/permissions.ts was extracted to break import cycles.",
			AvoidedCycle:      "permission rule evaluation <-> permission mode definitions",
			IsRuntimeCritical: false,
			IsCycleBreaker:    true,
		},
		{
			DependencyID:      "context_cached_claudemd_breaks_cycle",
			Source:            "context.ts",
			Target:            "setCachedClaudeMdContent()",
			Kind:              CrossFileDependencyKindCycleBreaker,
			PDFSection:        "Appendix A.3",
			Responsibility:    "Cached CLAUDE.md content setter that prevents a filesystem permission cycle.",
			Evidence:          "setCachedClaudeMdContent() in context.ts avoids a cycle through the permissions/filesystem path.",
			AvoidedCycle:      "context assembly <-> permissions/filesystem path",
			IsRuntimeCritical: false,
			IsCycleBreaker:    true,
		},
	}
}
