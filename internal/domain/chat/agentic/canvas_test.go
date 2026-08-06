package agentic

import (
	"encoding/json"
	"testing"
)

// collectCanvasEvents drains a buffered channel and returns all events.
func collectCanvasEvents(ch chan RunEvent) []RunEvent {
	close(ch)
	var events []RunEvent
	for e := range ch {
		events = append(events, e)
	}
	return events
}

func TestHandleCanvasUpdate_Markdown(t *testing.T) {
	ch := make(chan RunEvent, 10)
	input := json.RawMessage(`{"content":"# Hello\nWorld","format":"markdown","title":"My Doc"}`)

	result := HandleCanvasUpdate("call-1", input, ch)

	if result.Error != nil {
		t.Fatalf("unexpected error: %s", *result.Error)
	}

	events := collectCanvasEvents(ch)
	canvasEvents := filterByType(events, EventCanvasUpdate)
	if len(canvasEvents) != 1 {
		t.Fatalf("expected 1 canvas_update event, got %d", len(canvasEvents))
	}

	var data CanvasUpdateData
	if err := json.Unmarshal(canvasEvents[0].Data, &data); err != nil {
		t.Fatalf("unmarshal CanvasUpdateData: %v", err)
	}
	if data.Format != CanvasFormatMarkdown {
		t.Errorf("expected format markdown, got %s", data.Format)
	}
	if data.Title != "My Doc" {
		t.Errorf("expected title 'My Doc', got %q", data.Title)
	}
	if data.Content != "# Hello\nWorld" {
		t.Errorf("unexpected content: %q", data.Content)
	}
	if data.ID == "" {
		t.Error("expected non-empty artifact ID")
	}
}

func TestHandleCanvasUpdate_DefaultsToMarkdown(t *testing.T) {
	ch := make(chan RunEvent, 10)
	input := json.RawMessage(`{"content":"hello"}`)

	result := HandleCanvasUpdate("call-1", input, ch)

	if result.Error != nil {
		t.Fatalf("unexpected error: %s", *result.Error)
	}
	events := collectCanvasEvents(ch)
	canvasEvents := filterByType(events, EventCanvasUpdate)
	if len(canvasEvents) != 1 {
		t.Fatalf("expected 1 canvas event, got %d", len(canvasEvents))
	}
	var data CanvasUpdateData
	if err := json.Unmarshal(canvasEvents[0].Data, &data); err != nil {
		t.Fatalf("unmarshal canvas event: %v", err)
	}
	if data.Format != CanvasFormatMarkdown {
		t.Errorf("expected default markdown, got %s", data.Format)
	}
}

func TestHandleCanvasUpdate_EmptyContent(t *testing.T) {
	ch := make(chan RunEvent, 10)
	input := json.RawMessage(`{"content":""}`)

	result := HandleCanvasUpdate("call-1", input, ch)

	if result.Error == nil {
		t.Error("expected error for empty content")
	}
}

func TestHandleCanvasUpdate_PreservesID(t *testing.T) {
	ch := make(chan RunEvent, 10)
	input := json.RawMessage(`{"content":"updated","id":"my-artifact-id"}`)

	result := HandleCanvasUpdate("call-1", input, ch)

	if result.Error != nil {
		t.Fatalf("unexpected error: %s", *result.Error)
	}
	events := collectCanvasEvents(ch)
	canvasEvents := filterByType(events, EventCanvasUpdate)
	var data CanvasUpdateData
	if err := json.Unmarshal(canvasEvents[0].Data, &data); err != nil {
		t.Fatalf("unmarshal canvas event: %v", err)
	}
	if data.ID != "my-artifact-id" {
		t.Errorf("expected ID 'my-artifact-id', got %q", data.ID)
	}
}

