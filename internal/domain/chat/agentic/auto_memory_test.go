package agentic

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func validAutoDecision() AutoMemoryDecision {
	return AutoMemoryDecision{
		Type: AutoMemoryTypeUser, Key: "name", Value: "Alice",
		Confidence: 0.9, Rationale: "name declaration heuristic",
	}
}

func TestAutoMemory_TypeEnumIsBounded(t *testing.T) {
	for _, mt := range AllAutoMemoryTypes() {
		assert.True(t, IsValidAutoMemoryType(mt))
	}
	assert.False(t, IsValidAutoMemoryType(AutoMemoryType("trivia")))
	assert.Equal(t, 4, len(AllAutoMemoryTypes()))
}

func TestAutoMemory_DefaultConfig_HasSensibleDefaults(t *testing.T) {
	cfg := DefaultAutoMemoryConfig()
	assert.Equal(t, 0.5, cfg.MinConfidence)
	assert.Equal(t, 10, cfg.MaxPerTurn)
	// Sensitive keys block list must include common ones.
	for _, k := range []string{"password", "credit_card", "ssn", "api_key", "secret"} {
		assert.Contains(t, cfg.AdminBlockedKeys, k)
	}
}

func TestFilterByConfig_DropsBelowMinConfidence(t *testing.T) {
	decisions := []AutoMemoryDecision{
		{Type: AutoMemoryTypeUser, Key: "name", Value: "Alice", Confidence: 0.4},
		{Type: AutoMemoryTypeUser, Key: "role", Value: "engineer", Confidence: 0.8},
	}
	cfg := AutoMemoryConfig{MinConfidence: 0.5}
	survivors, dropped := FilterByConfig(decisions, cfg)
	require.Len(t, survivors, 1)
	assert.Equal(t, "role", survivors[0].Key)
	require.Len(t, dropped, 1)
	assert.Contains(t, dropped[0].Reason, "confidence")
}

func TestFilterByConfig_DropsAdminBlockedKeys(t *testing.T) {
	decisions := []AutoMemoryDecision{
		{Type: AutoMemoryTypeUser, Key: "password", Value: "x", Confidence: 0.99},
		{Type: AutoMemoryTypeUser, Key: "name", Value: "Alice", Confidence: 0.9},
	}
	cfg := AutoMemoryConfig{
		MinConfidence: 0.5, AdminBlockedKeys: []string{"password"},
	}
	survivors, dropped := FilterByConfig(decisions, cfg)
	require.Len(t, survivors, 1)
	assert.Equal(t, "name", survivors[0].Key)
	require.Len(t, dropped, 1)
	assert.Contains(t, dropped[0].Reason, "block list")
}

func TestFilterByConfig_BlockedKeyMatchIsCaseInsensitive(t *testing.T) {
	decisions := []AutoMemoryDecision{
		{Type: AutoMemoryTypeUser, Key: "PASSWORD", Value: "x", Confidence: 0.99},
	}
	cfg := AutoMemoryConfig{AdminBlockedKeys: []string{"password"}}
	survivors, _ := FilterByConfig(decisions, cfg)
	assert.Len(t, survivors, 0, "block list match must be case-insensitive")
}

func TestFilterByConfig_RespectsMaxPerTurn(t *testing.T) {
	decisions := []AutoMemoryDecision{}
	for i := 0; i < 5; i++ {
		decisions = append(decisions, AutoMemoryDecision{
			Type: AutoMemoryTypeUser, Key: "k" + string(rune('a'+i)),
			Value: "v", Confidence: 0.9,
		})
	}
	cfg := AutoMemoryConfig{MinConfidence: 0.5, MaxPerTurn: 2}
	survivors, dropped := FilterByConfig(decisions, cfg)
	assert.Len(t, survivors, 2)
	assert.Len(t, dropped, 3)
	for _, d := range dropped {
		assert.Contains(t, d.Reason, "max per turn")
	}
}

func TestFilterByConfig_ZeroMaxPerTurnIsUnlimited(t *testing.T) {
	decisions := []AutoMemoryDecision{}
	for i := 0; i < 50; i++ {
		decisions = append(decisions, AutoMemoryDecision{
			Type: AutoMemoryTypeUser, Key: "k", Value: "v", Confidence: 0.9,
		})
	}
	cfg := AutoMemoryConfig{MinConfidence: 0.5, MaxPerTurn: 0}
	survivors, _ := FilterByConfig(decisions, cfg)
	assert.Len(t, survivors, 50)
}

