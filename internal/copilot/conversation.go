package copilot

import (
	"crypto/rand"
	"math/big"
)

const base62Chars = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
const conversationIDLen = 20

// newConversationID generates a 20-character URL-safe base62 random string
// matching the format observed from copilot.microsoft.com conversation IDs
// (e.g. "iOFzza28WMqXibF8618kT").
func newConversationID() string {
	result := make([]byte, conversationIDLen)
	base := big.NewInt(int64(len(base62Chars)))
	for i := range result {
		n, err := rand.Int(rand.Reader, base)
		if err != nil {
			// Fallback: use 'a' to avoid panic; should never happen
			result[i] = 'a'
			continue
		}
		result[i] = base62Chars[n.Int64()]
	}
	return string(result)
}
