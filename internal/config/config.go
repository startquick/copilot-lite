// Package config provides configuration loading and management.
package config

import (
	"log/slog"
	"os"
	"strconv"
	"strings"

	"github.com/BurntSushi/toml"
)

// Config is the root configuration structure.
type Config struct {
	App     AppConfig     `toml:"app"`
	Copilot CopilotConfig `toml:"copilot"`
	Retry   RetryConfig   `toml:"retry"`
	Token   TokenConfig   `toml:"token"`
}

// CopilotConfig contains Copilot WebSocket client settings.
type CopilotConfig struct {
	WSURL                     string `toml:"ws_url"`
	WSAPIVersion              string `toml:"ws_api_version"`
	AccessToken               string `toml:"access_token"` // legacy override (unused when OAuth is configured)
	NewConversationPerRequest bool   `toml:"new_conversation_per_request"`
	PingInterval              int    `toml:"ping_interval"` // seconds
	ReconnectMax              int    `toml:"reconnect_max"`
	UserAgent                 string `toml:"user_agent"`
	OAuthRedirectPort         int    `toml:"oauth_redirect_port"` // local port for OAuth2 callback listener
}

// AppConfig contains application settings.
type AppConfig struct {
	AppKey    string   `toml:"app_key"`
	Stream    bool     `toml:"stream"`
	FilterTags []string `toml:"filter_tags"`
	// Server settings
	Host          string `toml:"host"`
	Port          int    `toml:"port"`
	LogJSON       bool   `toml:"log_json"`
	LogLevel      string `toml:"log_level"`
	LogFilePath   string `toml:"log_file_path"`
	LogMaxSizeMB  int    `toml:"log_max_size_mb"`
	LogMaxBackups int    `toml:"log_max_backups"`
	// Database settings
	DBDriver       string `toml:"db_driver"`
	DBPath         string `toml:"db_path"`
	DBDSN          string `toml:"db_dsn"`
	RequestTimeout int    `toml:"request_timeout"` // default request timeout in seconds (non-LLM routes)
	// Security settings
	ReadHeaderTimeout int   `toml:"read_header_timeout"` // seconds, max time to read request headers
	MaxHeaderBytes    int   `toml:"max_header_bytes"`    // max size of request headers in bytes
	BodyLimit         int64 `toml:"body_limit"`          // default max request body size in bytes
	ChatBodyLimit     int64 `toml:"chat_body_limit"`     // max body size for chat completions in bytes
	AdminMaxFails     int   `toml:"admin_max_fails"`     // max auth failures before temporary IP lockout
	AdminWindowSec    int   `toml:"admin_window_sec"`    // time window in seconds for counting admin auth failures
	// Shutdown settings
	ShutdownGracePeriodSec int `toml:"shutdown_grace_period_sec"` // seconds to wait for in-flight requests on shutdown
}

// RetryConfig contains retry policy settings.
type RetryConfig struct {
	MaxTokens               int     `toml:"max_tokens"`
	PerTokenRetries         int     `toml:"per_token_retries"`
	ResetSessionStatusCodes []int   `toml:"reset_session_status_codes"`
	CoolingStatusCodes      []int   `toml:"cooling_status_codes"`
	RetryBackoffBase        float64 `toml:"retry_backoff_base"`
	RetryBackoffFactor      float64 `toml:"retry_backoff_factor"`
	RetryBackoffMax         float64 `toml:"retry_backoff_max"`
	RetryBudget             float64 `toml:"retry_budget"`
}

// TokenConfig contains token pool settings.
type TokenConfig struct {
	FailThreshold         int `toml:"fail_threshold"`
	UsageFlushIntervalSec int `toml:"usage_flush_interval_sec"`
	CoolCheckIntervalSec  int `toml:"cool_check_interval_sec"`
	// Model group fields
	BasicModels          []string `toml:"basic_models"`
	SuperModels          []string `toml:"super_models"`
	PreferredPool        string   `toml:"preferred_pool"`
	BasicCoolDurationMin int      `toml:"basic_cool_duration_min"`
	SuperCoolDurationMin int      `toml:"super_cool_duration_min"`
	DefaultChatQuota     int      `toml:"default_chat_quota"`
	DefaultImageQuota    int      `toml:"default_image_quota"`
	DefaultVideoQuota    int      `toml:"default_video_quota"`
	QuotaRecoveryMode    string   `toml:"quota_recovery_mode"`
	SelectionAlgorithm   string   `toml:"selection_algorithm" json:"selection_algorithm"`
	SuperQuotaThreshold  int      `toml:"super_quota_threshold"`
	// Health probe settings
	HealthProbeIntervalSec int `toml:"health_probe_interval_sec"` // seconds between full probe cycles
	HealthProbeConcurrency int `toml:"health_probe_concurrency"`  // max simultaneous probes
	// Circuit breaker settings
	CircuitBreakerFailThreshold      int `toml:"circuit_breaker_fail_threshold"`        // consecutive failures before opening
	CircuitBreakerHalfOpenTimeoutSec int `toml:"circuit_breaker_half_open_timeout_sec"` // seconds before half-open probe
}

