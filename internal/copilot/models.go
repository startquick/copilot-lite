package copilot

import "strings"

// modeMap maps model names to Copilot chat modes observed in DevTools.
// Modes confirmed: "smart" (standard), "reasoning" (o1-style deep think)
var modeMap = map[string]string{
	// Standard models → smart mode
	"gpt-4o":          "smart",
	"gpt-4":           "smart",
	"gpt-4o-mini":     "smart",
	"gpt-4-turbo":     "smart",
	"copilot-free":    "smart",
	"copilot-basic":   "smart",
	"copilot-premium": "smart",
	// Reasoning models → reasoning mode (o1/o3 style, deeper think)
	"o1":              "reasoning",
	"o1-mini":         "reasoning",
	"o3":              "reasoning",
	"o3-mini":         "reasoning",
	"copilot-reasoning": "reasoning",
}

// modelToMode returns the Copilot mode for a given model name.
// Falls back to "smart" for unknown models.
func modelToMode(model string) string {
	if mode, ok := modeMap[model]; ok {
		return mode
	}
	return "smart"
}

// flattenMessages converts an OpenAI-style messages slice into a single
// prompt string suitable for Copilot's single-content-block format.
//
// Rules:
//   - All messages except the final user message are prefixed with their role.
//   - The final user message is passed without a prefix.
//   - System messages are prefixed with "[system]: ".
//   - Assistant messages are prefixed with "[assistant]: ".
func flattenMessages(messages []Message) string {
	if len(messages) == 0 {
		return ""
	}

	var sb strings.Builder
	for i, m := range messages {
		isLast := i == len(messages)-1
		content := m.Content

		if isLast && m.Role == "user" {
			sb.WriteString(content)
		} else {
			switch m.Role {
			case "system":
				sb.WriteString("[system]: ")
			case "assistant":
				sb.WriteString("[assistant]: ")
			case "user":
				sb.WriteString("[user]: ")
			default:
				sb.WriteString("[" + m.Role + "]: ")
			}
			sb.WriteString(content)
		}

		if !isLast {
			sb.WriteString("\n\n")
		}
	}
	return sb.String()
}
