package copilot_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/websocket"
	"github.com/crmmc/copilotpi/internal/config"
	"github.com/crmmc/copilotpi/internal/copilot"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

// mockCopilotServer starts a local WebSocket server that behaves like Copilot.
func mockCopilotServer(t *testing.T, handler func(conn *websocket.Conn)) *httptest.Server {
	t.Helper()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Errorf("upgrade error: %v", err)
			return
		}
		defer conn.Close()
		handler(conn)
	}))
	return ts
}

// TestChatStreaming verifies that appendText events are assembled into StreamEvents.
func TestChatStreaming(t *testing.T) {
	ts := mockCopilotServer(t, func(conn *websocket.Conn) {
		// Simulate Copilot: send appendText chunks then done
		chunks := []string{"Hello", " World", "!"}
		for _, chunk := range chunks {
			msg := map[string]interface{}{"event": "appendText", "text": chunk}
			data, _ := json.Marshal(msg)
			_ = conn.WriteMessage(websocket.TextMessage, data)
		}
		done := map[string]string{"event": "done", "messageId": "m1"}
		data, _ := json.Marshal(done)
		_ = conn.WriteMessage(websocket.TextMessage, data)
	})
	defer ts.Close()

	// Build wsURL from test server URL
	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http")

	cfg := &config.CopilotConfig{
		WSURL:        wsURL,
		WSAPIVersion: "2",
		PingInterval: 60,
		ReconnectMax: 0,
		UserAgent:    "test-agent",
	}

	client, err := copilot.NewClient("cookie=test", cfg)
	if err != nil {
		t.Fatalf("NewClient error: %v", err)
	}
	defer client.Close()

	req := &copilot.ChatRequest{
		Messages: []copilot.Message{{Role: "user", Content: "Hi"}},
		Model:    "gpt-4o",
		Stream:   true,
	}

	ch, err := client.Chat(context.Background(), req)
	if err != nil {
		t.Fatalf("Chat error: %v", err)
	}

	var got strings.Builder
	for ev := range ch {
		if ev.Err != nil {
			t.Fatalf("unexpected stream error: %v", ev.Err)
		}
		if ev.Done {
			break
		}
		got.WriteString(ev.Text)
	}

	if want := "Hello World!"; got.String() != want {
		t.Errorf("assembled text = %q, want %q", got.String(), want)
	}
}
