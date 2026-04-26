package token

import (
	"context"
	"testing"
	"time"

	"github.com/crmmc/copilotpi/internal/config"
	"github.com/crmmc/copilotpi/internal/store"
)

func TestQuota_Consume(t *testing.T) {
	cfg := &config.TokenConfig{FailThreshold: 3}
	m := NewTokenManager(cfg)

	token := &store.Token{
		ID:        1,
		Token:     "test-token",
		Pool:      PoolBasic,
		Status:    string(StatusActive),
		ChatQuota: 10,
	}
	m.AddToken(token)

	t.Run("deducts quota and returns remaining", func(t *testing.T) {
		remaining, err := m.Consume(1, CategoryChat, 1)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if remaining != 9 {
			t.Errorf("expected remaining=9, got %d", remaining)
		}

		// Verify token state updated
		tok := m.GetToken(1)
		if tok.ChatQuota != 9 {
			t.Errorf("expected token.ChatQuota=9, got %d", tok.ChatQuota)
		}
	})

	t.Run("marks token dirty", func(t *testing.T) {
		// Consume again
		m.Consume(1, CategoryChat, 1)
		dirty := m.GetDirtyTokens()
		found := false
		for _, d := range dirty {
			if d.ID == 1 {
				found = true
				break
			}
		}
		if !found {
			t.Error("expected token to be marked dirty")
		}
	})

	t.Run("returns error when quota is zero", func(t *testing.T) {
		// Set quota to 0
		tok := m.GetToken(1)
		tok.ChatQuota = 0

		_, err := m.Consume(1, CategoryChat, 1)
		if err != ErrNoQuota {
			t.Errorf("expected ErrNoQuota, got %v", err)
		}
	})

	t.Run("marks token cooling when quota reaches zero", func(t *testing.T) {
		tok := m.GetToken(1)
		tok.ChatQuota = 1
		tok.ImageQuota = 0
		tok.VideoQuota = 0
		tok.Status = string(StatusActive)
		tok.CoolUntil = nil

		remaining, err := m.Consume(1, CategoryChat, 1)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if remaining != 0 {
			t.Fatalf("expected remaining=0, got %d", remaining)
		}
		if tok.Status != string(StatusCooling) {
			t.Errorf("expected status=cooling, got %s", tok.Status)
		}
		if tok.CoolUntil == nil {
			t.Error("expected CoolUntil to be set")
		}
	})

	t.Run("returns error for non-existent token", func(t *testing.T) {
		_, err := m.Consume(999, CategoryChat, 1)
		if err != ErrTokenNotFound {
			t.Errorf("expected ErrTokenNotFound, got %v", err)
		}
	})
}

// TestQuota_SyncQuota verifies that SyncQuota is a no-op (returns nil) for Copilot.
func TestQuota_SyncQuota(t *testing.T) {
	cfg := &config.TokenConfig{FailThreshold: 3}
	m := NewTokenManager(cfg)

	token := &store.Token{
		ID:        2,
		Token:     "test-token-2",
		Pool:      PoolBasic,
		Status:    string(StatusActive),
		ChatQuota: 10,
	}
	m.AddToken(token)

	ctx := context.Background()
	// SyncQuota is a no-op — must return nil
	err := m.SyncQuota(ctx, token, "http://ignored")
	if err != nil {
		t.Fatalf("SyncQuota() returned unexpected error: %v", err)
	}
	// Token state must be unchanged
	if token.ChatQuota != 10 {
		t.Errorf("SyncQuota() should not modify ChatQuota; got %d, want 10", token.ChatQuota)
	}
}

// TestDetectImportProfile verifies that all Copilot tokens are classified as premium (PoolSuper).
func TestDetectImportProfile(t *testing.T) {
	profile, err := DetectImportProfile(context.Background(), "cookie-bundle-value", "http://ignored", &config.TokenConfig{
		DefaultChatQuota:  150,
		DefaultImageQuota: 9,
		DefaultVideoQuota: 4,
	})
	if err != nil {
		t.Fatalf("DetectImportProfile() error: %v", err)
	}
	if profile.Pool != PoolSuper {
		t.Errorf("expected Pool=%s, got %s", PoolSuper, profile.Pool)
	}
	if profile.Priority != 10 {
		t.Errorf("expected Priority=10, got %d", profile.Priority)
	}
	if profile.ChatQuota != 150 {
		t.Errorf("expected ChatQuota=150, got %d", profile.ChatQuota)
	}
	if profile.ImageQuota != 9 {
		t.Errorf("expected ImageQuota=9, got %d", profile.ImageQuota)
	}
	if profile.VideoQuota != 4 {
		t.Errorf("expected VideoQuota=4, got %d", profile.VideoQuota)
	}
}

func TestDetectImportProfile_DefaultsWhenConfigNil(t *testing.T) {
	profile, err := DetectImportProfile(context.Background(), "bundle", "http://ignored", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if profile.Pool != PoolSuper {
		t.Errorf("expected PoolSuper, got %s", profile.Pool)
	}
	if profile.ChatQuota != 200 {
		t.Errorf("expected default ChatQuota=200, got %d", profile.ChatQuota)
	}
}

func TestCoolingDuration_RespectsConfig(t *testing.T) {
	cfg := &config.TokenConfig{
		FailThreshold:       3,
		SuperCoolDurationMin: 60,
		BasicCoolDurationMin: 30,
	}
	m := NewTokenManager(cfg)

	superToken := &store.Token{Pool: PoolSuper}
	basicToken := &store.Token{Pool: PoolBasic}

	superDur := m.coolingDurationForToken(superToken)
	if superDur != 60*time.Minute {
		t.Errorf("expected 60m cooling for super, got %v", superDur)
	}

	basicDur := m.coolingDurationForToken(basicToken)
	if basicDur != 30*time.Minute {
		t.Errorf("expected 30m cooling for basic, got %v", basicDur)
	}
}
