package flow

import (
	"errors"
	"strings"

	"github.com/crmmc/copilotpi/internal/copilot"
)

// buildCopilotRequest converts a flow.ChatRequest into a copilot.ChatRequest.
//
// It formats tool history into text, flattens multimodal content to plain
// text, and prepends a tool prompt when tools are present.
func (f *ChatFlow) buildCopilotRequest(req *ChatRequest) (*copilot.ChatRequest, error) {
	// Format tool history: convert tool_calls and tool-role messages to text
	formattedMessages := FormatToolHistory(req.Messages)

	// Build tool prompt if tools present
	toolPrompt := BuildToolPrompt(req.Tools, req.ToolChoice, req.ParallelToolCalls)

	msgs := make([]copilot.Message, 0, len(formattedMessages))

	for i, m := range formattedMessages {
		textContent := extractTextContent(m.Content)

		// Prepend tool prompt to first system message
		if i == 0 && m.Role == "system" && toolPrompt != "" {
			textContent = toolPrompt + "\n\n" + textContent
		}

		msgs = append(msgs, copilot.Message{Role: m.Role, Content: textContent})
	}

	// If no system message but tools present, prepend one
	if toolPrompt != "" && (len(msgs) == 0 || msgs[0].Role != "system") {
		sysMsg := copilot.Message{Role: "system", Content: toolPrompt}
		msgs = append([]copilot.Message{sysMsg}, msgs...)
	}

	// Validate: reject if all messages have empty content
	if allCopilotMessagesEmpty(msgs) {
		return nil, errors.New("all messages have empty content")
	}

	return &copilot.ChatRequest{
		Messages: msgs,
		Model:    req.Model,
		Stream:   true, // always stream
	}, nil
}

// extractTextContent converts any message content type to a plain string.
func extractTextContent(content any) string {
	switch c := content.(type) {
	case string:
		return c
	case []any:
		// Multimodal block array — extract text blocks only
		var sb strings.Builder
		for _, item := range c {
			if block, ok := item.(map[string]any); ok {
				if block["type"] == "text" {
					if text, ok := block["text"].(string); ok {
						if sb.Len() > 0 {
							sb.WriteString("\n")
						}
						sb.WriteString(text)
					}
				}
			}
		}
		return sb.String()
	case map[string]any:
		if text, ok := c["text"].(string); ok {
			return text
		}
	}
	return ""
}

// allCopilotMessagesEmpty returns true if every message has empty content.
func allCopilotMessagesEmpty(messages []copilot.Message) bool {
	for _, m := range messages {
		if strings.TrimSpace(m.Content) != "" {
			return false
		}
	}
	return true
}
