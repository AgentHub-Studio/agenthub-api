package agentic

// PackageStructureRegistry maps the TypeScript package layout of Claude Code
// (v2.1.88) to runtime responsibilities as described in Appendix A of the paper.
//
// Two sub-registries are provided:
//   - KeyFileRegistry  — Table 7: key source files by approximate size and responsibility
//   - ConditionalToolRegistry — Table 8: tool availability categories (mode/env/flag/null-check)
//
// PDF reference: Appendix A §A.1 (Table 7) and §A.2 (Table 8).

// ------------------- A.1 Key File Registry -------------------

// KeyFileSize classifies the approximate on-disk size of a source file.
type KeyFileSize string

const (
	// KeyFileSizeSmall represents files ≤30 KB.
	KeyFileSizeSmall KeyFileSize = "small"
	// KeyFileSizeMedium represents files 31–100 KB.
	KeyFileSizeMedium KeyFileSize = "medium"
	// KeyFileSizeLarge represents files that are large (no exact figure given in paper).
	KeyFileSizeLarge KeyFileSize = "large"
)

// KeyFileLayer names the architectural layer a key file belongs to.
type KeyFileLayer string

const (
	KeyFileLayerEntryAndStartup   KeyFileLayer = "entry_and_startup"
	KeyFileLayerCoreLoop          KeyFileLayer = "core_loop"
	KeyFileLayerToolsAndCommands  KeyFileLayer = "tools_and_commands"
	KeyFileLayerContextAndMemory  KeyFileLayer = "context_and_memory"
	KeyFileLayerPersistence       KeyFileLayer = "persistence"
	KeyFileLayerServicesIntegration KeyFileLayer = "services_and_integration"
)

// KeyFileProfile describes a single key source file extracted from the package.
type KeyFileProfile struct {
	// Slug is a stable identifier derived from the file path (dots and slashes replaced with underscores).
	Slug string

	// Label is the file path relative to src/ as shown in Table 7.
	Label string

	// PDFSection is the appendix reference in the paper.
	PDFSection string

	// ApproxSizeKB is the approximate size in KB when stated numerically, or 0 when "Large".
	ApproxSizeKB int

	// SizeCategory is the qualitative size bucket from Table 7.
	SizeCategory KeyFileSize

	// Responsibility is the brief runtime description from Table 7.
	Responsibility string

	// Layer is the architectural grouping from Figure 9.
	Layer KeyFileLayer

	// IsEntryPoint is true when the file is listed as an application entry point.
	IsEntryPoint bool
}

// KeyFileRegistry is the registry of key source files described in Appendix A.1.
type KeyFileRegistry struct {
	profiles []KeyFileProfile
}

// NewKeyFileRegistry returns a registry pre-seeded with the 9 files from Table 7.
func NewKeyFileRegistry() *KeyFileRegistry {
	return &KeyFileRegistry{
		profiles: []KeyFileProfile{
			{
				Slug:           "main_tsx",
				Label:          "main.tsx",
				PDFSection:     "Appendix A.1 Table 7",
				ApproxSizeKB:   804,
				SizeCategory:   KeyFileSizeMedium, // ≤1000 KB but large nominal; treated as medium for the enum
				Responsibility: "Entry point, mode dispatch, setup",
				Layer:          KeyFileLayerEntryAndStartup,
				IsEntryPoint:   true,
			},
			{
				Slug:           "query_ts",
				Label:          "query.ts",
				PDFSection:     "Appendix A.1 Table 7",
				ApproxSizeKB:   68,
				SizeCategory:   KeyFileSizeMedium,
				Responsibility: "Core agent loop, 5 context shapers",
				Layer:          KeyFileLayerCoreLoop,
				IsEntryPoint:   false,
			},
			{
				Slug:           "QueryEngine_ts",
				Label:          "QueryEngine.ts",
				PDFSection:     "Appendix A.1 Table 7",
				ApproxSizeKB:   47,
				SizeCategory:   KeyFileSizeMedium,
				Responsibility: "SDK/headless conversation wrapper",
				Layer:          KeyFileLayerCoreLoop,
				IsEntryPoint:   false,
			},
			{
				Slug:           "Tool_ts",
				Label:          "Tool.ts",
				PDFSection:     "Appendix A.1 Table 7",
				ApproxSizeKB:   30,
				SizeCategory:   KeyFileSizeSmall,
				Responsibility: "Tool interface, types, utilities",
				Layer:          KeyFileLayerToolsAndCommands,
				IsEntryPoint:   false,
			},
			{
				Slug:           "history_ts",
				Label:          "history.ts",
				PDFSection:     "Appendix A.1 Table 7",
				ApproxSizeKB:   14,
				SizeCategory:   KeyFileSizeSmall,
				Responsibility: "Global prompt history",
				Layer:          KeyFileLayerPersistence,
				IsEntryPoint:   false,
			},
			{
				Slug:           "mcp_client_ts",
				Label:          "mcp/client.ts",
				PDFSection:     "Appendix A.1 Table 7",
				ApproxSizeKB:   0,
				SizeCategory:   KeyFileSizeLarge,
				Responsibility: "MCP client (8+ transport variants)",
				Layer:          KeyFileLayerServicesIntegration,
				IsEntryPoint:   false,
			},
			{
				Slug:           "compact_ts",
				Label:          "compact.ts",
				PDFSection:     "Appendix A.1 Table 7",
				ApproxSizeKB:   0,
				SizeCategory:   KeyFileSizeLarge,
				Responsibility: "Compaction engine",
				Layer:          KeyFileLayerContextAndMemory,
				IsEntryPoint:   false,
			},
			{
				Slug:           "AgentTool_tsx",
				Label:          "AgentTool.tsx",
				PDFSection:     "Appendix A.1 Table 7",
				ApproxSizeKB:   0,
				SizeCategory:   KeyFileSizeLarge,
				Responsibility: "Agent tool, subagent dispatch",
				Layer:          KeyFileLayerToolsAndCommands,
				IsEntryPoint:   false,
			},
			{
				Slug:           "runAgent_ts",
				Label:          "runAgent.ts",
				PDFSection:     "Appendix A.1 Table 7",
				ApproxSizeKB:   0,
				SizeCategory:   KeyFileSizeLarge,
				Responsibility: "21-parameter agent lifecycle",
				Layer:          KeyFileLayerEntryAndStartup,
				IsEntryPoint:   false,
			},
		},
	}
}

