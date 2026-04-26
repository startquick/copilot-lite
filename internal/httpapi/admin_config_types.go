package httpapi

// ConfigResponse mirrors config.Config but with masked secrets for API responses.
type ConfigResponse struct {
	App   AppConfigResponse   `json:"app"`
	Retry RetryConfigResponse `json:"retry"`
	Token TokenConfigResponse `json:"token"`
}

// AppConfigResponse is the API response for app config (secrets masked).
type AppConfigResponse struct {
	AppKey            string   `json:"app_key"`
	Stream            bool     `json:"stream"`
	FilterTags        []string `json:"filter_tags"`
	Host              string   `json:"host"`
	Port              int      `json:"port"`
	LogJSON           bool     `json:"log_json"`
	LogLevel          string   `json:"log_level"`
	DBDriver          string   `json:"db_driver"`
	DBPath            string   `json:"db_path"`
	DBDSN             string   `json:"db_dsn"`
	RequestTimeout    int      `json:"request_timeout"`
	ReadHeaderTimeout int      `json:"read_header_timeout"`
	MaxHeaderBytes    int      `json:"max_header_bytes"`
	BodyLimit         int64    `json:"body_limit"`
	ChatBodyLimit     int64    `json:"chat_body_limit"`
	AdminMaxFails     int      `json:"admin_max_fails"`
	AdminWindowSec    int      `json:"admin_window_sec"`
}

// RetryConfigResponse is the API response for retry config.
type RetryConfigResponse struct {
	MaxTokens               int     `json:"max_tokens"`
	PerTokenRetries         int     `json:"per_token_retries"`
	ResetSessionStatusCodes []int   `json:"reset_session_status_codes"`
	CoolingStatusCodes      []int   `json:"cooling_status_codes"`
	RetryBackoffBase        float64 `json:"retry_backoff_base"`
	RetryBackoffFactor      float64 `json:"retry_backoff_factor"`
	RetryBackoffMax         float64 `json:"retry_backoff_max"`
	RetryBudget             float64 `json:"retry_budget"`
}

// TokenConfigResponse is the API response for token config.
type TokenConfigResponse struct {
	FailThreshold         int      `json:"fail_threshold"`
	UsageFlushIntervalSec int      `json:"usage_flush_interval_sec"`
	CoolCheckIntervalSec  int      `json:"cool_check_interval_sec"`
	BasicModels           []string `json:"basic_models"`
	SuperModels           []string `json:"super_models"`
	PreferredPool         string   `json:"preferred_pool"`
	BasicCoolDurationMin  int      `json:"basic_cool_duration_min"`
	SuperCoolDurationMin  int      `json:"super_cool_duration_min"`
	DefaultChatQuota      int      `json:"default_chat_quota"`
	DefaultImageQuota     int      `json:"default_image_quota"`
	DefaultVideoQuota     int      `json:"default_video_quota"`
	QuotaRecoveryMode     string   `json:"quota_recovery_mode"`
	SelectionAlgorithm    string   `json:"selection_algorithm"`
}

// ConfigUpdateRequest represents the fields that can be hot-reloaded.
type ConfigUpdateRequest struct {
	App   *AppConfigUpdate   `json:"app,omitempty"`
	Retry *RetryConfigUpdate `json:"retry,omitempty"`
	Token *TokenConfigUpdate `json:"token,omitempty"`
}

// AppConfigUpdate contains hot-reloadable app config fields.
type AppConfigUpdate struct {
	AppKey            *string   `json:"app_key,omitempty"`
	RequestTimeout    *int      `json:"request_timeout,omitempty"`
	Stream            *bool     `json:"stream,omitempty"`
	FilterTags        *[]string `json:"filter_tags,omitempty"`
	ReadHeaderTimeout *int      `json:"read_header_timeout,omitempty"`
	MaxHeaderBytes    *int      `json:"max_header_bytes,omitempty"`
	BodyLimit         *int64    `json:"body_limit,omitempty"`
	ChatBodyLimit     *int64    `json:"chat_body_limit,omitempty"`
	AdminMaxFails     *int      `json:"admin_max_fails,omitempty"`
	AdminWindowSec    *int      `json:"admin_window_sec,omitempty"`
}

// RetryConfigUpdate contains hot-reloadable retry config fields.
type RetryConfigUpdate struct {
	MaxTokens               *int     `json:"max_tokens,omitempty"`
	PerTokenRetries         *int     `json:"per_token_retries,omitempty"`
	ResetSessionStatusCodes *[]int   `json:"reset_session_status_codes,omitempty"`
	CoolingStatusCodes      *[]int   `json:"cooling_status_codes,omitempty"`
	RetryBackoffBase        *float64 `json:"retry_backoff_base,omitempty"`
	RetryBackoffFactor      *float64 `json:"retry_backoff_factor,omitempty"`
	RetryBackoffMax         *float64 `json:"retry_backoff_max,omitempty"`
	RetryBudget             *float64 `json:"retry_budget,omitempty"`
}

// TokenConfigUpdate contains hot-reloadable token config fields.
type TokenConfigUpdate struct {
	FailThreshold         *int      `json:"fail_threshold,omitempty"`
	CoolCheckIntervalSec  *int      `json:"cool_check_interval_sec,omitempty"`
	UsageFlushIntervalSec *int      `json:"usage_flush_interval_sec,omitempty"`
	BasicModels           *[]string `json:"basic_models,omitempty"`
	SuperModels           *[]string `json:"super_models,omitempty"`
	PreferredPool         *string   `json:"preferred_pool,omitempty"`
	BasicCoolDurationMin  *int      `json:"basic_cool_duration_min,omitempty"`
	SuperCoolDurationMin  *int      `json:"super_cool_duration_min,omitempty"`
	DefaultChatQuota      *int      `json:"default_chat_quota,omitempty"`
	QuotaRecoveryMode     *string   `json:"quota_recovery_mode,omitempty"`
	SelectionAlgorithm    *string   `json:"selection_algorithm,omitempty"`
}
