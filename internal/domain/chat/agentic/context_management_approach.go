package agentic

// ContextManagementApproach identifies one of the five context-management
// strategies from arXiv:2604.14228v1 Table 6 (§13.2).
type ContextManagementApproach string

const (
	// ContextMgmtSimpleTruncation — drops oldest messages when the window fills.
	// Coarse granularity; loses information at every compaction.
	ContextMgmtSimpleTruncation ContextManagementApproach = "simple_truncation"
	// ContextMgmtSlidingWindow — keeps a fixed-size recent-history window.
	// Medium granularity; preserves recency but discards older turns wholesale.
	ContextMgmtSlidingWindow ContextManagementApproach = "sliding_window"
	// ContextMgmtRAG — retrieves relevant snippets from a persistent store.
	// Fine granularity; loses nothing but increases latency per turn.
	ContextMgmtRAG ContextManagementApproach = "rag"
	// ContextMgmtSingleSummarization — one-pass LLM compression of the conversation.
	// Coarse granularity; fast but lossy and non-reversible.
	ContextMgmtSingleSummarization ContextManagementApproach = "single_summarization"
	// ContextMgmtGraduatedCompaction — multi-layer pipeline with escalating strategies.
	// Very fine granularity; the approach Claude Code uses (§7.3 five-stage pipeline).
	ContextMgmtGraduatedCompaction ContextManagementApproach = "graduated_compaction"
)

// ContextManagementGranularity classifies how precisely a strategy manages context.
type ContextManagementGranularity string

const (
	ContextMgmtGranularityCoarse   ContextManagementGranularity = "coarse"
	ContextMgmtGranularityMedium   ContextManagementGranularity = "medium"
	ContextMgmtGranularityFine     ContextManagementGranularity = "fine"
	ContextMgmtGranularityVeryFine ContextManagementGranularity = "very_fine"
)

// ContextManagementApproachProfile holds Table 6 characteristics for one strategy.
type ContextManagementApproachProfile struct {
	Approach    ContextManagementApproach
	Mechanism   string                       // human-readable description of the mechanism
	Granularity ContextManagementGranularity
	// IsAgentHubApproach flags the strategy AgentHub uses for its agent context pipeline.
	// AgentHub uses graduated_compaction adapted from Claude Code's §7.3 five-stage pipeline.
	IsAgentHubApproach bool
}

var contextManagementApproachProfiles = map[ContextManagementApproach]ContextManagementApproachProfile{
	ContextMgmtSimpleTruncation: {
		Approach:           ContextMgmtSimpleTruncation,
		Mechanism:          "Drop oldest messages",
		Granularity:        ContextMgmtGranularityCoarse,
		IsAgentHubApproach: false,
	},
	ContextMgmtSlidingWindow: {
		Approach:           ContextMgmtSlidingWindow,
		Mechanism:          "Fixed-size recent history",
		Granularity:        ContextMgmtGranularityMedium,
		IsAgentHubApproach: false,
	},
	ContextMgmtRAG: {
		Approach:           ContextMgmtRAG,
		Mechanism:          "Retrieve relevant snippets",
		Granularity:        ContextMgmtGranularityFine,
		IsAgentHubApproach: false,
	},
	ContextMgmtSingleSummarization: {
		Approach:           ContextMgmtSingleSummarization,
		Mechanism:          "One-pass compress",
		Granularity:        ContextMgmtGranularityCoarse,
		IsAgentHubApproach: false,
	},
	ContextMgmtGraduatedCompaction: {
		Approach:           ContextMgmtGraduatedCompaction,
		Mechanism:          "Multi-layer pipeline",
		Granularity:        ContextMgmtGranularityVeryFine,
		IsAgentHubApproach: true,
	},
}

// ContextManagementApproachSequence is the canonical Table 6 row ordering.
var ContextManagementApproachSequence = []ContextManagementApproach{
	ContextMgmtSimpleTruncation,
	ContextMgmtSlidingWindow,
	ContextMgmtRAG,
	ContextMgmtSingleSummarization,
	ContextMgmtGraduatedCompaction,
}

// ContextManagementApproachRegistry provides structured access to the Table 6 taxonomy.
type ContextManagementApproachRegistry struct{}

// NewContextManagementApproachRegistry returns a ready-to-use registry.
func NewContextManagementApproachRegistry() *ContextManagementApproachRegistry {
	return &ContextManagementApproachRegistry{}
}

// Profile returns the Table 6 profile for the given approach. Returns false if unknown.
func (r *ContextManagementApproachRegistry) Profile(a ContextManagementApproach) (ContextManagementApproachProfile, bool) {
	p, ok := contextManagementApproachProfiles[a]
	return p, ok
}

// AllApproaches returns all five profiles in Table 6 row order.
func (r *ContextManagementApproachRegistry) AllApproaches() []ContextManagementApproachProfile {
	result := make([]ContextManagementApproachProfile, len(ContextManagementApproachSequence))
	for i, a := range ContextManagementApproachSequence {
		result[i] = contextManagementApproachProfiles[a]
	}
	return result
}

// ApproachesAtGranularity returns all profiles with the given granularity level.
func (r *ContextManagementApproachRegistry) ApproachesAtGranularity(g ContextManagementGranularity) []ContextManagementApproachProfile {
	var result []ContextManagementApproachProfile
	for _, a := range ContextManagementApproachSequence {
		p := contextManagementApproachProfiles[a]
		if p.Granularity == g {
			result = append(result, p)
		}
	}
	return result
}

// AgentHubApproach returns the approach used by the AgentHub platform.
// Always returns graduated_compaction, which adapts Claude Code's §7.3 pipeline.
func (r *ContextManagementApproachRegistry) AgentHubApproach() ContextManagementApproachProfile {
	return contextManagementApproachProfiles[ContextMgmtGraduatedCompaction]
}

// IsValidApproach returns true if the approach identifier is one of the five Table 6 entries.
func (r *ContextManagementApproachRegistry) IsValidApproach(a ContextManagementApproach) bool {
	_, ok := contextManagementApproachProfiles[a]
	return ok
}