// FindKeyFileBySlug returns the profile for the given slug, or false if not found.
func (r *KeyFileRegistry) FindKeyFileBySlug(slug string) (*KeyFileProfile, bool) {
	for i := range r.profiles {
		if r.profiles[i].Slug == slug {
			return &r.profiles[i], true
		}
	}
	return nil, false
}

// AllKeyFiles returns all key file profiles in Table 7 order.
func (r *KeyFileRegistry) AllKeyFiles() []KeyFileProfile {
	out := make([]KeyFileProfile, len(r.profiles))
	copy(out, r.profiles)
	return out
}

// ByLayer returns all key files belonging to the given architectural layer.
func (r *KeyFileRegistry) ByLayer(layer KeyFileLayer) []KeyFileProfile {
	var out []KeyFileProfile
	for _, p := range r.profiles {
		if p.Layer == layer {
			out = append(out, p)
		}
	}
	return out
}

// EntryPoints returns all files marked as entry points.
func (r *KeyFileRegistry) EntryPoints() []KeyFileProfile {
	var out []KeyFileProfile
	for _, p := range r.profiles {
		if p.IsEntryPoint {
			out = append(out, p)
		}
	}
	return out
}

// BySizeCategory returns all key files with the given size category.
func (r *KeyFileRegistry) BySizeCategory(size KeyFileSize) []KeyFileProfile {
	var out []KeyFileProfile
	for _, p := range r.profiles {
		if p.SizeCategory == size {
			out = append(out, p)
		}
	}
	return out
}

// SeedKeyFileCount is the number of profiles seeded into the registry (Table 7).
const SeedKeyFileCount = 9

// SeedKeyFileSlugs lists all slugs in Table 7 order.
var SeedKeyFileSlugs = []string{
	"main_tsx",
	"query_ts",
	"QueryEngine_ts",
	"Tool_ts",
	"history_ts",
	"mcp_client_ts",
	"compact_ts",
	"AgentTool_tsx",
	"runAgent_ts",
}

// ------------------- A.2 Conditional Tool Registry -------------------

// ToolAvailabilityCategory classifies when/why a tool is included in the tool set.
type ToolAvailabilityCategory string

const (
	// ToolAvailAlwaysIncluded — tools always present regardless of mode or environment.
	ToolAvailAlwaysIncluded ToolAvailabilityCategory = "always_included"
	// ToolAvailEnvironment — tools included only under specific runtime environments (OS, embedded flag).
	ToolAvailEnvironment ToolAvailabilityCategory = "environment"
	// ToolAvailFeatureFlag — tools gated by a compile-time or runtime feature flag.
	ToolAvailFeatureFlag ToolAvailabilityCategory = "feature_flag"
	// ToolAvailNullChecked — tools included only when a backing capability is non-nil at startup.
	ToolAvailNullChecked ToolAvailabilityCategory = "null_checked"
)

