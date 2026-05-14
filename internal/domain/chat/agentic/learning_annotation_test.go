package agentic

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---- LearningAnnotationKind ----

func TestLearningAnnotationKind_IsValid_AllValid(t *testing.T) {
	for _, k := range AllLearningAnnotationKinds() {
		assert.True(t, k.IsValid(), "expected %q to be valid", k)
	}
}

func TestLearningAnnotationKind_IsValid_Invalid(t *testing.T) {
	assert.False(t, LearningAnnotationKind("").IsValid())
	assert.False(t, LearningAnnotationKind("unknown").IsValid())
	assert.False(t, LearningAnnotationKind("Pattern").IsValid())
}

func TestLearningAnnotationKind_AllKinds_Count(t *testing.T) {
	assert.Equal(t, 5, len(AllLearningAnnotationKinds()))
}

func TestLearningAnnotationKind_AllKinds_Unique(t *testing.T) {
	seen := map[LearningAnnotationKind]bool{}
	for _, k := range AllLearningAnnotationKinds() {
		assert.False(t, seen[k], "duplicate kind %q", k)
		seen[k] = true
	}
}

func TestLearningAnnotationKind_ClosedSet(t *testing.T) {
	expected := map[LearningAnnotationKind]bool{
		LearningAnnotationKindPattern:      true,
		LearningAnnotationKindAntiPattern:  true,
		LearningAnnotationKindTip:          true,
		LearningAnnotationKindOptimization: true,
		LearningAnnotationKindKnowledgeGap: true,
	}
	for _, k := range AllLearningAnnotationKinds() {
		assert.True(t, expected[k], "unexpected kind %q", k)
	}
	assert.Equal(t, 5, len(expected))
}

// ---- LearningAnnotationAudience ----

func TestLearningAnnotationAudience_IsValid_AllValid(t *testing.T) {
	for _, a := range AllLearningAnnotationAudiences() {
		assert.True(t, a.IsValid(), "expected %q to be valid", a)
	}
}

func TestLearningAnnotationAudience_IsValid_Invalid(t *testing.T) {
	assert.False(t, LearningAnnotationAudience("").IsValid())
	assert.False(t, LearningAnnotationAudience("expert").IsValid())
	assert.False(t, LearningAnnotationAudience("Beginner").IsValid())
}

func TestLearningAnnotationAudience_AllAudiences_Count(t *testing.T) {
	assert.Equal(t, 3, len(AllLearningAnnotationAudiences()))
}

func TestLearningAnnotationAudience_LevelOrdering(t *testing.T) {
	assert.Less(t,
		learningAudienceLevel(LearningAnnotationAudienceBeginner),
		learningAudienceLevel(LearningAnnotationAudienceIntermediate))
	assert.Less(t,
		learningAudienceLevel(LearningAnnotationAudienceIntermediate),
		learningAudienceLevel(LearningAnnotationAudienceAdvanced))
}

func TestLearningAnnotationAudience_InvalidLevelIsNegative(t *testing.T) {
	assert.Less(t, learningAudienceLevel(LearningAnnotationAudience("bogus")), 0)
}

// ---- LearningAnnotation.Validate ----

func validLearningAnnotation() *LearningAnnotation {
	return &LearningAnnotation{
		AnnotationID: "ann-001",
		SessionID:    "sess-abc",
		AgentSlug:    "helper-agent",
		Kind:         LearningAnnotationKindTip,
		Audience:     LearningAnnotationAudienceBeginner,
		Title:        "Use skill reuse",
		Content:      "You can bind the same skill to multiple agents.",
		EmittedAt:    time.Now(),
	}
}

func TestLearningAnnotation_Validate_Happy(t *testing.T) {
	require.NoError(t, validLearningAnnotation().Validate())
}

func TestLearningAnnotation_Validate_Nil(t *testing.T) {
	var a *LearningAnnotation
	assert.ErrorIs(t, a.Validate(), ErrLearningAnnotationNil)
}

func TestLearningAnnotation_Validate_IDEmpty(t *testing.T) {
	a := validLearningAnnotation()
	a.AnnotationID = ""
	assert.ErrorIs(t, a.Validate(), ErrLearningAnnotationIDEmpty)
}

