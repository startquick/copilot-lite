package openai

import "github.com/crmmc/copilotpi/internal/flow"

// ChatRequest represents an OpenAI-compatible chat completion request.
type ChatRequest struct {
	Model             string        `json:"model"`
	Messages          []ChatMessage `json:"messages"`
	Stream            *bool         `json:"stream,omitempty"`
	Temperature       *float64      `json:"temperature,omitempty"`
	TopP              *float64      `json:"top_p,omitempty"`
	MaxTokens         *int          `json:"max_tokens,omitempty"`
	Stop              []string      `json:"stop,omitempty"`
	PresencePenalty   *float64      `json:"presence_penalty,omitempty"`
	FrequencyPenalty  *float64      `json:"frequency_penalty,omitempty"`
	ReasoningEffort   string        `json:"reasoning_effort,omitempty"`
	Tools             []flow.Tool   `json:"tools,omitempty"`
	ToolChoice        any           `json:"tool_choice,omitempty"`
	ParallelToolCalls *bool         `json:"parallel_tool_calls,omitempty"`
}

// ChatMessage represents a single message in the conversation.
type ChatMessage struct {
	Role       string          `json:"role"`
	Content    any             `json:"content"`
	ToolCalls  []flow.ToolCall `json:"tool_calls,omitempty"`
	Name       string          `json:"name,omitempty"`
	ToolCallID string          `json:"tool_call_id,omitempty"`
}
