package token

import (
	"errors"
	"sync"
	"time"

	"github.com/crmmc/copilotpi/internal/config"
	"github.com/crmmc/copilotpi/internal/store"
)

var (
	// ErrNoTokenAvailable is returned when no token is available in the pool.
	ErrNoTokenAvailable = errors.New("no token available")
)

// TokenManager manages token pools and state transitions.
type TokenManager struct {
	cfg      *config.TokenConfig
	pools    map[string]*TokenPool
	tokens   map[uint]*store.Token  // all tokens by ID for quick lookup
	dirty    map[uint]struct{}      // tokens that need persistence
	circuits map[uint]*CircuitBreaker // per-token circuit breakers (in-memory only)
	mu       sync.RWMutex
}

// NewTokenManager creates a new token manager.
func NewTokenManager(cfg *config.TokenConfig) *TokenManager {
	return &TokenManager{
		cfg:      cfg,
		pools:    make(map[string]*TokenPool),
		tokens:   make(map[uint]*store.Token),
		dirty:    make(map[uint]struct{}),
		circuits: make(map[uint]*CircuitBreaker),
	}
}

// AddToken adds a token to the appropriate pool.
func (m *TokenManager) AddToken(token *store.Token) {
	m.mu.Lock()
	defer m.mu.Unlock()

	normalizeTokenQuotaBaselines(token)

	pool, ok := m.pools[token.Pool]
	if !ok {
		pool = NewTokenPool(token.Pool)
		m.pools[token.Pool] = pool
	}
	pool.Add(token)
	m.tokens[token.ID] = token

	// Initialize a fresh circuit breaker for this token (in-memory only, never persisted).
	failThreshold := 3
	halfOpenTimeout := 60
	if m.cfg != nil {
		if m.cfg.CircuitBreakerFailThreshold > 0 {
			failThreshold = m.cfg.CircuitBreakerFailThreshold
		}
		if m.cfg.CircuitBreakerHalfOpenTimeoutSec > 0 {
			halfOpenTimeout = m.cfg.CircuitBreakerHalfOpenTimeoutSec
		}
	}
	m.circuits[token.ID] = newCircuitBreaker(failThreshold, halfOpenTimeout)
}

// RemoveToken removes a token from its pool.
func (m *TokenManager) RemoveToken(id uint) {
	m.mu.Lock()
	defer m.mu.Unlock()

	token, ok := m.tokens[id]
	if !ok {
		return
	}
	if pool, ok := m.pools[token.Pool]; ok {
		pool.Remove(id)
	}
	delete(m.tokens, id)
	delete(m.dirty, id)
	delete(m.circuits, id)
}

// GetToken returns a token by ID.
func (m *TokenManager) GetToken(id uint) *store.Token {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.tokens[id]
}

// Pick selects a token from the specified pool using the configured selection algorithm.
// Returns ErrNoTokenAvailable when no active tokens are available (no cooling fallback).
func (m *TokenManager) Pick(poolName string, cat QuotaCategory) (*store.Token, error) {
	return m.pick(poolName, cat, nil)
}

// PickExcluding selects a token while skipping excluded token IDs.
func (m *TokenManager) PickExcluding(poolName string, cat QuotaCategory, exclude map[uint]struct{}) (*store.Token, error) {
	return m.pick(poolName, cat, exclude)
}

func (m *TokenManager) pick(poolName string, cat QuotaCategory, exclude map[uint]struct{}) (*store.Token, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	pool, ok := m.pools[poolName]
	if !ok {
		return nil, ErrNoTokenAvailable
	}

	algo := m.cfg.SelectionAlgorithm
	if algo == "" {
		algo = AlgoHighQuotaFirst
	}

	// Build a combined exclusion set that includes tokens with open circuits.
	// We avoid mutating the caller's map by lazily copying it on first modification.
	combinedExclude := exclude
	copied := false
	for id, cb := range m.circuits {
		if !cb.AllowRequest() {
			if !copied {
				// Shallow-copy the caller's map so we don't mutate it.
				newExclude := make(map[uint]struct{}, len(exclude)+len(m.circuits))
				for k, v := range exclude {
					newExclude[k] = v
				}
				combinedExclude = newExclude
				copied = true
			}
			combinedExclude[id] = struct{}{}
		}
	}

	if token := pool.SelectExcluding(algo, cat, combinedExclude); token != nil {
		return token, nil
	}

	return nil, ErrNoTokenAvailable
}

