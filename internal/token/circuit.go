package token

import (
"sync"
"time"
)

// cbState represents the state of a circuit breaker.
type cbState int

const (
cbClosed   cbState = iota // Normal operation — requests flow through.
cbOpen                    // Failure threshold reached — requests are blocked.
cbHalfOpen                // Cooldown elapsed — one probe request is allowed.
)

// CircuitBreaker is a per-token in-memory circuit breaker.
// It is orthogonal to the token Status field and is never persisted.
// A process restart resets all circuit breakers to cbClosed.
type CircuitBreaker struct {
mu              sync.Mutex
state           cbState
failures        int
failThreshold   int
halfOpenAt      time.Time
halfOpenTimeout time.Duration
}

// newCircuitBreaker creates a new CircuitBreaker with the given config-derived thresholds.
func newCircuitBreaker(failThreshold, halfOpenTimeoutSec int) *CircuitBreaker {
if failThreshold <= 0 {
failThreshold = 3
}
timeout := time.Duration(halfOpenTimeoutSec) * time.Second
if timeout <= 0 {
timeout = 60 * time.Second
}
return &CircuitBreaker{
state:           cbClosed,
failThreshold:   failThreshold,
halfOpenTimeout: timeout,
}
}

// AllowRequest returns true if a request should be routed to this token.
// CLOSED and HALF-OPEN allow requests. OPEN blocks until halfOpenTimeout elapses.
func (cb *CircuitBreaker) AllowRequest() bool {
cb.mu.Lock()
defer cb.mu.Unlock()

switch cb.state {
case cbClosed:
return true
case cbOpen:
if time.Now().After(cb.halfOpenAt) {
cb.state = cbHalfOpen
return true // allow one probe through
}
return false
case cbHalfOpen:
return true
default:
return true
}
}

// RecordFailure records a failure outcome.
// CLOSED: increments failures counter; transitions to OPEN when threshold is reached.
// HALF-OPEN: transitions back to OPEN (probe failed), resetting the timer.
func (cb *CircuitBreaker) RecordFailure() {
cb.mu.Lock()
defer cb.mu.Unlock()

switch cb.state {
case cbClosed:
cb.failures++
if cb.failures >= cb.failThreshold {
cb.state = cbOpen
cb.halfOpenAt = time.Now().Add(cb.halfOpenTimeout)
}
case cbHalfOpen:
// Probe failed  re-open circuit and reset timer
cb.state = cbOpen
cb.halfOpenAt = time.Now().Add(cb.halfOpenTimeout)
case cbOpen:
// Already open  nothing to do
}
}

// RecordSuccess records a successful outcome.
// Transitions to CLOSED from any state and resets the failure counter.
func (cb *CircuitBreaker) RecordSuccess() {
cb.mu.Lock()
defer cb.mu.Unlock()

cb.state = cbClosed
cb.failures = 0
}

// State returns the current state as a string (for logging/debugging).
func (cb *CircuitBreaker) State() string {
cb.mu.Lock()
defer cb.mu.Unlock()
switch cb.state {
case cbClosed:
return "closed"
case cbOpen:
return "open"
case cbHalfOpen:
return "half-open"
default:
return "unknown"
}
}
