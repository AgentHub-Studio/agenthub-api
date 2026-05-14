package agentic

// ClaudeMDMemoryType identifies one of the four CLAUDE.md instruction-file
// memory types defined in §7.2 of the Claude Code architecture paper.
//
// §7.2: "CLAUDE.md files follow a multi-level loading hierarchy. The source
// header (claudemd.ts) defines four memory types."
//
// Loading order is reverse-priority: later-loaded files receive more model
// attention because they appear closer to the end of the context window.
// Files closer to the current working directory load later and therefore
// have higher priority than files higher in the tree.
type ClaudeMDMemoryType string

const (
	// ClaudeMDMemoryTypeManaged is OS-level policy for all users.
	// Example path: /etc/claude-code/CLAUDE.md (Linux).
	// §7.2: "Managed memory (e.g. /etc/claude-code/CLAUDE.md on Linux):
	// OS-level policy for all users."
	ClaudeMDMemoryTypeManaged ClaudeMDMemoryType = "managed"

	// ClaudeMDMemoryTypeUser is private global instructions per user.
	// Example path: ~/.claude/CLAUDE.md.
	// §7.2: "User memory (~/.claude/CLAUDE.md): private global instructions."
	ClaudeMDMemoryTypeUser ClaudeMDMemoryType = "user"

	// ClaudeMDMemoryTypeProject is instructions checked into the codebase.
	// Covers CLAUDE.md, .claude/CLAUDE.md, and .claude/rules/*.md in project roots.
	// §7.2: "Project memory (CLAUDE.md, .claude/CLAUDE.md, and .claude/rules/*.md
	// in project roots): instructions checked into the codebase."
	ClaudeMDMemoryTypeProject ClaudeMDMemoryType = "project"

	// ClaudeMDMemoryTypeLocal is gitignored private project-specific instructions.
	// Example path: CLAUDE.local.md in project roots.
	// §7.2: "Local memory (CLAUDE.local.md in project roots): gitignored,
	// for private project-specific instructions."
	ClaudeMDMemoryTypeLocal ClaudeMDMemoryType = "local"
)

// claudeMDMemoryTypes is the canonical load order: managed→user→project→local.
// Later entries have higher priority (reverse-order-of-priority principle).
var claudeMDMemoryTypes = []ClaudeMDMemoryType{
	ClaudeMDMemoryTypeManaged,
	ClaudeMDMemoryTypeUser,
	ClaudeMDMemoryTypeProject,
	ClaudeMDMemoryTypeLocal,
}

// ClaudeMDMemoryTypeProfile is the immutable characteristics of one memory type.
type ClaudeMDMemoryTypeProfile struct {
	Type ClaudeMDMemoryType

	// LoadOrder is the canonical position (1=managed, 2=user, 3=project, 4=local).
	// Higher LoadOrder means later loading and therefore higher effective priority.
	LoadOrder int

	// IsGitIgnored indicates the file is intentionally excluded from version control.
	IsGitIgnored bool

	// IsCheckedIn indicates the file is expected to be checked into the repository.
	IsCheckedIn bool

	// IsUserScoped indicates the file applies only to the current OS user.
	IsUserScoped bool

	// IsSystemScoped indicates the file is OS-administered (e.g. managed policy).
	IsSystemScoped bool

	// ExamplePaths lists representative file paths for this memory type.
	ExamplePaths []string

	// LoadStrategy describes how files of this type are discovered.
	// "static" = single fixed path; "discovery" = walk from CWD to root.
	LoadStrategy string
}

var claudeMDMemoryTypeProfiles = map[ClaudeMDMemoryType]ClaudeMDMemoryTypeProfile{
	ClaudeMDMemoryTypeManaged: {
		Type:           ClaudeMDMemoryTypeManaged,
		LoadOrder:      1,
		IsGitIgnored:   false,
		IsCheckedIn:    false,
		IsUserScoped:   false,
		IsSystemScoped: true,
		ExamplePaths:   []string{"/etc/claude-code/CLAUDE.md"},
		LoadStrategy:   "static",
	},
	ClaudeMDMemoryTypeUser: {
		Type:           ClaudeMDMemoryTypeUser,
		LoadOrder:      2,
		IsGitIgnored:   false,
		IsCheckedIn:    false,
		IsUserScoped:   true,
		IsSystemScoped: false,
		ExamplePaths:   []string{"~/.claude/CLAUDE.md"},
		LoadStrategy:   "static",
	},
	ClaudeMDMemoryTypeProject: {
		Type:           ClaudeMDMemoryTypeProject,
		LoadOrder:      3,
		IsGitIgnored:   false,
		IsCheckedIn:    true,
		IsUserScoped:   false,
		IsSystemScoped: false,
		ExamplePaths:   []string{"CLAUDE.md", ".claude/CLAUDE.md", ".claude/rules/*.md"},
		LoadStrategy:   "discovery",
	},
	ClaudeMDMemoryTypeLocal: {
		Type:           ClaudeMDMemoryTypeLocal,
		LoadOrder:      4,
		IsGitIgnored:   true,
		IsCheckedIn:    false,
		IsUserScoped:   true,
		IsSystemScoped: false,
		ExamplePaths:   []string{"CLAUDE.local.md"},
		LoadStrategy:   "discovery",
	},
}

