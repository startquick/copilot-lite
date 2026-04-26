package copilot

import (
	"regexp"
	"testing"
)

// UUID v4 format: 8-4-4-4-12 hex chars
var rUUID = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

func TestNewConversationID(t *testing.T) {
	seen := make(map[string]bool)

	for i := 0; i < 100; i++ {
		id := newConversationID()

		if !rUUID.MatchString(id) {
			t.Errorf("ID %q is not valid UUID v4 format", id)
		}

		if seen[id] {
			t.Errorf("Duplicate conversation ID generated: %q", id)
		}
		seen[id] = true
	}
}

func TestExtractAccessToken(t *testing.T) {
	tests := []struct {
		name   string
		cookie string
		want   string
	}{
		{
			name:   "extracts _U cookie",
			cookie: "MUID=abc123; _U=myAccessToken123; SRCHUID=xyz",
			want:   "myAccessToken123",
		},
		{
			name:   "no _U cookie returns empty",
			cookie: "MUID=abc123; SRCHUID=xyz",
			want:   "",
		},
		{
			name:   "_U at start",
			cookie: "_U=firstToken; MUID=abc",
			want:   "firstToken",
		},
		{
			name:   "empty bundle",
			cookie: "",
			want:   "",
		},
		{
			name:   "_U with complex JWT value",
			cookie: "MUID=x; _U=eyJhbGciOiJSUzI1NiJ9.payload.sig; SRCHUID=y",
			want:   "eyJhbGciOiJSUzI1NiJ9.payload.sig",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractAccessToken(tt.cookie)
			if got != tt.want {
				t.Errorf("extractAccessToken() = %q, want %q", got, tt.want)
			}
		})
	}
}
