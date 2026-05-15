package agentic

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// helper: populate a journal with one annotation per kind.
func fillJournal(t *testing.T, sid string) *LearningAnnotationJournal {
	t.Helper()
	j, err := NewLearningAnnotationJournal(sid)
	require.NoError(t, err)
	for i, kind := range AllLearningAnnotationKinds() {
		a := &LearningAnnotation{
			AnnotationID: sid + "-" + string(kind),
			SessionID:    sid,
			AgentSlug:    "test-agent",
			Kind:         kind,
			Audience:     LearningAnnotationAudienceBeginner,
			Title:        "Title for " + string(kind),
			Content:      strings.Repeat("Content ", i+3),
			EmittedAt:    time.Now(),
		}
		require.NoError(t, j.Add(a))
	}
	return j
}

func TestLearningDocumentRenderer_InvalidAudience(t *testing.T) {
	_, err := NewLearningDocumentRenderer("unknown", LearningDocumentMarkdown)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrLearningRendererInvalidAudience)
}

func TestLearningDocumentRenderer_InvalidFormat(t *testing.T) {
	_, err := NewLearningDocumentRenderer(LearningAnnotationAudienceBeginner, "json")
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrLearningRendererInvalidFormat)
}

func TestLearningDocumentRenderer_NilJournal(t *testing.T) {
	r, err := NewLearningDocumentRenderer(LearningAnnotationAudienceBeginner, LearningDocumentMarkdown)
	require.NoError(t, err)
	_, err = r.Render(nil)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrLearningRendererNilJournal)
}

func TestLearningDocumentRenderer_EmptyJournalProducesEmptyDoc(t *testing.T) {
	j, _ := NewLearningAnnotationJournal("empty-session")
	r, _ := NewLearningDocumentRenderer(LearningAnnotationAudienceBeginner, LearningDocumentMarkdown)
	doc, err := r.Render(j)
	require.NoError(t, err)
	assert.True(t, doc.IsEmpty())
	assert.Equal(t, 0, doc.FilteredAnnotations)
	assert.Empty(t, doc.Sections)
}

func TestLearningDocumentRenderer_MarkdownBodyContainsSessionAndAudience(t *testing.T) {
	j := fillJournal(t, "sess-md-1")
	r, _ := NewLearningDocumentRenderer(LearningAnnotationAudienceBeginner, LearningDocumentMarkdown)
	doc, err := r.Render(j)
	require.NoError(t, err)
	assert.Contains(t, doc.Body, "sess-md-1")
	assert.Contains(t, doc.Body, "beginner")
	assert.True(t, strings.HasPrefix(doc.Body, "# Learning Report"))
}

func TestLearningDocumentRenderer_PlainBodyContainsSessionAndAudience(t *testing.T) {
	j := fillJournal(t, "sess-plain-1")
	r, _ := NewLearningDocumentRenderer(LearningAnnotationAudienceBeginner, LearningDocumentPlainText)
	doc, err := r.Render(j)
	require.NoError(t, err)
	assert.Contains(t, doc.Body, "sess-plain-1")
	assert.Contains(t, doc.Body, "beginner")
	assert.True(t, strings.HasPrefix(doc.Body, "LEARNING REPORT"))
}

func TestLearningDocumentRenderer_AllKindsProduceSections(t *testing.T) {
	j := fillJournal(t, "sess-kinds")
	r, _ := NewLearningDocumentRenderer(LearningAnnotationAudienceBeginner, LearningDocumentMarkdown)
	doc, err := r.Render(j)
	require.NoError(t, err)
	assert.Equal(t, 5, len(doc.Sections))
	for _, kind := range AllLearningAnnotationKinds() {
		assert.True(t, doc.HasKind(kind), "missing section for kind %q", kind)
	}
}

func TestLearningDocumentRenderer_TotalVsFilteredCounts(t *testing.T) {
	j, _ := NewLearningAnnotationJournal("sess-counts")
	for _, pair := range []struct {
		id  string
		aud LearningAnnotationAudience
	}{
		{"a1", LearningAnnotationAudienceBeginner},
		{"a2", LearningAnnotationAudienceIntermediate},
		{"a3", LearningAnnotationAudienceAdvanced},
	} {
		_ = j.Add(&LearningAnnotation{
			AnnotationID: pair.id, SessionID: "sess-counts",
			AgentSlug: "ag", Kind: LearningAnnotationKindTip,
			Audience: pair.aud, Title: "T", Content: "C", EmittedAt: time.Now(),
		})
	}
	r, _ := NewLearningDocumentRenderer(LearningAnnotationAudienceBeginner, LearningDocumentMarkdown)
	doc, err := r.Render(j)
	require.NoError(t, err)
	assert.Equal(t, 3, doc.TotalAnnotations)
	assert.Equal(t, 1, doc.FilteredAnnotations, "beginner sees only beginner annotations")
}

func TestLearningDocumentRenderer_SessionIDMatchesJournal(t *testing.T) {
	j := fillJournal(t, "my-session")
	r, _ := NewLearningDocumentRenderer(LearningAnnotationAudienceAdvanced, LearningDocumentMarkdown)
	doc, err := r.Render(j)
	require.NoError(t, err)
	assert.Equal(t, "my-session", doc.SessionID)
}

func TestLearningDocumentRenderer_FormatIsValid(t *testing.T) {
	assert.True(t, LearningDocumentMarkdown.IsValid())
	assert.True(t, LearningDocumentPlainText.IsValid())
	assert.False(t, LearningDocumentFormat("html").IsValid())
}

func TestLearningDocumentRenderer_RelatedFeatureSlugsAppearInMarkdown(t *testing.T) {
	j, _ := NewLearningAnnotationJournal("sess-slugs")
	_ = j.Add(&LearningAnnotation{
		AnnotationID: "a1", SessionID: "sess-slugs",
		AgentSlug: "ag", Kind: LearningAnnotationKindPattern,
		Audience: LearningAnnotationAudienceBeginner, Title: "T", Content: "C",
		RelatedFeatureSlugs: []string{"rag-search", "memory-hierarchy"},
		EmittedAt:           time.Now(),
	})
	r, _ := NewLearningDocumentRenderer(LearningAnnotationAudienceBeginner, LearningDocumentMarkdown)
	doc, err := r.Render(j)
	require.NoError(t, err)
	assert.Contains(t, doc.Body, "rag-search")
	assert.Contains(t, doc.Body, "memory-hierarchy")
}
