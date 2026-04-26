package flow

import (
	"context"
	"time"

	"github.com/crmmc/copilotpi/internal/copilot"
)

// streamEvents reads from the copilot event channel and forwards converted
// flow.StreamEvents to outCh.
//
// Returns: (success, usage, estimated, ttft, err)
func (f *ChatFlow) streamEvents(ctx context.Context, eventCh <-chan copilot.StreamEvent, outCh chan<- StreamEvent, tools []Tool) (bool, *Usage, bool, time.Duration, error) {
	var outputChars int
	var ttft time.Duration
	streamStart := time.Now()
	gotFirstToken := false
	filterTags := f.filterTags()
	tokenFilter := newStreamTokenFilter(filterTags)
	toolParser := newStreamToolCallParser(tools)

	for {
		select {
		case <-ctx.Done():
			return false, nil, false, 0, ctx.Err()

		case ev, ok := <-eventCh:
			if !ok {
				// Channel closed normally — flush and finish
				outputChars += flushStreamParsers(outCh, nil, streamStart, &ttft, &gotFirstToken, tokenFilter, toolParser)
				usage := &Usage{CompletionTokens: estimateTokens(outputChars)}
				usage.TotalTokens = usage.PromptTokens + usage.CompletionTokens
				stop := "stop"
				outCh <- StreamEvent{FinishReason: &stop, Usage: usage}
				return true, usage, true, ttft, nil
			}

			// Convert copilot event → flow event
			flowEvent := parseCopilotEvent(ev)

			if flowEvent.Error != nil {
				return false, nil, false, 0, flowEvent.Error
			}

			// Handle done signal from copilot
			if ev.Done {
				outputChars += flushStreamParsers(outCh, nil, streamStart, &ttft, &gotFirstToken, tokenFilter, toolParser)
				usage := &Usage{CompletionTokens: estimateTokens(outputChars)}
				usage.TotalTokens = usage.PromptTokens + usage.CompletionTokens
				stop := "stop"
				outCh <- StreamEvent{FinishReason: &stop, Usage: usage}
				return true, usage, true, ttft, nil
			}

			// Filter and forward text
			flowEvent = tokenFilter.Apply(flowEvent)
			flowEvent.Content, flowEvent.ToolCalls = toolParser.Push(flowEvent.Content)
			outputChars += emitStreamEvent(outCh, nil, streamStart, &ttft, &gotFirstToken, flowEvent)
		}
	}
}

func flushStreamParsers(outCh chan<- StreamEvent, dl DownloadFunc, streamStart time.Time, ttft *time.Duration, gotFirstToken *bool, tokenFilter *streamTokenFilter, toolParser *streamToolCallParser) int {
	var outputChars int
	pending := tokenFilter.Flush("")
	if pending != "" {
		text, calls := toolParser.Push(pending)
		outputChars += emitStreamEvent(outCh, dl, streamStart, ttft, gotFirstToken, StreamEvent{
			Content:   text,
			ToolCalls: calls,
		})
	}

	text, calls := toolParser.Flush()
	outputChars += emitStreamEvent(outCh, dl, streamStart, ttft, gotFirstToken, StreamEvent{
		Content:   text,
		ToolCalls: calls,
	})
	return outputChars
}

func emitStreamEvent(outCh chan<- StreamEvent, _ DownloadFunc, streamStart time.Time, ttft *time.Duration, gotFirstToken *bool, event StreamEvent) int {
	contentLen := len(event.Content)
	if !*gotFirstToken && contentLen > 0 {
		*ttft = time.Since(streamStart)
		*gotFirstToken = true
	}
	if event.Content == "" && len(event.ToolCalls) == 0 && event.Usage == nil {
		return 0
	}
	outCh <- event
	return contentLen
}

// estimateTokens provides a rough token count from character length.
// ~4 chars per token for English, ~2 for CJK — use 3 as a balanced average.
func estimateTokens(chars int) int {
	if chars <= 0 {
		return 0
	}
	return (chars + 2) / 3
}
