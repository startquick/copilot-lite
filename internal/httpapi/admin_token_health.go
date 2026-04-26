package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	tkn "github.com/crmmc/copilotpi/internal/token"
	"github.com/go-chi/chi/v5"
)

// TokenHealthProber defines the interface for on-demand token probing.
type TokenHealthProber interface {
	ProbeToken(ctx context.Context, id uint) (status string, chatQuota int, probeErr error)
}

// TokenHealthResponse is the response for the token health endpoint.
type TokenHealthResponse struct {
	TokenID    uint   `json:"token_id"`
	Status     string `json:"status"`
	ChatQuota  int    `json:"chat_quota"`
	ProbeError string `json:"probe_error,omitempty"`
}

// handleTokenHealth returns a handler for GET /admin/tokens/{id}/health.
// It performs a synchronous on-demand probe and returns the result.
func handleTokenHealth(prober TokenHealthProber) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		idStr := chi.URLParam(r, "id")
		id, err := strconv.ParseUint(idStr, 10, 32)
		if err != nil {
			WriteError(w, 400, "invalid_request", "invalid_id",
				"Invalid token ID")
			return
		}

		status, chatQuota, probeErr := prober.ProbeToken(r.Context(), uint(id))
		if errors.Is(probeErr, tkn.ErrTokenNotFound) {
			WriteError(w, 404, "not_found", "token_not_found",
				"Token not found")
			return
		}

		resp := TokenHealthResponse{
			TokenID:   uint(id),
			Status:    status,
			ChatQuota: chatQuota,
		}
		if probeErr != nil {
			resp.ProbeError = probeErr.Error()
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}
}
