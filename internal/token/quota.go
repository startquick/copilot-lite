package token

import (
	"context"
	"errors"
	"time"

	"github.com/crmmc/copilotpi/internal/config"
	"github.com/crmmc/copilotpi/internal/store"
)

var (
	// ErrNoQuota is returned when token has no remaining quota.
	ErrNoQuota = errors.New("no quota remaining")
	// ErrTokenNotFound is returned when token ID does not exist.
	ErrTokenNotFound = errors.New("token not found")
)

// ImportProfile captures the upstream-derived plan classification for a token.
type ImportProfile struct {
	Pool              string
	Priority          int
	ChatQuota         int
	InitialChatQuota  int
	ImageQuota        int
	InitialImageQuota int
	VideoQuota        int
	InitialVideoQuota int
}

const minCoolingDuration = 5 * time.Minute

// defaultSuperQuotaThreshold is the fallback classification threshold when config is zero.
const defaultSuperQuotaThreshold = 100

// Consume deducts quota from the token for the given category.
// cost allows variable deduction for different model types.
// Returns remaining quota after deduction.
func (m *TokenManager) Consume(tokenID uint, cat QuotaCategory, cost int) (remaining int, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	token, ok := m.tokens[tokenID]
	if !ok {
		return 0, ErrTokenNotFound
	}

	cur := GetQuota(token, cat)
	if cur <= 0 {
		return 0, ErrNoQuota
	}

	if cost <= 0 {
		cost = 1
	}
	newVal := cur - cost
	if newVal < 0 {
		newVal = 0
	}
	SetQuota(token, cat, newVal)

	now := time.Now()
	token.LastUsed = &now

	// Only enter cooling if ALL categories are exhausted
	if token.ChatQuota <= 0 && token.ImageQuota <= 0 && token.VideoQuota <= 0 {
		coolUntil := now.Add(m.coolingDurationForToken(token))
		token.Status = string(StatusCooling)
		token.CoolUntil = &coolUntil
	}
	m.dirty[tokenID] = struct{}{}

	return newVal, nil
}

// SyncQuota is a no-op for Copilot — there is no upstream quota API.
// Token quota is managed entirely by local consumption tracking and auto-restore.
func (m *TokenManager) SyncQuota(_ context.Context, _ *store.Token, _ string) error {
	return nil
}

// DetectImportProfile returns a default premium profile for a Copilot cookie bundle.
// Since Copilot tokens come from M365 Premium accounts, all tokens are classified
// as premium (PoolSuper) with priority 10.
func DetectImportProfile(_ context.Context, _ string, _ string, cfg *config.TokenConfig) (*ImportProfile, error) {
	chatQuota := 200
	imageQuota := 20
	videoQuota := 10
	if cfg != nil {
		if cfg.DefaultChatQuota > 0 {
			chatQuota = cfg.DefaultChatQuota
		}
		if cfg.DefaultImageQuota > 0 {
			imageQuota = cfg.DefaultImageQuota
		}
		if cfg.DefaultVideoQuota > 0 {
			videoQuota = cfg.DefaultVideoQuota
		}
	}

	return &ImportProfile{
		Pool:              PoolSuper,
		Priority:          10,
		ChatQuota:         chatQuota,
		InitialChatQuota:  chatQuota,
		ImageQuota:        imageQuota,
		InitialImageQuota: imageQuota,
		VideoQuota:        videoQuota,
		InitialVideoQuota: videoQuota,
	}, nil
}

func classifyQuotaCapacity(chatQuota, threshold int) (pool string, priority int) {
	if threshold <= 0 {
		threshold = defaultSuperQuotaThreshold
	}
	if chatQuota >= threshold {
		return PoolSuper, 10
	}
	return PoolBasic, 0
}

// superQuotaThreshold returns the effective classification threshold from config.
func (m *TokenManager) superQuotaThreshold() int {
	return effectiveSuperQuotaThreshold(m.cfg)
}

// effectiveSuperQuotaThreshold resolves the threshold from a TokenConfig pointer (nil-safe).
func effectiveSuperQuotaThreshold(cfg *config.TokenConfig) int {
	if cfg != nil && cfg.SuperQuotaThreshold > 0 {
		return cfg.SuperQuotaThreshold
	}
	return defaultSuperQuotaThreshold
}

func (m *TokenManager) coolingDurationForToken(token *store.Token) time.Duration {
	if token == nil || m.cfg == nil {
		return minCoolingDuration
	}
	var duration time.Duration
	switch token.Pool {
	case PoolSuper:
		duration = time.Duration(m.cfg.SuperCoolDurationMin) * time.Minute
	default:
		duration = time.Duration(m.cfg.BasicCoolDurationMin) * time.Minute
	}
	if duration < minCoolingDuration {
		return minCoolingDuration
	}
	return duration
}
