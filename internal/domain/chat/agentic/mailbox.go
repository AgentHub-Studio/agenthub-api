package agentic

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

// sendMessageToolName is the builtin tool name for inter-agent messaging.
const sendMessageToolName = "send_message"

// AgentMessage is a message exchanged between sub-agents via the mailbox.
type AgentMessage struct {
	ID        string    `json:"id"`
	From      string    `json:"from"`
	To        string    `json:"to"`
	Content   string    `json:"content"`
	Timestamp time.Time `json:"timestamp"`
	Read      bool      `json:"read"`
}

// Mailbox provides in-memory inter-agent messaging within a session.
// Messages are keyed by session ID so all sub-agents in the same run share
// the same mailbox. The mailbox is ephemeral — cleaned up when the run ends.
type Mailbox struct {
	mu       sync.Mutex
	sessions map[uuid.UUID][]AgentMessage
}

// NewMailbox creates a new Mailbox.
func NewMailbox() *Mailbox {
	return &Mailbox{
		sessions: make(map[uuid.UUID][]AgentMessage),
	}
}

// Send delivers a message from one sub-agent to another (or broadcast with to="*").
func (m *Mailbox) Send(sessionID uuid.UUID, from, to, content string) AgentMessage {
	m.mu.Lock()
	defer m.mu.Unlock()

	msg := AgentMessage{
		ID:        uuid.New().String(),
		From:      from,
		To:        to,
		Content:   content,
		Timestamp: time.Now(),
	}
	m.sessions[sessionID] = append(m.sessions[sessionID], msg)
	return msg
}

// ReadUnread returns all unread messages addressed to recipientID (or broadcast "*")
// and marks them as read.
func (m *Mailbox) ReadUnread(sessionID uuid.UUID, recipientID string) []AgentMessage {
	m.mu.Lock()
	defer m.mu.Unlock()

	msgs := m.sessions[sessionID]
	var result []AgentMessage
	for i := range msgs {
		if msgs[i].Read {
			continue
		}
		if msgs[i].To == recipientID || msgs[i].To == "*" {
			msgs[i].Read = true
			result = append(result, msgs[i])
		}
	}
	return result
}

// PendingCount returns the number of unread messages for a recipient.
func (m *Mailbox) PendingCount(sessionID uuid.UUID, recipientID string) int {
	m.mu.Lock()
	defer m.mu.Unlock()

	count := 0
	for _, msg := range m.sessions[sessionID] {
		if !msg.Read && (msg.To == recipientID || msg.To == "*") {
			count++
		}
	}
	return count
}

// Cleanup removes all messages for a session. Called at run end.
func (m *Mailbox) Cleanup(sessionID uuid.UUID) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.sessions, sessionID)
}

// Len returns the total number of messages in a session (read and unread).
func (m *Mailbox) Len(sessionID uuid.UUID) int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.sessions[sessionID])
}

// SendMessageInput is the expected JSON input for the send_message builtin tool.
type SendMessageInput struct {
	To      string `json:"to"`
	Message string `json:"message"`
}

// SendMessageOutput is the response from the send_message tool.
type SendMessageOutput struct {
	Status    string `json:"status"`
	MessageID string `json:"messageId"`
	To        string `json:"to"`
}

// HandleSendMessage processes a send_message tool call from a sub-agent.
// It validates input, delivers the message, and returns a ToolExecResult.
func HandleSendMessage(
	mailbox *Mailbox,
	sessionID uuid.UUID,
	subtaskID string,
	rawInput json.RawMessage,
	ch chan<- RunEvent,
) ToolExecResult {
	start := time.Now()

	var input SendMessageInput
	if err := json.Unmarshal(rawInput, &input); err != nil {
		errMsg := fmt.Sprintf("invalid send_message input: %s", err.Error())
		return ToolExecResult{Error: &errMsg, LatencyMs: 0}
	}
	if input.Message == "" {
		errMsg := "send_message requires a 'message' parameter"
		return ToolExecResult{Error: &errMsg, LatencyMs: 0}
	}
	if input.To == "" {
		errMsg := "send_message requires a 'to' parameter (subtask ID or '*' for broadcast)"
		return ToolExecResult{Error: &errMsg, LatencyMs: 0}
	}

	msg := mailbox.Send(sessionID, subtaskID, input.To, input.Message)

	// Emit agent_message event for frontend visibility.
	ch <- NewRunEvent(EventAgentMessage, AgentMessageData{
		ID:      msg.ID,
		From:    msg.From,
		To:      msg.To,
		Content: msg.Content,
	})

	output, _ := json.Marshal(SendMessageOutput{
		Status:    "delivered",
		MessageID: msg.ID,
		To:        input.To,
	})
	return ToolExecResult{
		Output:    output,
		LatencyMs: time.Since(start).Milliseconds(),
	}
}

// DrainMailbox reads pending messages for a sub-agent and formats them as
// a system message to inject into the conversation context.
// Returns empty string if no pending messages.
func DrainMailbox(mailbox *Mailbox, sessionID uuid.UUID, recipientID string) string {
	if mailbox == nil {
		return ""
	}
	msgs := mailbox.ReadUnread(sessionID, recipientID)
	if len(msgs) == 0 {
		return ""
	}

	result := "\n\n--- Messages from other agents ---\n"
	for _, msg := range msgs {
		result += fmt.Sprintf("[From %s]: %s\n", msg.From, msg.Content)
	}
	result += "--- End of messages ---"
	return result
}

// IsSendMessageToolCall returns true if the tool call is for the send_message builtin.
func IsSendMessageToolCall(name string) bool {
	return name == sendMessageToolName
}

// sendMessageTool returns the builtin send_message tool definition.
func sendMessageTool() LLMTool {
	return LLMTool{
		Name: sendMessageToolName,
		Description: "Send a message to another sub-agent in the same session. " +
			"Use this to share findings, coordinate work, or request information from other agents. " +
			"Set 'to' to a specific subtask ID or '*' to broadcast to all agents.",
		InputSchema: json.RawMessage(`{
			"type": "object",
			"properties": {
				"to": {
					"type": "string",
					"description": "Recipient subtask ID, or '*' to broadcast to all agents"
				},
				"message": {
					"type": "string",
					"description": "The message content to send"
				}
			},
			"required": ["to", "message"]
		}`),
	}
}
