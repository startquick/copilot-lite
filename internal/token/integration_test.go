package token_test

import (
	"context"
	"testing"
	"time"

	"github.com/crmmc/copilotpi/internal/config"
	"github.com/crmmc/copilotpi/internal/store"
	"github.com/crmmc/copilotpi/internal/token"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func setupIntegrationDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}
	if err := db.AutoMigrate(&store.Token{}); err != nil {
		t.Fatalf("failed to migrate: %v", err)
	}
	return db
}

// TestIntegration_QuotaLifecycle verifies that consumption reduces quota
// and is persisted to the database via the periodic flusher.
func TestIntegration_QuotaLifecycle(t *testing.T) {
	db := setupIntegrationDB(t)
	cfg := &config.TokenConfig{FailThreshold: 3}
	manager := token.NewTokenManager(cfg)

	// Create token in DB
	tok := &store.Token{
		Token:     "integration-token",
		Pool:      token.PoolBasic,
		Status:    string(token.StatusActive),
		ChatQuota: 10,
	}
	db.Create(tok)
	manager.AddToken(tok)

	// Start persister
	persister := token.NewPersister(manager, db)
	ctx, cancel := context.WithCancel(context.Background())
	persister.Start(ctx, 50*time.Millisecond)

	t.Run("consume reduces quota and persists", func(t *testing.T) {
		remaining, err := manager.Consume(tok.ID, token.CategoryChat, 1)
		if err != nil {
			t.Fatalf("consume failed: %v", err)
		}
		if remaining != 9 {
			t.Errorf("expected remaining=9, got %d", remaining)
		}

		// Wait for periodic flush
		time.Sleep(100 * time.Millisecond)

		// Verify persisted
		var dbTok store.Token
		db.First(&dbTok, tok.ID)
		if dbTok.ChatQuota != 9 {
			t.Errorf("expected DB quota=9, got %d", dbTok.ChatQuota)
		}
	})

	t.Run("SyncQuota is no-op for Copilot", func(t *testing.T) {
		// SyncQuota does nothing in CopilotPi — no upstream quota API
		err := manager.SyncQuota(ctx, tok, "")
		if err != nil {
			t.Errorf("SyncQuota should not error, got: %v", err)
		}
		// Quota unchanged
		if tok.ChatQuota != 9 {
			t.Errorf("SyncQuota should not change quota, got %d", tok.ChatQuota)
		}
	})

	cancel()
	persister.Stop()
}

func TestIntegration_CoolingRecovery(t *testing.T) {
	db := setupIntegrationDB(t)
	cfg := &config.TokenConfig{FailThreshold: 3}
	manager := token.NewTokenManager(cfg)

	// Create cooling token with expired CoolUntil
	coolUntil := time.Now().Add(-1 * time.Minute)
	tok := &store.Token{
		Token:     "cooling-token",
		Pool:      token.PoolBasic,
		Status:    string(token.StatusCooling),
		ChatQuota: 0,
		CoolUntil: &coolUntil,
	}
	db.Create(tok)
	manager.AddToken(tok)

	// Start scheduler (RecoveryModeAuto — no upstream API needed) and persister
	scheduler := token.NewScheduler(manager, &config.TokenConfig{QuotaRecoveryMode: token.RecoveryModeAuto}, "")
	persister := token.NewPersister(manager, db)

	ctx, cancel := context.WithCancel(context.Background())
	scheduler.Start(ctx)
	persister.Start(ctx, 50*time.Millisecond)

	// Wait for refresh and persist
	time.Sleep(200 * time.Millisecond)

	cancel()
	scheduler.Stop()
	persister.Stop()

	// Verify persisted - read from DB to avoid race
	var dbTok store.Token
	db.First(&dbTok, tok.ID)
	if dbTok.Status != string(token.StatusActive) {
		t.Errorf("expected DB status=active, got %s", dbTok.Status)
	}
}

func TestIntegration_FullCycle(t *testing.T) {
	db := setupIntegrationDB(t)
	cfg := &config.TokenConfig{FailThreshold: 3}
	manager := token.NewTokenManager(cfg)

	// Create multiple tokens
	for i := 0; i < 5; i++ {
		tok := &store.Token{
			Token:     "token-" + string(rune('a'+i)),
			Pool:      token.PoolBasic,
			Status:    string(token.StatusActive),
			ChatQuota: 10,
		}
		db.Create(tok)
		manager.AddToken(tok)
	}

	// Start components
	scheduler := token.NewScheduler(manager, &config.TokenConfig{QuotaRecoveryMode: token.RecoveryModeAuto}, "")
	persister := token.NewPersister(manager, db)

	ctx, cancel := context.WithCancel(context.Background())
	scheduler.Start(ctx)
	persister.Start(ctx, 50*time.Millisecond)

	// Consume from all tokens
	for i := uint(1); i <= 5; i++ {
		_, err := manager.Consume(i, token.CategoryChat, 1)
		if err != nil {
			t.Errorf("consume token %d failed: %v", i, err)
		}
	}

	// Wait for persist
	time.Sleep(100 * time.Millisecond)

	// Verify all persisted
	var tokens []store.Token
	db.Find(&tokens)
	for _, tok := range tokens {
		if tok.ChatQuota != 9 {
			t.Errorf("token %d: expected quota=9, got %d", tok.ID, tok.ChatQuota)
		}
	}

	cancel()
	scheduler.Stop()
	persister.Stop()
}
