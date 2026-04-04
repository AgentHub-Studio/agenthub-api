package agentic

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Deterministic agent ID formatting and parsing.
//
// Inspired by Claude Code's agentId.ts — deterministic agent IDs in
// the format "agentName@teamName" with request ID generation and
// parsing. Deterministic IDs enable reconnection after restarts,
// human-readable debugging, and predictable message routing without
// lookup. Agent names must NOT contain '@' (used as separator).

// AgentIdentity holds the parsed components of an agent ID.
type AgentIdentity struct {
	AgentName string
	TeamName  string
}

// RequestIdentity holds the parsed components of a request ID.
type RequestIdentity struct {
	RequestType string
	Timestamp   int64
	AgentID     string
}

// FormatAgentID formats an agent ID as "agentName@teamName".
func FormatAgentID(agentName, teamName string) string {
	return agentName + "@" + teamName
}

// ParseAgentID parses an agent ID into its components.
// Returns nil if the ID doesn't contain the '@' separator.
func ParseAgentID(agentID string) *AgentIdentity {
	idx := strings.Index(agentID, "@")
	if idx == -1 {
		return nil
	}
	return &AgentIdentity{
		AgentName: agentID[:idx],
		TeamName:  agentID[idx+1:],
	}
}

// GenerateRequestID creates a request ID in the format
// "{requestType}-{timestamp}@{agentId}".
func GenerateRequestID(requestType, agentID string) string {
	ts := time.Now().UnixMilli()
	return fmt.Sprintf("%s-%d@%s", requestType, ts, agentID)
}

// GenerateRequestIDAt creates a request ID with a specific timestamp.
func GenerateRequestIDAt(requestType, agentID string, ts int64) string {
	return fmt.Sprintf("%s-%d@%s", requestType, ts, agentID)
}

// ParseRequestID parses a request ID into its components.
// Returns nil if the ID doesn't match the expected format.
func ParseRequestID(requestID string) *RequestIdentity {
	atIdx := strings.Index(requestID, "@")
	if atIdx == -1 {
		return nil
	}

	prefix := requestID[:atIdx]
	agentID := requestID[atIdx+1:]

	dashIdx := strings.LastIndex(prefix, "-")
	if dashIdx == -1 {
		return nil
	}

	requestType := prefix[:dashIdx]
	timestampStr := prefix[dashIdx+1:]

	timestamp, err := strconv.ParseInt(timestampStr, 10, 64)
	if err != nil {
		return nil
	}

	return &RequestIdentity{
		RequestType: requestType,
		Timestamp:   timestamp,
		AgentID:     agentID,
	}
}

// SanitizeAgentName removes '@' characters from a name to make it
// safe for use in agent IDs.
func SanitizeAgentName(name string) string {
	return strings.ReplaceAll(name, "@", "")
}
