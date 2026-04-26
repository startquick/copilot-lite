package copilot

import (
	"crypto/rand"
	"fmt"
	"strings"
)

// newConversationID generates a UUID v4 string matching the clientSessionId
// format observed in Copilot WebSocket connections:
// e.g. "f4drea7ff-cf63-4184-b317-565097877113"
func newConversationID() string {
	var b [16]byte
	_, err := rand.Read(b[:])
	if err != nil {
		// Fallback: return a zero UUID rather than panic
		return "00000000-0000-4000-8000-000000000000"
	}
	// Set version 4 and variant bits
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

// extractAccessToken attempts to extract the _U cookie value from a cookie
// bundle string (the format pasted from DevTools: "key=value; key2=value2").
// The _U cookie is used by Copilot as the access token in the WebSocket URL.
// Returns empty string if not found — caller should still proceed with cookie auth.
func extractAccessToken(cookieBundle string) string {
	for _, part := range strings.Split(cookieBundle, ";") {
		part = strings.TrimSpace(part)
		if strings.HasPrefix(part, "_U=") {
			return strings.TrimPrefix(part, "_U=")
		}
	}
	return ""
}
