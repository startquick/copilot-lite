package httpapi

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/crmmc/copilotpi/internal/config"
	"github.com/crmmc/copilotpi/internal/store"
	"github.com/go-chi/chi/v5"
)

// TokenReplaceRequest is the request body for replacing a token's raw value.
type TokenReplaceRequest struct {
	Token          string `json:"token"`
	Reclassify     *bool  `json:"reclassify,omitempty"`
	PreserveRemark *bool  `json:"preserve_remark,omitempty"`
}

func handleReplaceToken(ts TokenStoreInterface, syncer TokenPoolSyncer, cfg *config.Config) http.HandlerFunc {
	return handleReplaceTokenFromProvider(ts, syncer, func() *config.TokenConfig {
		if cfg == nil {
			return nil
		}
		return &cfg.Token
	})
}

func handleReplaceTokenFromProvider(ts TokenStoreInterface, syncer TokenPoolSyncer, getCfg func() *config.TokenConfig) http.HandlerFunc {
	return handleReplaceTokenFromProviderWithProfiler(ts, syncer, getCfg, defaultTokenImportProfiler(defaultTokenImportBaseURL))
}

func handleReplaceTokenFromProviderWithProfiler(ts TokenStoreInterface, syncer TokenPoolSyncer, getCfg func() *config.TokenConfig, profiler tokenImportProfiler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		idStr := chi.URLParam(r, "id")
		id, err := strconv.ParseUint(idStr, 10, 32)
		if err != nil {
			WriteError(w, http.StatusBadRequest, "invalid_request", "invalid_id",
				"Invalid token ID")
			return
		}

		token, err := ts.GetToken(r.Context(), uint(id))
		if err != nil {
			if errors.Is(err, store.ErrNotFound) {
				WriteError(w, http.StatusNotFound, "not_found", "token_not_found",
					"Token not found")
				return
			}
			WriteError(w, http.StatusInternalServerError, "server_error", "get_failed",
				"Failed to get token")
			return
		}

		var req TokenReplaceRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			WriteError(w, http.StatusBadRequest, "invalid_request", "invalid_json",
				"Invalid JSON in request body")
			return
		}

		req.Token = strings.TrimSpace(req.Token)
		if req.Token == "" {
			WriteError(w, http.StatusBadRequest, "invalid_request", "missing_token",
				"Token is required")
			return
		}
		if len(req.Token) < 20 {
			WriteError(w, http.StatusBadRequest, "invalid_request", "token_too_short",
				"Token must be at least 20 characters")
			return
		}

		reclassify := req.Reclassify == nil || *req.Reclassify
		preserveRemark := req.PreserveRemark == nil || *req.PreserveRemark
		nextRemark := ""
		if preserveRemark {
			nextRemark = token.Remark
		}

		token.Token = req.Token
		token.Status = store.TokenStatusActive
		token.StatusReason = ""
		token.FailCount = 0
		token.CoolUntil = nil
		token.LastUsed = nil

		tokenCfg := &config.TokenConfig{}
		if getCfg != nil {
			if current := getCfg(); current != nil {
				tokenCfg = current
			}
		}

		if reclassify {
			if profiler != nil {
				if profile, err := profiler(r.Context(), req.Token, tokenCfg); err == nil && profile != nil {
					token.Pool = profile.Pool
					token.Priority = profile.Priority
					token.ChatQuota = profile.ChatQuota
					token.InitialChatQuota = profile.InitialChatQuota
					token.ImageQuota = profile.ImageQuota
					token.InitialImageQuota = profile.InitialImageQuota
					token.VideoQuota = profile.VideoQuota
					token.InitialVideoQuota = profile.InitialVideoQuota
					nextRemark = appendImportRemark(nextRemark, classifyImportRemark(profile.Pool))
				} else {
					token.ChatQuota = resolveImportQuota(nil, tokenCfg)
					token.InitialChatQuota = token.ChatQuota
					if tokenCfg.DefaultImageQuota > 0 {
						token.ImageQuota = tokenCfg.DefaultImageQuota
						token.InitialImageQuota = tokenCfg.DefaultImageQuota
					}
					if tokenCfg.DefaultVideoQuota > 0 {
						token.VideoQuota = tokenCfg.DefaultVideoQuota
						token.InitialVideoQuota = tokenCfg.DefaultVideoQuota
					}
					nextRemark = appendImportRemark(nextRemark, "pending auto-detect")
				}
			} else {
				token.ChatQuota = resolveImportQuota(nil, tokenCfg)
				token.InitialChatQuota = token.ChatQuota
				if tokenCfg.DefaultImageQuota > 0 {
					token.ImageQuota = tokenCfg.DefaultImageQuota
					token.InitialImageQuota = tokenCfg.DefaultImageQuota
				}
				if tokenCfg.DefaultVideoQuota > 0 {
					token.VideoQuota = tokenCfg.DefaultVideoQuota
					token.InitialVideoQuota = tokenCfg.DefaultVideoQuota
				}
				nextRemark = appendImportRemark(nextRemark, "pending auto-detect")
			}
		}

		token.Remark = nextRemark

		if err := ts.UpdateToken(r.Context(), token); err != nil {
			WriteError(w, http.StatusInternalServerError, "server_error", "update_failed",
				"Failed to replace token")
			return
		}

		if syncer != nil {
			if err := syncer.SyncToken(r.Context(), uint(id)); err != nil {
				slog.Warn("failed to sync replaced token to pool", "token_id", id, "error", err)
			}
		}

		WriteJSON(w, http.StatusOK, tokenToResponse(token))
	}
}
