package channel

import (
	"context"
	"encoding/json"
	"fmt"
)

// CustomAdapter is a pass-through adapter for generic webhooks.
// It expects the inbound body to contain a JSON object with a "text" field.
// No signature verification is performed; the inbound token alone authenticates.
// Outbound replies are not sent (the caller is expected to poll or use a separate webhook).
type CustomAdapter struct{}

// customInbound is the expected JSON shape for a CUSTOM channel inbound request.
type customInbound struct {
	Text    string `json:"text"`
	Sender  string `json:"sender,omitempty"`
	ReplyTo string `json:"replyTo,omitempty"`
}

// VerifyRequest performs no signature check for CUSTOM channels.
func (a *CustomAdapter) VerifyRequest(_ context.Context, _ Channel, _ map[string]string, _ []byte) error {
	return nil
}

// ParseMessage extracts text, sender, and replyTo from the JSON body.
func (a *CustomAdapter) ParseMessage(_ context.Context, _ Channel, body []byte) (InboundMessage, error) {
	var req customInbound
	if err := json.Unmarshal(body, &req); err != nil {
		return InboundMessage{}, fmt.Errorf("custom: parse error: %w", err)
	}
	raw, _ := json.Marshal(req)
	return InboundMessage{
		SenderID: req.Sender,
		Text:     req.Text,
		ReplyTo:  req.ReplyTo,
		Raw:      raw,
	}, nil
}

// SendReply is a no-op for CUSTOM channels.
// Callers that need outbound delivery should implement their own webhook.
func (a *CustomAdapter) SendReply(_ context.Context, _ Channel, _ OutboundMessage) error {
	return nil
}

// Ensure CustomAdapter satisfies the Adapter interface at compile time.
var _ Adapter = (*CustomAdapter)(nil)
