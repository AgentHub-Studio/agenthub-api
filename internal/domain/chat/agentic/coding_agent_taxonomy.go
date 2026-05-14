package agentic

// CodingAgentCategory classifies AI coding tools by degree of autonomous action.
// §13.1 Table 5: four categories from inline completion to fully autonomous.
type CodingAgentCategory string

const (
	// AgentCategoryInlineCompletion suggests code fragments in the editor without autonomous action.
	AgentCategoryInlineCompletion CodingAgentCategory = "inline_completion"
	// AgentCategoryChatIntegrated adds conversational interaction and multi-file edits;
	// remains IDE-coupled. Web-first products like AgentHub fall here.
	AgentCategoryChatIntegrated CodingAgentCategory = "chat_integrated"
	// AgentCategoryAgenticCLI operates from the command line with autonomous tool-use loops.
	AgentCategoryAgenticCLI CodingAgentCategory = "agentic_cli"
	// AgentCategoryFullyAutonomous aims for minimal human supervision in sandboxed cloud environments.
	AgentCategoryFullyAutonomous CodingAgentCategory = "fully_autonomous"
)

// AgentExecutionPattern names the primary runtime pattern for each category.
type AgentExecutionPattern string

const (
	AgentExecPatternEditorPlugin      AgentExecutionPattern = "editor_plugin"
	AgentExecPatternIDECoupledProduct AgentExecutionPattern = "ide_coupled_product"
	AgentExecPatternToolUseLoop       AgentExecutionPattern = "tool_use_loop"
	AgentExecPatternSandboxPlanning   AgentExecutionPattern = "sandbox_planning"
)

// AgentIsolationModel names the isolation boundary used by each category.
type AgentIsolationModel string

const (
	AgentIsolationNone            AgentIsolationModel = "none"
	AgentIsolationIDE             AgentIsolationModel = "ide_environment"
	AgentIsolationPermissionGates AgentIsolationModel = "permission_gates"
	AgentIsolationSandbox         AgentIsolationModel = "container_sandbox"
)

// TaxonomyGradientIndex maps each category to an autonomous-action level (0=passive → 3=autonomous).
type TaxonomyGradientIndex int

const (
	TaxonomyGradientPassive    TaxonomyGradientIndex = 0
	TaxonomyGradientInteractive TaxonomyGradientIndex = 1
	TaxonomyGradientAgentic    TaxonomyGradientIndex = 2
	TaxonomyGradientAutonomous TaxonomyGradientIndex = 3
)

// CodingAgentProfile is the immutable characteristics of one taxonomy category.
type CodingAgentProfile struct {
	Category        CodingAgentCategory
	Label           string
	GradientIndex   TaxonomyGradientIndex // 0=passive → 3=autonomous
	ExecutionPattern AgentExecutionPattern
	IsolationModel  AgentIsolationModel
	ExampleSystems  []string
	// IsAgentHubTarget indicates that AgentHub's web platform overlaps this category.
	IsAgentHubTarget bool
}

var codingAgentProfiles = map[CodingAgentCategory]CodingAgentProfile{
	AgentCategoryInlineCompletion: {
		Category:        AgentCategoryInlineCompletion,
		Label:           "Inline Completion",
		GradientIndex:   TaxonomyGradientPassive,
		ExecutionPattern: AgentExecPatternEditorPlugin,
		IsolationModel:  AgentIsolationNone,
		ExampleSystems:  []string{"GitHub Copilot", "Tabnine"},
		IsAgentHubTarget: false,
	},
	AgentCategoryChatIntegrated: {
		Category:        AgentCategoryChatIntegrated,
		Label:           "Chat-Integrated",
		GradientIndex:   TaxonomyGradientInteractive,
		ExecutionPattern: AgentExecPatternIDECoupledProduct,
		IsolationModel:  AgentIsolationIDE,
		ExampleSystems:  []string{"Cursor", "Windsurf", "Cody", "AgentHub"},
		IsAgentHubTarget: true,
	},
	AgentCategoryAgenticCLI: {
		Category:        AgentCategoryAgenticCLI,
		Label:           "Agentic CLI",
		GradientIndex:   TaxonomyGradientAgentic,
		ExecutionPattern: AgentExecPatternToolUseLoop,
		IsolationModel:  AgentIsolationPermissionGates,
		ExampleSystems:  []string{"Claude Code", "Codex CLI", "Aider"},
		IsAgentHubTarget: true,
	},
	AgentCategoryFullyAutonomous: {
		Category:        AgentCategoryFullyAutonomous,
		Label:           "Fully Autonomous",
		GradientIndex:   TaxonomyGradientAutonomous,
		ExecutionPattern: AgentExecPatternSandboxPlanning,
		IsolationModel:  AgentIsolationSandbox,
		ExampleSystems:  []string{"Devin", "SWE-Agent", "OpenHands"},
		IsAgentHubTarget: false,
	},
}