// ConditionalToolProfile describes one example tool from Table 8 and its availability condition.
type ConditionalToolProfile struct {
	// Slug is a stable, lower-kebab-case identifier.
	Slug string

	// Label is the tool name as written in Table 8.
	Label string

	// PDFSection is the appendix reference.
	PDFSection string

	// Category is the availability bucket from Table 8.
	Category ToolAvailabilityCategory

	// InclusionCondition describes the specific guard condition (feature flag name, OS, etc.).
	InclusionCondition string

	// MinToolSetSize is the minimum tool count visible in the smallest build (3 in simple mode).
	// Only meaningful for always-included tools; 0 otherwise.
	MinToolSetSize int
}

// ConditionalToolRegistry is the registry of tool availability categories from Appendix A.2.
type ConditionalToolRegistry struct {
	profiles []ConditionalToolProfile
}

// NewConditionalToolRegistry returns a registry pre-seeded with the examples from Table 8.
func NewConditionalToolRegistry() *ConditionalToolRegistry {
	return &ConditionalToolRegistry{
		profiles: []ConditionalToolProfile{
			// Always included
			{
				Slug:               "agent-tool",
				Label:              "AgentTool",
				PDFSection:         "Appendix A.2 Table 8",
				Category:           ToolAvailAlwaysIncluded,
				InclusionCondition: "unconditional — always in tool set",
				MinToolSetSize:     3,
			},
			{
				Slug:               "bash-tool",
				Label:              "BashTool",
				PDFSection:         "Appendix A.2 Table 8",
				Category:           ToolAvailAlwaysIncluded,
				InclusionCondition: "unconditional — always in tool set",
				MinToolSetSize:     3,
			},
			{
				Slug:               "file-read-tool",
				Label:              "FileReadTool",
				PDFSection:         "Appendix A.2 Table 8",
				Category:           ToolAvailAlwaysIncluded,
				InclusionCondition: "unconditional — always in tool set",
				MinToolSetSize:     3,
			},
			{
				Slug:               "file-edit-tool",
				Label:              "FileEditTool",
				PDFSection:         "Appendix A.2 Table 8",
				Category:           ToolAvailAlwaysIncluded,
				InclusionCondition: "unconditional — always in tool set",
				MinToolSetSize:     0,
			},
			{
				Slug:               "file-write-tool",
				Label:              "FileWriteTool",
				PDFSection:         "Appendix A.2 Table 8",
				Category:           ToolAvailAlwaysIncluded,
				InclusionCondition: "unconditional — always in tool set",
				MinToolSetSize:     0,
			},
			{
				Slug:               "skill-tool",
				Label:              "SkillTool",
				PDFSection:         "Appendix A.2 Table 8",
				Category:           ToolAvailAlwaysIncluded,
				InclusionCondition: "unconditional — always in tool set",
				MinToolSetSize:     0,
			},
			{
				Slug:               "web-fetch-tool",
				Label:              "WebFetchTool",
				PDFSection:         "Appendix A.2 Table 8",
				Category:           ToolAvailAlwaysIncluded,
				InclusionCondition: "unconditional — always in tool set",
				MinToolSetSize:     0,
			},
			{
				Slug:               "web-search-tool",
				Label:              "WebSearchTool",
				PDFSection:         "Appendix A.2 Table 8",
				Category:           ToolAvailAlwaysIncluded,
				InclusionCondition: "unconditional — always in tool set",
				MinToolSetSize:     0,
			},
			// Environment-gated
			{
				Slug:               "glob-tool",
				Label:              "GlobTool/GrepTool",
				PDFSection:         "Appendix A.2 Table 8",
				Category:           ToolAvailEnvironment,
				InclusionCondition: "excluded when embedded=true",
				MinToolSetSize:     0,
			},
			{
				Slug:               "config-tool",
				Label:              "ConfigTool",
				PDFSection:         "Appendix A.2 Table 8",
				Category:           ToolAvailEnvironment,
				InclusionCondition: "ant-only (internal Anthropic build)",
				MinToolSetSize:     0,
			},
			{
				Slug:               "powershell-tool",
				Label:              "PowerShellTool",
				PDFSection:         "Appendix A.2 Table 8",
				Category:           ToolAvailEnvironment,
				InclusionCondition: "Windows platform only",
				MinToolSetSize:     0,
			},
			// Feature-flag gated
			{
				Slug:               "task-create-tool",
				Label:              "TaskCreate/Get/Update/List",
				PDFSection:         "Appendix A.2 Table 8",
				Category:           ToolAvailFeatureFlag,
				InclusionCondition: "todoV2 feature flag",
				MinToolSetSize:     0,
			},
			{
				Slug:               "enter-worktree-tool",
				Label:              "EnterWorktreeTool",
				PDFSection:         "Appendix A.2 Table 8",
				Category:           ToolAvailFeatureFlag,
				InclusionCondition: "worktree feature flag",
				MinToolSetSize:     0,
			},
			{
				Slug:               "team-tools",
				Label:              "TeamTools",
				PDFSection:         "Appendix A.2 Table 8",
				Category:           ToolAvailFeatureFlag,
				InclusionCondition: "swarms feature flag",
				MinToolSetSize:     0,
			},
			{
				Slug:               "tool-search-tool",
				Label:              "ToolSearchTool",
				PDFSection:         "Appendix A.2 Table 8",
				Category:           ToolAvailFeatureFlag,
				InclusionCondition: "deferred tool search feature flag",
				MinToolSetSize:     0,
			},
			// Null-checked
			{
				Slug:               "suggest-background-pr-tool",
				Label:              "SuggestBackgroundPRTool",
				PDFSection:         "Appendix A.2 Table 8",
				Category:           ToolAvailNullChecked,
				InclusionCondition: "non-nil background PR capability at startup",
				MinToolSetSize:     0,
			},
			{
				Slug:               "web-browser-tool",
				Label:              "WebBrowserTool",
				PDFSection:         "Appendix A.2 Table 8",
				Category:           ToolAvailNullChecked,
				InclusionCondition: "non-nil browser driver at startup",
				MinToolSetSize:     0,
			},
			{
				Slug:               "remote-trigger-tool",
				Label:              "RemoteTriggerTool",
				PDFSection:         "Appendix A.2 Table 8",
				Category:           ToolAvailNullChecked,
				InclusionCondition: "non-nil remote trigger backend at startup",
				MinToolSetSize:     0,
			},
			{
				Slug:               "monitor-tool",
				Label:              "MonitorTool",
				PDFSection:         "Appendix A.2 Table 8",
				Category:           ToolAvailNullChecked,
				InclusionCondition: "non-nil monitor capability at startup",
				MinToolSetSize:     0,
			},
			{
				Slug:               "sleep-tool",
				Label:              "SleepTool",
				PDFSection:         "Appendix A.2 Table 8",
				Category:           ToolAvailNullChecked,
				InclusionCondition: "non-nil sleep capability at startup",
				MinToolSetSize:     0,
			},
		},
	}
}

