package agentic

// ToolPoolAssemblyStep identifies one of the five steps in the tool pool assembly pipeline.
// §6.2: "The assembleToolPool() function … is the single source of truth for combining
// built-in tools with MCP tools. The assembly follows a five-step pipeline."
type ToolPoolAssemblyStep string

const (
	// ToolPoolStepBaseEnumeration collects the 19 core built-in tools plus up to 35 optional
	// built-in tools from the global registry, yielding up to 54 candidates.
	ToolPoolStepBaseEnumeration ToolPoolAssemblyStep = "base_tool_enumeration"
	// ToolPoolStepModeFiltering removes tools that are not applicable to the current operating
	// mode (e.g., worktree tools are excluded in non-CLI environments).
	ToolPoolStepModeFiltering ToolPoolAssemblyStep = "mode_filtering"
	// ToolPoolStepDenyRulePrefiltering applies permission-layer deny rules before MCP tools
	// are merged, preventing denied built-ins from polluting the dedup namespace.
	ToolPoolStepDenyRulePrefiltering ToolPoolAssemblyStep = "deny_rule_prefiltering"
	// ToolPoolStepMCPIntegration merges tools advertised by enabled MCP servers into the pool.
	// MCP tools are added after built-in filtering to ensure built-in precedence on name collisions.
	ToolPoolStepMCPIntegration ToolPoolAssemblyStep = "mcp_tool_integration"
	// ToolPoolStepDeduplication enforces name-uniqueness across all sources using the
	// deterministic precedence ladder: builtin > skill > mcp > subagent > extension.
	ToolPoolStepDeduplication ToolPoolAssemblyStep = "deduplication"
)

// ToolPoolAssemblyStepProfile holds the immutable characteristics of one assembly step.
type ToolPoolAssemblyStepProfile struct {
	Step            ToolPoolAssemblyStep
	StepOrder       int  // 1–5, canonical execution order per §6.2
	IsAlwaysActive  bool // true = runs unconditionally; false = skipped when the relevant source is absent
	CanFilterTools  bool // true = this step may remove tools from the pool
	AlwaysPrecedesMCP bool // true = step is guaranteed to complete before MCP tools are merged
}

var toolPoolAssemblyStepProfiles = map[ToolPoolAssemblyStep]ToolPoolAssemblyStepProfile{
	ToolPoolStepBaseEnumeration: {
		Step: ToolPoolStepBaseEnumeration, StepOrder: 1,
		IsAlwaysActive: true, CanFilterTools: false, AlwaysPrecedesMCP: true,
	},
	ToolPoolStepModeFiltering: {
		Step: ToolPoolStepModeFiltering, StepOrder: 2,
		IsAlwaysActive: true, CanFilterTools: true, AlwaysPrecedesMCP: true,
	},
	ToolPoolStepDenyRulePrefiltering: {
		Step: ToolPoolStepDenyRulePrefiltering, StepOrder: 3,
		IsAlwaysActive: true, CanFilterTools: true, AlwaysPrecedesMCP: true,
	},
	ToolPoolStepMCPIntegration: {
		Step: ToolPoolStepMCPIntegration, StepOrder: 4,
		IsAlwaysActive: false, CanFilterTools: false, AlwaysPrecedesMCP: false,
	},
	ToolPoolStepDeduplication: {
		Step: ToolPoolStepDeduplication, StepOrder: 5,
		IsAlwaysActive: true, CanFilterTools: true, AlwaysPrecedesMCP: false,
	},
}

// ToolPoolAssemblySequence is the canonical ordered slice of all 5 assembly steps per §6.2.
var ToolPoolAssemblySequence = []ToolPoolAssemblyStep{
	ToolPoolStepBaseEnumeration,
	ToolPoolStepModeFiltering,
	ToolPoolStepDenyRulePrefiltering,
	ToolPoolStepMCPIntegration,
	ToolPoolStepDeduplication,
}

// ToolPoolAssemblyRegistry provides queries over the §6.2 five-step tool pool assembly pipeline.
type ToolPoolAssemblyRegistry struct{}

// NewToolPoolAssemblyRegistry returns a ready-to-use registry.
func NewToolPoolAssemblyRegistry() *ToolPoolAssemblyRegistry {
	return &ToolPoolAssemblyRegistry{}
}

// Profile returns the immutable profile for the given step.
// Returns false if the step is unknown.
func (r *ToolPoolAssemblyRegistry) Profile(step ToolPoolAssemblyStep) (ToolPoolAssemblyStepProfile, bool) {
	p, ok := toolPoolAssemblyStepProfiles[step]
	return p, ok
}

// AllSteps returns all five steps in execution order as a defensive copy.
func (r *ToolPoolAssemblyRegistry) AllSteps() []ToolPoolAssemblyStep {
	result := make([]ToolPoolAssemblyStep, len(ToolPoolAssemblySequence))
	copy(result, ToolPoolAssemblySequence)
	return result
}

// FilteringSteps returns all steps that may remove tools from the pool, in execution order.
func (r *ToolPoolAssemblyRegistry) FilteringSteps() []ToolPoolAssemblyStep {
	var result []ToolPoolAssemblyStep
	for _, s := range ToolPoolAssemblySequence {
		if toolPoolAssemblyStepProfiles[s].CanFilterTools {
			result = append(result, s)
		}
	}
	return result
}

// PreMCPSteps returns all steps guaranteed to complete before MCP tool integration.
func (r *ToolPoolAssemblyRegistry) PreMCPSteps() []ToolPoolAssemblyStep {
	var result []ToolPoolAssemblyStep
	for _, s := range ToolPoolAssemblySequence {
		if toolPoolAssemblyStepProfiles[s].AlwaysPrecedesMCP {
			result = append(result, s)
		}
	}
	return result
}

// AlwaysActiveSteps returns all steps that run regardless of configuration.
func (r *ToolPoolAssemblyRegistry) AlwaysActiveSteps() []ToolPoolAssemblyStep {
	var result []ToolPoolAssemblyStep
	for _, s := range ToolPoolAssemblySequence {
		if toolPoolAssemblyStepProfiles[s].IsAlwaysActive {
			result = append(result, s)
		}
	}
	return result
}

// IsToolPoolAssemblySequentiallyOrdered validates that ToolPoolAssemblySequence has step_orders 1..5 in order.
func IsToolPoolAssemblySequentiallyOrdered() bool {
	for i, step := range ToolPoolAssemblySequence {
		if toolPoolAssemblyStepProfiles[step].StepOrder != i+1 {
			return false
		}
	}
	return true
}
