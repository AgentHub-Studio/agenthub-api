package agentic

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
)

// PermissionDecision represents the outcome of evaluating a tool call against permission rules.
type PermissionDecision string

const (
	PermissionAllow   PermissionDecision = "allow"
	PermissionDeny    PermissionDecision = "deny"
	PermissionConfirm PermissionDecision = "confirm"
)

// PermissionMode controls how tool permissions are evaluated.
// Inspired by Claude Code's PermissionMode.ts.
type PermissionMode string

const (
	// PermissionModeDefault prompts for each potentially dangerous operation.
	PermissionModeDefault PermissionMode = "default"
	// PermissionModeAllowEdits auto-allows read-only tools and known-safe tools
	// but still prompts for destructive operations.
	PermissionModeAllowEdits PermissionMode = "allow_edits"
	// PermissionModeBypass auto-allows most operations (some remain immune).
	// Only deny rules and bypass-immune safety checks can block execution.
	PermissionModeBypass PermissionMode = "bypass"
	// PermissionModeDontAsk auto-denies any operation that would normally prompt.
	PermissionModeDontAsk PermissionMode = "dont_ask"
)

// PermissionRules defines the allow/deny/confirm patterns for tool usage.
// JSON shape: {"allow":["tool(pattern)"],"deny":["tool(pattern)"],"confirm":["tool(pattern)"],"mode":"default"}
type PermissionRules struct {
	Allow   []string       `json:"allow,omitempty"`
	Deny    []string       `json:"deny,omitempty"`
	Confirm []string       `json:"confirm,omitempty"`
	Mode    PermissionMode `json:"mode,omitempty"`
}

// ParsePermissionRules parses raw JSON into PermissionRules.
// Returns nil if the input is empty or invalid.
func ParsePermissionRules(raw json.RawMessage) *PermissionRules {
	if len(raw) == 0 {
		return nil
	}
	var rules PermissionRules
	if err := json.Unmarshal(raw, &rules); err != nil {
		return nil
	}
	if len(rules.Allow) == 0 && len(rules.Deny) == 0 && len(rules.Confirm) == 0 {
		return nil
	}
	return &rules
}

// EvaluatePermission checks a tool call against the permission rules and mode.
// Evaluation order: deny (checked first) > confirm > mode-based > allow > default allow.
//
// Pattern syntax: "tool_name" or "tool_name(input_pattern)"
// - "execute-sql" matches any call to execute-sql
// - "execute-sql(SELECT *)" matches calls where input contains "SELECT *"
// - "*" matches all tools
// - "http-*" matches tools starting with "http-"
func EvaluatePermission(rules *PermissionRules, toolName string, toolInput string) PermissionDecision {
	if rules == nil {
		return PermissionAllow
	}

	// Deny rules checked first — highest priority (bypass-immune).
	for _, pattern := range rules.Deny {
		if matchesPermissionPattern(pattern, toolName, toolInput) {
			return PermissionDeny
		}
	}

	// Confirm rules checked next.
	for _, pattern := range rules.Confirm {
		if matchesPermissionPattern(pattern, toolName, toolInput) {
			// In bypass mode, auto-allow even confirm rules.
			if rules.Mode == PermissionModeBypass {
				return PermissionAllow
			}
			// In dont_ask mode, auto-deny instead of prompting.
			if rules.Mode == PermissionModeDontAsk {
				return PermissionDeny
			}
			return PermissionConfirm
		}
	}

	// Allow rules — if any are defined, only matching tools are allowed.
	if len(rules.Allow) > 0 {
		for _, pattern := range rules.Allow {
			if matchesPermissionPattern(pattern, toolName, toolInput) {
				return PermissionAllow
			}
		}
		// If allow rules exist but none matched, check mode.
		if rules.Mode == PermissionModeBypass {
			return PermissionAllow
		}
		return PermissionDeny
	}

	// No allow rules defined = everything is allowed by default.
	return PermissionAllow
}

