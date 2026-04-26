package flow

import (
	"context"
	"errors"
	"testing"

	"github.com/crmmc/copilotpi/internal/copilot"
)

func TestIsTransportError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"disconnected error", copilot.ErrDisconnected, true},
		{"deadline exceeded", context.DeadlineExceeded, true},
		{"500 status", errors.New("unexpected status 500: boom"), true},
		{"503 status", errors.New("503 Service Unavailable"), true},
		{"rate limited", copilot.ErrRateLimited, false},
		{"forbidden", copilot.ErrForbidden, false},
		{"invalid token", copilot.ErrInvalidToken, false},
		{"nil error", nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isTransportError(tt.err); got != tt.want {
				t.Errorf("isTransportError(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}
