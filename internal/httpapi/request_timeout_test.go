package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/crmmc/copilotpi/internal/config"
	"github.com/crmmc/copilotpi/internal/flow"
)

func TestRouteTimeout(t *testing.T) {
	cfg := &config.Config{
		App: config.AppConfig{RequestTimeout: 90},
	}

	tests := []struct {
		name   string
		method string
		path   string
		want   time.Duration
	}{
		{name: "openai chat uses app.request_timeout", method: http.MethodPost, path: "/v1/chat/completions", want: 90 * time.Second},
		{name: "anthropic messages uses app.request_timeout", method: http.MethodPost, path: "/v1/messages", want: 90 * time.Second},
		{name: "other POST uses app.request_timeout", method: http.MethodPost, path: "/admin/tokens/batch", want: 90 * time.Second},
		{name: "GET uses app.request_timeout", method: http.MethodGet, path: "/v1/chat/completions", want: 90 * time.Second},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := routeTimeout(cfg, tt.method, tt.path)
			if got != tt.want {
				t.Fatalf("routeTimeout() = %s, want %s", got, tt.want)
			}
		})
	}
}

func TestRouteTimeout_NilConfig(t *testing.T) {
	got := routeTimeout(nil, http.MethodPost, "/v1/chat/completions")
	if got != 300*time.Second {
		t.Fatalf("routeTimeout(nil) for chat = %s, want 300s", got)
	}
}

func TestRouteTimeout_ZeroRequestTimeout(t *testing.T) {
	// Chat routes fall back to 300s when request_timeout is 0
	cfg := &config.Config{App: config.AppConfig{RequestTimeout: 0}}
	got := routeTimeout(cfg, http.MethodPost, "/v1/chat/completions")
	if got != 300*time.Second {
		t.Fatalf("routeTimeout(zero) = %s, want 300s", got)
	}
}

func TestRouteTimeout_ZeroRequestTimeoutOther(t *testing.T) {
	cfg := &config.Config{App: config.AppConfig{RequestTimeout: 0}}
	got := routeTimeout(cfg, http.MethodGet, "/admin/tokens")
	if got != defaultRequestTimeout {
		t.Fatalf("routeTimeout(zero request_timeout) = %s, want %s", got, defaultRequestTimeout)
	}
}

func TestRequestTimeoutMiddleware_UsesRouteSpecificDeadline(t *testing.T) {
	cfg := &config.Config{
		App: config.AppConfig{RequestTimeout: 90},
	}

	handler := requestTimeoutMiddleware(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		deadline, ok := r.Context().Deadline()
		if !ok {
			t.Fatal("expected context deadline")
		}
		remaining := time.Until(deadline)
		_, _ = w.Write([]byte(strconv.FormatInt(int64(remaining/time.Second), 10)))
	}))

	tests := []struct {
		name string
		path string
		want time.Duration
	}{
		{name: "openai chat route uses app.request_timeout", path: "/v1/chat/completions", want: 90 * time.Second},
		{name: "anthropic messages route uses app.request_timeout", path: "/v1/messages", want: 90 * time.Second},
		{name: "default route uses app.request_timeout", path: "/admin/tokens", want: 90 * time.Second},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, tt.path, nil)
			rr := httptest.NewRecorder()

			handler.ServeHTTP(rr, req)

			gotSeconds, err := strconv.Atoi(rr.Body.String())
			if err != nil {
				t.Fatalf("parse response: %v", err)
			}
			got := time.Duration(gotSeconds) * time.Second
			if got < tt.want-2*time.Second || got > tt.want {
				t.Fatalf("deadline = %s, want around %s", got, tt.want)
			}
		})
	}
}

func TestRequestTimeoutMiddleware_HotReload(t *testing.T) {
	cfg := &config.Config{App: config.AppConfig{RequestTimeout: 120}}
	handler := requestTimeoutMiddleware(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		deadline, _ := r.Context().Deadline()
		remaining := time.Until(deadline)
		_, _ = w.Write([]byte(strconv.FormatInt(int64(remaining/time.Second), 10)))
	}))

	// First request: 120s
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	got, _ := strconv.Atoi(rr.Body.String())
	if got < 118 || got > 120 {
		t.Fatalf("before hot-reload: deadline = %ds, want ~120s", got)
	}

	// Hot-reload: change timeout to 600s
	cfg.App.RequestTimeout = 600

	req = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	got, _ = strconv.Atoi(rr.Body.String())
	if got < 598 || got > 600 {
		t.Fatalf("after hot-reload: deadline = %ds, want ~600s", got)
	}

	req = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	got, _ = strconv.Atoi(rr.Body.String())
	if got < 598 || got > 600 {
		t.Fatalf("anthropic after hot-reload: deadline = %ds, want ~600s", got)
	}
}

func TestBridgeFlowContext(t *testing.T) {
	ctx := context.WithValue(context.Background(), apiKeyIDKey, uint(7))
	bridged := BridgeFlowContext(ctx)
	id := flow.FlowAPIKeyIDFromContext(bridged)
	if id != 7 {
		t.Fatalf("FlowAPIKeyIDFromContext() = %d, want 7", id)
	}
}
