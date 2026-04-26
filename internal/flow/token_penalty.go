package flow

import (
	"context"
	"errors"

	"github.com/crmmc/copilotpi/internal/copilot"
)

const (
	httpStatusServerErrorMin = 500
	httpStatusServerErrorMax = 599
)

func isTransportError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	if errors.Is(err, copilot.ErrDisconnected) {
		return true
	}
	statusCode, ok := extractStatusCode(err)
	if !ok {
		return false
	}
	return statusCode >= httpStatusServerErrorMin && statusCode <= httpStatusServerErrorMax
}
