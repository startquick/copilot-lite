package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/crmmc/copilotpi/internal/config"
	"github.com/crmmc/copilotpi/internal/store"
	tokenPkg "github.com/crmmc/copilotpi/internal/token"
	"github.com/go-chi/chi/v5"
)

type mockTokenSyncer struct {
	syncedIDs []uint
}

func (m *mockTokenSyncer) AddToPool(_ *store.Token) {}

func (m *mockTokenSyncer) RemoveFromPool(_ uint) {}

func (m *mockTokenSyncer) SyncToken(_ context.Context, id uint) error {
	m.syncedIDs = append(m.syncedIDs, id)
	return nil
}

func TestHandleReplaceToken_ReclassifiesAndSyncs(t *testing.T) {
	mockStore := newMockTokenStore()
	mockStore.tokens[1] = &store.Token{
		ID:           1,
		Token:        "old_token_value_long_enough",
		Pool:         "ssoBasic",
		Status:       store.TokenStatusDisabled,
		StatusReason: "manual disable",
		Remark:       "team-a",
		ChatQuota:    10,
	}
	syncer := &mockTokenSyncer{}

	handler := handleReplaceTokenFromProviderWithProfiler(mockStore, syncer, func() *config.TokenConfig {
		return &config.TokenConfig{DefaultImageQuota: 11, DefaultVideoQuota: 6}
	}, func(ctx context.Context, authToken string, cfg *config.TokenConfig) (*tokenPkg.ImportProfile, error) {
		return &tokenPkg.ImportProfile{
			Pool:              tokenPkg.PoolSuper,
			Priority:          10,
			ChatQuota:         45,
			InitialChatQuota:  45,
			ImageQuota:        11,
			InitialImageQuota: 11,
			VideoQuota:        6,
			InitialVideoQuota: 6,
		}, nil
	})

	body := `{"token":"new_token_value_long_enough_12345","reclassify":true,"preserve_remark":true}`
	req := httptest.NewRequest(http.MethodPost, "/admin/tokens/1/replace", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "1")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	updated := mockStore.tokens[1]
	if updated.Token != "new_token_value_long_enough_12345" {
		t.Fatalf("expected raw token replaced, got %q", updated.Token)
	}
	if updated.Status != store.TokenStatusActive {
		t.Fatalf("expected status active, got %q", updated.Status)
	}
	if updated.Pool != tokenPkg.PoolSuper || updated.Priority != 10 {
		t.Fatalf("expected super profile, got pool=%q priority=%d", updated.Pool, updated.Priority)
	}
	if updated.ChatQuota != 45 || updated.ImageQuota != 11 || updated.VideoQuota != 6 {
		t.Fatalf("unexpected quotas: %+v", updated)
	}
	if !strings.Contains(updated.Remark, "team-a") || !strings.Contains(updated.Remark, "auto-detected: paid") {
		t.Fatalf("expected preserved remark with classification, got %q", updated.Remark)
	}
	if len(syncer.syncedIDs) != 1 || syncer.syncedIDs[0] != 1 {
		t.Fatalf("expected sync for token 1, got %v", syncer.syncedIDs)
	}

	var resp TokenResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Token == updated.Token {
		t.Fatalf("expected masked token response, got raw token %q", resp.Token)
	}
}

func TestHandleReplaceToken_InvalidShortToken(t *testing.T) {
	mockStore := newMockTokenStore()
	mockStore.tokens[1] = &store.Token{
		ID:     1,
		Token:  "old_token_value_long_enough",
		Status: store.TokenStatusActive,
	}

	body := `{"token":"too_short"}`
	req := httptest.NewRequest(http.MethodPost, "/admin/tokens/1/replace", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "1")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	rec := httptest.NewRecorder()

	handleReplaceTokenFromProviderWithProfiler(mockStore, nil, func() *config.TokenConfig { return nil }, nil).ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}