// TaxonomyGradient is the canonical ordered slice from least to most autonomous.
var TaxonomyGradient = []CodingAgentCategory{
	AgentCategoryInlineCompletion,
	AgentCategoryChatIntegrated,
	AgentCategoryAgenticCLI,
	AgentCategoryFullyAutonomous,
}

// CodingAgentTaxonomyRegistry provides queries over the §13.1 taxonomy.
type CodingAgentTaxonomyRegistry struct{}

// NewCodingAgentTaxonomyRegistry returns a ready-to-use registry.
func NewCodingAgentTaxonomyRegistry() *CodingAgentTaxonomyRegistry {
	return &CodingAgentTaxonomyRegistry{}
}

// Profile returns the immutable profile for the given category.
// Returns false if the category is unknown.
func (r *CodingAgentTaxonomyRegistry) Profile(cat CodingAgentCategory) (CodingAgentProfile, bool) {
	p, ok := codingAgentProfiles[cat]
	return p, ok
}

// AllCategories returns all four categories in gradient order (passive → autonomous).
func (r *CodingAgentTaxonomyRegistry) AllCategories() []CodingAgentCategory {
	result := make([]CodingAgentCategory, len(TaxonomyGradient))
	copy(result, TaxonomyGradient)
	return result
}

// AgentHubTargetCategories returns the categories that AgentHub's web platform covers.
// Per §13.1: AgentHub is primarily chat_integrated; background agents push into agentic_cli.
func (r *CodingAgentTaxonomyRegistry) AgentHubTargetCategories() []CodingAgentCategory {
	var result []CodingAgentCategory
	for _, cat := range TaxonomyGradient {
		if codingAgentProfiles[cat].IsAgentHubTarget {
			result = append(result, cat)
		}
	}
	return result
}

// FindByExecutionPattern returns the category matching the given execution pattern.
// Returns false if not found.
func (r *CodingAgentTaxonomyRegistry) FindByExecutionPattern(pattern AgentExecutionPattern) (CodingAgentCategory, bool) {
	for _, cat := range TaxonomyGradient {
		if codingAgentProfiles[cat].ExecutionPattern == pattern {
			return cat, true
		}
	}
	return "", false
}

// FindByIsolationModel returns all categories using the given isolation model.
func (r *CodingAgentTaxonomyRegistry) FindByIsolationModel(model AgentIsolationModel) []CodingAgentCategory {
	var result []CodingAgentCategory
	for _, cat := range TaxonomyGradient {
		if codingAgentProfiles[cat].IsolationModel == model {
			result = append(result, cat)
		}
	}
	return result
}

// IsMoreAutonomousThan returns true when a has a higher gradient index than b.
func (r *CodingAgentTaxonomyRegistry) IsMoreAutonomousThan(a, b CodingAgentCategory) bool {
	pa, aok := codingAgentProfiles[a]
	pb, bok := codingAgentProfiles[b]
	if !aok || !bok {
		return false
	}
	return pa.GradientIndex > pb.GradientIndex
}
