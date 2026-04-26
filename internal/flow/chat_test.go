package flow

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/crmmc/copilotpi/internal/config"
	"github.com/crmmc/copilotpi/internal/copilot"
	"github.com/crmmc/copilotpi/internal/store"
	tkn "github.com/crmmc/copilotpi/internal/token"
)

// testFlowTokenConfig returns a token config for flow tests using Copilot model names.
func testFlowTokenConfig() *config.TokenConfig {
	return &config.TokenConfig{
		BasicModels:   []string{"copilot-free", "copilot-basic"},
		SuperModels:   []string{"gpt-4o", "gpt-4o-mini", "o1", "o3", "copilot-premium"},
		PreferredPool: "premium",
	}
}

// mockTokenService implements TokenServicer for testing.
type mockTokenService struct {
	mu             sync.Mutex
	tokens         []*store.Token
	pickIndex      int
	pickErr        error
	consumeCalls   []uint
	consumeErr     error
	successCalls   []uint
	rateLimitCalls []uint
	errorCalls     []uint
	disabledCalls  []uint
	expiredCalls   []uint
}

func (m *mockTokenService) Pick(pool string, _ tkn.QuotaCategory) (*store.Token, error) {
	return m.PickExcluding(pool, tkn.CategoryChat, nil)
}

func (m *mockTokenService) PickExcluding(pool string, _ tkn.QuotaCategory, exclude map[uint]struct{}) (*store.Token, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.pickErr != nil {
		return nil, m.pickErr
	}
	for m.pickIndex < len(m.tokens) {
		t := m.tokens[m.pickIndex]
		m.pickIndex++
		if _, skipped := exclude[t.ID]; skipped {
			continue
		}
		return t, nil
	}
	return nil, errors.New("no tokens available")
}

func (m *mockTokenService) Consume(tokenID uint, _ tkn.QuotaCategory, _ int) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.consumeCalls = append(m.consumeCalls, tokenID)
	if m.consumeErr != nil {
		return 0, m.consumeErr
	}
	return 99, nil
}

func (m *mockTokenService) ReportSuccess(id uint) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.successCalls = append(m.successCalls, id)
}

func (m *mockTokenService) ReportRateLimit(id uint, reason string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.rateLimitCalls = append(m.rateLimitCalls, id)
}

func (m *mockTokenService) ReportError(id uint, reason string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.errorCalls = append(m.errorCalls, id)
}

func (m *mockTokenService) MarkDisabled(id uint, reason string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.disabledCalls = append(m.disabledCalls, id)
}

func (m *mockTokenService) MarkExpired(id uint, reason string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.expiredCalls = append(m.expiredCalls, id)
}

func (m *mockTokenService) MarkCircuitFailure(id uint) {}
func (m *mockTokenService) MarkCircuitSuccess(id uint) {}

// mockCopilotClient implements copilot.Client for testing.
type mockCopilotClient struct {
	mu        sync.Mutex
	events    []copilot.StreamEvent
	chatErr   error
	callCount int
	lastReq   *copilot.ChatRequest
}

func (m *mockCopilotClient) Chat(ctx context.Context, req *copilot.ChatRequest) (<-chan copilot.StreamEvent, error) {
	m.mu.Lock()
	m.callCount++
	m.lastReq = req
	events := m.events
	chatErr := m.chatErr
	m.mu.Unlock()

	if chatErr != nil {
		return nil, chatErr
	}

	ch := make(chan copilot.StreamEvent, len(events)+1)
	for _, e := range events {
		ch <- e
	}
	ch <- copilot.StreamEvent{Done: true}
	close(ch)
	return ch, nil
}

func (m *mockCopilotClient) ResetSession() error { return nil }
func (m *mockCopilotClient) Close() error        { return nil }
func (m *mockCopilotClient) DownloadURL(_ context.Context, _ string) ([]byte, error) {
	return nil, nil
}



