package copilot

import "net/http"

// buildUpgradeHeaders constructs the HTTP headers used during the WebSocket
// upgrade handshake for Copilot. The access token is passed as a URL query
// parameter (accessToken=) rather than a cookie.
func buildUpgradeHeaders(userAgent string) http.Header {
	h := http.Header{}
	h.Set("Origin", "https://copilot.microsoft.com")
	h.Set("User-Agent", userAgent)
	h.Set("Accept-Language", "en-US,en;q=0.9")
	h.Set("Cache-Control", "no-cache")
	h.Set("Pragma", "no-cache")
	// Sec-WebSocket headers are set automatically by gorilla/websocket dialer
	return h
}
