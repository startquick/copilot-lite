package config

// DefaultConfig returns the default configuration.
func DefaultConfig() *Config {
	return &Config{
		App: AppConfig{
			AppKey:                 "",
			Stream:                 true,
			FilterTags:             []string{},
			Host:                   "0.0.0.0",
			Port:                   8080,
			LogJSON:                false,
			LogLevel:               "info",
			LogFilePath:            "logs/copilotpi.log",
			LogMaxSizeMB:           50,
			LogMaxBackups:          3,
			DBDriver:               "sqlite",
			DBPath:                 "data/copilotpi.db",
			DBDSN:                  "",
			RequestTimeout:         60,
			ReadHeaderTimeout:      10,
			MaxHeaderBytes:         1 << 20,  // 1MB
			BodyLimit:              1 << 20,  // 1MB
			ChatBodyLimit:          10 << 20, // 10MB
			AdminMaxFails:          10,
			AdminWindowSec:         300, // 5 minutes
			ShutdownGracePeriodSec: 30,
		},
		Copilot: CopilotConfig{
			WSURL:                    "wss://copilot.microsoft.com/c/api/chat",
			WSAPIVersion:             "2",
			NewConversationPerRequest: true,
			PingInterval:             25,
			ReconnectMax:             3,
			UserAgent:                "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/147.0.0.0 Safari/537.36 Edg/147.0.0.0",
		},
		Retry: RetryConfig{
			MaxTokens:               5,
			PerTokenRetries:         2,
			ResetSessionStatusCodes: []int{},
			CoolingStatusCodes:      []int{429},
			RetryBackoffBase:        0.5,
			RetryBackoffFactor:      2.0,
			RetryBackoffMax:         20.0,
			RetryBudget:             60.0,
		},
		Token: TokenConfig{
			FailThreshold:         5,
			UsageFlushIntervalSec: 30,
			CoolCheckIntervalSec:  30,
			BasicModels: []string{"copilot-free", "copilot-basic"},
			SuperModels: []string{
				"copilot-free", "copilot-basic", "copilot-premium",
			},
			PreferredPool:        "premium",
			BasicCoolDurationMin: 60,
			SuperCoolDurationMin: 30,
			DefaultChatQuota:     200,
			DefaultImageQuota:    0,
			DefaultVideoQuota:    0,
			QuotaRecoveryMode:   "auto",
			SelectionAlgorithm:  "high_quota_first",
			SuperQuotaThreshold: 0,
			// Health probe defaults
			HealthProbeIntervalSec: 300,
			HealthProbeConcurrency: 3,
			// Circuit breaker defaults
			CircuitBreakerFailThreshold:      3,
			CircuitBreakerHalfOpenTimeoutSec: 60,
		},
	}
}
