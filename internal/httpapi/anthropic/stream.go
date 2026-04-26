package anthropic

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/crmmc/copilotpi/internal/flow"
)

// streamResponse writes flow.StreamEvent items as Anthropic SSE events.
func (h *Handler) streamResponse(w http.ResponseWriter, r *http.Request, eventCh <-chan flow.StreamEvent, req *MessageRequest) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		h.writeError(w, http.StatusInternalServerError, "server_error", "Streaming not supported by client")
		return
	}

	// Set required headers for SSE
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)

	writeEvent := func(event string, data any) {
		bytes, _ := json.Marshal(data)
		fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, bytes)
		flusher.Flush()
	}

	msgID := generateID()

	// 1. message_start
	writeEvent("message_start", map[string]any{
		"type": "message_start",
		"message": map[string]any{
			"id":            msgID,
			"type":          "message",
			"role":          "assistant",
			"model":         req.Model,
			"stop_reason":   nil,
			"stop_sequence": nil,
			"content":       []any{},
			"usage": map[string]any{
				"input_tokens":  0, // We cannot estimate perfectly immediately, Grok flow sends at the end
				"output_tokens": 0,
			},
		},
	})

	// 2. content_block_start
	writeEvent("content_block_start", map[string]any{
		"type":  "content_block_start",
		"index": 0,
		"content_block": map[string]any{
			"type": "text",
			"text": "",
		},
	})

	// 3. content_block_delta ...
	for ev := range eventCh {
		if ev.Error != nil {
			// Anthropic error stream event
			writeEvent("error", ErrorResponse{
				Type: "error",
				Error: ErrorInfo{
					Type:    "api_error",
					Message: ev.Error.Error(),
				},
			})
			return // End stream gracefully on error
		}

		if ev.Content != "" {
			writeEvent("content_block_delta", map[string]any{
				"type":  "content_block_delta",
				"index": 0,
				"delta": map[string]any{
					"type": "text_delta",
					"text": ev.Content,
				},
			})
		}
	}

	// 4. content_block_stop
	writeEvent("content_block_stop", map[string]any{
		"type":  "content_block_stop",
		"index": 0,
	})

	// 5. message_delta
	writeEvent("message_delta", map[string]any{
		"type":  "message_delta",
		"delta": map[string]any{
			"stop_reason":   "end_turn",
			"stop_sequence": nil,
		},
		"usage": map[string]any{
			"output_tokens": 0, // In GrokPi lite, actual usage tokens log in background
		},
	})

	// 6. message_stop
	writeEvent("message_stop", map[string]any{
		"type": "message_stop",
	})
}

// blockingResponse fully loads all stream texts into a single MessageResponse.
func (h *Handler) blockingResponse(w http.ResponseWriter, r *http.Request, eventCh <-chan flow.StreamEvent, req *MessageRequest) {
	var fullContent string

	for ev := range eventCh {
		if ev.Error != nil {
			h.writeError(w, http.StatusInternalServerError, "api_error", ev.Error.Error())
			return
		}
		fullContent += ev.Content
	}

	resp := MessageResponse{
		ID:           generateID(),
		Type:         "message",
		Role:         "assistant",
		Model:        req.Model,
		StopReason:   "end_turn",
		StopSequence: "",
		Usage: &UsageData{
			InputTokens:  0, // Same note as streaming
			OutputTokens: 0,
		},
		Content: []ContentBlock{
			{
				Type: "text",
				Text: fullContent,
			},
		},
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(resp)
}