// matchesPermissionPattern checks if a tool call matches a permission pattern.
// Pattern: "tool_pattern" or "tool_pattern(input_pattern)"
func matchesPermissionPattern(pattern, toolName, toolInput string) bool {
	toolPattern, inputPattern := parsePattern(pattern)

	// Match tool name with glob.
	matched, err := filepath.Match(toolPattern, toolName)
	if err != nil || !matched {
		return false
	}

	// If no input pattern specified, tool name match is sufficient.
	if inputPattern == "" {
		return true
	}

	// Match input pattern as substring (case-insensitive).
	return strings.Contains(
		strings.ToLower(toolInput),
		strings.ToLower(inputPattern),
	)
}

// parsePattern splits "tool(input)" into ("tool", "input").
// Returns ("pattern", "") if no parentheses.
func parsePattern(pattern string) (string, string) {
	idx := strings.IndexByte(pattern, '(')
	if idx < 0 {
		return strings.TrimSpace(pattern), ""
	}
	toolPart := strings.TrimSpace(pattern[:idx])
	rest := pattern[idx+1:]
	// Find closing paren.
	end := strings.LastIndexByte(rest, ')')
	if end < 0 {
		return toolPart, strings.TrimSpace(rest)
	}
	return toolPart, strings.TrimSpace(rest[:end])
}

// FormatDeniedError creates a user-friendly error message for denied tool calls.
func FormatDeniedError(toolName string) string {
	return fmt.Sprintf("Tool '%s' is not permitted by the agent's permission rules.", toolName)
}

// --- Denial tracking ---

// DenialEntry records metadata about a single tool denial event.
// Inspired by Claude Code's denial tracking in utils/permissions/denialTracking.ts.
type DenialEntry struct {
	// ToolName is the denied tool.
	ToolName string `json:"toolName"`
	// Reason explains why the tool was denied (e.g. "permission_deny", "dangerous_sql").
	Reason string `json:"reason"`
	// InputSnippet is a truncated preview of the tool input (first 200 chars).
	InputSnippet string `json:"inputSnippet,omitempty"`
	// TurnIndex is the agentic turn when the denial occurred.
	TurnIndex int `json:"turnIndex"`
}

const (
	// maxDenialHistorySize limits the number of denial entries retained.
	maxDenialHistorySize = 50
	// denialInputSnippetMaxLen truncates tool input in denial entries.
	denialInputSnippetMaxLen = 200
	// autoModeLoopThreshold is how many times the same tool+input must be denied
	// before declaring an auto-mode loop.
	autoModeLoopThreshold = 3
)

// PermissionDenialTracker tracks consecutive and total tool denials to detect when the LLM
// is stuck in a denial loop. Enhanced with per-denial metadata and auto-mode loop
// detection (detects when the LLM repeatedly attempts the same denied action).
//
// Inspired by Claude Code's denialTracking.ts and autoModeDenials.ts.
type PermissionDenialTracker struct {
	ConsecutiveDenials int
	TotalDenials       int
	// History stores recent denial entries for diagnostics.
	History []DenialEntry
	// loopCounts maps "toolName:inputHash" to denial count for loop detection.
	loopCounts map[string]int
}

// DenialLimits defines the thresholds for denial escalation.
var DenialLimits = struct {
	MaxConsecutive int
	MaxTotal       int
}{
	MaxConsecutive: 3,
	MaxTotal:       20,
}

// RecordDenial increments counters and records metadata about the denial.
func (d *PermissionDenialTracker) RecordDenial() {
	d.ConsecutiveDenials++
	d.TotalDenials++
}

