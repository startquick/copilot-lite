package copilot

import (
	"regexp"
	"testing"
)

var rUUID = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
var rBase62 = regexp.MustCompile(`^[0-9A-Za-z]{20}$`)

func TestNewClientSessionID(t *testing.T) {
	seen := make(map[string]bool)
	for i := 0; i < 100; i++ {
		id := newClientSessionID()
		if !rUUID.MatchString(id) {
			t.Errorf("clientSessionId %q is not valid UUID v4", id)
		}
		if seen[id] {
			t.Errorf("duplicate clientSessionId: %q", id)
		}
		seen[id] = true
	}
}

func TestNewConversationID(t *testing.T) {
	seen := make(map[string]bool)
	for i := 0; i < 100; i++ {
		id := newConversationID()
		if !rBase62.MatchString(id) {
			t.Errorf("conversationId %q is not valid base62-20", id)
		}
		if seen[id] {
			t.Errorf("duplicate conversationId: %q", id)
		}
		seen[id] = true
	}
}