// FindConditionalToolBySlug returns the profile for the given slug, or false if not found.
func (r *ConditionalToolRegistry) FindConditionalToolBySlug(slug string) (*ConditionalToolProfile, bool) {
	for i := range r.profiles {
		if r.profiles[i].Slug == slug {
			return &r.profiles[i], true
		}
	}
	return nil, false
}

// AllConditionalTools returns all conditional tool profiles.
func (r *ConditionalToolRegistry) AllConditionalTools() []ConditionalToolProfile {
	out := make([]ConditionalToolProfile, len(r.profiles))
	copy(out, r.profiles)
	return out
}

// ByCategory returns all tool profiles in the given availability category.
func (r *ConditionalToolRegistry) ByCategory(cat ToolAvailabilityCategory) []ConditionalToolProfile {
	var out []ConditionalToolProfile
	for _, p := range r.profiles {
		if p.Category == cat {
			out = append(out, p)
		}
	}
	return out
}

// AlwaysIncluded is a convenience wrapper returning always-included tools.
func (r *ConditionalToolRegistry) AlwaysIncluded() []ConditionalToolProfile {
	return r.ByCategory(ToolAvailAlwaysIncluded)
}

// EnvironmentGated is a convenience wrapper returning environment-gated tools.
func (r *ConditionalToolRegistry) EnvironmentGated() []ConditionalToolProfile {
	return r.ByCategory(ToolAvailEnvironment)
}

// FeatureFlagGated is a convenience wrapper returning feature-flag-gated tools.
func (r *ConditionalToolRegistry) FeatureFlagGated() []ConditionalToolProfile {
	return r.ByCategory(ToolAvailFeatureFlag)
}

// NullChecked is a convenience wrapper returning null-checked tools.
func (r *ConditionalToolRegistry) NullChecked() []ConditionalToolProfile {
	return r.ByCategory(ToolAvailNullChecked)
}

// SeedConditionalToolCount is the number of tool profiles seeded into the registry (Table 8 examples).
const SeedConditionalToolCount = 20

// SeedConditionalToolSlugs lists all slugs from Table 8.
var SeedConditionalToolSlugs = []string{
	"agent-tool",
	"bash-tool",
	"file-read-tool",
	"file-edit-tool",
	"file-write-tool",
	"skill-tool",
	"web-fetch-tool",
	"web-search-tool",
	"glob-tool",
	"config-tool",
	"powershell-tool",
	"task-create-tool",
	"enter-worktree-tool",
	"team-tools",
	"tool-search-tool",
	"suggest-background-pr-tool",
	"web-browser-tool",
	"remote-trigger-tool",
	"monitor-tool",
	"sleep-tool",
}
