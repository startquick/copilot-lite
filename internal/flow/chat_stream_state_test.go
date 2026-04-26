package flow

import (
	"context"
	"testing"

	"github.com/crmmc/copilotpi/internal/copilot"
	"github.com/crmmc/copilotpi/internal/store"
)

func TestChatFlow_TextPassthrough(t *testing.T) {
	tokenSvc := &mockTokenService{
		tokens: []*store.Token{{ID: 1, Token: "tok1", Pool: "basic"}},
	}
	client := &mockCopilotClient{
		events: []copilot.StreamEvent{
			{Text: "Hello"},
			{Text: " World"},
			{Text: "!"},
		},
	}
	flow := NewChatFlow(tokenSvc, func(token string) copilot.Client { return client }, &ChatFlowConfig{
		RetryConfig: DefaultRetryConfig(),
		TokenConfig: testFlowTokenConfig(),
	})

	ch, err := flow.Complete(context.Background(), &ChatRequest{
		Messages: []Message{{Role: "user", Content: "test"}},
		Model:    "copilot-free",
	})
	if err != nil {
		t.Fatalf("Complete() error = %v", err)
	}

	var content string
	for event := range ch {
		content += event.Content
	}
	if content != "Hello World!" {
		t.Fatalf("unexpected content: %q", content)
	}
}

func TestChatFlow_ToolCallsAcrossChunks(t *testing.T) {
	tokenSvc := &mockTokenService{
		tokens: []*store.Token{{ID: 1, Token: "tok1", Pool: "basic"}},
	}
	client := &mockCopilotClient{
		events: []copilot.StreamEvent{
			{Text: "I'll check.<tool_"},
			{Text: `call>{"name":"get_weather","arguments":{"location":"Tokyo"}}`},
			{Text: "</tool_call>done"},
		},
	}
	flow := NewChatFlow(tokenSvc, func(token string) copilot.Client { return client }, &ChatFlowConfig{
		RetryConfig: DefaultRetryConfig(),
		TokenConfig: testFlowTokenConfig(),
	})

	ch, err := flow.Complete(context.Background(), &ChatRequest{
		Messages: []Message{{Role: "user", Content: "weather"}},
		Model:    "copilot-free",
		Tools: []Tool{{
			Type: "function",
			Function: Function{
				Name:       "get_weather",
				Parameters: map[string]any{"type": "object"},
			},
		}},
		ToolChoice: "auto",
	})
	if err != nil {
		t.Fatalf("Complete() error = %v", err)
	}

	var content string
	var found []ToolCall
	for event := range ch {
		content += event.Content
		if len(event.ToolCalls) > 0 {
			found = append(found, event.ToolCalls...)
		}
	}
	if content != "I'll check.done" {
		t.Fatalf("unexpected streamed content: %q", content)
	}
	if len(found) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(found))
	}
	if found[0].Function.Name != "get_weather" {
		t.Fatalf("unexpected tool name: %q", found[0].Function.Name)
	}
	if found[0].Function.Arguments != `{"location":"Tokyo"}` {
		t.Fatalf("unexpected tool arguments: %q", found[0].Function.Arguments)
	}
}
