package flow

import (
	"context"
	"time"

	"github.com/crmmc/copilotpi/internal/copilot"
	"github.com/crmmc/copilotpi/internal/store"
)

// parseCopilotEvent converts a copilot.StreamEvent into a flow-level StreamEvent.
// Copilot streams plain text tokens; there is no JSON envelope to decode.
func parseCopilotEvent(ev copilot.StreamEvent) StreamEvent {
	if ev.Err != nil {
		return StreamEvent{Error: ev.Err}
	}
	return StreamEvent{Content: ev.Text}
}

// estimatePromptTokens estimates input token count from request messages.
func (f *ChatFlow) estimatePromptTokens(req *ChatRequest) int {
	var chars int
	for _, m := range req.Messages {
		chars += len(m.Role)
		switch c := m.Content.(type) {
		case string:
			chars += len(c)
		}
	}
	return estimateTokens(chars)
}

// recordUsage records an API usage log entry via the buffer (non-blocking).
func (f *ChatFlow) recordUsage(apiKeyID uint, tokenID uint, model, endpoint string, status int, latency time.Duration, ttft time.Duration, tokensInput, tokensOutput int, estimated bool) {
	if f.usageLog == nil {
		return
	}
	_ = f.usageLog.Record(context.Background(), &store.UsageLog{
		APIKeyID:     apiKeyID,
		TokenID:      tokenID,
		Model:        model,
		Endpoint:     endpoint,
		Status:       status,
		DurationMs:   latency.Milliseconds(),
		TTFTMs:       int(ttft.Milliseconds()),
		CacheTokens:  0,
		TokensInput:  tokensInput,
		TokensOutput: tokensOutput,
		Estimated:    estimated,
		CreatedAt:    time.Now(),
	})
}
