package agentic

import (
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
)

// Canvas tool names — intercepted by the runner before reaching skill-runtime.
const (
	canvasUpdateName   = "canvas_update"
	canvasFeedbackName = "canvas_feedback"
	canvasExportName   = "canvas_export_table"
)

// IsCanvasToolCall returns true if the tool name is one of the canvas builtins.
func IsCanvasToolCall(name string) bool {
	switch name {
	case canvasUpdateName, canvasFeedbackName, canvasExportName:
		return true
	}
	return false
}

// canvasUpdateInput is the parsed input for canvas_update.
type canvasUpdateInput struct {
	Title      string `json:"title"`
	Content    string `json:"content"`
	Format     string `json:"format"` // "markdown" | "html" | "table"
	Exportable bool   `json:"exportable"`
	// ID allows the agent to update an existing canvas artifact.
	// When empty, a new ID is generated.
	ID string `json:"id"`
}

// canvasFeedbackInput is the parsed input for canvas_feedback.
type canvasFeedbackInput struct {
	Question string `json:"question"`
	// Type controls the feedback widget: "thumbs", "stars", or "text".
	Type string `json:"type"`
}

// canvasExportTableInput is the parsed input for canvas_export_table.
type canvasExportTableInput struct {
	Title    string     `json:"title"`
	Columns  []string   `json:"columns"`
	Rows     [][]string `json:"rows"`
	Filename string     `json:"filename"`
}

// HandleCanvasUpdate processes a canvas_update tool call.
// Emits an EventCanvasUpdate SSE event and returns a success result.
func HandleCanvasUpdate(callID string, raw json.RawMessage, ch chan<- RunEvent) ToolExecResult {
	var input canvasUpdateInput
	if err := json.Unmarshal(raw, &input); err != nil {
		errMsg := fmt.Sprintf("canvas_update: invalid input: %s", err.Error())
		return ToolExecResult{Error: &errMsg, ToolName: canvasUpdateName}
	}
	if input.Content == "" {
		errMsg := "canvas_update: content is required"
		return ToolExecResult{Error: &errMsg, ToolName: canvasUpdateName}
	}

	format := CanvasFormat(input.Format)
	switch format {
	case CanvasFormatMarkdown, CanvasFormatHTML, CanvasFormatTable:
		// valid
	default:
		format = CanvasFormatMarkdown // default to markdown
	}

	artifactID := input.ID
	if artifactID == "" {
		artifactID = uuid.New().String()[:8]
	}

	data := CanvasUpdateData{
		ID:         artifactID,
		Title:      input.Title,
		Format:     format,
		Content:    input.Content,
		Exportable: input.Exportable,
	}
	ch <- NewRunEvent(EventCanvasUpdate, data)

	result, _ := json.Marshal(map[string]string{
		"status":      "rendered",
		"artifact_id": artifactID,
	})
	return ToolExecResult{Output: result, ToolName: canvasUpdateName}
}

// HandleCanvasFeedback processes a canvas_feedback tool call.
// Converts to an elicitation input_request with a single question and returns
// a placeholder result (the actual response arrives via POST /elicitation/{id}/respond).
func HandleCanvasFeedback(callID string, raw json.RawMessage, ch chan<- RunEvent) ToolExecResult {
	var input canvasFeedbackInput
	if err := json.Unmarshal(raw, &input); err != nil {
		errMsg := fmt.Sprintf("canvas_feedback: invalid input: %s", err.Error())
		return ToolExecResult{Error: &errMsg, ToolName: canvasFeedbackName}
	}
	if input.Question == "" {
		errMsg := "canvas_feedback: question is required"
		return ToolExecResult{Error: &errMsg, ToolName: canvasFeedbackName}
	}

	feedbackType := input.Type
	if feedbackType == "" {
		feedbackType = "text"
	}

	// Build an elicitation form for feedback collection.
	requestID := uuid.New().String()
	var elements []UiElementDto
	switch feedbackType {
	case "thumbs":
		elements = []UiElementDto{{
			Type:  "radio",
			ID:    "feedback",
			Label: input.Question,
			Options: []UiOptionDto{
				{Value: "positive", Label: "👍 Yes"},
				{Value: "negative", Label: "👎 No"},
			},
		}}
	case "stars":
		elements = []UiElementDto{{
			Type:  "radio",
			ID:    "feedback",
			Label: input.Question,
			Options: []UiOptionDto{
				{Value: "1", Label: "⭐"},
				{Value: "2", Label: "⭐⭐"},
				{Value: "3", Label: "⭐⭐⭐"},
				{Value: "4", Label: "⭐⭐⭐⭐"},
				{Value: "5", Label: "⭐⭐⭐⭐⭐"},
			},
		}}
	default: // "text"
		elements = []UiElementDto{{
			Type:  "text",
			ID:    "feedback",
			Label: input.Question,
		}}
	}

	payload := UiFormPayloadDto{
		Type:        "feedback",
		Title:       input.Question,
		Elements:    elements,
		SubmitLabel: "Submit",
	}
	payloadJSON, _ := json.Marshal(payload)

	ch <- NewRunEvent(EventInputRequest, InputRequestData{
		RequestID: requestID,
		Payload:   payloadJSON,
	})

	result, _ := json.Marshal(map[string]string{
		"status":     "pending",
		"request_id": requestID,
	})
	return ToolExecResult{Output: result, ToolName: canvasFeedbackName}
}

