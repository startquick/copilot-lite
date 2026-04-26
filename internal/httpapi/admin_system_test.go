package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/crmmc/copilotpi/internal/config"
	"github.com/crmmc/copilotpi/internal/store"
)

type mockSystemConfigStore struct {
	values map[string]string
}

func (m *mockSystemConfigStore) Get(key string) (string, error) {
	return m.values[key], nil
}

func (m *mockSystemConfigStore) GetAll() (map[string]string, error) {
	if m.values == nil {
		return map[string]string{}, nil
	}
	return m.values, nil
}

func TestHandleSystemStatus_APIKeys(t *testing.T) {
	ctx := context.Background()
	ts := newMockTokenStore()
	ts.CreateToken(ctx, &store.Token{Status: "active"})
	ts.CreateToken(ctx, &store.Token{Status: "active"})
	ts.CreateToken(ctx, &store.Token{Status: "disabled"})

	aks := newMockAPIKeyStore()
	// Add keys with specific statuses to get total=3, active=2
	for i := 0; i < 7; i++ {
		aks.Create(ctx, &store.APIKey{Name: "a", Status: "active"})
	}
	for i := 0; i < 3; i++ {
		aks.Create(ctx, &store.APIKey{Name: "i", Status: "inactive"})
	}

	handler := handleSystemStatus(ts, aks, time.Now(), "1.0.0", nil)

	req := httptest.NewRequest(http.MethodGet, "/system/status", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var resp SystemStatusResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.APIKeys.Total != 10 {
		t.Errorf("expected api_keys.total=10, got %d", resp.APIKeys.Total)
	}
	if resp.APIKeys.Active != 7 {
		t.Errorf("expected api_keys.active=7, got %d", resp.APIKeys.Active)
	}
	if resp.Tokens.Total != 3 {
		t.Errorf("expected tokens.total=3, got %d", resp.Tokens.Total)
	}
	if resp.Tokens.Active != 2 {
		t.Errorf("expected tokens.active=2, got %d", resp.Tokens.Active)
	}
	if resp.Config.AppKeySource != "default" {
		t.Errorf("expected default app_key source, got %q", resp.Config.AppKeySource)
	}
}

func TestHandleSystemStatus_NilAPIKeyStore(t *testing.T) {
	ts := newMockTokenStore()

	handler := handleSystemStatus(ts, nil, time.Now(), "1.0.0", nil)

	req := httptest.NewRequest(http.MethodGet, "/system/status", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var resp SystemStatusResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// With nil apiKeyStore, should return zeros (no panic)
	if resp.APIKeys.Total != 0 {
		t.Errorf("expected api_keys.total=0, got %d", resp.APIKeys.Total)
	}
	if resp.APIKeys.Active != 0 {
		t.Errorf("expected api_keys.active=0, got %d", resp.APIKeys.Active)
	}
}

func TestBuildSystemConfigInspector_ConfigSource(t *testing.T) {
	inspector := buildSystemConfigInspector(func() *config.Config {
		cfg := config.DefaultConfig()
		cfg.App.AppKey = "file-key"
		return cfg
	}, nil)

	status := inspector()
	if status.AppKeySource != "config" {
		t.Fatalf("expected config source, got %q", status.AppKeySource)
	}
	if status.HasDBOverrides {
		t.Fatalf("expected no DB overrides")
	}
}

func TestBuildSystemConfigInspector_DBSourceAndOverrideCount(t *testing.T) {
	inspector := buildSystemConfigInspector(func() *config.Config {
		cfg := config.DefaultConfig()
		cfg.App.AppKey = "runtime-key"
		return cfg
	}, &mockSystemConfigStore{values: map[string]string{
		"app.app_key":      "db-key",
		"token.fail_count": "5",
	}})

	status := inspector()
	if status.AppKeySource != "db" {
		t.Fatalf("expected db source, got %q", status.AppKeySource)
	}
	if !status.HasDBOverrides {
		t.Fatalf("expected DB overrides true")
	}
	if status.DBOverrideCount != 2 {
		t.Fatalf("expected DB override count 2, got %d", status.DBOverrideCount)
	}
}