func TestLearningAnnotation_Validate_SessionEmpty(t *testing.T) {
	a := validLearningAnnotation()
	a.SessionID = ""
	assert.ErrorIs(t, a.Validate(), ErrLearningAnnotationSessionEmpty)
}

func TestLearningAnnotation_Validate_AgentEmpty(t *testing.T) {
	a := validLearningAnnotation()
	a.AgentSlug = ""
	assert.ErrorIs(t, a.Validate(), ErrLearningAnnotationAgentEmpty)
}

func TestLearningAnnotation_Validate_KindInvalid(t *testing.T) {
	a := validLearningAnnotation()
	a.Kind = "bogus"
	assert.ErrorIs(t, a.Validate(), ErrLearningAnnotationKindInvalid)
}

func TestLearningAnnotation_Validate_AudienceInvalid(t *testing.T) {
	a := validLearningAnnotation()
	a.Audience = "expert"
	assert.ErrorIs(t, a.Validate(), ErrLearningAnnotationAudienceInvalid)
}

func TestLearningAnnotation_Validate_TitleEmpty(t *testing.T) {
	a := validLearningAnnotation()
	a.Title = ""
	assert.ErrorIs(t, a.Validate(), ErrLearningAnnotationTitleEmpty)
}

func TestLearningAnnotation_Validate_ContentEmpty(t *testing.T) {
	a := validLearningAnnotation()
	a.Content = ""
	assert.ErrorIs(t, a.Validate(), ErrLearningAnnotationContentEmpty)
}

// ---- LearningAnnotationJournal ----

func TestLearningAnnotationJournal_New_EmptySessionReturnsError(t *testing.T) {
	_, err := NewLearningAnnotationJournal("")
	assert.ErrorIs(t, err, ErrLearningAnnotationSessionEmpty)
}

func TestLearningAnnotationJournal_New_OK(t *testing.T) {
	j, err := NewLearningAnnotationJournal("sess-1")
	require.NoError(t, err)
	assert.Equal(t, "sess-1", j.SessionID())
	assert.Equal(t, 0, j.Size())
}

func TestLearningAnnotationJournal_Add_NilReturnsError(t *testing.T) {
	j, _ := NewLearningAnnotationJournal("s")
	assert.ErrorIs(t, j.Add(nil), ErrLearningAnnotationNil)
}

func TestLearningAnnotationJournal_Add_InvalidAnnotationReturnsError(t *testing.T) {
	j, _ := NewLearningAnnotationJournal("sess-1")
	a := validLearningAnnotation()
	a.SessionID = "sess-1"
	a.Title = ""
	assert.ErrorIs(t, j.Add(a), ErrLearningAnnotationTitleEmpty)
}

func TestLearningAnnotationJournal_Add_SessionMismatchReturnsError(t *testing.T) {
	j, _ := NewLearningAnnotationJournal("sess-1")
	a := validLearningAnnotation()
	a.SessionID = "sess-OTHER"
	err := j.Add(a)
	assert.True(t, errors.Is(err, ErrLearningAnnotationSessionMismatch),
		"expected session mismatch, got %v", err)
}

func TestLearningAnnotationJournal_AddAndListAll(t *testing.T) {
	j, _ := NewLearningAnnotationJournal("sess-1")
	for i := 0; i < 3; i++ {
		a := validLearningAnnotation()
		a.SessionID = "sess-1"
		a.AnnotationID = "ann-" + string(rune('0'+i))
		require.NoError(t, j.Add(a))
	}
	assert.Equal(t, 3, j.Size())
	all := j.ListAll()
	assert.Len(t, all, 3)
}

func TestLearningAnnotationJournal_ListAll_IsDefensiveCopy(t *testing.T) {
	j, _ := NewLearningAnnotationJournal("sess-1")
	a := validLearningAnnotation()
	a.SessionID = "sess-1"
	require.NoError(t, j.Add(a))
	first := j.ListAll()
	first[0] = nil
	// journal must still return real entry
	second := j.ListAll()
	assert.NotNil(t, second[0])
}