func TestChatFlow_Success(t *testing.T) {
	tokenSvc := &mockTokenService{
		tokens: []*store.Token{{ID: 1, Token: "tok1", Pool: "basic"}},
	}

	client := &mockCopilotClient{
		events: []copilot.StreamEvent{
			{Text: "Hello world"},
		},
	}

	cfg := &ChatFlowConfig{RetryConfig: DefaultRetryConfig(), TokenConfig: testFlowTokenConfig()}
	flow := NewChatFlow(tokenSvc, func(token string) copilot.Client { return client }, cfg)

	req := &ChatRequest{
		Messages: []Message{{Role: "user", Content: "Hi"}},
		Model:    "copilot-free",
	}

	ctx := context.Background()
	ch, err := flow.Complete(ctx, req)
	if err != nil {
		t.Fatalf("Complete() error = %v", err)
	}

	var events []StreamEvent
	for e := range ch {
		events = append(events, e)
	}

	if len(events) == 0 {
		t.Fatal("expected at least one event")
	}

	// Check success was reported
	if len(tokenSvc.successCalls) != 1 || tokenSvc.successCalls[0] != 1 {
		t.Errorf("expected success reported for token 1, got %v", tokenSvc.successCalls)
	}
}

func TestChatFlow_RetryOnRateLimit(t *testing.T) {
	tokenSvc := &mockTokenService{
		tokens: []*store.Token{
			{ID: 1, Token: "tok1", Pool: "basic"},
			{ID: 2, Token: "tok2", Pool: "basic"},
		},
	}

	callCount := 0

	clientFactory := func(token string) copilot.Client {
		callCount++
		if callCount <= 2 {
			return &mockCopilotClient{chatErr: copilot.ErrRateLimited}
		}
		return &mockCopilotClient{
			events: []copilot.StreamEvent{{Text: "Success"}},
		}
	}

	cfg := &ChatFlowConfig{RetryConfig: &RetryConfig{
		MaxTokens:       6,
		PerTokenRetries: 2,
		BaseDelay:       time.Millisecond, // fast for tests
		MaxDelay:        10 * time.Millisecond,
		JitterFactor:    0,
	}, TokenConfig: testFlowTokenConfig()}
	flow := NewChatFlow(tokenSvc, clientFactory, cfg)

	req := &ChatRequest{
		Messages: []Message{{Role: "user", Content: "Hi"}},
		Model:    "copilot-free",
	}

	ctx := context.Background()
	ch, err := flow.Complete(ctx, req)
	if err != nil {
		t.Fatalf("Complete() error = %v", err)
	}

	for range ch {
		// drain channel
	}

	// Should have rate limit reports
	if len(tokenSvc.rateLimitCalls) < 2 {
		t.Errorf("expected at least 2 rate limit reports, got %v", tokenSvc.rateLimitCalls)
	}
}

func TestChatFlow_TokenRotation(t *testing.T) {
	tokenSvc := &mockTokenService{
		tokens: []*store.Token{
			{ID: 1, Token: "tok1", Pool: "basic"},
			{ID: 2, Token: "tok2", Pool: "basic"},
			{ID: 3, Token: "tok3", Pool: "basic"},
		},
	}

	// Track which tokens were used
	var usedTokens []string
	var mu sync.Mutex

	clientFactory := func(token string) copilot.Client {
		mu.Lock()
		usedTokens = append(usedTokens, token)
		mu.Unlock()

		// First two tokens fail, third succeeds
		if token == "tok1" || token == "tok2" {
			return &mockCopilotClient{chatErr: copilot.ErrRateLimited}
		}
		return &mockCopilotClient{
			events: []copilot.StreamEvent{{Text: "Success"}},
		}
	}

	cfg := &ChatFlowConfig{RetryConfig: &RetryConfig{
		MaxTokens:       6,
		PerTokenRetries: 2,
		BaseDelay:       time.Millisecond,
		MaxDelay:        10 * time.Millisecond,
		JitterFactor:    0,
	}, TokenConfig: testFlowTokenConfig()}
	flow := NewChatFlow(tokenSvc, clientFactory, cfg)

	req := &ChatRequest{
		Messages: []Message{{Role: "user", Content: "Hi"}},
		Model:    "copilot-free",
	}

	ctx := context.Background()
	ch, _ := flow.Complete(ctx, req)
	for range ch {
	}

	// Should have rotated through tokens
	if len(usedTokens) < 3 {
		t.Errorf("expected at least 3 token uses, got %v", usedTokens)
	}
}

