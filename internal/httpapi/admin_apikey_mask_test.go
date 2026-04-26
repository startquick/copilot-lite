package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/crmmc/copilotpi/internal/store"
	"github.com/go-chi/chi/v5"
)

func TestHandleListAPIKeys_MasksKeyValue(t *testing.T) {
	ms := newMockAPIKeyStore()
	ms.keys = []*store.APIKey{
		{
			ID:     1,
			Key:    "sk-1234567890abcdef1234567890abcdef1234567890abcd",
			Name:   "masked",
			Status: "active",
		},
	}

	req := httptest.NewRequest(http.MethodGet, "/apikeys", nil)
	rec := httptest.NewRecorder()
	handleListAPIKeys(ms).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var resp struct {
		Data []APIKeyResponse `json:"data"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(resp.Data) != 1 {
		t.Fatalf("expected 1 key, got %d", len(resp.Data))
	}
	if resp.Data[0].Key == ms.keys[0].Key {
		t.Fatalf("expected masked key, got raw value %q", resp.Data[0].Key)
	}
	if resp.Data[0].Key != maskKey(ms.keys[0].Key) {
		t.Fatalf("expected masked key %q, got %q", maskKey(ms.keys[0].Key), resp.Data[0].Key)
	}
}

func TestHandleGetAPIKey_MasksKeyValue(t *testing.T) {
	ms := newMockAPIKeyStore()
	ms.keys = []*store.APIKey{
		{
			ID:     1,
			Key:    "sk-abcdefabcdefabcdefabcdefabcdefabcdefabcdefabcd",
			Name:   "masked",
			Status: "active",
		},
	}

	req := httptest.NewRequest(http.MethodGet, "/apikeys/1", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "1")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	rec := httptest.NewRecorder()
	handleGetAPIKey(ms).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var resp APIKeyResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Key != maskKey(ms.keys[0].Key) {
		t.Fatalf("expected masked key %q, got %q", maskKey(ms.keys[0].Key), resp.Key)
	}
}
