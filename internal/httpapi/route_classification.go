package httpapi

import "net/http"

// isChatLikeRoute identifies routes that proxy long-lived chat-style requests
// and may carry larger multimodal payloads.
func isChatLikeRoute(method, path string) bool {
	if method != http.MethodPost {
		return false
	}

	switch path {
	case "/v1/chat/completions", "/v1/messages":
		return true
	default:
		return false
	}
}