func TestChatFlow_TokenRotation_ExcludesPreviouslyFailedActiveToken(t *testing.T) {
	tokenSvc := &mockTokenService{
		tokens: []*store.Token{
			{ID: 1, Token: "tok1", Pool: "basic"},
			{ID: 2, Token: "tok2", Pool: "basic"},
			{ID: 3, Token: "tok3", Pool: "basic"},
		},
	}

	var usedTokens []string
	var mu sync.Mutex
	attempts := make(map[string]int)

	clientFactory := func(token string) copilot.Client {
		mu.Lock()
		usedTokens = append(usedTokens, token)
		attempts[token]++
		count := attempts[token]
		mu.Unlock()

		if token == "tok3" {
			return &mockCopilotClient{events: []copilot.StreamEvent{{Text: "Success"}}}
		}

		// Generic retryable failure: token remains active, so exclusion must drive rotation.
		if count == 1 {
			return &mockCopilotClient{chatErr: errors.New("503 upstream unavailable")}
		}

		return &mockCopilotClient{chatErr: errors.New("503 upstream unavailable again")}
	}

	cfg := &ChatFlowConfig{RetryConfig: &RetryConfig{
		MaxTokens:       3,
		PerTokenRetries: 1,
		BaseDelay:       time.Millisecond,
		MaxDelay:        5 * time.Millisecond,
		JitterFactor:    0,
	}, TokenConfig: testFlowTokenConfig()}
	flow := NewChatFlow(tokenSvc, clientFactory, cfg)

	req := &ChatRequest{
		Messages: []Message{{Role: "user", Content: "Hi"}},
		Model:    "copilot-free",
	}

	ch, err := flow.Complete(context.Background(), req)
	if err != nil {
		t.Fatalf("Complete() error = %v", err)
	}
	for range ch {
	}

	if len(usedTokens) < 3 {
		t.Fatalf("expected at least 3 token attempts, got %v", usedTokens)
	}
	if usedTokens[0] != "tok1" || usedTokens[1] != "tok2" || usedTokens[2] != "tok3" {
		t.Fatalf("expected sequential rotation tok1 -> tok2 -> tok3 before reuse, got %v", usedTokens)
	}
}

func TestChatFlow_NonRetryableError(t *testing.T) {
	tokenSvc := &mockTokenService{
		tokens: []*store.Token{{ID: 1, Token: "tok1", Pool: "basic"}},
	}

	badReqErr := errors.New("400 Bad Request: invalid model")
	client := &mockCopilotClient{chatErr: badReqErr}

	cfg := &ChatFlowConfig{RetryConfig: DefaultRetryConfig(), TokenConfig: testFlowTokenConfig()}
	flow := NewChatFlow(tokenSvc, func(token string) copilot.Client { return client }, cfg)

	req := &ChatRequest{
		Messages: []Message{{Role: "user", Content: "Hi"}},
		Model:    "copilot-free",
	}

	ctx := context.Background()
	ch, _ := flow.Complete(ctx, req)

	var lastEvent StreamEvent
	for e := range ch {
		lastEvent = e
	}

	// Should get error without retrying (400 is non-recoverable)
	if lastEvent.Error == nil {
		t.Errorf("expected error event, got nil")
	}
}

func TestChatFlow_HandleError_Forbidden_MarksExpired(t *testing.T) {
	tokenSvc := &mockTokenService{}
	flow := &ChatFlow{tokenSvc: tokenSvc}
	cfg := DefaultRetryConfig()

	flow.handleError(1, copilot.ErrForbidden, cfg)

	if len(tokenSvc.expiredCalls) != 1 || tokenSvc.expiredCalls[0] != 1 {
		t.Errorf("token-level 403 should mark expired, got %v", tokenSvc.expiredCalls)
	}
	if len(tokenSvc.rateLimitCalls) != 0 {
		t.Errorf("token-level 403 should not rate limit, got %v", tokenSvc.rateLimitCalls)
	}
}

func TestChatFlow_HandleError_TransportSkipsPenalty(t *testing.T) {
	tokenSvc := &mockTokenService{}
	flow := &ChatFlow{tokenSvc: tokenSvc}
	cfg := DefaultRetryConfig()

	flow.handleError(1, copilot.ErrDisconnected, cfg)
	flow.handleError(1, errors.New("503 Service Unavailable"), cfg)

	if len(tokenSvc.rateLimitCalls) != 0 {
		t.Errorf("expected no rate limit calls, got %v", tokenSvc.rateLimitCalls)
	}
	if len(tokenSvc.errorCalls) != 0 {
		t.Errorf("expected no error calls, got %v", tokenSvc.errorCalls)
	}
}

