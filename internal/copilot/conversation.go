package copilot

import (
	"crypto/rand"
	"fmt"
	"math/big"
)

const base62Chars = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"

// newClientSessionID generates a UUID v4 string for the clientSessionId
// URL query parameter in the WebSocket connection URL.
// Format observed: "14ecb8b7-1b6e-41ea-b242-3564c5b27a69"
func newClientSessionID() string {
	var b [16]byte
	_, err := rand.Read(b[:])
	if err != nil {
		return "00000000-0000-4000-8000-000000000000"
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

// newConversationID generates a 20-character base62 random string for use
// as conversationId inside WebSocket send payloads.
// Format observed: "MsXeur7REC4a3h1JAJKYa"
func newConversationID() string {
	result := make([]byte, 20)
	base := big.NewInt(int64(len(base62Chars)))
	for i := range result {
		n, err := rand.Int(rand.Reader, base)
		if err != nil {
			result[i] = 'a'
			continue
		}
		result[i] = base62Chars[n.Int64()]
	}
	return string(result)
}

// extractAccessToken attempts to extract the _U cookie value from a cookie
// bundle string. Returns empty string if not found.
func extractAccessToken(cookieBundle string) string {
	// kept for reference — access token is now stored in config directly
	return ""
}
