package agentic

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBDD_LearningAnnotationJournal(t *testing.T) {
	t.Run("Scenario_BeginnerSeesOnlyBeginnerNotes", func(t *testing.T) {
		// Given a journal with annotations at all three audience levels,
		// When a beginner operator requests their learning feed,
		// Then only beginner-targeted annotations surface.
		j, err := NewLearningAnnotationJournal("sess-bdd-1")
		require.NoError(t, err)

		for _, pair := range []struct {
			id  string
			aud LearningAnnotationAudience
		}{
			{"beg-1", LearningAnnotationAudienceBeginner},
			{"int-1", LearningAnnotationAudienceIntermediate},
			{"adv-1", LearningAnnotationAudienceAdvanced},
		} {
			a := &LearningAnnotation{
				AnnotationID: pair.id,
				SessionID:    "sess-bdd-1",
				AgentSlug:    "tutor",
				Kind:         LearningAnnotationKindTip,
				Audience:     pair.aud,
				Title:        "T",
				Content:      "C",
				EmittedAt:    time.Now(),
			}
			require.NoError(t, j.Add(a))
		}

		feed := j.FilterForAudience(LearningAnnotationAudienceBeginner)
		assert.Len(t, feed, 1)
		assert.Equal(t, LearningAnnotationAudienceBeginner, feed[0].Audience)
	})

	t.Run("Scenario_IntermediateOperatorSeesBeginnerAndOwnNotes", func(t *testing.T) {
		// Given a journal with beginner and intermediate annotations,
		// When an intermediate operator requests their learning feed,
		// Then both beginner and intermediate annotations surface
		// (progressive disclosure: more experienced = more content).
		j, err := NewLearningAnnotationJournal("sess-bdd-2")
		require.NoError(t, err)

		add := func(id string, aud LearningAnnotationAudience) {
			a := &LearningAnnotation{
				AnnotationID: id, SessionID: "sess-bdd-2",
				AgentSlug: "tutor", Kind: LearningAnnotationKindPattern,
				Audience: aud, Title: "T", Content: "C", EmittedAt: time.Now(),
			}
			require.NoError(t, j.Add(a))
		}
		add("b1", LearningAnnotationAudienceBeginner)
		add("b2", LearningAnnotationAudienceBeginner)
		add("i1", LearningAnnotationAudienceIntermediate)
		add("av1", LearningAnnotationAudienceAdvanced)

		feed := j.FilterForAudience(LearningAnnotationAudienceIntermediate)
		assert.Len(t, feed, 3)
		audiencesSeen := map[LearningAnnotationAudience]bool{}
		for _, e := range feed {
			audiencesSeen[e.Audience] = true
		}
		assert.True(t, audiencesSeen[LearningAnnotationAudienceBeginner])
		assert.True(t, audiencesSeen[LearningAnnotationAudienceIntermediate])
		assert.False(t, audiencesSeen[LearningAnnotationAudienceAdvanced])
	})

	t.Run("Scenario_AdvancedOperatorSeesEntireJournal", func(t *testing.T) {
		// Given a journal with annotations at all levels,
		// When an advanced operator requests their learning feed,
		// Then all annotations surface (full progressive disclosure).
		j, err := NewLearningAnnotationJournal("sess-bdd-3")
		require.NoError(t, err)

		for _, pair := range []struct {
			id  string
			aud LearningAnnotationAudience
		}{
			{"b1", LearningAnnotationAudienceBeginner},
			{"i1", LearningAnnotationAudienceIntermediate},
			{"a1", LearningAnnotationAudienceAdvanced},
			{"a2", LearningAnnotationAudienceAdvanced},
		} {
			a := &LearningAnnotation{
				AnnotationID: pair.id, SessionID: "sess-bdd-3",
				AgentSlug: "tutor", Kind: LearningAnnotationKindOptimization,
				Audience: pair.aud, Title: "T", Content: "C", EmittedAt: time.Now(),
			}
			require.NoError(t, j.Add(a))
		}

		feed := j.FilterForAudience(LearningAnnotationAudienceAdvanced)
		assert.Len(t, feed, 4)
	})

	t.Run("Scenario_JournalDropsOldestWhenAtCapacity", func(t *testing.T) {
		// Given a journal that has reached its 50-entry cap,
		// When a new annotation is added,
		// Then the oldest entry is silently dropped and size stays at 50.
		j, err := NewLearningAnnotationJournal("sess-cap")
		require.NoError(t, err)

		for i := 0; i < LearningAnnotationJournalMaxSize; i++ {
			a := &LearningAnnotation{
				AnnotationID: "fill-" + string(rune('a'+i%26)),
				SessionID:    "sess-cap",
				AgentSlug:    "filler",
				Kind:         LearningAnnotationKindTip,
				Audience:     LearningAnnotationAudienceBeginner,
				Title:        "Filler",
				Content:      "Fill",
				EmittedAt:    time.Now(),
			}
			require.NoError(t, j.Add(a))
		}
		assert.Equal(t, LearningAnnotationJournalMaxSize, j.Size())

		overflow := &LearningAnnotation{
			AnnotationID: "overflow", SessionID: "sess-cap",
			AgentSlug: "tutor", Kind: LearningAnnotationKindAntiPattern,
			Audience: LearningAnnotationAudienceAdvanced, Title: "New", Content: "New",
			EmittedAt: time.Now(),
		}
		require.NoError(t, j.Add(overflow))

		assert.Equal(t, LearningAnnotationJournalMaxSize, j.Size(),
			"journal must stay at cap after overflow")
		last := j.ListAll()[LearningAnnotationJournalMaxSize-1]
		assert.Equal(t, "overflow", last.AnnotationID,
			"most recent entry must be the overflow annotation")
	})

	t.Run("Scenario_AntiPatternAnnotationsAreFilterableByKind", func(t *testing.T) {
		// Given a journal mixing multiple kinds,
		// When admin filters by anti_pattern,
		// Then only anti-pattern notes are returned for targeted remediation.
		j, err := NewLearningAnnotationJournal("sess-bdd-4")
		require.NoError(t, err)

		addKind := func(id string, k LearningAnnotationKind) {
			a := &LearningAnnotation{
				AnnotationID: id, SessionID: "sess-bdd-4",
				AgentSlug: "advisor", Kind: k, Audience: LearningAnnotationAudienceBeginner,
				Title: "T", Content: "C", EmittedAt: time.Now(),
			}
			require.NoError(t, j.Add(a))
		}
		addKind("tip-1", LearningAnnotationKindTip)
		addKind("ap-1", LearningAnnotationKindAntiPattern)
		addKind("ap-2", LearningAnnotationKindAntiPattern)
		addKind("pat-1", LearningAnnotationKindPattern)

		antiPatterns := j.ListByKind(LearningAnnotationKindAntiPattern)
		assert.Len(t, antiPatterns, 2)
		for _, ap := range antiPatterns {
			assert.Equal(t, LearningAnnotationKindAntiPattern, ap.Kind)
		}
	})

	t.Run("Scenario_JournalBoundToSingleSession", func(t *testing.T) {
		// Given a journal bound to session A,
		// When an annotation from session B is added,
		// Then the add is rejected with a session-mismatch error
		// (prevents cross-session contamination of the learning feed).
		j, err := NewLearningAnnotationJournal("sess-A")
		require.NoError(t, err)

		wrong := &LearningAnnotation{
			AnnotationID: "x1", SessionID: "sess-B",
			AgentSlug: "agent", Kind: LearningAnnotationKindTip,
			Audience: LearningAnnotationAudienceBeginner, Title: "T", Content: "C",
			EmittedAt: time.Now(),
		}
		err = j.Add(wrong)
		assert.Error(t, err)
		assert.ErrorIs(t, err, ErrLearningAnnotationSessionMismatch)
		assert.Equal(t, 0, j.Size())
	})

	t.Run("Scenario_KnowledgeGapAnnotationsAccelerateOnboarding", func(t *testing.T) {
		// Given an operator is unaware of the skill-reuse feature,
		// When the agent detects repeated manual skill duplication,
		// Then a knowledge_gap annotation is emitted so the operator
		// can discover the feature organically from the learning feed.
		j, err := NewLearningAnnotationJournal("sess-gap")
		require.NoError(t, err)

		gap := &LearningAnnotation{
			AnnotationID: "gap-skill-reuse",
			SessionID:    "sess-gap",
			AgentSlug:    "advisor",
			Kind:         LearningAnnotationKindKnowledgeGap,
			Audience:     LearningAnnotationAudienceIntermediate,
			Title:        "Skill Reuse Available",
			Content:      "You created identical skills in two agents. Consider binding a single shared skill to both.",
			RelatedFeatureSlugs: []string{"skill-tool-binding"},
			EmittedAt:    time.Now(),
		}
		require.NoError(t, j.Add(gap))

		gaps := j.ListByKind(LearningAnnotationKindKnowledgeGap)
		require.Len(t, gaps, 1)
		assert.Equal(t, "gap-skill-reuse", gaps[0].AnnotationID)
		assert.Equal(t, []string{"skill-tool-binding"}, gaps[0].RelatedFeatureSlugs)
	})
}