func TestChatFlow_WithTools(t *testing.T) {
	tokenSvc := &mockTokenService{
		tokens: []*store.Token{{ID: 1, Token: "tok1", Pool: "basic"}},
	}

	client := &mockCopilotClient{
		events: []copilot.StreamEvent{
			{Text: "I'll check the weather.\n<tool_call>\n{\"name\":\"get_weather\",\"arguments\":\"{\\\"location\\\":\\\"Tokyo\\\"}\"}\n</tool_call>"},
		},
	}

	cfg := &ChatFlowConfig{RetryConfig: DefaultRetryConfig(), TokenConfig: testFlowTokenConfig()}
	flow := NewChatFlow(tokenSvc, func(token string) copilot.Client { return client }, cfg)

	req := &ChatRequest{
		Messages: []Message{{Role: "user", Content: "What's the weather in Tokyo?"}},
		Model:    "copilot-free",
		Tools: []Tool{
			{
				Type: "function",
				Function: Function{
					Name:        "get_weather",
					Description: "Get weather for a location",
					Parameters: map[string]any{
						"type": "object",
						"properties": map[string]any{
							"location": map[string]any{"type": "string"},
						},
						"required": []string{"location"},
					},
				},
			},
		},
		ToolChoice: "auto",
	}

	ctx := context.Background()
	ch, err := flow.Complete(ctx, req)
	if err != nil {
		t.Fatalf("Complete() error = %v", err)
	}

	var events []StreamEvent
	for e := range ch {
		events = append(events, e)
	}

	if len(events) == 0 {
		t.Fatal("expected at least one event")
	}

	var foundToolCalls []ToolCall
	for _, e := range events {
		if len(e.ToolCalls) > 0 {
			foundToolCalls = e.ToolCalls
		}
	}
	if len(foundToolCalls) != 1 {
		t.Errorf("expected 1 tool call across events, got %d", len(foundToolCalls))
	}
	if len(foundToolCalls) > 0 && foundToolCalls[0].Function.Name != "get_weather" {
		t.Errorf("expected tool name 'get_weather', got %q", foundToolCalls[0].Function.Name)
	}
}

func TestChatFlow_WithMultimodalContent(t *testing.T) {
	tokenSvc := &mockTokenService{
		tokens: []*store.Token{{ID: 1, Token: "tok1", Pool: "basic"}},
	}

	client := &mockCopilotClient{
		events: []copilot.StreamEvent{
			{Text: "I see a cat in the image."},
		},
	}

	cfg := &ChatFlowConfig{RetryConfig: DefaultRetryConfig(), TokenConfig: testFlowTokenConfig()}
	flow := NewChatFlow(tokenSvc, func(token string) copilot.Client { return client }, cfg)

	req := &ChatRequest{
		Messages: []Message{
			{
				Role: "user",
				Content: []any{
					map[string]any{"type": "text", "text": "What's in this image?"},
					map[string]any{
						"type": "image_url",
						"image_url": map[string]any{
							"url": "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNk+M9QDwADhgGAWjR9awAAAABJRU5ErkJggg==",
						},
					},
				},
			},
		},
		Model: "copilot-free",
	}

	ctx := context.Background()
	ch, err := flow.Complete(ctx, req)
	if err != nil {
		t.Fatalf("Complete() error = %v", err)
	}

	var events []StreamEvent
	for e := range ch {
		if e.Error != nil {
			t.Fatalf("unexpected error: %v", e.Error)
		}
		events = append(events, e)
	}

	if len(events) == 0 {
		t.Fatal("expected at least one event")
	}

	found := false
	for _, e := range events {
		if e.Content != "" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected content in response")
	}
}

// mockUsageRecorder implements UsageRecorder for testing.
type mockUsageRecorder struct {
	mu      sync.Mutex
	records []*store.UsageLog
}

func (m *mockUsageRecorder) Record(ctx context.Context, log *store.UsageLog) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.records = append(m.records, log)
	return nil
}

