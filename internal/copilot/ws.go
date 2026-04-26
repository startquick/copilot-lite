package copilot

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/crmmc/copilotpi/internal/config"
)

// TokenProvider supplies a live access token for WebSocket authentication.
// Implementations must check token expiry and refresh if necessary.
type TokenProvider interface {
	AccessToken(ctx context.Context) (string, error)
}

// wsClient is the WebSocket-backed Copilot client.
type wsClient struct {
	tokenProvider TokenProvider
	cfg           *config.CopilotConfig

	mu          sync.Mutex
	conn        *websocket.Conn
	consentSent bool
	closed      bool
}

// newWSClient creates a new wsClient but does not connect yet.
// The connection is established lazily on the first Chat() call.
func newWSClient(provider TokenProvider, cfg *config.CopilotConfig) (*wsClient, error) {
	if cfg == nil {
		cfg = &config.CopilotConfig{
			WSURL:        "wss://copilot.microsoft.com/c/api/chat",
			WSAPIVersion: "2",
			PingInterval: 25,
			ReconnectMax: 3,
			UserAgent:    "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/147.0.0.0 Safari/537.36 Edg/147.0.0.0",
		}
	}
	return &wsClient{
		tokenProvider: provider,
		cfg:           cfg,
	}, nil
}

// connect establishes a fresh WebSocket connection to Copilot.
// MUST be called with c.mu held.
func (c *wsClient) connect() error {
	if c.closed {
		return ErrDisconnected
	}

	// Obtain a fresh access token — refreshes if expired.
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var accessToken string
	if c.tokenProvider != nil {
		tok, err := c.tokenProvider.AccessToken(ctx)
		if err != nil {
			return fmt.Errorf("%w: %v", ErrInvalidToken, err)
		}
		accessToken = tok
	}

	if accessToken == "" {
		return ErrInvalidToken
	}

	sessionID := newClientSessionID()

	// URL format observed in DevTools:
	// wss://copilot.microsoft.com/c/api/chat?cursor=0&api-version=2&clientSessionId=<UUID>&accessToken=<JWT>
	wsURL := fmt.Sprintf("%s?cursor=0&api-version=%s&clientSessionId=%s&accessToken=%s",
		c.cfg.WSURL, c.cfg.WSAPIVersion, sessionID, accessToken)

	headers := buildUpgradeHeaders(c.cfg.UserAgent)

	dialer := websocket.Dialer{
		HandshakeTimeout: 20 * time.Second,
	}

	conn, resp, err := dialer.Dial(wsURL, headers)
	if err != nil {
		if resp != nil {
			slog.Debug("copilot: websocket upgrade failed",
				"status", resp.StatusCode,
				"url", c.cfg.WSURL)
			switch resp.StatusCode {
			case http.StatusUnauthorized:
				return ErrInvalidToken
			case http.StatusForbidden:
				return ErrForbidden
			case http.StatusTooManyRequests:
				return ErrRateLimited
			}
		} else {
			slog.Debug("copilot: websocket dial failed (no HTTP response)", "error", err)
		}
		return fmt.Errorf("%w: %v", ErrDisconnected, err)
	}

	c.conn = conn
	c.consentSent = false
	slog.Debug("copilot: websocket connected", "url", c.cfg.WSURL)
	return nil
}

// ensureConnected ensures an active WebSocket connection, reconnecting if needed.
// MUST be called with c.mu held.
func (c *wsClient) ensureConnected() error {
	if c.conn != nil {
		return nil
	}
	var lastErr error
	for attempt := 0; attempt <= c.cfg.ReconnectMax; attempt++ {
		if attempt > 0 {
			backoff := time.Duration(attempt) * 2 * time.Second
			slog.Debug("copilot: reconnect attempt", "attempt", attempt, "backoff", backoff)
			time.Sleep(backoff)
		}
		if err := c.connect(); err != nil {
			lastErr = err
			// Non-retryable errors
			if isNonRetryableConnErr(err) {
				return err
			}
			continue
		}
		return nil
	}
	return fmt.Errorf("%w: %v", ErrDisconnected, lastErr)
}

// sendConsent waits for the server's initial "connected" event, then sends
// reportLocalConsents. MUST be called with c.mu held after connect.
func (c *wsClient) sendConsent() error {
	if c.consentSent {
		return nil
	}

	// Drain initial server messages until we see the "connected" event.
	// Copilot sends {type:"system", event:"connected"} and {event:"sequenceAck"}
	// before it is ready to receive messages.
	_ = c.conn.SetReadDeadline(time.Now().Add(10 * time.Second))
	for {
		_, rawMsg, err := c.conn.ReadMessage()
		if err != nil {
			_ = c.conn.SetReadDeadline(time.Time{})
			return fmt.Errorf("waiting for connected event: %w", err)
		}
		slog.Debug("copilot: handshake event", "raw", string(rawMsg))

		var ev struct {
			Type  string `json:"type"`
			Event string `json:"event"`
		}
		_ = json.Unmarshal(rawMsg, &ev)

		// Observed format: {"event":"connected","requestId":"...","id":"0"}
		// Break on "connected" event (no "type" wrapper needed)
		if ev.Event == "connected" {
			break
		}
	}
	_ = c.conn.SetReadDeadline(time.Time{}) // reset deadline

	msg := map[string]interface{}{
		"event":           "reportLocalConsents",
		"grantedConsents": []interface{}{},
	}
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	if err := c.conn.WriteMessage(websocket.TextMessage, data); err != nil {
		return err
	}
	c.consentSent = true
	slog.Debug("copilot: consent sent")
	return nil
}

// startPingLoop starts a background goroutine that pings the server every
// PingInterval seconds to keep the connection alive.
func (c *wsClient) startPingLoop(ctx context.Context) {
	interval := time.Duration(c.cfg.PingInterval) * time.Second
	if interval <= 0 {
		interval = 25 * time.Second
	}
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				c.mu.Lock()
				if c.conn == nil || c.closed {
					c.mu.Unlock()
					return
				}
				ping := map[string]string{"event": "ping"}
				data, _ := json.Marshal(ping)
				_ = c.conn.WriteMessage(websocket.TextMessage, data)
				c.mu.Unlock()
			}
		}
	}()
}

// ResetSession drops the current connection, forcing reconnect on next use.
func (c *wsClient) ResetSession() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.conn != nil {
		_ = c.conn.Close()
		c.conn = nil
	}
	c.consentSent = false
	return nil
}

// Close permanently shuts down the client.
func (c *wsClient) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.closed = true
	if c.conn != nil {
		err := c.conn.Close()
		c.conn = nil
		return err
	}
	return nil
}

// DownloadURL is a no-op stub for the Client interface (Copilot has no
// media download use case in the current scope).
func (c *wsClient) DownloadURL(_ context.Context, _ string) ([]byte, error) {
	return nil, nil
}

// isNonRetryableConnErr returns true for errors that should not be retried.
func isNonRetryableConnErr(err error) bool {
	return err == ErrInvalidToken || err == ErrForbidden
}
