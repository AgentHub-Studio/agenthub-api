package agentic

import (
	"fmt"
	"strings"
	"time"
)

// LearningDocumentRenderer serializes a LearningAnnotationJournal into a
// structured learning document suitable for display in the AgentHub web UI
// or export as a session-end report.
//
// PDF arXiv:2604.14228v1 §11: agents should augment human understanding by
// surfacing teachable moments at session end. The renderer is the bridge
// between the in-memory journal and the human-readable artifact.
type LearningDocumentRenderer struct {
	audience LearningAnnotationAudience
	format   LearningDocumentFormat
}

// LearningDocumentFormat controls the serialization output.
type LearningDocumentFormat string

const (
	// LearningDocumentMarkdown produces GitHub-flavored Markdown.
	LearningDocumentMarkdown LearningDocumentFormat = "markdown"
	// LearningDocumentPlainText produces plain text without markup.
	LearningDocumentPlainText LearningDocumentFormat = "plain"
)

// IsValid returns true when f is in the closed set.
func (f LearningDocumentFormat) IsValid() bool {
	return f == LearningDocumentMarkdown || f == LearningDocumentPlainText
}

// LearningDocument is the output artifact of the renderer.
type LearningDocument struct {
	// SessionID is the session this document was generated from.
	SessionID string
	// Audience is the audience level the document was filtered for.
	Audience LearningAnnotationAudience
	// Format is the serialization format.
	Format LearningDocumentFormat
	// GeneratedAt is when the document was produced.
	GeneratedAt time.Time
	// TotalAnnotations is the count of annotations in the source journal.
	TotalAnnotations int
	// FilteredAnnotations is the count after audience filtering.
	FilteredAnnotations int
	// Sections groups annotations by kind.
	Sections []LearningDocumentSection
	// Body is the serialized output in the requested format.
	Body string
}

// LearningDocumentSection groups annotations of the same kind.
type LearningDocumentSection struct {
	Kind        LearningAnnotationKind
	Title       string
	Annotations []*LearningAnnotation
}

// sectionTitle returns a display-friendly title for a kind.
func sectionTitle(kind LearningAnnotationKind) string {
	switch kind {
	case LearningAnnotationKindPattern:
		return "Patterns Worth Repeating"
	case LearningAnnotationKindAntiPattern:
		return "Anti-patterns to Avoid"
	case LearningAnnotationKindTip:
		return "Tips & Shortcuts"
	case LearningAnnotationKindOptimization:
		return "Optimization Opportunities"
	case LearningAnnotationKindKnowledgeGap:
		return "Capabilities You May Not Know"
	default:
		return string(kind)
	}
}

// NewLearningDocumentRenderer creates a renderer for the given audience and format.
// Returns ErrLearningRendererInvalidAudience or ErrLearningRendererInvalidFormat on bad input.
func NewLearningDocumentRenderer(
	audience LearningAnnotationAudience,
	format LearningDocumentFormat,
) (*LearningDocumentRenderer, error) {
	if !audience.IsValid() {
		return nil, ErrLearningRendererInvalidAudience
	}
	if !format.IsValid() {
		return nil, ErrLearningRendererInvalidFormat
	}
	return &LearningDocumentRenderer{audience: audience, format: format}, nil
}

var (
	ErrLearningRendererInvalidAudience = fmt.Errorf("learning renderer: audience not in closed set")
	ErrLearningRendererInvalidFormat   = fmt.Errorf("learning renderer: format not in closed set (markdown|plain)")
	ErrLearningRendererNilJournal      = fmt.Errorf("learning renderer: journal must not be nil")
)

// Render produces a LearningDocument from the given journal.
// The journal's annotations are filtered by the renderer's audience level
// (progressive disclosure) and grouped by kind in a canonical order.
func (r *LearningDocumentRenderer) Render(j *LearningAnnotationJournal) (*LearningDocument, error) {
	if j == nil {
		return nil, ErrLearningRendererNilJournal
	}

	all := j.ListAll()
	filtered := j.FilterForAudience(r.audience)

	sections := r.buildSections(filtered)
	body := r.serialize(j.SessionID(), sections)

	return &LearningDocument{
		SessionID:           j.SessionID(),
		Audience:            r.audience,
		Format:              r.format,
		GeneratedAt:         time.Now().UTC(),
		TotalAnnotations:    len(all),
		FilteredAnnotations: len(filtered),
		Sections:            sections,
		Body:                body,
	}, nil
}