// ClaudeMDMemoryTypeRegistry provides queries over the §7.2 four memory types.
type ClaudeMDMemoryTypeRegistry struct{}

// NewClaudeMDMemoryTypeRegistry returns a ready-to-use registry.
func NewClaudeMDMemoryTypeRegistry() *ClaudeMDMemoryTypeRegistry {
	return &ClaudeMDMemoryTypeRegistry{}
}

// Profile returns the immutable profile for the given memory type.
// Returns (zero-value, false) if the type is unknown.
func (r *ClaudeMDMemoryTypeRegistry) Profile(t ClaudeMDMemoryType) (ClaudeMDMemoryTypeProfile, bool) {
	p, ok := claudeMDMemoryTypeProfiles[t]
	return p, ok
}

// AllTypes returns all four memory types in load order (managed→user→project→local).
// This is a defensive copy.
func (r *ClaudeMDMemoryTypeRegistry) AllTypes() []ClaudeMDMemoryType {
	result := make([]ClaudeMDMemoryType, len(claudeMDMemoryTypes))
	copy(result, claudeMDMemoryTypes)
	return result
}

// EffectivePriorityOrder returns types in effective priority order (highest priority last
// in load order = local→project→user→managed). Files loaded later win model attention.
func (r *ClaudeMDMemoryTypeRegistry) EffectivePriorityOrder() []ClaudeMDMemoryType {
	all := r.AllTypes()
	// Reverse: index 3=local has highest priority, index 0=managed lowest.
	result := make([]ClaudeMDMemoryType, len(all))
	for i, t := range all {
		result[len(all)-1-i] = t
	}
	return result
}

// HighestPriority returns the memory type with the highest effective priority.
// §7.2: "Files closer to the current directory have higher priority (loaded later)."
// Local memory (loaded last) wins.
func (r *ClaudeMDMemoryTypeRegistry) HighestPriority() ClaudeMDMemoryType {
	return ClaudeMDMemoryTypeLocal
}

// LowestPriority returns the memory type with the lowest effective priority.
// Managed memory is loaded first, so it has the least model attention.
func (r *ClaudeMDMemoryTypeRegistry) LowestPriority() ClaudeMDMemoryType {
	return ClaudeMDMemoryTypeManaged
}

// TypesWithStrategy returns memory types that use the given load strategy.
func (r *ClaudeMDMemoryTypeRegistry) TypesWithStrategy(strategy string) []ClaudeMDMemoryType {
	var result []ClaudeMDMemoryType
	for _, t := range claudeMDMemoryTypes {
		if claudeMDMemoryTypeProfiles[t].LoadStrategy == strategy {
			result = append(result, t)
		}
	}
	return result
}

// CheckedInTypes returns types whose files are expected to live in the repository.
func (r *ClaudeMDMemoryTypeRegistry) CheckedInTypes() []ClaudeMDMemoryType {
	var result []ClaudeMDMemoryType
	for _, t := range claudeMDMemoryTypes {
		if claudeMDMemoryTypeProfiles[t].IsCheckedIn {
			result = append(result, t)
		}
	}
	return result
}

// GitIgnoredTypes returns types whose files are intentionally gitignored.
func (r *ClaudeMDMemoryTypeRegistry) GitIgnoredTypes() []ClaudeMDMemoryType {
	var result []ClaudeMDMemoryType
	for _, t := range claudeMDMemoryTypes {
		if claudeMDMemoryTypeProfiles[t].IsGitIgnored {
			result = append(result, t)
		}
	}
	return result
}

// IsValidClaudeMDMemoryType returns true for the four recognized type strings.
func IsValidClaudeMDMemoryType(s ClaudeMDMemoryType) bool {
	_, ok := claudeMDMemoryTypeProfiles[s]
	return ok
}

// ClaudeMDLoadOrderIsAscending validates that claudeMDMemoryTypes has load orders 1..4
// in increasing order. This is a structural invariant of the §7.2 architecture.
func ClaudeMDLoadOrderIsAscending() bool {
	for i, t := range claudeMDMemoryTypes {
		if claudeMDMemoryTypeProfiles[t].LoadOrder != i+1 {
			return false
		}
	}
	return true
}
