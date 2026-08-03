package chat

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// DynamicRetrievalConfig defines the tenant-default policy used to select skills.
// It is stored in public.tenant_chat_default.retrieval_config.
type DynamicRetrievalConfig struct {
	TopK           int     `json:"topK"`
	DriftThreshold float64 `json:"driftThreshold"`
	MaxStickySize  int     `json:"maxStickySize"`
	AllowDrift     bool    `json:"allowDrift"`
	RefreshPolicy  string  `json:"refreshPolicy"`
	MinScore       float64 `json:"minScore"`
}

// DefaultDynamicRetrievalConfig is intentionally conservative: a session starts
// with a small tool set and only refreshes when the topic changes materially.
func DefaultDynamicRetrievalConfig() DynamicRetrievalConfig {
	return DynamicRetrievalConfig{
		TopK:           8,
		DriftThreshold: 0.55,
		MaxStickySize:  15,
		AllowDrift:     true,
		RefreshPolicy:  "drift_or_invalidation",
		MinScore:       0.30,
	}
}

// Normalize applies safe defaults to incomplete persisted JSON while preserving
// explicitly configured values.
func (c DynamicRetrievalConfig) Normalize() DynamicRetrievalConfig {
	defaults := DefaultDynamicRetrievalConfig()
	if c.TopK <= 0 {
		c.TopK = defaults.TopK
	}
	if c.DriftThreshold <= 0 || c.DriftThreshold > 2 {
		c.DriftThreshold = defaults.DriftThreshold
	}
	if c.MaxStickySize <= 0 {
		c.MaxStickySize = defaults.MaxStickySize
	}
	if c.MaxStickySize < c.TopK {
		c.MaxStickySize = c.TopK
	}
	if c.MinScore < 0 || c.MinScore > 1 {
		c.MinScore = defaults.MinScore
	}
	if strings.TrimSpace(c.RefreshPolicy) == "" {
		c.RefreshPolicy = defaults.RefreshPolicy
	}
	return c
}

// DynamicPersona is the server-owned tenant default used by DYNAMIC_SKILL
// sessions. It is deliberately not accepted from the public create-session
// request, which prevents selecting another tenant's persona by UUID.
type DynamicPersona struct {
	ID               uuid.UUID
	TenantID         string
	Name             string
	SystemPrompt     string
	ModelConfig      json.RawMessage
	RetrievalConfig  DynamicRetrievalConfig
	EnableManagement bool
}

// DynamicSkillSetSnapshot is persisted in chat_session.sticky_skill_set.
// Source hashes make a skill edit or deletion an explicit invalidation event.
type DynamicSkillSetSnapshot struct {
	SkillIDs           []uuid.UUID       `json:"skillIds"`
	SourceHashes       map[string]string `json:"sourceHashes"`
	QueryEmbedding     []float32         `json:"queryEmbedding"`
	QueryEmbeddingHash string            `json:"queryEmbeddingHash"`
	RetrievedAt        time.Time         `json:"retrievedAt"`
	ScoreP50           float64           `json:"scoreP50"`
	Source             string            `json:"source"`
}

// Empty reports whether no retrieval has yet been persisted for the session.
func (s DynamicSkillSetSnapshot) Empty() bool {
	return len(s.SkillIDs) == 0 && len(s.QueryEmbedding) == 0
}

// ParseDynamicSkillSetSnapshot accepts the legacy [] JSON default as an empty
// snapshot, allowing already-created sessions to upgrade without failing runs.
func ParseDynamicSkillSetSnapshot(raw json.RawMessage) (DynamicSkillSetSnapshot, error) {
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" || trimmed == "null" || trimmed == "{}" || trimmed == "[]" {
		return DynamicSkillSetSnapshot{}, nil
	}
	var snapshot DynamicSkillSetSnapshot
	if err := json.Unmarshal(raw, &snapshot); err != nil {
		return DynamicSkillSetSnapshot{}, fmt.Errorf("chat: decode sticky skill set: %w", err)
	}
	if snapshot.SourceHashes == nil {
		snapshot.SourceHashes = map[string]string{}
	}
	return snapshot, nil
}

// HashDynamicQueryEmbedding returns a stable identifier for audit and cache
// diagnostics without storing a second copy of the vector in related records.
func HashDynamicQueryEmbedding(embedding []float32) string {
	encoded, _ := json.Marshal(embedding)
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:])
}

// DynamicSessionStore is intentionally narrower than Repository. Existing
// non-Postgres test doubles remain valid while production code can load the
// server-owned persona and atomically persist the sticky set.
type DynamicSessionStore interface {
	GetTenantChatDefault(ctx context.Context, tenantID string) (DynamicPersona, error)
	UpdateSessionDynamicSkillSet(ctx context.Context, sessionID uuid.UUID, snapshot DynamicSkillSetSnapshot) error
}
