package token

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/crmmc/copilotpi/internal/config"
	"github.com/crmmc/copilotpi/internal/copilot"
	"github.com/crmmc/copilotpi/internal/store"
)

// HealthProber runs periodic background health probes against active tokens.
// It calls SyncQuota on each active token concurrently (bounded by a semaphore),
// transitioning tokens to expired state before a real request fails.
type HealthProber struct {
	manager *TokenManager
	cfg     *config.TokenConfig
	baseURL string
	stopCh  chan struct{}
	once    sync.Once
}

// NewHealthProber creates a HealthProber attached to the given manager.
// Call Start(ctx) to begin background probing.
func NewHealthProber(manager *TokenManager, cfg *config.TokenConfig, baseURL string) *HealthProber {
	return &HealthProber{
		manager: manager,
		cfg:     cfg,
		baseURL: baseURL,
		stopCh:  make(chan struct{}),
	}
}

// Start launches the background probe goroutine and returns immediately.
func (p *HealthProber) Start(ctx context.Context) {
	safeGo("token_health_prober", func() {
		p.run(ctx)
	})
}

// Stop signals the prober to exit. Idempotent.
func (p *HealthProber) Stop() {
	p.once.Do(func() { close(p.stopCh) })
}

// ProbeToken performs a synchronous single-token probe for the health endpoint.
// Returns status, chatQuota, and any probe error encountered.
// Returns ErrTokenNotFound if the ID is not in the manager.
func (p *HealthProber) ProbeToken(ctx context.Context, id uint) (status string, chatQuota int, probeErr error) {
	token := p.manager.GetToken(id)
	if token == nil {
		return "", 0, ErrTokenNotFound
	}

	probeErr = p.probeOne(ctx, token)

	// Re-read from manager after probe (SyncQuota mutates token in-place under lock).
	if t := p.manager.GetToken(id); t != nil {
		status = t.Status
		chatQuota = t.ChatQuota
	}
	return status, chatQuota, probeErr
}

func (p *HealthProber) run(ctx context.Context) {
	interval := p.probeInterval()
	timer := time.NewTimer(interval)
	defer timer.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-p.stopCh:
			return
		case <-timer.C:
			p.runCycle(ctx)
			timer.Reset(p.probeInterval())
		}
	}
}

func (p *HealthProber) runCycle(ctx context.Context) {
	// Collect snapshot of active tokens under read lock.
	p.manager.mu.RLock()
	activeTokens := make([]*store.Token, 0, len(p.manager.tokens))
	for _, t := range p.manager.tokens {
		if t.Status == string(StatusActive) {
			activeTokens = append(activeTokens, t)
		}
	}
	p.manager.mu.RUnlock()

	if len(activeTokens) == 0 {
		return
	}

	slog.Debug("token_probe: starting probe cycle", "active_tokens", len(activeTokens))

	concurrency := p.probeConcurrency()
	sem := make(chan struct{}, concurrency)
	var wg sync.WaitGroup

	for _, tok := range activeTokens {
		// Check cancellation before launching each goroutine.
		select {
		case <-ctx.Done():
			break
		case <-p.stopCh:
			break
		default:
		}

		wg.Add(1)
		sem <- struct{}{}
		go func(t *store.Token) {
			defer wg.Done()
			defer func() { <-sem }()
			_ = p.probeOne(ctx, t)
		}(tok)
	}
	wg.Wait()
	slog.Debug("token_probe: probe cycle complete", "probed", len(activeTokens))
}

// probeOne calls SyncQuota for a single token and handles the result.
// For Copilot, SyncQuota validates the cookie bundle is still usable.
func (p *HealthProber) probeOne(ctx context.Context, token *store.Token) error {
	err := p.manager.SyncQuota(ctx, token, p.baseURL)
	if err == nil {
		return nil
	}

	if copilot.IsAuthError(err) {
		slog.Warn("token_probe: auth error, marking expired",
			"token_id", token.ID, "error", err)
		p.manager.MarkExpired(token.ID, "probe:auth_error")
		return err
	}

	// Transient network or HTTP error — log only, do not change token state.
	slog.Warn("token_probe: transient error, no state change",
		"token_id", token.ID, "error", err)
	return err
}

func (p *HealthProber) probeInterval() time.Duration {
	if p.cfg != nil && p.cfg.HealthProbeIntervalSec > 0 {
		return time.Duration(p.cfg.HealthProbeIntervalSec) * time.Second
	}
	return 300 * time.Second
}

func (p *HealthProber) probeConcurrency() int {
	if p.cfg != nil && p.cfg.HealthProbeConcurrency > 0 {
		return p.cfg.HealthProbeConcurrency
	}
	return 3
}
