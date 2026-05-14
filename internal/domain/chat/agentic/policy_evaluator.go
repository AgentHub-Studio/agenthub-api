package agentic

import (
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"sync"
)

// PERM-007 — Policy evaluator/classifier.
//
// PDF arXiv:2604.14228v1 §5.3 (Permission classifier — picks a stance
// when no explicit policy matches). Acts as the fallback decider when
// PERM-001..006 (declarative ACLs) all abstain. The classifier scans
// priority-ordered rules and emits one of: allow / deny / escalate /
// abstain — plus a confidence and a trace for audit.
//
// Distinct from neighbouring abstractions:
//   - PERM-001/002/003 ACL policies = declarative tenant/user/role rules.
//   - PERM-006 PreFilter = stance gate before tool exec.
//   - PERM-010 audit retention = where decisions are stored.
//   - PERM-007 (this) = fallback decision engine for things the
//     declarative ACLs don't cover. Priority-ordered, first-match-wins.

// PolicyEvaluatorDecision bounded enum.
type PolicyEvaluatorDecision string

const (
	PolicyEvaluatorDecisionAllow    PolicyEvaluatorDecision = "allow"
	PolicyEvaluatorDecisionDeny     PolicyEvaluatorDecision = "deny"
	PolicyEvaluatorDecisionEscalate PolicyEvaluatorDecision = "escalate"
	PolicyEvaluatorDecisionAbstain  PolicyEvaluatorDecision = "abstain"
)

var allPolicyEvaluatorDecisions = []PolicyEvaluatorDecision{
	PolicyEvaluatorDecisionAllow,
	PolicyEvaluatorDecisionDeny,
	PolicyEvaluatorDecisionEscalate,
	PolicyEvaluatorDecisionAbstain,
}

// IsValidPolicyEvaluatorDecision returns true for the bounded set.
func IsValidPolicyEvaluatorDecision(d PolicyEvaluatorDecision) bool {
	for _, v := range allPolicyEvaluatorDecisions {
		if d == v {
			return true
		}
	}
	return false
}

// AllPolicyEvaluatorDecisions returns a defensive copy.
func AllPolicyEvaluatorDecisions() []PolicyEvaluatorDecision {
	out := make([]PolicyEvaluatorDecision, len(allPolicyEvaluatorDecisions))
	copy(out, allPolicyEvaluatorDecisions)
	return out
}

// PolicyEvaluatorConfidence bounded enum.
type PolicyEvaluatorConfidence string

const (
	PolicyEvaluatorConfidenceLow    PolicyEvaluatorConfidence = "low"
	PolicyEvaluatorConfidenceMedium PolicyEvaluatorConfidence = "medium"
	PolicyEvaluatorConfidenceHigh   PolicyEvaluatorConfidence = "high"
)

var allPolicyEvaluatorConfidences = []PolicyEvaluatorConfidence{
	PolicyEvaluatorConfidenceLow,
	PolicyEvaluatorConfidenceMedium,
	PolicyEvaluatorConfidenceHigh,
}

// IsValidPolicyEvaluatorConfidence returns true for the bounded set.
func IsValidPolicyEvaluatorConfidence(c PolicyEvaluatorConfidence) bool {
	for _, v := range allPolicyEvaluatorConfidences {
		if c == v {
			return true
		}
	}
	return false
}

// AllPolicyEvaluatorConfidences returns a defensive copy.
func AllPolicyEvaluatorConfidences() []PolicyEvaluatorConfidence {
	out := make([]PolicyEvaluatorConfidence, len(allPolicyEvaluatorConfidences))
	copy(out, allPolicyEvaluatorConfidences)
	return out
}

// PolicyEvaluatorRule is one priority-ordered fallback rule. The first
// rule (sorted by Priority ascending) whose ToolNamePattern matches
// wins; the classifier emits its Decision + Confidence.
type PolicyEvaluatorRule struct {
	RuleID          string
	ToolNamePattern string // regex
	Description     string
	Decision        PolicyEvaluatorDecision
	Confidence      PolicyEvaluatorConfidence
	Priority        int // lower fires first
	compiled        *regexp.Regexp
}

