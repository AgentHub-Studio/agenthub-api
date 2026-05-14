package agentic

// AgentQueryPipelineStep identifies one of the nine fixed steps in the agentic query loop.
// §4.1: "Each turn follows a fixed sequence (Figure 2, query.ts)."
type AgentQueryPipelineStep string

const (
	// QueryStepSettingsResolution destructures immutable parameters (system prompt, permissions, model config).
	QueryStepSettingsResolution AgentQueryPipelineStep = "settings_resolution"
	// QueryStepMutableStateInit creates a single State object for messages, tool context, recovery counters.
	QueryStepMutableStateInit AgentQueryPipelineStep = "mutable_state_init"
	// QueryStepContextAssembly retrieves messages via getMessagesAfterCompactBoundary().
	QueryStepContextAssembly AgentQueryPipelineStep = "context_assembly"
	// QueryStepPreModelShapers executes the five sequential context shapers (budget, snip, micro, collapse, auto).
	QueryStepPreModelShapers AgentQueryPipelineStep = "pre_model_shapers"
	// QueryStepModelCall streams the model response via deps.callModel() with full context.
	QueryStepModelCall AgentQueryPipelineStep = "model_call"
	// QueryStepToolUseDispatch routes tool_use blocks to the tool orchestration layer (§4.2).
	QueryStepToolUseDispatch AgentQueryPipelineStep = "tool_use_dispatch"
	// QueryStepPermissionGate evaluates each tool request through the seven-layer permission system (§5).
	QueryStepPermissionGate AgentQueryPipelineStep = "permission_gate"
	// QueryStepToolExecution executes approved tools and adds tool_result messages; loop continues.
	QueryStepToolExecution AgentQueryPipelineStep = "tool_execution"
	// QueryStepStopCondition checks termination: no tool use, maxTurns, overflow, hook, abort.
	QueryStepStopCondition AgentQueryPipelineStep = "stop_condition"
)

// QueryPipelinePhase groups steps by their role in the execution cycle.
type QueryPipelinePhase string

const (
	QueryPhaseSetup       QueryPipelinePhase = "setup"       // steps 1-2: one-time per turn setup
	QueryPhaseContext     QueryPipelinePhase = "context"     // steps 3-4: context preparation before model
	QueryPhaseReasoning   QueryPipelinePhase = "reasoning"   // step 5: LLM inference
	QueryPhaseExecution   QueryPipelinePhase = "execution"   // steps 6-8: tool dispatch, permission, execution
	QueryPhaseTermination QueryPipelinePhase = "termination" // step 9: stop condition evaluation
)

// QueryPipelineStepProfile is the immutable characteristics of one pipeline step.
type QueryPipelineStepProfile struct {
	Step           AgentQueryPipelineStep
	StepOrder      int                // 1–9, canonical execution order per §4.1
	Phase          QueryPipelinePhase
	CanBlock       bool // this step can halt the pipeline (denied permission, context overflow, etc.)
	IsRetryable    bool // recovery mechanism may re-attempt this step (§4.4)
	IsPerIteration bool // true = runs every loop iteration; false = one-time turn setup
}