// HandleCanvasExportTable processes a canvas_export_table tool call.
// Converts column/row data to a table artifact and emits EventCanvasUpdate.
func HandleCanvasExportTable(callID string, raw json.RawMessage, ch chan<- RunEvent) ToolExecResult {
	var input canvasExportTableInput
	if err := json.Unmarshal(raw, &input); err != nil {
		errMsg := fmt.Sprintf("canvas_export_table: invalid input: %s", err.Error())
		return ToolExecResult{Error: &errMsg, ToolName: canvasExportName}
	}
	if len(input.Columns) == 0 {
		errMsg := "canvas_export_table: columns is required"
		return ToolExecResult{Error: &errMsg, ToolName: canvasExportName}
	}

	// Encode structured table as JSON for the table canvas format.
	tableData := map[string]interface{}{
		"columns":  input.Columns,
		"rows":     input.Rows,
		"filename": input.Filename,
	}
	content, _ := json.Marshal(tableData)

	artifactID := uuid.New().String()[:8]
	title := input.Title
	if title == "" {
		title = "Table"
	}

	data := CanvasUpdateData{
		ID:         artifactID,
		Title:      title,
		Format:     CanvasFormatTable,
		Content:    string(content),
		Exportable: true,
	}
	ch <- NewRunEvent(EventCanvasUpdate, data)

	result, _ := json.Marshal(map[string]string{
		"status":      "exported",
		"artifact_id": artifactID,
		"rows":        fmt.Sprintf("%d", len(input.Rows)),
		"columns":     fmt.Sprintf("%d", len(input.Columns)),
	})
	return ToolExecResult{Output: result, ToolName: canvasExportName}
}

// --- LLMTool definitions ---

// canvasUpdateTool returns the builtin canvas_update tool definition.
// Allows the LLM to render rich content (markdown, HTML, table) in the canvas panel.
func canvasUpdateTool() LLMTool {
	return LLMTool{
		Name:     canvasUpdateName,
		Builtin:  true,
		ReadOnly: true,
		Description: `Render rich content in the canvas panel alongside the chat.
Use this to present formatted output that benefits from visual rendering:
- Markdown documents, reports, or summaries
- HTML-formatted content with structure
- Tabular data (use canvas_export_table for downloadable tables)

The canvas panel is displayed to the right of the chat window. Each call
renders or updates an artifact identified by its ID.

Formats:
- "markdown" (default): rendered with full markdown support and code syntax highlighting
- "html": sanitized HTML rendered in a sandboxed iframe
- "table": use canvas_export_table instead for table data

Set exportable=true to show a download button for the artifact.`,
		InputSchema: json.RawMessage(`{
			"type": "object",
			"properties": {
				"content": {
					"type": "string",
					"description": "The content to render. For markdown: valid CommonMark. For html: sanitized HTML."
				},
				"format": {
					"type": "string",
					"enum": ["markdown", "html"],
					"description": "Rendering format. Defaults to markdown.",
					"default": "markdown"
				},
				"title": {
					"type": "string",
					"description": "Optional title shown above the artifact"
				},
				"id": {
					"type": "string",
					"description": "Optional artifact ID. When provided, replaces an existing artifact with the same ID."
				},
				"exportable": {
					"type": "boolean",
					"description": "When true, shows a download button. Defaults to false.",
					"default": false
				}
			},
			"required": ["content"]
		}`),
	}
}

// canvasFeedbackTool returns the builtin canvas_feedback tool definition.
// Allows the LLM to collect structured feedback on canvas artifacts.
func canvasFeedbackTool() LLMTool {
	return LLMTool{
		Name:     canvasFeedbackName,
		Builtin:  true,
		ReadOnly: true,
		Description: `Collect structured feedback from the user about the canvas content.
Presents a lightweight feedback widget in the chat.

Types:
- "thumbs": simple 👍/👎 rating
- "stars": 1–5 star rating
- "text": free-text feedback

Use after rendering a canvas artifact to ask if the output meets the user's needs.`,
		InputSchema: json.RawMessage(`{
			"type": "object",
			"properties": {
				"question": {
					"type": "string",
					"description": "The feedback question to present to the user"
				},
				"type": {
					"type": "string",
					"enum": ["thumbs", "stars", "text"],
					"description": "Feedback widget type. Defaults to text.",
					"default": "text"
				}
			},
			"required": ["question"]
		}`),
	}
}

// canvasExportTableTool returns the builtin canvas_export_table tool definition.
// Allows the LLM to render tabular data as an interactive, downloadable table.
func canvasExportTableTool() LLMTool {
	return LLMTool{
		Name:     canvasExportName,
		Builtin:  true,
		ReadOnly: true,
		Description: `Render tabular data as an interactive table in the canvas panel with a CSV download button.

Use this instead of canvas_update when you have structured row/column data.
The user can sort columns and download the table as a CSV file.`,
		InputSchema: json.RawMessage(`{
			"type": "object",
			"properties": {
				"columns": {
					"type": "array",
					"items": {"type": "string"},
					"description": "Column header names"
				},
				"rows": {
					"type": "array",
					"items": {
						"type": "array",
						"items": {"type": "string"}
					},
					"description": "Table rows. Each row is an array of string values matching the columns order."
				},
				"title": {
					"type": "string",
					"description": "Optional table title shown above the artifact"
				},
				"filename": {
					"type": "string",
					"description": "Filename hint for CSV export (without extension). Defaults to 'export'."
				}
			},
			"required": ["columns", "rows"]
		}`),
	}
}