// RecordDenialWithMetadata increments counters and stores a detailed denial entry.
func (d *PermissionDenialTracker) RecordDenialWithMetadata(toolName, reason, toolInput string, turnIndex int) {
	d.RecordDenial()

	snippet := toolInput
	if len(snippet) > denialInputSnippetMaxLen {
		snippet = snippet[:denialInputSnippetMaxLen]
	}

	entry := DenialEntry{
		ToolName:     toolName,
		Reason:       reason,
		InputSnippet: snippet,
		TurnIndex:    turnIndex,
	}

	// Append to history, trimming oldest if over limit.
	d.History = append(d.History, entry)
	if len(d.History) > maxDenialHistorySize {
		d.History = d.History[len(d.History)-maxDenialHistorySize:]
	}

	// Track loop counts.
	if d.loopCounts == nil {
		d.loopCounts = make(map[string]int)
	}
	loopKey := denialLoopKey(toolName, snippet)
	d.loopCounts[loopKey]++
}

// RecordSuccess resets the consecutive counter (total is not affected).
func (d *PermissionDenialTracker) RecordSuccess() {
	d.ConsecutiveDenials = 0
}

// ShouldEscalate returns true when denials have exceeded safe thresholds,
// indicating the LLM is stuck and the run should be interrupted.
func (d *PermissionDenialTracker) ShouldEscalate() bool {
	return d.ConsecutiveDenials >= DenialLimits.MaxConsecutive ||
		d.TotalDenials >= DenialLimits.MaxTotal
}

// IsAutoModeLoop returns true when the LLM is repeatedly attempting the same
// denied action, indicating it's stuck in a loop rather than trying different
// approaches. This is distinct from ShouldEscalate which tracks any denials.
//
// Inspired by Claude Code's auto-mode denial loop detection in autoModeDenials.ts.
func (d *PermissionDenialTracker) IsAutoModeLoop() bool {
	if d.loopCounts == nil {
		return false
	}
	for _, count := range d.loopCounts {
		if count >= autoModeLoopThreshold {
			return true
		}
	}
	return false
}

// LoopingTools returns the tool names that are stuck in a denial loop.
func (d *PermissionDenialTracker) LoopingTools() []string {
	if d.loopCounts == nil {
		return nil
	}
	var tools []string
	seen := map[string]bool{}
	for key, count := range d.loopCounts {
		if count >= autoModeLoopThreshold {
			// Extract tool name from "toolName:hash" key.
			toolName := key
			if idx := strings.LastIndex(key, ":"); idx > 0 {
				toolName = key[:idx]
			}
			if !seen[toolName] {
				tools = append(tools, toolName)
				seen[toolName] = true
			}
		}
	}
	return tools
}

// RecentDenials returns the last N denial entries.
func (d *PermissionDenialTracker) RecentDenials(n int) []DenialEntry {
	if n <= 0 || len(d.History) == 0 {
		return nil
	}
	if n > len(d.History) {
		n = len(d.History)
	}
	return d.History[len(d.History)-n:]
}

// denialLoopKey generates a deduplication key for auto-mode loop detection.
// Uses tool name + first 100 chars of input as a fingerprint.
func denialLoopKey(toolName, inputSnippet string) string {
	key := toolName + ":"
	if len(inputSnippet) > 100 {
		key += inputSnippet[:100]
	} else {
		key += inputSnippet
	}
	return key
}

// dangerousCommandPatterns lists shell commands that should never be auto-approved.
// Matches against tool input for shell/exec tools. Inspired by Claude Code's
// dangerousPatterns.ts — prevents auto-approval of commands that could cause harm.
var dangerousCommandPatterns = []string{
	"sudo", "su ",
	"rm -rf", "rm -r",
	"chmod", "chown",
	"| sh", "| bash",
	"eval ", "exec ",
	"python ", "python3 ", "node ", "ruby ", "perl ",
	"ssh ", "scp ",
	"dd ", "mkfs",
	"iptables", "ufw",
	"> /dev/", ">> /dev/",
	"kill ", "killall",
	"env ", "export ",
	"git push --force", "git reset --hard",
	"drop table", "drop database", "truncate table",
	"delete from",
}

// ContainsDangerousCommand returns true if the input contains any dangerous command pattern.
// Used to prevent auto-approval of risky shell commands even when allow rules match.
func ContainsDangerousCommand(input string) bool {
	lower := strings.ToLower(input)
	for _, pat := range dangerousCommandPatterns {
		if strings.Contains(lower, pat) {
			return true
		}
	}
	return false
}

