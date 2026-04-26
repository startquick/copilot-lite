package copilot

import (
	"regexp"
	"testing"
)

func TestNewConversationID(t *testing.T) {
	rBase62 := regexp.MustCompile(`^[0-9A-Za-z]{20}$`)
	seen := make(map[string]bool)

	for i := 0; i < 100; i++ {
		id := newConversationID()

		if len(id) != conversationIDLen {
			t.Errorf("ID length = %d, want %d", len(id), conversationIDLen)
		}

		if !rBase62.MatchString(id) {
			t.Errorf("ID %q contains non-base62 characters", id)
		}

		if seen[id] {
			t.Errorf("Duplicate conversation ID generated: %q", id)
		}
		seen[id] = true
	}
}
