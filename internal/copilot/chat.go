package copilot

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
)

// wsEvent is the generic inbound WebSocket message envelope.
type wsEvent struct {
	Event      string `json:"event"`
	Type       string `json:"type"`
	Text       string `json:"text"`
	MessageID  string `json:"messageId"`
	PartID     string `json:"partId"`
	Code       int    `json:"code"`
	StatusCode int    `json:"statusCode"`
	Error      string `json:"error"`
}

// sendMsg marshals and sends a JSON message on the WebSocket.
// MUST be called with c.mu held.
func (c *wsClient) sendMsg(v interface{}) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return c.conn.WriteMessage(1 /* TextMessage */, data)
}

// Chat sends a chat request to Copilot over WebSocket and returns a channel
// of StreamEvents. The channel is closed when the server sends "done" or on error.
func (c *wsClient) Chat(ctx context.Context, req *ChatRequest) (<-chan StreamEvent, error) {
	c.mu.Lock()

	if err := c.ensureConnected(); err != nil {
		c.mu.Unlock()
		return nil, err
	}

	if err := c.sendConsent(); err != nil {
		// Consent failed — drop connection and surface error
		_ = c.conn.Close()
		c.conn = nil
		c.mu.Unlock()
		return nil, fmt.Errorf("%w: consent failed: %v", ErrDisconnected, err)
	}

	// Determine conversation ID
	convID := req.ConversationID
	if convID == "" {
		convID = newConversationID()
	}

	// Flatten messages into a single text block
	flatText := flattenMessages(req.Messages)

	// Map model to Copilot mode
	mode := modelToMode(req.Model)

	// Build send payload
	payload := map[string]interface{}{
		"event":          "send",
		"conversationId": convID,
		"content": []map[string]string{
			{"type": "text", "text": flatText},
		},
		"mode":    mode,
		"context": map[string]interface{}{},
	}

	if err := c.sendMsg(payload); err != nil {
		_ = c.conn.Close()
		c.conn = nil
		c.mu.Unlock()
		return nil, fmt.Errorf("%w: send failed: %v", ErrDisconnected, err)
	}

	// Capture conn for this stream read — we unlock while reading
	conn := c.conn
	c.mu.Unlock()

	out := make(chan StreamEvent, 64)

	go func() {
		defer close(out)
		for {
			// Respect context cancellation
			if ctx.Err() != nil {
				out <- StreamEvent{Err: ctx.Err()}
				return
			}

			_, msg, err := conn.ReadMessage()
			if err != nil {
				// Connection dropped mid-stream
				c.mu.Lock()
				if c.conn == conn {
					c.conn = nil // invalidate so next call reconnects
				}
				c.mu.Unlock()
				out <- StreamEvent{Err: fmt.Errorf("%w: read error: %v", ErrDisconnected, err)}
				return
			}

			var ev wsEvent
			if err := json.Unmarshal(msg, &ev); err != nil {
				slog.Debug("copilot: failed to parse ws message", "raw", string(msg), "error", err)
				continue
			}

			switch ev.Event {
			case "appendText":
				if ev.Text != "" {
					out <- StreamEvent{Text: ev.Text}
				}

			case "done":
				out <- StreamEvent{Done: true}
				return

			case "pong":
				// Keepalive response — ignore

			case "error":
				code := ev.Code
				if code == 0 {
					code = ev.StatusCode
				}
				switch code {
				case 401:
					out <- StreamEvent{Err: ErrInvalidToken}
				case 403:
					out <- StreamEvent{Err: ErrForbidden}
				case 429:
					out <- StreamEvent{Err: ErrRateLimited}
				default:
					out <- StreamEvent{Err: fmt.Errorf("copilot: server error code=%d msg=%s", code, ev.Error)}
				}
				return

			default:
				// partCompleted, sequenceAck, connected, etc. — ignore
				slog.Debug("copilot: ignored ws event", "event", ev.Event)
			}
		}
	}()

	return out, nil
}