func TestChatFlow_HotReload(t *testing.T) {
	tokenSvc := &mockTokenService{
		tokens: []*store.Token{
			{ID: 1, Token: "tok1", Pool: "basic"},
			{ID: 2, Token: "tok2", Pool: "basic"},
		},
	}

	callCount := 0
	// Client always fails with retryable error
	clientFactory := func(token string) copilot.Client {
		callCount++
		return &mockCopilotClient{chatErr: copilot.ErrRateLimited}
	}

	// Start with MaxTokens=1, PerTokenRetries=1 (only 1 attempt total)
	currentMax := 1
	currentPerToken := 1
	cfg := &ChatFlowConfig{
		RetryConfig: &RetryConfig{
			MaxTokens:       6, // fallback, should not be used
			PerTokenRetries: 2,
			BaseDelay:       time.Millisecond,
			MaxDelay:        10 * time.Millisecond,
			JitterFactor:    0,
		},
		RetryConfigProvider: func() *RetryConfig {
			return &RetryConfig{
				MaxTokens:       currentMax,
				PerTokenRetries: currentPerToken,
				BaseDelay:       time.Millisecond,
				MaxDelay:        10 * time.Millisecond,
				JitterFactor:    0,
			}
		},
		TokenConfig: testFlowTokenConfig(),
	}
	f := NewChatFlow(tokenSvc, clientFactory, cfg)

	req := &ChatRequest{
		Messages: []Message{{Role: "user", Content: "Hi"}},
		Model:    "copilot-free",
	}

	ctx := context.Background()
	ch, _ := f.Complete(ctx, req)
	for range ch {
	}

	// With MaxTokens=1 from provider, should have only 1 attempt
	if callCount != 1 {
		t.Errorf("expected 1 attempt from hot-reload provider (MaxTokens=1), got %d", callCount)
	}
}

func TestChatFlow_RecordUsageAPIKeyID(t *testing.T) {
	tokenSvc := &mockTokenService{
		tokens: []*store.Token{{ID: 1, Token: "tok1", Pool: "basic"}},
	}

	client := &mockCopilotClient{
		events: []copilot.StreamEvent{{Text: "Hello"}},
	}

	cfg := &ChatFlowConfig{RetryConfig: DefaultRetryConfig(), TokenConfig: testFlowTokenConfig()}
	f := NewChatFlow(tokenSvc, func(token string) copilot.Client { return client }, cfg)

	recorder := &mockUsageRecorder{}
	f.SetUsageRecorder(recorder)

	req := &ChatRequest{
		Messages: []Message{{Role: "user", Content: "Hi"}},
		Model:    "copilot-free",
	}

	// Set FlowAPIKeyIDKey in context
	ctx := context.WithValue(context.Background(), FlowAPIKeyIDKey, uint(42))
	ch, err := f.Complete(ctx, req)
	if err != nil {
		t.Fatalf("Complete() error = %v", err)
	}
	for range ch {
	}

	// Wait briefly for async recording
	time.Sleep(50 * time.Millisecond)

	recorder.mu.Lock()
	defer recorder.mu.Unlock()
	if len(recorder.records) != 1 {
		t.Fatalf("expected 1 usage record, got %d", len(recorder.records))
	}
	if recorder.records[0].APIKeyID != 42 {
		t.Errorf("expected APIKeyID=42, got %d", recorder.records[0].APIKeyID)
	}
}

func TestChatFlow_EstimatedTrue_WhenNoUsageFromUpstream(t *testing.T) {
	tokenSvc := &mockTokenService{
		tokens: []*store.Token{{ID: 1, Token: "tok1", Pool: "basic"}},
	}

	client := &mockCopilotClient{
		events: []copilot.StreamEvent{{Text: "Hello world response"}},
	}

	cfg := &ChatFlowConfig{RetryConfig: DefaultRetryConfig(), TokenConfig: testFlowTokenConfig()}
	f := NewChatFlow(tokenSvc, func(token string) copilot.Client { return client }, cfg)

	recorder := &mockUsageRecorder{}
	f.SetUsageRecorder(recorder)

	req := &ChatRequest{
		Messages: []Message{{Role: "user", Content: "Hi"}},
		Model:    "copilot-free",
	}

	ch, err := f.Complete(context.Background(), req)
	if err != nil {
		t.Fatalf("Complete() error = %v", err)
	}
	for range ch {
	}

	time.Sleep(50 * time.Millisecond)

	recorder.mu.Lock()
	defer recorder.mu.Unlock()
	if len(recorder.records) != 1 {
		t.Fatalf("expected 1 usage record, got %d", len(recorder.records))
	}
	// Copilot always estimates (no upstream token count)
	if !recorder.records[0].Estimated {
		t.Error("expected Estimated=true for Copilot (no upstream usage data)")
	}
}