// MarkCircuitFailure records a failure on the circuit breaker for the given token.
// Call this after a retryable upstream error. Safe to call with an unknown ID.
func (m *TokenManager) MarkCircuitFailure(id uint) {
	m.mu.RLock()
	cb, ok := m.circuits[id]
	m.mu.RUnlock()
	if ok {
		cb.RecordFailure()
	}
}

// MarkCircuitSuccess records a successful request on the circuit breaker for the given token.
// Call this after a successful upstream response. Safe to call with an unknown ID.
func (m *TokenManager) MarkCircuitSuccess(id uint) {
	m.mu.RLock()
	cb, ok := m.circuits[id]
	m.mu.RUnlock()
	if ok {
		cb.RecordSuccess()
	}
}

// MarkCooling transitions a token to cooling state.
func (m *TokenManager) MarkCooling(id uint, duration time.Duration, reason string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	token, ok := m.tokens[id]
	if !ok {
		return
	}

	coolUntil := time.Now().Add(duration)
	token.Status = string(StatusCooling)
	token.StatusReason = reason
	token.CoolUntil = &coolUntil
	m.dirty[id] = struct{}{}
}

// MarkSuccess transitions a token back to active state.
func (m *TokenManager) MarkSuccess(id uint) {
	m.mu.Lock()
	defer m.mu.Unlock()

	token, ok := m.tokens[id]
	if !ok {
		return
	}

	token.Status = string(StatusActive)
	token.StatusReason = ""
	token.CoolUntil = nil
	token.FailCount = 0
	m.dirty[id] = struct{}{}
}

// MarkFailed increments fail count and disables if threshold reached.
// When FailThreshold <= 0, the token is never auto-disabled (unlimited).
func (m *TokenManager) MarkFailed(id uint, reason string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	token, ok := m.tokens[id]
	if !ok {
		return
	}

	token.FailCount++
	if m.cfg.FailThreshold > 0 && token.FailCount >= m.cfg.FailThreshold {
		token.Status = string(StatusDisabled)
		token.StatusReason = reason
	}
	m.dirty[id] = struct{}{}
}

// MarkDisabled transitions a token to disabled state (manual user action).
func (m *TokenManager) MarkDisabled(id uint, reason string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	token, ok := m.tokens[id]
	if !ok {
		return
	}

	token.Status = string(StatusDisabled)
	token.StatusReason = reason
	m.dirty[id] = struct{}{}
}

// MarkExpired transitions a token to expired state (auto-detected invalid, e.g. 401).
func (m *TokenManager) MarkExpired(id uint, reason string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	token, ok := m.tokens[id]
	if !ok {
		return
	}

	token.Status = string(StatusExpired)
	token.StatusReason = reason
	m.dirty[id] = struct{}{}
}

// TokenSnapshot holds a copy of token data for safe persistence.
type TokenSnapshot struct {
	ID                uint
	Status            string
	StatusReason      string
	ChatQuota         int
	InitialChatQuota  int
	ImageQuota        int
	InitialImageQuota int
	VideoQuota        int
	InitialVideoQuota int
	FailCount         int
	CoolUntil         *time.Time
	LastUsed          *time.Time
}

