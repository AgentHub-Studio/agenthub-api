package channel

import "context"

// Adapter is the interface every channel platform must implement.
// VerifyRequest validates the platform's inbound request signature/token.
// ParseMessage extracts the normalised InboundMessage from the raw HTTP body.
// SendReply delivers a reply through the platform.
type Adapter interface {
	// VerifyRequest checks the platform-specific authentication of an inbound request.
	// Returns nil when the request is authentic, a non-nil error otherwise.
	VerifyRequest(ctx context.Context, ch Channel, headers map[string]string, body []byte) error

	// ParseMessage extracts a normalised InboundMessage from the raw platform payload.
	ParseMessage(ctx context.Context, ch Channel, body []byte) (InboundMessage, error)

	// SendReply delivers msg back through the platform channel.
	// Implementations should be idempotent where the platform allows it.
	SendReply(ctx context.Context, ch Channel, msg OutboundMessage) error
}

// Registry maps ChannelType values to their concrete Adapter implementations.
// Adapters are registered at startup; the HTTP inbound handler looks up the adapter
// by the channel's type and delegates verification, parsing, and reply sending.
type Registry struct {
	adapters map[ChannelType]Adapter
}

// NewRegistry creates an empty adapter registry.
func NewRegistry() *Registry {
	return &Registry{adapters: make(map[ChannelType]Adapter)}
}

// Register adds an adapter for the given channel type. Panics on duplicate.
func (r *Registry) Register(t ChannelType, a Adapter) {
	if _, ok := r.adapters[t]; ok {
		panic("channel: adapter already registered for type " + string(t))
	}
	r.adapters[t] = a
}

// Lookup returns the adapter for t, or (nil, false) if not found.
func (r *Registry) Lookup(t ChannelType) (Adapter, bool) {
	a, ok := r.adapters[t]
	return a, ok
}