// kindOrder defines the canonical section order for the rendered document.
var kindOrder = []LearningAnnotationKind{
	LearningAnnotationKindPattern,
	LearningAnnotationKindTip,
	LearningAnnotationKindOptimization,
	LearningAnnotationKindAntiPattern,
	LearningAnnotationKindKnowledgeGap,
}

func (r *LearningDocumentRenderer) buildSections(annotations []*LearningAnnotation) []LearningDocumentSection {
	byKind := make(map[LearningAnnotationKind][]*LearningAnnotation)
	for _, a := range annotations {
		byKind[a.Kind] = append(byKind[a.Kind], a)
	}
	var sections []LearningDocumentSection
	for _, kind := range kindOrder {
		items := byKind[kind]
		if len(items) == 0 {
			continue
		}
		sections = append(sections, LearningDocumentSection{
			Kind:        kind,
			Title:       sectionTitle(kind),
			Annotations: items,
		})
	}
	return sections
}

func (r *LearningDocumentRenderer) serialize(sessionID string, sections []LearningDocumentSection) string {
	if r.format == LearningDocumentMarkdown {
		return r.serializeMarkdown(sessionID, sections)
	}
	return r.serializePlain(sessionID, sections)
}

func (r *LearningDocumentRenderer) serializeMarkdown(sessionID string, sections []LearningDocumentSection) string {
	var sb strings.Builder
	sb.WriteString("# Learning Report\n\n")
	sb.WriteString(fmt.Sprintf("**Session:** `%s`  \n", sessionID))
	sb.WriteString(fmt.Sprintf("**Audience:** %s\n\n", r.audience))

	if len(sections) == 0 {
		sb.WriteString("_No learning annotations recorded for this session._\n")
		return sb.String()
	}

	for _, sec := range sections {
		sb.WriteString(fmt.Sprintf("## %s\n\n", sec.Title))
		for _, a := range sec.Annotations {
			sb.WriteString(fmt.Sprintf("### %s\n\n", a.Title))
			sb.WriteString(a.Content)
			sb.WriteString("\n")
			if len(a.RelatedFeatureSlugs) > 0 {
				sb.WriteString(fmt.Sprintf("\n_Related features: %s_\n",
					strings.Join(a.RelatedFeatureSlugs, ", ")))
			}
			sb.WriteString("\n")
		}
	}
	return sb.String()
}

func (r *LearningDocumentRenderer) serializePlain(sessionID string, sections []LearningDocumentSection) string {
	var sb strings.Builder
	sb.WriteString("LEARNING REPORT\n")
	sb.WriteString(strings.Repeat("=", 40) + "\n")
	sb.WriteString(fmt.Sprintf("Session: %s\n", sessionID))
	sb.WriteString(fmt.Sprintf("Audience: %s\n\n", r.audience))

	if len(sections) == 0 {
		sb.WriteString("No learning annotations recorded for this session.\n")
		return sb.String()
	}

	for _, sec := range sections {
		sb.WriteString(strings.ToUpper(sec.Title) + "\n")
		sb.WriteString(strings.Repeat("-", 30) + "\n")
		for i, a := range sec.Annotations {
			sb.WriteString(fmt.Sprintf("%d. %s\n", i+1, a.Title))
			sb.WriteString(fmt.Sprintf("   %s\n\n", a.Content))
		}
	}
	return sb.String()
}

// IsEmpty returns true when the document has no filtered annotations.
func (d *LearningDocument) IsEmpty() bool {
	return d.FilteredAnnotations == 0
}

// HasKind returns true when the document includes a section for the given kind.
func (d *LearningDocument) HasKind(kind LearningAnnotationKind) bool {
	for _, s := range d.Sections {
		if s.Kind == kind {
			return true
		}
	}
	return false
}