func TestHandleCanvasFeedback_Thumbs(t *testing.T) {
	ch := make(chan RunEvent, 10)
	input := json.RawMessage(`{"question":"Was this helpful?","type":"thumbs"}`)

	result := HandleCanvasFeedback("call-1", input, ch)

	if result.Error != nil {
		t.Fatalf("unexpected error: %s", *result.Error)
	}
	events := collectCanvasEvents(ch)
	inputEvents := filterByType(events, EventInputRequest)
	if len(inputEvents) != 1 {
		t.Fatalf("expected 1 input_request event, got %d", len(inputEvents))
	}
	var data InputRequestData
	if err := json.Unmarshal(inputEvents[0].Data, &data); err != nil {
		t.Fatalf("unmarshal input request event: %v", err)
	}
	if data.RequestID == "" {
		t.Error("expected non-empty requestId")
	}
	var payload UiFormPayloadDto
	if err := json.Unmarshal(data.Payload, &payload); err != nil {
		t.Fatalf("unmarshal input request payload: %v", err)
	}
	if len(payload.Elements) == 0 {
		t.Error("expected form elements")
	}
	if payload.Elements[0].Type != "radio" {
		t.Errorf("expected radio element for thumbs, got %s", payload.Elements[0].Type)
	}
}

func TestHandleCanvasFeedback_EmptyQuestion(t *testing.T) {
	ch := make(chan RunEvent, 10)
	input := json.RawMessage(`{"question":""}`)

	result := HandleCanvasFeedback("call-1", input, ch)

	if result.Error == nil {
		t.Error("expected error for empty question")
	}
}

func TestHandleCanvasExportTable(t *testing.T) {
	ch := make(chan RunEvent, 10)
	input := json.RawMessage(`{
		"title": "Sales Data",
		"columns": ["Product","Q1","Q2"],
		"rows": [["Widget","100","200"],["Gadget","50","75"]],
		"filename": "sales-2024"
	}`)

	result := HandleCanvasExportTable("call-1", input, ch)

	if result.Error != nil {
		t.Fatalf("unexpected error: %s", *result.Error)
	}
	events := collectCanvasEvents(ch)
	canvasEvents := filterByType(events, EventCanvasUpdate)
	if len(canvasEvents) != 1 {
		t.Fatalf("expected 1 canvas event, got %d", len(canvasEvents))
	}
	var data CanvasUpdateData
	if err := json.Unmarshal(canvasEvents[0].Data, &data); err != nil {
		t.Fatalf("unmarshal canvas event: %v", err)
	}
	if data.Format != CanvasFormatTable {
		t.Errorf("expected table format, got %s", data.Format)
	}
	if !data.Exportable {
		t.Error("expected exportable=true for table")
	}
	if data.Title != "Sales Data" {
		t.Errorf("expected title 'Sales Data', got %q", data.Title)
	}
}

func TestHandleCanvasExportTable_MissingColumns(t *testing.T) {
	ch := make(chan RunEvent, 10)
	input := json.RawMessage(`{"rows":[["a","b"]]}`)

	result := HandleCanvasExportTable("call-1", input, ch)

	if result.Error == nil {
		t.Error("expected error for missing columns")
	}
}

func TestIsCanvasToolCall(t *testing.T) {
	cases := []struct {
		name     string
		expected bool
	}{
		{canvasUpdateName, true},
		{canvasFeedbackName, true},
		{canvasExportName, true},
		{"ask_user", false},
		{"tool_search", false},
		{"my_custom_skill", false},
	}
	for _, c := range cases {
		got := IsCanvasToolCall(c.name)
		if got != c.expected {
			t.Errorf("IsCanvasToolCall(%q) = %v, want %v", c.name, got, c.expected)
		}
	}
}

func TestCanvasToolDefinitions_HaveBuiltinFlag(t *testing.T) {
	tools := []LLMTool{canvasUpdateTool(), canvasFeedbackTool(), canvasExportTableTool()}
	for _, tool := range tools {
		if !tool.Builtin {
			t.Errorf("canvas tool %q should have Builtin=true", tool.Name)
		}
		if tool.Name == "" {
			t.Error("canvas tool should have a name")
		}
		if len(tool.InputSchema) == 0 {
			t.Errorf("canvas tool %q should have an input schema", tool.Name)
		}
	}
}

// filterByType returns events matching the given type from a slice.
func filterByType(events []RunEvent, typ RunEventType) []RunEvent {
	var out []RunEvent
	for _, e := range events {
		if e.Type == typ {
			out = append(out, e)
		}
	}
	return out
}
