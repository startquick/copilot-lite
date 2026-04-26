package copilot

import (
	"testing"
)

func TestModelToMode(t *testing.T) {
	tests := []struct {
		model string
		want  string
	}{
		{"gpt-4o", "smart"},
		{"gpt-4", "smart"},
		{"gpt-4o-mini", "smart"},
		{"copilot-free", "smart"},
		{"copilot-premium", "smart"},
		{"o1", "reasoning"},       // reasoning models map to Copilot reasoning mode
		{"o1-mini", "reasoning"},
		{"o3", "reasoning"},
		{"o3-mini", "reasoning"},
		{"unknown-model", "smart"}, // fallback
		{"", "smart"},              // empty → fallback
	}
	for _, tc := range tests {
		got := modelToMode(tc.model)
		if got != tc.want {
			t.Errorf("modelToMode(%q) = %q, want %q", tc.model, got, tc.want)
		}
	}
}

func TestFlattenMessages(t *testing.T) {
	tests := []struct {
		name     string
		messages []Message
		want     string
	}{
		{
			name:     "empty",
			messages: []Message{},
			want:     "",
		},
		{
			name:     "single user message",
			messages: []Message{{Role: "user", Content: "Hello"}},
			want:     "Hello",
		},
		{
			name: "system + user",
			messages: []Message{
				{Role: "system", Content: "Be helpful."},
				{Role: "user", Content: "Hi"},
			},
			want: "[system]: Be helpful.\n\nHi",
		},
		{
			name: "system + assistant + user",
			messages: []Message{
				{Role: "system", Content: "You are a bot."},
				{Role: "assistant", Content: "How can I help?"},
				{Role: "user", Content: "Tell me a joke"},
			},
			want: "[system]: You are a bot.\n\n[assistant]: How can I help?\n\nTell me a joke",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := flattenMessages(tc.messages)
			if got != tc.want {
				t.Errorf("flattenMessages() = %q, want %q", got, tc.want)
			}
		})
	}
}
