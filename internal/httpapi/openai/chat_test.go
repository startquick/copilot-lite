package openai

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/crmmc/copilotpi/internal/flow"
	"github.com/crmmc/copilotpi/internal/httpapi"
	"github.com/crmmc/copilotpi/internal/store"
	tkn "github.com/crmmc/copilotpi/internal/token"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHandleChat_MissingMessages(t *testing.T) {
	s := httpapi.NewServer(&httpapi.ServerConfig{ChatProviders: []httpapi.ChatProvider{&Handler{}}})
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions",
		strings.NewReader(`{"model":"gpt-4o"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	s.Router().ServeHTTP(w, req)

	assert.Equal(t, 400, w.Code)
	var resp httpapi.APIError
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, "invalid_request_error", resp.Error.Type)
	assert.Contains(t, resp.Error.Message, "messages")
}

func TestHandleChat_EmptyMessages(t *testing.T) {
	s := httpapi.NewServer(&httpapi.ServerConfig{ChatProviders: []httpapi.ChatProvider{&Handler{}}})
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions",
		strings.NewReader(`{"model":"gpt-4o","messages":[]}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	s.Router().ServeHTTP(w, req)

	assert.Equal(t, 400, w.Code)
	var resp httpapi.APIError
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, "invalid_request_error", resp.Error.Type)
}

func TestHandleChat_InvalidJSON(t *testing.T) {
	s := httpapi.NewServer(&httpapi.ServerConfig{ChatProviders: []httpapi.ChatProvider{&Handler{}}})
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions",
		strings.NewReader(`{invalid json`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	s.Router().ServeHTTP(w, req)

	assert.Equal(t, 400, w.Code)
}

func TestToFlowRequest_PropagatesSamplingParams(t *testing.T) {
	temp := 0.0
	topP := 0.25
	maxTokens := 128
	req := &ChatRequest{
		Model: "gpt-4o",
		Messages: []ChatMessage{
			{Role: "user", Content: "hi"},
		},
		Temperature: &temp,
		TopP:        &topP,
		MaxTokens:   &maxTokens,
	}

	flowReq := toFlowRequest(req)
	require.NotNil(t, flowReq.Temperature)
	require.NotNil(t, flowReq.TopP)
	require.NotNil(t, flowReq.MaxTokens)
	assert.Equal(t, temp, *flowReq.Temperature)
	assert.Equal(t, topP, *flowReq.TopP)
	assert.Equal(t, maxTokens, *flowReq.MaxTokens)
}

func TestNormalizeChatRequestDefaults(t *testing.T) {
	req := &ChatRequest{
		Model: "gpt-4o",
		Messages: []ChatMessage{
			{Role: "user", Content: "hi"},
		},
	}
	normalized, err := normalizeChatRequest(req, nil)
	require.Nil(t, err)
	require.NotNil(t, normalized.Temperature)
	require.NotNil(t, normalized.TopP)
	assert.Equal(t, defaultChatTemperature, *normalized.Temperature)
	assert.Equal(t, defaultChatTopP, *normalized.TopP)
	assert.False(t, isStreamEnabled(normalized.Stream))
	require.NotNil(t, normalized.ParallelToolCalls)
	assert.True(t, *normalized.ParallelToolCalls)
}

func TestNormalizeChatRequest_InvalidToolChoice(t *testing.T) {
	req := &ChatRequest{
		Model: "gpt-4o",
		Messages: []ChatMessage{
			{Role: "user", Content: "hi"},
		},
		ToolChoice: "invalid",
	}

	_, err := normalizeChatRequest(req, nil)
	require.NotNil(t, err)
	assert.Equal(t, "invalid_tool_choice", err.code)
}

func TestNormalizeChatRequest_InvalidToolObject(t *testing.T) {
	req := &ChatRequest{
		Model: "gpt-4o",
		Messages: []ChatMessage{
			{Role: "user", Content: "hi"},
		},
		ToolChoice: map[string]any{
			"type": "not_function",
		},
	}

	_, err := normalizeChatRequest(req, nil)
	require.NotNil(t, err)
	assert.Equal(t, "invalid_tool_choice", err.code)
}

func TestNormalizeChatRequest_InvalidToolDefinition(t *testing.T) {
	req := &ChatRequest{
		Model: "gpt-4o",
		Messages: []ChatMessage{
			{Role: "user", Content: "hi"},
		},
		Tools: []flow.Tool{
			{Type: "invalid_type", Function: flow.Function{Name: "search"}},
		},
	}

	_, err := normalizeChatRequest(req, nil)
	require.NotNil(t, err)
	assert.Equal(t, "invalid_tool_type", err.code)
}

// chatMockTokenSvc is a minimal TokenServicer for httpapi chat tests.
type chatMockTokenSvc struct{}

func (m *chatMockTokenSvc) Pick(pool string, _ tkn.QuotaCategory) (*store.Token, error) {
	return &store.Token{ID: 1, Token: "tok-test", Pool: pool}, nil
}
func (m *chatMockTokenSvc) PickExcluding(pool string, _ tkn.QuotaCategory, _ map[uint]struct{}) (*store.Token, error) {
	return m.Pick(pool, tkn.CategoryChat)
}
func (m *chatMockTokenSvc) Consume(tokenID uint, _ tkn.QuotaCategory, _ int) (int, error) {
	return 99, nil
}
func (m *chatMockTokenSvc) ReportSuccess(id uint)                  {}
func (m *chatMockTokenSvc) ReportRateLimit(id uint, reason string) {}
func (m *chatMockTokenSvc) ReportError(id uint, reason string)     {}
func (m *chatMockTokenSvc) MarkExpired(id uint, reason string)     {}
func (m *chatMockTokenSvc) MarkCircuitFailure(id uint)             {}
func (m *chatMockTokenSvc) MarkCircuitSuccess(id uint)             {}

type chatMockAPIKeyStore struct{}

func (m *chatMockAPIKeyStore) List(context.Context, int, int, string) ([]*store.APIKey, int64, error) {
	return nil, 0, nil
}

func (m *chatMockAPIKeyStore) GetByID(context.Context, uint) (*store.APIKey, error) {
	return nil, store.ErrNotFound
}

func (m *chatMockAPIKeyStore) GetByKey(_ context.Context, key string) (*store.APIKey, error) {
	if key != "test-api-key" {
		return nil, store.ErrNotFound
	}
	return &store.APIKey{ID: 42, Key: key, Name: "test", Status: "active"}, nil
}

func (m *chatMockAPIKeyStore) Create(context.Context, *store.APIKey) error { return nil }
func (m *chatMockAPIKeyStore) Update(context.Context, *store.APIKey) error { return nil }
func (m *chatMockAPIKeyStore) Delete(context.Context, uint) error          { return nil }
func (m *chatMockAPIKeyStore) Regenerate(context.Context, uint) (string, error) {
	return "", nil
}

func (m *chatMockAPIKeyStore) CountByStatus(context.Context) (int, int, int, int, int, error) {
	return 0, 0, 0, 0, 0, nil
}

func (m *chatMockAPIKeyStore) IncrementUsage(context.Context, uint) error { return nil }
func (m *chatMockAPIKeyStore) ResetDailyUsage(context.Context) error      { return nil }