func TestHeuristicClassifier_ExplicitRememberStatement(t *testing.T) {
	c := NewHeuristicAutoMemoryClassifier()
	got, err := c.Classify(context.Background(), "Remember that the deploy window is Friday 8pm")
	require.NoError(t, err)
	require.NotEmpty(t, got)
	assert.Equal(t, AutoMemoryTypeProject, got[0].Type)
	assert.Equal(t, "explicit_note", got[0].Key)
	assert.GreaterOrEqual(t, got[0].Confidence, 0.9)
}

func TestHeuristicClassifier_PortugueseRememberStatement(t *testing.T) {
	c := NewHeuristicAutoMemoryClassifier()
	got, err := c.Classify(context.Background(), "lembre-se que sextas após 18h não deploy")
	require.NoError(t, err)
	require.NotEmpty(t, got)
	assert.Equal(t, AutoMemoryTypeProject, got[0].Type)
}

func TestHeuristicClassifier_NameDeclaration(t *testing.T) {
	c := NewHeuristicAutoMemoryClassifier()
	got, err := c.Classify(context.Background(), "My name is Alice and I work in payments.")
	require.NoError(t, err)
	require.NotEmpty(t, got)
	// Find user-name decision.
	found := false
	for _, d := range got {
		if d.Type == AutoMemoryTypeUser && d.Key == "name" {
			assert.Equal(t, "Alice", d.Value)
			found = true
		}
	}
	assert.True(t, found, "name declaration should be classified")
}

func TestHeuristicClassifier_PortugueseNameDeclaration(t *testing.T) {
	c := NewHeuristicAutoMemoryClassifier()
	got, _ := c.Classify(context.Background(), "Meu nome é Bob")
	found := false
	for _, d := range got {
		if d.Type == AutoMemoryTypeUser && d.Key == "name" {
			assert.Equal(t, "Bob", d.Value)
			found = true
		}
	}
	assert.True(t, found)
}

func TestHeuristicClassifier_CorrectiveLanguageBecomesFeedback(t *testing.T) {
	c := NewHeuristicAutoMemoryClassifier()
	got, err := c.Classify(context.Background(), "Don't summarize at the end of every reply.")
	require.NoError(t, err)
	require.NotEmpty(t, got)
	found := false
	for _, d := range got {
		if d.Type == AutoMemoryTypeFeedback && d.Key == "correction" {
			found = true
		}
	}
	assert.True(t, found)
}

func TestHeuristicClassifier_URLBecomesReference(t *testing.T) {
	c := NewHeuristicAutoMemoryClassifier()
	got, err := c.Classify(context.Background(), "Check out https://example.com/docs for the API.")
	require.NoError(t, err)
	require.NotEmpty(t, got)
	found := false
	for _, d := range got {
		if d.Type == AutoMemoryTypeReference && d.Key == "url" {
			assert.Contains(t, d.Value, "https://example.com/docs")
			found = true
		}
	}
	assert.True(t, found)
}

func TestHeuristicClassifier_ResultsSortedByConfidenceDescending(t *testing.T) {
	c := NewHeuristicAutoMemoryClassifier()
	got, _ := c.Classify(context.Background(),
		"Remember that my name is Alice and check https://example.com — don't suggest pickle.")
	require.GreaterOrEqual(t, len(got), 2)
	for i := 1; i < len(got); i++ {
		assert.GreaterOrEqual(t, got[i-1].Confidence, got[i].Confidence)
	}
}

func TestHeuristicClassifier_TruncatesLongValues(t *testing.T) {
	c := &HeuristicAutoMemoryClassifier{MaxBodyLength: 50}
	huge := strings.Repeat("x", 1000)
	got, _ := c.Classify(context.Background(), "Remember that "+huge)
	require.NotEmpty(t, got)
	assert.LessOrEqual(t, len(got[0].Value), 50)
	assert.True(t, strings.HasSuffix(got[0].Value, "..."))
}

