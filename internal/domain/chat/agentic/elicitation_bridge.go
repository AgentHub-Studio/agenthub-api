package agentic

import "encoding/json"

// buildUiFormPayload converts ElicitationParams into a UiFormPayloadDto-compatible
// JSON payload that the Flutter client uses to render a dynamic form.
//
// It supports two input formats:
//   - "questions" mode (Claude Code style): each AskUserQuestion becomes a UiElementDto.
//   - "schema" mode (legacy): JSON Schema properties are mapped to UiElementDto.
//   - "url" mode: a single read-only text element showing the URL.
func buildUiFormPayload(params ElicitationParams) json.RawMessage {
	payload := UiFormPayloadDto{
		Type:        "form",
		Title:       params.Message,
		SubmitLabel: "Confirmar",
	}

	switch {
	case params.Mode == ElicitationModeURL:
		payload.Elements = []UiElementDto{
			{Type: "text", ID: "url", Label: params.URL},
		}

	case len(params.Questions) > 0:
		payload.Elements = buildFromQuestions(params.Questions)

	case len(params.RequestedSchema) > 0:
		payload.Elements = buildFormElements(params.RequestedSchema)

	default:
		payload.Elements = defaultTextElement()
	}

	raw, _ := json.Marshal(payload)
	return raw
}

// buildFromQuestions converts AskUserQuestion slice (Claude Code style) into
// UiElementDto entries. This is the preferred path — the LLM sends structured
// questions with type, options, and descriptions.
func buildFromQuestions(questions []AskUserQuestion) []UiElementDto {
	if len(questions) == 0 {
		return defaultTextElement()
	}

	elements := make([]UiElementDto, 0, len(questions))
	for _, q := range questions {
		el := UiElementDto{
			ID:    q.ID,
			Label: q.Question,
		}

		// Default required to true unless explicitly set to false.
		if q.Required == nil || *q.Required {
			el.Required = true
		}

		switch q.Type {
		case "select":
			el.Type = "select"
			opts := make([]UiOptionDto, len(q.Options))
			for j, o := range q.Options {
				opts[j] = UiOptionDto{Value: o.Label, Label: o.Label, Description: o.Description}
			}
			el.Options = opts

		case "confirm":
			el.Type = "checkbox"

		default: // "text" or empty
			el.Type = "text"
		}

		elements = append(elements, el)
	}

	return elements
}

// buildFormElements parses a JSON Schema object and converts its properties
// into UiElementDto entries. This is the legacy path for backwards compatibility
// when the LLM sends a schema instead of questions.
func buildFormElements(schema json.RawMessage) []UiElementDto {
	if len(schema) == 0 {
		return defaultTextElement()
	}

	var s struct {
		Properties map[string]struct {
			Type        string   `json:"type"`
			Title       string   `json:"title"`
			Description string   `json:"description"`
			Enum        []string `json:"enum"`
		} `json:"properties"`
		Required []string `json:"required"`
	}

	if err := json.Unmarshal(schema, &s); err != nil || len(s.Properties) == 0 {
		return defaultTextElement()
	}

	required := make(map[string]bool, len(s.Required))
	for _, r := range s.Required {
		required[r] = true
	}

	elements := make([]UiElementDto, 0, len(s.Properties))
	for name, prop := range s.Properties {
		label := prop.Title
		if label == "" {
			label = prop.Description
		}
		if label == "" {
			label = name
		}

		el := UiElementDto{
			ID:       name,
			Label:    label,
			Required: required[name],
		}

		switch {
		case len(prop.Enum) > 0:
			el.Type = "select"
			opts := make([]UiOptionDto, len(prop.Enum))
			for j, v := range prop.Enum {
				opts[j] = UiOptionDto{Value: v, Label: v}
			}
			el.Options = opts
		case prop.Type == "boolean":
			el.Type = "checkbox"
		default:
			el.Type = "text"
		}

		elements = append(elements, el)
	}

	return elements
}

// defaultTextElement returns a single text input as a fallback when the LLM
// calls ask_user without questions or schema.
func defaultTextElement() []UiElementDto {
	return []UiElementDto{
		{
			Type:     "text",
			ID:       "response",
			Label:    "Response",
			Required: true,
		},
	}
}
