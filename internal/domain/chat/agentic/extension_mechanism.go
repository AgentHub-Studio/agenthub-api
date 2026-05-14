package agentic

// ExtensionMechanism identifies one of the four extension mechanisms from
// PDF arXiv:2604.14228v1 Table 2. Each mechanism has a distinct context cost
// and an insertion point in the agent loop (Figure 5).
type ExtensionMechanism string

const (
	// ExtensionMechanismHooks — lifecycle interception + event-driven automation.
	// Zero context cost by default; fires at execute():pre/post tool.
	ExtensionMechanismHooks ExtensionMechanism = "hooks"
	// ExtensionMechanismSkills — domain-specific instructions + meta-tool invocation.
	// Low context cost (descriptions only); fires at assemble():context injection.
	ExtensionMechanismSkills ExtensionMechanism = "skills"
	// ExtensionMechanismPlugins — multi-component packaging and distribution.
	// Medium context cost (varies by included components); fires at all three points.
	ExtensionMechanismPlugins ExtensionMechanism = "plugins"
	// ExtensionMechanismMCPServers — external service integration, multi-transport.
	// High context cost (tool schemas); fires at model():tool pool.
	ExtensionMechanismMCPServers ExtensionMechanism = "mcp_servers"
)

// ExtensionInsertionPoint identifies where in the agent loop an extension mechanism
// applies (Figure 5: assemble(), model(), execute(), or all three).
type ExtensionInsertionPoint string

const (
	// ExtensionInsertionAssemble — assemble() phase; extends what the model sees (context).
	ExtensionInsertionAssemble ExtensionInsertionPoint = "assemble"
	// ExtensionInsertionModel — model() phase; extends what the model can reach (tool pool).
	ExtensionInsertionModel ExtensionInsertionPoint = "model"
	// ExtensionInsertionExecute — execute() phase; controls whether/how an action runs.
	ExtensionInsertionExecute ExtensionInsertionPoint = "execute"
	// ExtensionInsertionAll — applies at all three agent-loop insertion points.
	ExtensionInsertionAll ExtensionInsertionPoint = "all"
)

// ExtensionMechanismProfile holds the Table 2 characteristics of one extension mechanism.
type ExtensionMechanismProfile struct {
	Mechanism              ExtensionMechanism
	UniqueCapability       string                    // one-line description from Table 2
	ContextCostCategory    ExtensionContextCostCategory // zero|small|medium|large per §6.3 ordering
	InsertionPoint         ExtensionInsertionPoint
	IsZeroCostByDefault    bool // true = hooks only; cost only incurred when context injection is opted-in
	CoversAllInsertPoints  bool // true = plugins only; all three loop phases
}

var extensionMechanismProfiles = map[ExtensionMechanism]ExtensionMechanismProfile{
	ExtensionMechanismHooks: {
		Mechanism:           ExtensionMechanismHooks,
		UniqueCapability:    "Lifecycle interception and event-driven automation",
		ContextCostCategory: ExtensionContextCostMicro,
		InsertionPoint:      ExtensionInsertionExecute,
		IsZeroCostByDefault: true,
		CoversAllInsertPoints: false,
	},
	ExtensionMechanismSkills: {
		Mechanism:           ExtensionMechanismSkills,
		UniqueCapability:    "Domain-specific instructions and meta-tool invocation",
		ContextCostCategory: ExtensionContextCostSmall,
		InsertionPoint:      ExtensionInsertionAssemble,
		IsZeroCostByDefault: false,
		CoversAllInsertPoints: false,
	},
	ExtensionMechanismPlugins: {
		Mechanism:           ExtensionMechanismPlugins,
		UniqueCapability:    "Multi-component packaging and distribution",
		ContextCostCategory: ExtensionContextCostMedium,
		InsertionPoint:      ExtensionInsertionAll,
		IsZeroCostByDefault: false,
		CoversAllInsertPoints: true,
	},
	ExtensionMechanismMCPServers: {
		Mechanism:           ExtensionMechanismMCPServers,
		UniqueCapability:    "External service integration with multi-transport support",
		ContextCostCategory: ExtensionContextCostLarge,
		InsertionPoint:      ExtensionInsertionModel,
		IsZeroCostByDefault: false,
		CoversAllInsertPoints: false,
	},
}

// ExtensionMechanismSequence is the canonical ordering by context cost (cheapest first),
// reflecting the "graduated context-cost ordering" described in §6.3.
var ExtensionMechanismSequence = []ExtensionMechanism{
	ExtensionMechanismHooks,
	ExtensionMechanismSkills,
	ExtensionMechanismPlugins,
	ExtensionMechanismMCPServers,
}

// ExtensionMechanismProfileRegistry provides queries over the Table 2 taxonomy.
type ExtensionMechanismProfileRegistry struct{}

// NewExtensionMechanismProfileRegistry returns a ready-to-use registry.
func NewExtensionMechanismProfileRegistry() *ExtensionMechanismProfileRegistry {
	return &ExtensionMechanismProfileRegistry{}
}

// Profile returns the immutable characteristics of the given mechanism.
// Returns false if the mechanism is unknown.
func (r *ExtensionMechanismProfileRegistry) Profile(m ExtensionMechanism) (ExtensionMechanismProfile, bool) {
	p, ok := extensionMechanismProfiles[m]
	return p, ok
}

// AllMechanisms returns all four mechanisms ordered by context cost (cheapest first).
func (r *ExtensionMechanismProfileRegistry) AllMechanisms() []ExtensionMechanism {
	result := make([]ExtensionMechanism, len(ExtensionMechanismSequence))
	copy(result, ExtensionMechanismSequence)
	return result
}

// ZeroCostMechanisms returns mechanisms that are zero-cost by default.
func (r *ExtensionMechanismProfileRegistry) ZeroCostMechanisms() []ExtensionMechanism {
	var result []ExtensionMechanism
	for _, m := range ExtensionMechanismSequence {
		if extensionMechanismProfiles[m].IsZeroCostByDefault {
			result = append(result, m)
		}
	}
	return result
}

// MechanismsAtInsertionPoint returns mechanisms that operate at the given insertion point.
// Plugins (all) are included for every point.
func (r *ExtensionMechanismProfileRegistry) MechanismsAtInsertionPoint(ip ExtensionInsertionPoint) []ExtensionMechanism {
	var result []ExtensionMechanism
	for _, m := range ExtensionMechanismSequence {
		p := extensionMechanismProfiles[m]
		if p.InsertionPoint == ip || p.CoversAllInsertPoints {
			result = append(result, m)
		}
	}
	return result
}

// IsExtensionMechanismSequencedByCost verifies the canonical sequence is cheapest-first.
func IsExtensionMechanismSequencedByCost() bool {
	costs := []ExtensionContextCostCategory{
		ExtensionContextCostMicro, ExtensionContextCostSmall,
		ExtensionContextCostMedium, ExtensionContextCostLarge,
	}
	costRank := map[ExtensionContextCostCategory]int{}
	for i, c := range costs {
		costRank[c] = i
	}
	for i := 1; i < len(ExtensionMechanismSequence); i++ {
		prev := extensionMechanismProfiles[ExtensionMechanismSequence[i-1]].ContextCostCategory
		curr := extensionMechanismProfiles[ExtensionMechanismSequence[i]].ContextCostCategory
		if costRank[prev] > costRank[curr] {
			return false
		}
	}
	return true
}
