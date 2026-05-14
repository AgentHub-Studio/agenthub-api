package agentic

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// BDD scenarios for HUMAN-007 — Learning-oriented documentation.
//
// PDF arXiv:2604.14228v1 §11: agents should augment human understanding by
// surfacing teachable moments at session end. The LearningDocumentRenderer
// is the bridge between the in-memory LearningAnnotationJournal and a
// human-readable learning report — completing the HUMAN-007 feature by
// providing the "documentation" serialization layer.

func TestBDD_LearningDocumentRenderer(t *testing.T) {
	t.Run("Scenario_RendererProducesMarkdownReportAtSessionEnd", func(t *testing.T) {
		// Given an agent session with several teachable moments recorded
		//       (PDF §11: surface teachable moments at session end),
		j, err := NewLearningAnnotationJournal("sess-bdd-render-1")
		require.NoError(t, err)
		require.NoError(t, j.Add(&LearningAnnotation{
			AnnotationID: "r1", SessionID: "sess-bdd-render-1",
			AgentSlug: "research-agent", Kind: LearningAnnotationKindPattern,
			Audience: LearningAnnotationAudienceBeginner,
			Title:    "Use RAG for large document sets",
			Content:  "Retrieving from vector index is 10× faster than full-text scan.",
			EmittedAt: time.Now(),
		}))

		// When the renderer produces a Markdown document for beginner audience,
		r, err := NewLearningDocumentRenderer(LearningAnnotationAudienceBeginner, LearningDocumentMarkdown)
		require.NoError(t, err)
		doc, err := r.Render(j)
		require.NoError(t, err)

		// Then the report starts with a Markdown heading, includes the session,
		// and includes the annotation title and content.
		assert.True(t, strings.HasPrefix(doc.Body, "# Learning Report"),
			"Markdown report must start with top-level heading")
		assert.Contains(t, doc.Body, "sess-bdd-render-1",
			"session ID must appear so the operator can cross-reference")
		assert.Contains(t, doc.Body, "Use RAG for large document sets",
			"annotation title must surface in the report")
	})

	t.Run("Scenario_ProgressiveDisclosureFiltersAudienceCorrectly", func(t *testing.T) {
		// Given a journal with a beginner tip and an advanced anti-pattern,
		j, err := NewLearningAnnotationJournal("sess-bdd-render-2")
		require.NoError(t, err)
		require.NoError(t, j.Add(&LearningAnnotation{
			AnnotationID: "beg", SessionID: "sess-bdd-render-2",
			AgentSlug: "ag", Kind: LearningAnnotationKindTip,
			Audience: LearningAnnotationAudienceBeginner,
			Title: "Keep prompts concise", Content: "Shorter prompts reduce latency.",
			EmittedAt: time.Now(),
		}))
		require.NoError(t, j.Add(&LearningAnnotation{
			AnnotationID: "adv", SessionID: "sess-bdd-render-2",
			AgentSlug: "ag", Kind: LearningAnnotationKindAntiPattern,
			Audience: LearningAnnotationAudienceAdvanced,
			Title: "Do not embed secrets in prompts", Content: "Use env injection instead.",
			EmittedAt: time.Now(),
		}))

		// When a beginner requests the report,
		r, err := NewLearningDocumentRenderer(LearningAnnotationAudienceBeginner, LearningDocumentMarkdown)
		require.NoError(t, err)
		doc, err := r.Render(j)
		require.NoError(t, err)

		// Then only the beginner tip surfaces — advanced anti-pattern is withheld
		// (PDF §3.2: progressive disclosure — reveal complexity as confidence grows).
		assert.Equal(t, 1, doc.FilteredAnnotations,
			"beginner filter must exclude advanced annotation")
		assert.Contains(t, doc.Body, "Keep prompts concise")
		assert.NotContains(t, doc.Body, "Do not embed secrets",
			"advanced content must not leak into beginner report")
	})

	t.Run("Scenario_EmptyJournalProducesGracefulEmptyReport", func(t *testing.T) {
		// Given an agent session with no annotations (common for short runs),
		j, err := NewLearningAnnotationJournal("sess-bdd-render-3")
		require.NoError(t, err)

		// When the renderer produces a report,
		r, err := NewLearningDocumentRenderer(LearningAnnotationAudienceIntermediate, LearningDocumentMarkdown)
		require.NoError(t, err)
		doc, err := r.Render(j)
		require.NoError(t, err)

		// Then the document is marked empty and the body signals it gracefully
		// rather than producing an empty string that could confuse the UI.
		assert.True(t, doc.IsEmpty())
		assert.Contains(t, doc.Body, "No learning annotations",
			"empty report must produce a human-readable placeholder")
	})

	t.Run("Scenario_SectionsArePresentedInCanonicalKindOrder", func(t *testing.T) {
		// Given a journal with annotations of every kind (in reverse order),
		j, err := NewLearningAnnotationJournal("sess-bdd-render-4")
		require.NoError(t, err)
		for _, kind := range []LearningAnnotationKind{
			LearningAnnotationKindKnowledgeGap,
			LearningAnnotationKindAntiPattern,
			LearningAnnotationKindOptimization,
			LearningAnnotationKindTip,
			LearningAnnotationKindPattern,
		} {
			require.NoError(t, j.Add(&LearningAnnotation{
				AnnotationID: "id-" + string(kind),
				SessionID:    "sess-bdd-render-4",
				AgentSlug:    "ag", Kind: kind,
				Audience: LearningAnnotationAudienceAdvanced,
				Title: string(kind), Content: "Detail.", EmittedAt: time.Now(),
			}))
		}

		// When rendered,
		r, err := NewLearningDocumentRenderer(LearningAnnotationAudienceAdvanced, LearningDocumentMarkdown)
		require.NoError(t, err)
		doc, err := r.Render(j)
		require.NoError(t, err)

		// Then sections appear in canonical order: pattern, tip, optimization,
		// anti_pattern, knowledge_gap — constructive before corrective.
		require.Len(t, doc.Sections, 5)
		assert.Equal(t, LearningAnnotationKindPattern, doc.Sections[0].Kind)
		assert.Equal(t, LearningAnnotationKindTip, doc.Sections[1].Kind)
		assert.Equal(t, LearningAnnotationKindOptimization, doc.Sections[2].Kind)
		assert.Equal(t, LearningAnnotationKindAntiPattern, doc.Sections[3].Kind)
		assert.Equal(t, LearningAnnotationKindKnowledgeGap, doc.Sections[4].Kind)
	})

	t.Run("Scenario_PlainTextFormatAvailableForLowBandwidth", func(t *testing.T) {
		// Given a journal with a tip annotation,
		j, err := NewLearningAnnotationJournal("sess-bdd-render-5")
		require.NoError(t, err)
		require.NoError(t, j.Add(&LearningAnnotation{
			AnnotationID: "t1", SessionID: "sess-bdd-render-5",
			AgentSlug: "ag", Kind: LearningAnnotationKindTip,
			Audience: LearningAnnotationAudienceBeginner,
			Title: "Batch requests", Content: "Batch reduces overhead.",
			EmittedAt: time.Now(),
		}))

		// When the plain-text format is requested (e.g. for webhook/email delivery),
		r, err := NewLearningDocumentRenderer(LearningAnnotationAudienceBeginner, LearningDocumentPlainText)
		require.NoError(t, err)
		doc, err := r.Render(j)
		require.NoError(t, err)

		// Then the output has no Markdown markup and starts with the plain header.
		assert.Equal(t, LearningDocumentPlainText, doc.Format)
		assert.False(t, strings.Contains(doc.Body, "##"),
			"plain text format must not contain Markdown heading syntax")
		assert.Contains(t, doc.Body, "Batch requests")
	})

	t.Run("Scenario_DocumentMetadataCapturesRenderContext", func(t *testing.T) {
		// Given a filled journal and a renderer configured for advanced audience,
		j := fillJournal(t, "sess-bdd-render-6")
		r, err := NewLearningDocumentRenderer(LearningAnnotationAudienceAdvanced, LearningDocumentMarkdown)
		require.NoError(t, err)

		// When rendered,
		doc, err := r.Render(j)
		require.NoError(t, err)

		// Then the document's metadata fields are set — operators can inspect
		// them without parsing the body (API consumers, dashboards).
		assert.Equal(t, "sess-bdd-render-6", doc.SessionID)
		assert.Equal(t, LearningAnnotationAudienceAdvanced, doc.Audience)
		assert.Equal(t, LearningDocumentMarkdown, doc.Format)
		assert.False(t, doc.GeneratedAt.IsZero())
		assert.Equal(t, 5, doc.TotalAnnotations, "fillJournal adds one per kind")
		assert.Equal(t, 5, doc.FilteredAnnotations, "advanced sees all annotations")
	})
}
