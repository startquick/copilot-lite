// Package copilot implements the Copilot WebSocket chat client.
package copilot

import "errors"

// Sentinel errors returned by the Copilot client.
var (
	// ErrDisconnected is returned when the WebSocket connection drops and
	// all reconnect attempts are exhausted.
	ErrDisconnected = errors.New("copilot: websocket disconnected")

	// ErrRateLimited is returned when Copilot signals rate limiting (HTTP 429
	// or an error event with code 429).
	ErrRateLimited = errors.New("copilot: rate limited")

	// ErrInvalidToken is returned when the WebSocket upgrade is rejected with
	// HTTP 401, indicating the cookie bundle is invalid or expired.
	ErrInvalidToken = errors.New("copilot: invalid token (401)")

	// ErrForbidden is returned on HTTP 403 during WebSocket upgrade.
	ErrForbidden = errors.New("copilot: forbidden (403)")

	// ErrStreamClosed is returned when the server closes the stream
	// unexpectedly before sending a "done" event.
	ErrStreamClosed = errors.New("copilot: stream closed unexpectedly")
)

// IsAuthError reports whether err indicates a Copilot authentication failure
// (invalid/expired cookie bundle or forbidden access).
func IsAuthError(err error) bool {
	return errors.Is(err, ErrInvalidToken) || errors.Is(err, ErrForbidden)
}
