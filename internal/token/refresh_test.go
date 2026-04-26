package token

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/crmmc/copilotpi/internal/config"
	"github.com/crmmc/copilotpi/internal/store"
)

// TestScheduler_Start verifies the scheduler restores expired cooling tokens to active.
// For CopilotPi, recovery is handled by time-based restore (no upstream quota API).
func TestScheduler_Start(t *testing.T) {
	cfg := &config.TokenConfig{FailThreshold: 3}
	m := NewTokenManager(cfg)

	// Add a cooling token with expired CoolUntil
	coolUntil := time.Now().Add(-1 * time.Minute)
	token := &store.Token{
		ID:        1,
		Token:     "test-token",
		Pool:      PoolBasic,
		Status:    string(StatusCooling),
		ChatQuota: 0,
		CoolUntil: &coolUntil,
	}
	m.AddToken(token)

	scheduler := NewScheduler(m, &config.TokenConfig{QuotaRecoveryMode: RecoveryModeAuto}, "")

	ctx, cancel := context.WithCancel(context.Background())
	scheduler.Start(ctx)

	// Wait for at least one refresh cycle
	time.Sleep(100 * time.Millisecond)

	cancel()
	scheduler.Stop()

	// Token should be restored to active
	if token.Status != string(StatusActive) {
		t.Errorf("expected token status=active, got %s", token.Status)
	}
}

func TestScheduler_OnlyRefreshesExpiredCoolingTokens(t *testing.T) {
	cfg := &config.TokenConfig{FailThreshold: 3}
	m := NewTokenManager(cfg)

	var refreshCount atomic.Int32
	_ = refreshCount // just track via status changes

	// Active token - should NOT be touched
	activeToken := &store.Token{
		ID:        1,
		Token:     "active-token",
		Pool:      PoolBasic,
		Status:    string(StatusActive),
		ChatQuota: 50,
	}
	m.AddToken(activeToken)

	// Cooling token with future CoolUntil - should NOT be refreshed
	futureCool := time.Now().Add(10 * time.Minute)
	futureCoolingToken := &store.Token{
		ID:        2,
		Token:     "future-cooling",
		Pool:      PoolBasic,
		Status:    string(StatusCooling),
		ChatQuota: 0,
		CoolUntil: &futureCool,
	}
	m.AddToken(futureCoolingToken)

	// Cooling token with expired CoolUntil - SHOULD be refreshed
	pastCool := time.Now().Add(-1 * time.Minute)
	expiredCoolingToken := &store.Token{
		ID:        3,
		Token:     "expired-cooling",
		Pool:      PoolBasic,
		Status:    string(StatusCooling),
		ChatQuota: 0,
		CoolUntil: &pastCool,
	}
	m.AddToken(expiredCoolingToken)

	scheduler := NewScheduler(m, &config.TokenConfig{QuotaRecoveryMode: RecoveryModeAuto}, "")

	ctx, cancel := context.WithCancel(context.Background())
	scheduler.Start(ctx)

	time.Sleep(100 * time.Millisecond)

	cancel()
	scheduler.Stop()

	// Active token should remain active
	if activeToken.Status != string(StatusActive) {
		t.Errorf("active token status changed unexpectedly: %s", activeToken.Status)
	}
	// Future cooling token should remain cooling
	if futureCoolingToken.Status != string(StatusCooling) {
		t.Errorf("future cooling token restored too early: %s", futureCoolingToken.Status)
	}
	// Expired cooling token should now be active
	if expiredCoolingToken.Status != string(StatusActive) {
		t.Errorf("expired cooling token not restored: %s", expiredCoolingToken.Status)
	}
}

func TestScheduler_ConcurrencyLimit(t *testing.T) {
	cfg := &config.TokenConfig{FailThreshold: 3}
	m := NewTokenManager(cfg)


	var tokens []*store.Token
	for i := uint(1); i <= 10; i++ {
		coolUntil := time.Now().Add(-1 * time.Minute)
		tok := &store.Token{
			ID:        i,
			Token:     "token-" + string(rune('0'+i)),
			Pool:      PoolBasic,
			Status:    string(StatusCooling),
			ChatQuota: 0,
			CoolUntil: &coolUntil,
		}
		m.AddToken(tok)
		tokens = append(tokens, tok)
	}

	scheduler := NewScheduler(m, &config.TokenConfig{QuotaRecoveryMode: RecoveryModeAuto}, "")

	ctx, cancel := context.WithCancel(context.Background())
	scheduler.Start(ctx)

	time.Sleep(200 * time.Millisecond)

	cancel()
	scheduler.Stop()

	// All expired cooling tokens should be restored
	for _, tok := range tokens {
		if tok.Status != string(StatusActive) {
			t.Errorf("token %d not restored: status=%s", tok.ID, tok.Status)
		}
	}
}

func TestScheduler_Stop(t *testing.T) {
	cfg := &config.TokenConfig{FailThreshold: 3}
	m := NewTokenManager(cfg)

	scheduler := NewScheduler(m, &config.TokenConfig{QuotaRecoveryMode: RecoveryModeAuto}, "")

	ctx, cancel := context.WithCancel(context.Background())
	scheduler.Start(ctx)

	time.Sleep(30 * time.Millisecond)

	// Stop should complete without blocking
	done := make(chan struct{})
	go func() {
		cancel()
		scheduler.Stop()
		close(done)
	}()

	select {
	case <-done:
		// OK
	case <-time.After(1 * time.Second):
		t.Error("Stop() blocked for too long")
	}
}