func TestHeuristicClassifier_NoMatchesReturnsEmpty(t *testing.T) {
	c := NewHeuristicAutoMemoryClassifier()
	got, err := c.Classify(context.Background(), "ok")
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestStore_Store_AssignsIDAndTimestamp(t *testing.T) {
	s := NewInMemoryAutoMemoryStore()
	r, err := s.Store(context.Background(), "t-1", "u-1", validAutoDecision())
	require.NoError(t, err)
	assert.NotEqual(t, "", r.ID.String())
	assert.False(t, r.StoredAt.IsZero())
}

func TestStore_Store_RejectsMissingTenantOrUser(t *testing.T) {
	s := NewInMemoryAutoMemoryStore()
	_, err := s.Store(context.Background(), "", "u-1", validAutoDecision())
	assert.True(t, errors.Is(err, ErrAutoMemoryTenantRequired))
	_, err = s.Store(context.Background(), "t-1", "", validAutoDecision())
	assert.True(t, errors.Is(err, ErrAutoMemoryUserRequired))
}

func TestStore_Store_RejectsInvalidDecision(t *testing.T) {
	s := NewInMemoryAutoMemoryStore()
	d := validAutoDecision()
	d.Type = "rogue"
	_, err := s.Store(context.Background(), "t", "u", d)
	assert.True(t, errors.Is(err, ErrAutoMemoryInvalidType))

	d2 := validAutoDecision()
	d2.Confidence = 1.5
	_, err = s.Store(context.Background(), "t", "u", d2)
	assert.True(t, errors.Is(err, ErrAutoMemoryConfidenceRange))
}

func TestStore_Find_RoundTrips(t *testing.T) {
	s := NewInMemoryAutoMemoryStore()
	r, _ := s.Store(context.Background(), "t", "u", validAutoDecision())
	got, err := s.Find(context.Background(), r.ID)
	require.NoError(t, err)
	assert.Equal(t, r.ID, got.ID)
}

func TestStore_ListByType_FiltersStrictly(t *testing.T) {
	s := NewInMemoryAutoMemoryStore()
	d1 := validAutoDecision()
	d2 := AutoMemoryDecision{Type: AutoMemoryTypeFeedback, Key: "x", Value: "y", Confidence: 0.9}
	_, _ = s.Store(context.Background(), "t", "u", d1)
	_, _ = s.Store(context.Background(), "t", "u", d2)

	got, _ := s.ListByType(context.Background(), "t", "u", AutoMemoryTypeUser)
	assert.Len(t, got, 1)
	assert.Equal(t, AutoMemoryTypeUser, got[0].Decision.Type)
}

func TestStore_ListByUser_TenantIsolation(t *testing.T) {
	s := NewInMemoryAutoMemoryStore()
	_, _ = s.Store(context.Background(), "t-a", "u", validAutoDecision())
	_, _ = s.Store(context.Background(), "t-b", "u", validAutoDecision())
	got, _ := s.ListByUser(context.Background(), "t-a", "u")
	assert.Len(t, got, 1)
	assert.Equal(t, "t-a", got[0].TenantID)
}

func TestPipeline_RoundTripStoresOnlySurvivors(t *testing.T) {
	classifier := NewHeuristicAutoMemoryClassifier()
	store := NewInMemoryAutoMemoryStore()
	cfg := AutoMemoryConfig{MinConfidence: 0.8, MaxPerTurn: 2,
		AdminBlockedKeys: []string{"password"}}

	res, err := RunAutoMemoryPipeline(context.Background(), classifier, store, cfg,
		"t-1", "u-1",
		"Remember that the deploy window is Friday and my name is Alice.")
	require.NoError(t, err)
	assert.NotEmpty(t, res.Stored, "high-confidence decisions stored")
	for _, r := range res.Stored {
		assert.GreaterOrEqual(t, r.Decision.Confidence, 0.8)
	}
}

func TestPipeline_RecordsDroppedReasonsForAudit(t *testing.T) {
	classifier := NewHeuristicAutoMemoryClassifier()
	store := NewInMemoryAutoMemoryStore()
	cfg := AutoMemoryConfig{MinConfidence: 0.99, MaxPerTurn: 0}

	res, _ := RunAutoMemoryPipeline(context.Background(), classifier, store, cfg,
		"t", "u", "Don't summarize at the end.")
	// Heuristic gives correction confidence=0.7 < 0.99 → dropped.
	assert.Empty(t, res.Stored)
	assert.NotEmpty(t, res.Dropped)
}

func TestPipeline_PropagatesClassifierError(t *testing.T) {
	store := NewInMemoryAutoMemoryStore()
	cfg := DefaultAutoMemoryConfig()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := RunAutoMemoryPipeline(ctx, NewHeuristicAutoMemoryClassifier(), store, cfg, "t", "u", "msg")
	assert.Error(t, err)
}

func TestStore_ConcurrentStoreIsSafe(t *testing.T) {
	s := NewInMemoryAutoMemoryStore()
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = s.Store(context.Background(), "t", "u", validAutoDecision())
		}()
	}
	wg.Wait()
	got, _ := s.ListByUser(context.Background(), "t", "u")
	assert.Len(t, got, 50)
}

func TestStore_ContextCancelled(t *testing.T) {
	s := NewInMemoryAutoMemoryStore()
	r, _ := s.Store(context.Background(), "t", "u", validAutoDecision())
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := s.Store(ctx, "t", "u", validAutoDecision())
	assert.Error(t, err)
	_, err = s.Find(ctx, r.ID)
	assert.Error(t, err)
	_, err = s.ListByType(ctx, "t", "u", AutoMemoryTypeUser)
	assert.Error(t, err)
	_, err = s.ListByUser(ctx, "t", "u")
	assert.Error(t, err)
}
