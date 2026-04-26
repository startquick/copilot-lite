package anthropic

import "github.com/crmmc/copilotpi/internal/flow"

// MessageRequest represents an Anthropic Messages API request.
type MessageRequest struct {
	Model         string         `json:"model"`
	Messages      []flow.Message `json:"messages"`
	System        any            `json:"system,omitempty"`
	MaxTokens     int            `json:"max_tokens,omitempty"`
	StopSequences []string       `json:"stop_sequences,omitempty"`
	Stream        bool           `json:"stream,omitempty"`
	Temperature   *float64       `json:"temperature,omitempty"`
	TopP          *float64       `json:"top_p,omitempty"`
	Tools         []flow.Tool    `json:"tools,omitempty"`
	ToolChoice    any            `json:"tool_choice,omitempty"`
}

// ContentBlock represents a content block returned by Anthropic.
type ContentBlock struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

// MessageResponse represents the blocking (non-stream) response from Anthropic.
type MessageResponse struct {
	ID           string         `json:"id"`
	Type         string         `json:"type"`
	Role         string         `json:"role"`
	Content      []ContentBlock `json:"content"`
	Model        string         `json:"model"`
	StopReason   string         `json:"stop_reason,omitempty"`
	StopSequence string         `json:"stop_sequence,omitempty"`
	Usage        *UsageData     `json:"usage,omitempty"`
}

// UsageData represents the token usage in Anthropic format.
type UsageData struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

// ErrorResponse represents an Anthropic API error.
type ErrorResponse struct {
	Type  string    `json:"type"`
	Error ErrorInfo `json:"error"`
}

// ErrorInfo represents details inside an Anthropic error.
type ErrorInfo struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}