// ApplyDBOverrides applies database config entries on top of file-based config.
// Priority: DB > config file > defaults.
func (c *Config) ApplyDBOverrides(kvs map[string]string) []string {
	var overridden []string

	for k, v := range kvs {
		matched := true
		switch k {
		case "app.app_key":
			c.App.AppKey = v
		case "app.stream":
			c.App.Stream = v == "true"
		case "app.filter_tags":
			if v != "" {
				c.App.FilterTags = strings.Split(v, ",")
			} else {
				c.App.FilterTags = []string{}
			}
		case "app.request_timeout":
			if n, err := strconv.Atoi(v); err == nil {
				c.App.RequestTimeout = n
			} else {
				slog.Warn("config: invalid int override ignored", "key", k, "value", v, "error", err)
			}
		case "app.read_header_timeout":
			if n, err := strconv.Atoi(v); err == nil {
				c.App.ReadHeaderTimeout = n
			} else {
				slog.Warn("config: invalid int override ignored", "key", k, "value", v, "error", err)
			}
		case "app.max_header_bytes":
			if n, err := strconv.Atoi(v); err == nil {
				c.App.MaxHeaderBytes = n
			} else {
				slog.Warn("config: invalid int override ignored", "key", k, "value", v, "error", err)
			}
		case "app.body_limit":
			if n, err := strconv.ParseInt(v, 10, 64); err == nil {
				c.App.BodyLimit = n
			} else {
				slog.Warn("config: invalid int64 override ignored", "key", k, "value", v, "error", err)
			}
		case "app.chat_body_limit":
			if n, err := strconv.ParseInt(v, 10, 64); err == nil {
				c.App.ChatBodyLimit = n
			} else {
				slog.Warn("config: invalid int64 override ignored", "key", k, "value", v, "error", err)
			}
		case "app.admin_max_fails":
			if n, err := strconv.Atoi(v); err == nil {
				c.App.AdminMaxFails = n
			} else {
				slog.Warn("config: invalid int override ignored", "key", k, "value", v, "error", err)
			}
		case "app.admin_window_sec":
			if n, err := strconv.Atoi(v); err == nil {
				c.App.AdminWindowSec = n
			} else {
				slog.Warn("config: invalid int override ignored", "key", k, "value", v, "error", err)
			}
		case "app.shutdown_grace_period_sec":
			if n, err := strconv.Atoi(v); err == nil {
				c.App.ShutdownGracePeriodSec = n
			} else {
				slog.Warn("config: invalid int override ignored", "key", k, "value", v, "error", err)
			}
		// Copilot overrides
		case "copilot.ws_url":
			c.Copilot.WSURL = v
		case "copilot.user_agent":
			if v != "" {
				c.Copilot.UserAgent = v
			}
		case "copilot.new_conversation_per_request":
			c.Copilot.NewConversationPerRequest = v == "true"
		case "copilot.ping_interval":
			if n, err := strconv.Atoi(v); err == nil {
				c.Copilot.PingInterval = n
			} else {
				slog.Warn("config: invalid int override ignored", "key", k, "value", v, "error", err)
			}
		// Retry overrides
		case "retry.max_tokens":
			if n, err := strconv.Atoi(v); err == nil {
				c.Retry.MaxTokens = n
			} else {
				slog.Warn("config: invalid int override ignored", "key", k, "value", v, "error", err)
			}
		case "retry.per_token_retries":
			if n, err := strconv.Atoi(v); err == nil {
				c.Retry.PerTokenRetries = n
			} else {
				slog.Warn("config: invalid int override ignored", "key", k, "value", v, "error", err)
			}
		case "retry.cooling_status_codes":
			if v != "" {
				parts := strings.Split(v, ",")
				codes := make([]int, 0, len(parts))
				for _, p := range parts {
					if n, err := strconv.Atoi(strings.TrimSpace(p)); err == nil {
						codes = append(codes, n)
					}
				}
				c.Retry.CoolingStatusCodes = codes
			} else {
				c.Retry.CoolingStatusCodes = []int{}
			}
		case "retry.retry_backoff_base":
			if f, err := strconv.ParseFloat(v, 64); err == nil {
				c.Retry.RetryBackoffBase = f
			} else {
				slog.Warn("config: invalid float override ignored", "key", k, "value", v, "error", err)
			}
		case "retry.retry_backoff_factor":
			if f, err := strconv.ParseFloat(v, 64); err == nil {
				c.Retry.RetryBackoffFactor = f
			} else {
				slog.Warn("config: invalid float override ignored", "key", k, "value", v, "error", err)
			}
		case "retry.retry_backoff_max":
			if f, err := strconv.ParseFloat(v, 64); err == nil {
				c.Retry.RetryBackoffMax = f
			} else {
				slog.Warn("config: invalid float override ignored", "key", k, "value", v, "error", err)
			}
		case "retry.retry_budget":
			if f, err := strconv.ParseFloat(v, 64); err == nil {
				c.Retry.RetryBudget = f
			} else {
				slog.Warn("config: invalid float override ignored", "key", k, "value", v, "error", err)
			}
		// Token overrides
		case "token.fail_threshold":
			if n, err := strconv.Atoi(v); err == nil {
				c.Token.FailThreshold = n
			} else {
				slog.Warn("config: invalid int override ignored", "key", k, "value", v, "error", err)
			}
		case "token.cool_check_interval_sec":
			if n, err := strconv.Atoi(v); err == nil {
				c.Token.CoolCheckIntervalSec = n
			} else {
				slog.Warn("config: invalid int override ignored", "key", k, "value", v, "error", err)
			}
		case "token.usage_flush_interval_sec":
			if n, err := strconv.Atoi(v); err == nil {
				c.Token.UsageFlushIntervalSec = n
			} else {
				slog.Warn("config: invalid int override ignored", "key", k, "value", v, "error", err)
			}
		case "token.basic_models":
			if v != "" {
				c.Token.BasicModels = splitTrimmed(v)
			}
		case "token.super_models":
			if v != "" {
				c.Token.SuperModels = splitTrimmed(v)
			}
		case "token.preferred_pool":
			c.Token.PreferredPool = v
		case "token.basic_cool_duration_min":
			if n, err := strconv.Atoi(v); err == nil {
				c.Token.BasicCoolDurationMin = n
			} else {
				slog.Warn("config: invalid int override ignored", "key", k, "value", v, "error", err)
			}
		case "token.super_cool_duration_min":
			if n, err := strconv.Atoi(v); err == nil {
				c.Token.SuperCoolDurationMin = n
			} else {
				slog.Warn("config: invalid int override ignored", "key", k, "value", v, "error", err)
			}
		case "token.default_chat_quota":
			if n, err := strconv.Atoi(v); err == nil {
				c.Token.DefaultChatQuota = n
			} else {
				slog.Warn("config: invalid int override ignored", "key", k, "value", v, "error", err)
			}
		case "token.quota_recovery_mode":
			c.Token.QuotaRecoveryMode = v
		case "token.selection_algorithm":
			if v != "" {
				c.Token.SelectionAlgorithm = v
			}
		case "token.health_probe_interval_sec":
			if n, err := strconv.Atoi(v); err == nil {
				c.Token.HealthProbeIntervalSec = n
			} else {
				slog.Warn("config: invalid int override ignored", "key", k, "value", v, "error", err)
			}
		case "token.health_probe_concurrency":
			if n, err := strconv.Atoi(v); err == nil {
				c.Token.HealthProbeConcurrency = n
			} else {
				slog.Warn("config: invalid int override ignored", "key", k, "value", v, "error", err)
			}
		case "token.circuit_breaker_fail_threshold":
			if n, err := strconv.Atoi(v); err == nil {
				c.Token.CircuitBreakerFailThreshold = n
			} else {
				slog.Warn("config: invalid int override ignored", "key", k, "value", v, "error", err)
			}
		case "token.circuit_breaker_half_open_timeout_sec":
			if n, err := strconv.Atoi(v); err == nil {
				c.Token.CircuitBreakerHalfOpenTimeoutSec = n
			} else {
				slog.Warn("config: invalid int override ignored", "key", k, "value", v, "error", err)
			}
		default:
			matched = false
		}
		if matched {
			overridden = append(overridden, k)
		}
	}
	return overridden
}

func splitTrimmed(v string) []string {
	parts := strings.Split(v, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			result = append(result, part)
		}
	}
	return result
}

// Load loads configuration from the given path.
// If the file does not exist, returns default configuration.
func Load(path string) (*Config, error) {
	cfg := DefaultConfig()

	if path == "" {
		return cfg, nil
	}

	if _, err := os.Stat(path); os.IsNotExist(err) {
		return cfg, nil
	}

	if _, err := toml.DecodeFile(path, cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}
