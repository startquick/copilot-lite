package copilot

import "net/http"

// buildUpgradeHeaders constructs the HTTP headers used during the WebSocket
// upgrade handshake for Copilot. The cookie bundle is passed verbatim as the
// Cookie header value (admin-pasted from DevTools).
func buildUpgradeHeaders(cookieBundle, userAgent string) http.Header {
	h := http.Header{}
	h.Set("Cookie", cookieBundle)
	h.Set("Origin", "https://copilot.microsoft.com")
	h.Set("User-Agent", userAgent)
	h.Set("Accept-Language", "en-US,en;q=0.9")
	h.Set("Cache-Control", "no-cache")
	h.Set("Pragma", "no-cache")
	// Sec-WebSocket headers are set automatically by gorilla/websocket dialer
	return h
}