var policyEvaluatorRuleIDRE = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*[a-z0-9]$`)

// Validate enforces invariants. Compiles ToolNamePattern.
func (r *PolicyEvaluatorRule) Validate() error {
	if !policyEvaluatorRuleIDRE.MatchString(r.RuleID) {
		return fmt.Errorf("%w: %q must be kebab-case", ErrPolicyEvaluatorBadRuleID, r.RuleID)
	}
	if strings.TrimSpace(r.ToolNamePattern) == "" {
		return ErrPolicyEvaluatorEmptyPattern
	}
	compiled, err := regexp.Compile(r.ToolNamePattern)
	if err != nil {
		return fmt.Errorf("%w: %q: %v", ErrPolicyEvaluatorBadPattern, r.ToolNamePattern, err)
	}
	r.compiled = compiled
	if !IsValidPolicyEvaluatorDecision(r.Decision) {
		return fmt.Errorf("%w: %q", ErrPolicyEvaluatorBadDecision, r.Decision)
	}
	if !IsValidPolicyEvaluatorConfidence(r.Confidence) {
		return fmt.Errorf("%w: %q", ErrPolicyEvaluatorBadConfidence, r.Confidence)
	}
	if r.Priority < 0 {
		return fmt.Errorf("%w: %d", ErrPolicyEvaluatorBadPriority, r.Priority)
	}
	if strings.TrimSpace(r.Description) == "" {
		return ErrPolicyEvaluatorEmptyDescription
	}
	return nil
}

// PolicyEvaluatorInput describes the tool call being classified.
type PolicyEvaluatorInput struct {
	ToolName    string
	AgentSlug   string
	TenantSlug  string
	ContextHint string // free-form hint string (e.g., "subagent_role=researcher")
}

// PolicyEvaluatorOutput is the classifier's decision plus trace.
type PolicyEvaluatorOutput struct {
	Decision     PolicyEvaluatorDecision
	Confidence   PolicyEvaluatorConfidence
	MatchedRule  string // RuleID of the rule that fired ("" if abstain)
	Rationale    string // human-readable explanation
}

// PolicyEvaluatorClassifier holds priority-ordered rules and answers
// Evaluate(input) → output. Thread-safe.
type PolicyEvaluatorClassifier struct {
	mu    sync.RWMutex
	rules []PolicyEvaluatorRule
}

// NewPolicyEvaluatorClassifier creates an empty classifier.
func NewPolicyEvaluatorClassifier() *PolicyEvaluatorClassifier {
	return &PolicyEvaluatorClassifier{}
}

// AddRule adds a rule. Rejects bad rules and duplicate RuleIDs.
func (c *PolicyEvaluatorClassifier) AddRule(rule PolicyEvaluatorRule) error {
	if err := rule.Validate(); err != nil {
		return err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, existing := range c.rules {
		if existing.RuleID == rule.RuleID {
			return fmt.Errorf("%w: %q", ErrPolicyEvaluatorDuplicateRuleID, rule.RuleID)
		}
	}
	c.rules = append(c.rules, rule)
	sort.Slice(c.rules, func(i, j int) bool {
		if c.rules[i].Priority != c.rules[j].Priority {
			return c.rules[i].Priority < c.rules[j].Priority
		}
		return c.rules[i].RuleID < c.rules[j].RuleID
	})
	return nil
}

// Evaluate runs the priority-ordered rules and returns the first
// matching one. Abstains if no rule matches.
func (c *PolicyEvaluatorClassifier) Evaluate(input PolicyEvaluatorInput) PolicyEvaluatorOutput {
	c.mu.RLock()
	defer c.mu.RUnlock()
	for _, rule := range c.rules {
		if rule.compiled.MatchString(input.ToolName) {
			return PolicyEvaluatorOutput{
				Decision:    rule.Decision,
				Confidence:  rule.Confidence,
				MatchedRule: rule.RuleID,
				Rationale:   rule.Description,
			}
		}
	}
	return PolicyEvaluatorOutput{
		Decision:    PolicyEvaluatorDecisionAbstain,
		Confidence:  PolicyEvaluatorConfidenceLow,
		MatchedRule: "",
		Rationale:   "no rule matched; declarative ACL must decide",
	}
}

// ListRules returns rules in priority order (defensive copy).
func (c *PolicyEvaluatorClassifier) ListRules() []PolicyEvaluatorRule {
	c.mu.RLock()
	defer c.mu.RUnlock()
	out := make([]PolicyEvaluatorRule, len(c.rules))
	copy(out, c.rules)
	return out
}

// Size returns the rule count.
func (c *PolicyEvaluatorClassifier) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.rules)
}

// Sentinel errors.
var (
	ErrPolicyEvaluatorBadRuleID         = errors.New("policy evaluator: rule_id must be kebab-case")
	ErrPolicyEvaluatorEmptyPattern      = errors.New("policy evaluator: tool_name_pattern required")
	ErrPolicyEvaluatorBadPattern        = errors.New("policy evaluator: tool_name_pattern must compile as regex")
	ErrPolicyEvaluatorBadDecision       = errors.New("policy evaluator: invalid decision")
	ErrPolicyEvaluatorBadConfidence     = errors.New("policy evaluator: invalid confidence")
	ErrPolicyEvaluatorBadPriority       = errors.New("policy evaluator: priority must be >= 0")
	ErrPolicyEvaluatorEmptyDescription  = errors.New("policy evaluator: description required")
	ErrPolicyEvaluatorDuplicateRuleID   = errors.New("policy evaluator: duplicate rule_id")
)