var queryPipelineStepProfiles = map[AgentQueryPipelineStep]QueryPipelineStepProfile{
	QueryStepSettingsResolution: {
		Step: QueryStepSettingsResolution, StepOrder: 1, Phase: QueryPhaseSetup,
		CanBlock: false, IsRetryable: false, IsPerIteration: false,
	},
	QueryStepMutableStateInit: {
		Step: QueryStepMutableStateInit, StepOrder: 2, Phase: QueryPhaseSetup,
		CanBlock: false, IsRetryable: false, IsPerIteration: false,
	},
	QueryStepContextAssembly: {
		Step: QueryStepContextAssembly, StepOrder: 3, Phase: QueryPhaseContext,
		CanBlock: false, IsRetryable: false, IsPerIteration: true,
	},
	QueryStepPreModelShapers: {
		Step: QueryStepPreModelShapers, StepOrder: 4, Phase: QueryPhaseContext,
		CanBlock: true, IsRetryable: false, IsPerIteration: true,
	},
	QueryStepModelCall: {
		Step: QueryStepModelCall, StepOrder: 5, Phase: QueryPhaseReasoning,
		CanBlock: true, IsRetryable: true, IsPerIteration: true,
	},
	QueryStepToolUseDispatch: {
		Step: QueryStepToolUseDispatch, StepOrder: 6, Phase: QueryPhaseExecution,
		CanBlock: false, IsRetryable: false, IsPerIteration: true,
	},
	QueryStepPermissionGate: {
		Step: QueryStepPermissionGate, StepOrder: 7, Phase: QueryPhaseExecution,
		CanBlock: true, IsRetryable: false, IsPerIteration: true,
	},
	QueryStepToolExecution: {
		Step: QueryStepToolExecution, StepOrder: 8, Phase: QueryPhaseExecution,
		CanBlock: false, IsRetryable: true, IsPerIteration: true,
	},
	QueryStepStopCondition: {
		Step: QueryStepStopCondition, StepOrder: 9, Phase: QueryPhaseTermination,
		CanBlock: true, IsRetryable: false, IsPerIteration: true,
	},
}

// QueryPipelineSequence is the canonical ordered slice of all 9 steps per §4.1.
var QueryPipelineSequence = []AgentQueryPipelineStep{
	QueryStepSettingsResolution,
	QueryStepMutableStateInit,
	QueryStepContextAssembly,
	QueryStepPreModelShapers,
	QueryStepModelCall,
	QueryStepToolUseDispatch,
	QueryStepPermissionGate,
	QueryStepToolExecution,
	QueryStepStopCondition,
}

// AgentQueryPipelineRegistry provides queries over the §4.1 nine-step pipeline.
type AgentQueryPipelineRegistry struct{}

// NewAgentQueryPipelineRegistry returns a ready-to-use registry.
func NewAgentQueryPipelineRegistry() *AgentQueryPipelineRegistry {
	return &AgentQueryPipelineRegistry{}
}

// Profile returns the immutable profile for the given step.
// Returns false if the step is unknown.
func (r *AgentQueryPipelineRegistry) Profile(step AgentQueryPipelineStep) (QueryPipelineStepProfile, bool) {
	p, ok := queryPipelineStepProfiles[step]
	return p, ok
}

// AllSteps returns all nine steps in execution order as a defensive copy.
func (r *AgentQueryPipelineRegistry) AllSteps() []AgentQueryPipelineStep {
	result := make([]AgentQueryPipelineStep, len(QueryPipelineSequence))
	copy(result, QueryPipelineSequence)
	return result
}

// StepsInPhase returns all steps belonging to the given phase, in execution order.
func (r *AgentQueryPipelineRegistry) StepsInPhase(phase QueryPipelinePhase) []AgentQueryPipelineStep {
	var result []AgentQueryPipelineStep
	for _, s := range QueryPipelineSequence {
		if queryPipelineStepProfiles[s].Phase == phase {
			result = append(result, s)
		}
	}
	return result
}

// BlockingSteps returns all steps that can halt the pipeline, in execution order.
func (r *AgentQueryPipelineRegistry) BlockingSteps() []AgentQueryPipelineStep {
	var result []AgentQueryPipelineStep
	for _, s := range QueryPipelineSequence {
		if queryPipelineStepProfiles[s].CanBlock {
			result = append(result, s)
		}
	}
	return result
}

// RetryableSteps returns all steps that may be re-attempted under a recovery mechanism (§4.4).
func (r *AgentQueryPipelineRegistry) RetryableSteps() []AgentQueryPipelineStep {
	var result []AgentQueryPipelineStep
	for _, s := range QueryPipelineSequence {
		if queryPipelineStepProfiles[s].IsRetryable {
			result = append(result, s)
		}
	}
	return result
}

// IsQueryPipelineSequentiallyOrdered validates that QueryPipelineSequence has step_orders 1..9 in order.
// This is a structural invariant of the §4.1 architecture.
func IsQueryPipelineSequentiallyOrdered() bool {
	for i, step := range QueryPipelineSequence {
		if queryPipelineStepProfiles[step].StepOrder != i+1 {
			return false
		}
	}
	return true
}
