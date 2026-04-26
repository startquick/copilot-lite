package openai

import (
	"net/http"

	"github.com/crmmc/copilotpi/internal/httpapi"
)

// isMediaModel always returns false for Copilot — image/video not supported.
func isMediaModel(_ string) bool { return false }

// handleMediaRoutes returns 501 for any model classified as a media model.
// For CopilotPi, no models are classified as media models, so this is a no-op.
func (h *Handler) handleMediaRoutes(w http.ResponseWriter, _ *http.Request, req *ChatRequest) bool {
	if isMediaModel(req.Model) {
		httpapi.WriteError(w, http.StatusNotImplemented, "server_error", "not_implemented",
			"Image and video generation are not supported by CopilotPi Gateway")
		return true
	}
	return false
}
