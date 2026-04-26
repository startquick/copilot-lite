package anthropic

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/crmmc/copilotpi/internal/flow"
	"github.com/crmmc/copilotpi/internal/httpapi"
	"github.com/crmmc/copilotpi/internal/token"
)

// SetupRoutes registers Anthropic-compatible API endpoints on the given router.
func (h *Handler) SetupRoutes(r chi.Router) {
	r.Post("/messages", h.handleMessages)
}

func (h *Handler) writeError(w http.ResponseWriter, statusCode int, errType, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(ErrorResponse{
		Type: "error",
		Error: ErrorInfo{
			Type:    errType,
			Message: message,
		},
	})
}

func (h *Handler) handleMessages(w http.ResponseWriter, r *http.Request) {
	var req MessageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_request_error", "Invalid JSON in request body")
		return
	}

	if req.Model == "" {
		h.writeError(w, http.StatusBadRequest, "invalid_request_error", "Missing required 'model' parameter")
		return
	}

	// Strip AI provider prefix if sent by clients like Cherry Studio (e.g., "grok/grok-4" -> "grok-4")
	if idx := strings.LastIndex(req.Model, "/"); idx != -1 {
		req.Model = req.Model[idx+1:]
	}

	// Validate model against GrokPi pools
	if cfg := h.currentConfig(); cfg != nil {
		if _, ok := token.GetPoolForModel(req.Model, &cfg.Token); !ok {
			h.writeError(w, http.StatusNotFound, "not_found_error", "The model `"+req.Model+"` does not exist")
			return
		}
	}
	if !httpapi.CheckModelWhitelist(r.Context(), req.Model) {
		h.writeError(w, http.StatusForbidden, "permission_error", "Model not allowed for this API key: "+req.Model)
		return
	}

	// Make flow request
	flowReq := toFlowRequest(&req)

	if h.ChatFlow == nil {
		h.writeError(w, http.StatusNotImplemented, "api_error", "Chat flow not configured")
		return
	}

	ctx := httpapi.BridgeFlowContext(r.Context())
	eventCh, err := h.ChatFlow.Complete(ctx, flowReq)
	if err != nil {
		h.writeStreamingOrJSONError(w, req.Stream, err)
		return
	}

	if req.Stream {
		h.streamResponse(w, r, eventCh, &req)
		return
	}
	h.blockingResponse(w, r, eventCh, &req)
}

func (h *Handler) writeStreamingOrJSONError(w http.ResponseWriter, isStream bool, err error) {
	// For simplicity, just return JSON error since connection negotiation might just have started
	h.writeError(w, http.StatusInternalServerError, "api_error", err.Error())
}

func toFlowRequest(req *MessageRequest) *flow.ChatRequest {
	messages := make([]flow.Message, 0, len(req.Messages)+1)

	// Incorporate Anthropic system prompt
	if req.System != nil {
		messages = append(messages, flow.Message{
			Role:    "system",
			Content: req.System,
		})
	}

	for _, m := range req.Messages {
		messages = append(messages, flow.Message{
			Role:      m.Role,
			Content:   m.Content,
			ToolCalls: m.ToolCalls,
		})
	}

	maxT := req.MaxTokens
	if maxT <= 0 {
		maxT = 4096 // Anthropic default if missing
	}

	flowReq := &flow.ChatRequest{
		Model:      req.Model,
		Messages:   messages,
		Stream:     true, // upstream grok flow always processes as stream
		Tools:      req.Tools,
		ToolChoice: req.ToolChoice,
		MaxTokens:  &maxT,
	}

	if req.Temperature != nil {
		flowReq.Temperature = req.Temperature
	}
	if req.TopP != nil {
		flowReq.TopP = req.TopP
	}

	return flowReq
}

func generateID() string {
	return "msg_" + time.Now().Format("20060102150405")
}