// GetDirtyTokens returns snapshots of tokens that have been modified.
// Returns copies to avoid race conditions with concurrent modifications.
// Call ClearDirty after successful persistence to avoid data loss on DB failure.
func (m *TokenManager) GetDirtyTokens() []TokenSnapshot {
	m.mu.Lock()
	defer m.mu.Unlock()

	result := make([]TokenSnapshot, 0, len(m.dirty))
	for id := range m.dirty {
		if token, ok := m.tokens[id]; ok {
			snapshot := TokenSnapshot{
				ID:                token.ID,
				Status:            token.Status,
				StatusReason:      token.StatusReason,
				ChatQuota:         token.ChatQuota,
				InitialChatQuota:  token.InitialChatQuota,
				ImageQuota:        token.ImageQuota,
				InitialImageQuota: token.InitialImageQuota,
				VideoQuota:        token.VideoQuota,
				InitialVideoQuota: token.InitialVideoQuota,
				FailCount:         token.FailCount,
			}
			if token.CoolUntil != nil {
				t := *token.CoolUntil
				snapshot.CoolUntil = &t
			}
			if token.LastUsed != nil {
				t := *token.LastUsed
				snapshot.LastUsed = &t
			}
			result = append(result, snapshot)
		}
	}
	return result
}

// ClearDirty removes the given token IDs from the dirty set.
// Call this only after successful persistence.
func (m *TokenManager) ClearDirty(ids []uint) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, id := range ids {
		delete(m.dirty, id)
	}
}

// GetPool returns a pool by name.
func (m *TokenManager) GetPool(name string) *TokenPool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.pools[name]
}

// GetTokenPool returns the pool name for a token by ID.
func (m *TokenManager) GetTokenPool(id uint) string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if token, ok := m.tokens[id]; ok {
		return token.Pool
	}
	return ""
}

// GetCoolingTokens returns all tokens in cooling state.
func (m *TokenManager) GetCoolingTokens() []TokenSnapshot {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]TokenSnapshot, 0)
	for _, token := range m.tokens {
		if Status(token.Status) == StatusCooling {
			snapshot := TokenSnapshot{
				ID:                token.ID,
				Status:            token.Status,
				StatusReason:      token.StatusReason,
				ChatQuota:         token.ChatQuota,
				InitialChatQuota:  token.InitialChatQuota,
				ImageQuota:        token.ImageQuota,
				InitialImageQuota: token.InitialImageQuota,
				VideoQuota:        token.VideoQuota,
				InitialVideoQuota: token.InitialVideoQuota,
				FailCount:         token.FailCount,
			}
			if token.CoolUntil != nil {
				t := *token.CoolUntil
				snapshot.CoolUntil = &t
			}
			if token.LastUsed != nil {
				t := *token.LastUsed
				snapshot.LastUsed = &t
			}
			result = append(result, snapshot)
		}
	}
	return result
}

// RestoreToken restores a single token to the given quotas and marks it active.
// Used by the auto-mode recovery scheduler when a token's cooling period expires.
func (m *TokenManager) RestoreToken(id uint, chatQuota, imageQuota, videoQuota int) {
	m.mu.Lock()
	defer m.mu.Unlock()

	t, ok := m.tokens[id]
	if !ok {
		return
	}

	t.ChatQuota = chatQuota
	t.InitialChatQuota = chatQuota
	t.ImageQuota = imageQuota
	t.InitialImageQuota = imageQuota
	t.VideoQuota = videoQuota
	t.InitialVideoQuota = videoQuota
	t.Status = string(StatusActive)
	t.StatusReason = ""
	t.CoolUntil = nil
	m.dirty[id] = struct{}{}
}

func normalizeTokenQuotaBaselines(token *store.Token) {
	if token == nil {
		return
	}
	if token.InitialChatQuota < token.ChatQuota {
		token.InitialChatQuota = token.ChatQuota
	}
	if token.InitialImageQuota < token.ImageQuota {
		token.InitialImageQuota = token.ImageQuota
	}
	if token.InitialVideoQuota < token.VideoQuota {
		token.InitialVideoQuota = token.VideoQuota
	}
}
