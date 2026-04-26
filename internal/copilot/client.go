package copilot

import (
	"context"

	"github.com/crmmc/copilotpi/internal/config"
)

// StreamEvent is a single streaming event from the Copilot chat endpoint.
type StreamEvent struct {
	// Text is the incremental text chunk from an appendText event.
	Text string
	// Done signals the end of the stream.
	Done bool
	// Err holds a transport/protocol error, if any.
	Err error
}

// Client is the interface that the flow layer uses to communicate with Copilot.
// It mirrors the contract of the old xai.Client so the flow layer can swap
// implementations without modification.
type Client interface {
	// Chat sends a chat request and returns a channel of StreamEvents.
	// The channel is closed after the Done event or an error.
	Chat(ctx context.Context, req *ChatRequest) (<-chan StreamEvent, error)

	// ResetSession discards the current WebSocket connection and forces a
	// fresh reconnect on the next Chat() call.
	ResetSession() error

	// Close shuts down the client and all its goroutines.
	Close() error

	// DownloadURL downloadsdata from a URL using the authenticated session.
	// For Copilot, this is a no-op stub returning nil (no media downloads).
	DownloadURL(ctx context.Context, url string) ([]byte, error)
}

// ChatRequest represents the parameters for a Copilot chat completion.
type ChatRequest struct {
	// ConversationID is the target conversation. If empty, a new one is created.
	ConversationID string
	// Messages is the OpenAI-style conversation history.
	Messages []Message
	// Model is the OpenAI model name requested by the caller (e.g. "gpt-4o").
	// It will be mapped to a Copilot mode internally.
	Model string
	// Stream controls whether to use streaming output.
	Stream bool
}

// Message is a single conversation turn.
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// NewClient creates a new Copilot WebSocket client for the given cookie bundle.
func NewClient(cookieBundle string, cfg *config.CopilotConfig) (Client, error) {
	return newWSClient(cookieBundle, cfg)
}