func TestLearningAnnotationJournal_CapAt50_DropsOldest(t *testing.T) {
	j, _ := NewLearningAnnotationJournal("sess-cap")
	for i := 0; i < LearningAnnotationJournalMaxSize+10; i++ {
		a := &LearningAnnotation{
			AnnotationID: "ann-" + string(rune('a'+i%26)) + string(rune('0'+i%10)),
			SessionID:    "sess-cap",
			AgentSlug:    "ag",
			Kind:         LearningAnnotationKindTip,
			Audience:     LearningAnnotationAudienceBeginner,
			Title:        "T",
			Content:      "C",
			EmittedAt:    time.Now(),
		}
		require.NoError(t, j.Add(a))
	}
	assert.Equal(t, LearningAnnotationJournalMaxSize, j.Size())
}

func TestLearningAnnotationJournal_ListByKind(t *testing.T) {
	j, _ := NewLearningAnnotationJournal("sess-k")
	addAnnotation := func(kind LearningAnnotationKind, id string) {
		a := validLearningAnnotation()
		a.SessionID = "sess-k"
		a.AnnotationID = id
		a.Kind = kind
		require.NoError(t, j.Add(a))
	}
	addAnnotation(LearningAnnotationKindTip, "tip-1")
	addAnnotation(LearningAnnotationKindTip, "tip-2")
	addAnnotation(LearningAnnotationKindPattern, "pat-1")

	tips := j.ListByKind(LearningAnnotationKindTip)
	assert.Len(t, tips, 2)
	pats := j.ListByKind(LearningAnnotationKindPattern)
	assert.Len(t, pats, 1)
	gaps := j.ListByKind(LearningAnnotationKindKnowledgeGap)
	assert.Empty(t, gaps)
}

func TestLearningAnnotationJournal_FilterForAudience_Progressive(t *testing.T) {
	j, _ := NewLearningAnnotationJournal("sess-aud")
	add := func(aud LearningAnnotationAudience, id string) {
		a := validLearningAnnotation()
		a.SessionID = "sess-aud"
		a.AnnotationID = id
		a.Audience = aud
		require.NoError(t, j.Add(a))
	}
	add(LearningAnnotationAudienceBeginner, "b1")
	add(LearningAnnotationAudienceIntermediate, "i1")
	add(LearningAnnotationAudienceAdvanced, "a1")

	beginnerView := j.FilterForAudience(LearningAnnotationAudienceBeginner)
	assert.Len(t, beginnerView, 1)

	intermediateView := j.FilterForAudience(LearningAnnotationAudienceIntermediate)
	assert.Len(t, intermediateView, 2)

	advancedView := j.FilterForAudience(LearningAnnotationAudienceAdvanced)
	assert.Len(t, advancedView, 3)
}

func TestLearningAnnotationJournal_Clear(t *testing.T) {
	j, _ := NewLearningAnnotationJournal("sess-clr")
	a := validLearningAnnotation()
	a.SessionID = "sess-clr"
	require.NoError(t, j.Add(a))
	assert.Equal(t, 1, j.Size())
	j.Clear()
	assert.Equal(t, 0, j.Size())
}

func TestLearningAnnotationJournal_AddStoresDefensiveCopy(t *testing.T) {
	j, _ := NewLearningAnnotationJournal("sess-cp")
	a := validLearningAnnotation()
	a.SessionID = "sess-cp"
	require.NoError(t, j.Add(a))
	// mutate original after add — journal must not be affected
	a.Title = "mutated"
	all := j.ListAll()
	assert.NotEqual(t, "mutated", all[0].Title)
}

func TestLearningAnnotationSlugRE_MatchesKebab(t *testing.T) {
	assert.True(t, LearningAnnotationSlugRE.MatchString("agent-tool-binding-pattern"))
	assert.True(t, LearningAnnotationSlugRE.MatchString("ab"))
	assert.False(t, LearningAnnotationSlugRE.MatchString("a"))
	assert.False(t, LearningAnnotationSlugRE.MatchString("-start"))
	assert.False(t, LearningAnnotationSlugRE.MatchString("end-"))
	assert.False(t, LearningAnnotationSlugRE.MatchString("UPPER"))
}

func TestLearningAnnotationJournalMaxSize_Is50(t *testing.T) {
	assert.Equal(t, 50, LearningAnnotationJournalMaxSize)
}