// dangerousSQLPatterns lists SQL patterns that should require confirmation.
// These patterns detect destructive or data-exfiltrating SQL operations.
// Inspired by Claude Code's dangerous pattern detection for tool inputs.
var dangerousSQLPatterns = []string{
	"drop table", "drop database", "drop schema", "drop index",
	"truncate table", "truncate ",
	"delete from",
	"alter table", "alter database",
	"create user", "drop user", "grant ", "revoke ",
	"update ", // bare UPDATE without WHERE
	"into outfile", "into dumpfile",
	"load_file", "load data",
	"exec ", "execute ",
	"xp_cmdshell", "sp_executesql",
	"; --", "' or ", "\" or ", "1=1",
}

// dangerousHTTPPatterns lists URL/header patterns that should require confirmation.
var dangerousHTTPPatterns = []string{
	"127.0.0.1", "localhost", "0.0.0.0",
	"169.254.169.254", // AWS metadata
	"metadata.google",  // GCP metadata
	"file://",
	"ftp://",
	"gopher://",
}

// ContainsDangerousSQLPattern returns true if the input contains destructive SQL patterns.
func ContainsDangerousSQLPattern(input string) bool {
	lower := strings.ToLower(input)
	for _, pat := range dangerousSQLPatterns {
		if strings.Contains(lower, pat) {
			return true
		}
	}
	return false
}

// ContainsDangerousHTTPPattern returns true if the input contains SSRF-risk URL patterns.
func ContainsDangerousHTTPPattern(input string) bool {
	lower := strings.ToLower(input)
	for _, pat := range dangerousHTTPPatterns {
		if strings.Contains(lower, pat) {
			return true
		}
	}
	return false
}

// ToolDangerLevel categorises how dangerous a tool invocation is.
type ToolDangerLevel string

const (
	DangerNone    ToolDangerLevel = ""
	DangerCommand ToolDangerLevel = "dangerous_command"
	DangerSQL     ToolDangerLevel = "dangerous_sql"
	DangerHTTP    ToolDangerLevel = "dangerous_http"
)

// DetectDangerousInput classifies the danger level of a tool input based on the tool type.
// Returns DangerNone if no dangerous patterns are detected.
// Note: DangerCommand is only returned for tools that could execute shell commands —
// callers should pass exec tool names separately for shell command detection.
func DetectDangerousInput(toolSlug, toolInput string) ToolDangerLevel {
	switch {
	case isSQLTool(toolSlug) && ContainsDangerousSQLPattern(toolInput):
		return DangerSQL
	case isHTTPTool(toolSlug) && ContainsDangerousHTTPPattern(toolInput):
		return DangerHTTP
	default:
		return DangerNone
	}
}

func isSQLTool(slug string) bool {
	return strings.Contains(slug, "sql") || strings.Contains(slug, "database") || strings.Contains(slug, "query")
}

func isHTTPTool(slug string) bool {
	return strings.Contains(slug, "http") || strings.Contains(slug, "api") || strings.Contains(slug, "web") || strings.Contains(slug, "fetch")
}

// EvaluatePermissionWithDangerCheck extends EvaluatePermission with dangerous input detection.
// If a tool's input matches a dangerous pattern, the decision is escalated from Allow to Confirm.
// This applies to shell tools, SQL tools, and HTTP tools.
func EvaluatePermissionWithDangerCheck(rules *PermissionRules, toolName, toolInput string, execToolNames []string) PermissionDecision {
	decision := EvaluatePermission(rules, toolName, toolInput)

	if decision == PermissionAllow {
		// Check shell/exec tools.
		for _, execTool := range execToolNames {
			if toolName == execTool && ContainsDangerousCommand(toolInput) {
				return PermissionConfirm
			}
		}
		// Check SQL and HTTP tools.
		danger := DetectDangerousInput(toolName, toolInput)
		if danger != DangerNone {
			return PermissionConfirm
		}
	}
	return decision
}
