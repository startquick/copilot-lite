// Package main is the entry point for CopilotPi.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/crmmc/copilotpi/internal/cache"
	"github.com/crmmc/copilotpi/internal/config"
	"github.com/crmmc/copilotpi/internal/copilot"
	"github.com/crmmc/copilotpi/internal/flow"
	"github.com/crmmc/copilotpi/internal/httpapi"
	"github.com/crmmc/copilotpi/internal/httpapi/anthropic"
	"github.com/crmmc/copilotpi/internal/httpapi/openai"
	"github.com/crmmc/copilotpi/internal/logging"
	"github.com/crmmc/copilotpi/internal/store"
	"github.com/crmmc/copilotpi/internal/token"
)

var (
	version   = "dev"
	buildTime = "unknown"
)

const serverWriteTimeout = 330 * time.Second
const tokenFlushInterval = 30 * time.Second
const defaultAdminAppKey = "QUICKstart012345+"

func main() {
	// Parse flags
	configPath := flag.String("config", "config.toml", "path to config file")
	showVersion := flag.Bool("version", false, "show version and exit")
	flag.Parse()

	if *showVersion {
		fmt.Printf("copilotpi %s (built %s)\n", version, buildTime)
		os.Exit(0)
	}

	// Load configuration
	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load config: %v\n", err)
		os.Exit(1)
	}
	if err := validateStartupConfig(cfg); err != nil {
		fmt.Fprintf(os.Stderr, "invalid startup config: %v\n", err)
		os.Exit(1)
	}

	// Setup logging
	logging.Setup(cfg.App.LogLevel, cfg.App.LogJSON, &logging.FileConfig{
		Path:       cfg.App.LogFilePath,
		MaxSizeMB:  cfg.App.LogMaxSizeMB,
		MaxBackups: cfg.App.LogMaxBackups,
	})
	logging.Info("starting copilotpi", "version", version, "config", *configPath)

	// Open database
	db, err := store.Open(cfg)
	if err != nil {
		logging.Error("failed to open database", "error", err)
		os.Exit(1)
	}
	defer store.Close(db)

	// Run migrations
	if err := store.AutoMigrate(db); err != nil {
		logging.Error("failed to migrate database", "error", err)
		os.Exit(1)
	}
	logging.Info("database ready", "driver", cfg.App.DBDriver)

	// Load DB config overrides (DB > config file > defaults)
	configStore := store.NewConfigStore(db)
	dbOverrides, err := configStore.GetAll()
	if err != nil {
		logging.Error("failed to load config overrides from database", "error", err)
	} else if len(dbOverrides) > 0 {
		overriddenKeys := cfg.ApplyDBOverrides(dbOverrides)
		if len(overriddenKeys) > 0 {
			logging.Warn("configuration logic overloaded from database", "overridden_keys_count", len(overriddenKeys))
			logging.Warn("infrastructure values in config.toml will be respected, but application settings in config.toml will be ignored")
		} else {
			logging.Info("applied database config overrides", "count", len(dbOverrides))
		}
	}
	runtimeCfg := config.NewRuntime(cfg)
	rootCtx, rootCancel := context.WithCancel(context.Background())
	defer rootCancel()

	// Create token service (Copilot auth uses cookie-bundle, no upstream URL needed)
	tokenStore := store.NewTokenStore(db)
	tokenSvc := token.NewTokenService(&cfg.Token, tokenStore, "")
	if err := tokenSvc.LoadTokens(rootCtx); err != nil {
		logging.Error("failed to load tokens", "error", err)
		os.Exit(1)
	}
	tokenSvc.StartTicker(rootCtx)
	logging.Info("token service ready", "stats", tokenSvc.Stats())

	// Start quota recovery scheduler (auto-replenish mode only for Copilot)
	scheduler := token.NewScheduler(tokenSvc.Manager(), &cfg.Token, "")
	scheduler.SetConfigProvider(func() *config.TokenConfig {
		return &runtimeCfg.Get().Token
	})
	scheduler.Start(rootCtx)
	logging.Info("token quota recovery scheduler started", "mode", cfg.Token.QuotaRecoveryMode)

	// Start token state persistence loop
	tokenPersister := token.NewPersister(tokenSvc.Manager(), db)
	tokenPersister.Start(rootCtx, tokenFlushInterval)
	logging.Info("token persistence loop started")

	// Create ChatFlow wired to copilot.Client factory
	copilotCfg := runtimeCfg.Get().Copilot
	chatFlow := flow.NewChatFlow(
		tokenSvc,
		func(cookieBundle string) copilot.Client {
			c, err := copilot.NewClient(cookieBundle, &copilotCfg)
			if err != nil {
				return nil
			}
			return c
		},
		&flow.ChatFlowConfig{
			RetryConfig: flow.DefaultRetryConfig(),
			RetryConfigProvider: func() *flow.RetryConfig {
				current := runtimeCfg.Get()
				retry := current.Retry
				return &flow.RetryConfig{
					MaxTokens:               retry.MaxTokens,
					PerTokenRetries:         retry.PerTokenRetries,
					BaseDelay:               time.Duration(retry.RetryBackoffBase * float64(time.Second)),
					MaxDelay:                time.Duration(retry.RetryBackoffMax * float64(time.Second)),
					JitterFactor:            0.25,
					BackoffFactor:           retry.RetryBackoffFactor,
					ResetSessionStatusCodes: append([]int(nil), retry.ResetSessionStatusCodes...),
					CoolingStatusCodes:      append([]int(nil), retry.CoolingStatusCodes...),
					RetryBudget:             time.Duration(retry.RetryBudget * float64(time.Second)),
				}
			},
			TokenConfigProvider: func() *config.TokenConfig {
				return &runtimeCfg.Get().Token
			},
			AppConfigProvider: func() *config.AppConfig {
				return &runtimeCfg.Get().App
			},
			FilterTagsProvider: func() []string {
				current := runtimeCfg.Get()
				return append([]string(nil), current.App.FilterTags...)
			},
		},
	)
	logging.Info("chat flow ready")

	// Create usage log store and buffer
	usageLogStore := store.NewUsageLogStore(db)
	flushInterval := time.Duration(cfg.Token.UsageFlushIntervalSec) * time.Second
	usageBuffer := flow.NewUsageBuffer(usageLogStore, flushInterval)
	usageBuffer.Start()
	chatFlow.SetUsageRecorder(usageBuffer)
	logging.Info("usage buffer ready", "flush_interval", flushInterval)

	// Create API key store
	apiKeyStore := store.NewAPIKeyStore(db)

	// Wire API key usage increment into chat flow (only on success)
	chatFlow.SetAPIKeyUsageInc(func(ctx context.Context, apiKeyID uint) {
		_ = apiKeyStore.IncrementUsage(ctx, apiKeyID)
	})

	// Create cache service
	cacheSvc := cache.NewService("data")
	logging.Info("cache service ready", "data_dir", "data")

	// Create OpenAI provider
	openaiHandler := &openai.Handler{
		ChatFlow: chatFlow,
		Cfg:      runtimeCfg.Get(),
		Runtime:  runtimeCfg,
	}

	// Create Anthropic provider
	anthropicHandler := &anthropic.Handler{
		ChatFlow: chatFlow,
		Cfg:      runtimeCfg.Get(),
		Runtime:  runtimeCfg,
	}

	// Create HTTP server
	srv := httpapi.NewServer(&httpapi.ServerConfig{
		AppKey:            runtimeCfg.Get().App.AppKey,
		Version:           version,
		Config:            runtimeCfg.Get(),
		Runtime:           runtimeCfg,
		ChatProviders:     []httpapi.ChatProvider{openaiHandler, anthropicHandler},
		TokenStore:        tokenStore,
		TokenRefresher:    tokenSvc,
		TokenPoolSyncer:   tokenSvc,
		TokenHealthProber: tokenSvc,
		UsageLogStore:     usageLogStore,
		APIKeyStore:       apiKeyStore,
		CacheService:      cacheSvc,
		ConfigStore:       configStore,
	})
	addr := fmt.Sprintf("%s:%d", cfg.App.Host, cfg.App.Port)

	httpServer := &http.Server{
		Addr:              addr,
		Handler:           srv.Router(),
		ReadTimeout:       30 * time.Second,
		ReadHeaderTimeout: time.Duration(cfg.App.ReadHeaderTimeout) * time.Second,
		WriteTimeout:      serverWriteTimeout,
		IdleTimeout:       120 * time.Second,
		MaxHeaderBytes:    cfg.App.MaxHeaderBytes,
	}

	// Start server in goroutine
	flow.SafeGo("http_server_listen", func() {
		logging.Info("server listening", "addr", addr)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logging.Error("server error", "error", err)
			os.Exit(1)
		}
	})

	// Start API Key daily usage reset ticker
	flow.SafeGo("apikey_daily_reset", func() {
		for {
			now := time.Now().UTC()
			nextMidnight := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, time.UTC)
			timer := time.NewTimer(nextMidnight.Sub(now))
			select {
			case <-rootCtx.Done():
				timer.Stop()
				return
			case <-timer.C:
			}
			if err := apiKeyStore.ResetDailyUsage(context.Background()); err != nil {
				logging.Error("failed to reset API key daily usage", "error", err)
			} else {
				logging.Info("API key daily usage reset complete")
			}
		}
	})

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logging.Info("shutting down server...")

	// Graceful shutdown: HTTP server first (stop accepting new requests)
	gracePeriod := time.Duration(cfg.App.ShutdownGracePeriodSec) * time.Second
	if gracePeriod <= 0 {
		gracePeriod = 30 * time.Second
	}
	ctx, cancel := context.WithTimeout(context.Background(), gracePeriod)
	defer cancel()

	if err := httpServer.Shutdown(ctx); err != nil {
		logging.Error("server shutdown error", "error", err)
	}
	rootCancel()

	scheduler.Stop()

	// Then flush remaining usage records
	usageBuffer.Stop()

	// Then flush dirty token state and stop persistence loop
	tokenPersister.Stop()
	if err := tokenSvc.FlushDirty(context.Background()); err != nil {
		logging.Error("failed to flush dirty tokens on shutdown", "error", err)
	}

	logging.Info("server stopped")
}

func validateStartupConfig(cfg *config.Config) error {
	if cfg == nil {
		return errors.New("configuration is nil")
	}
	switch cfg.App.AppKey {
	case "", defaultAdminAppKey:
		return fmt.Errorf("set app.app_key to a unique non-default value")
	default:
		return nil
	}
}
