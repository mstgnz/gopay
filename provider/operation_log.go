package provider

import (
	"context"
	"errors"
	"time"

	"github.com/mstgnz/gopay/infra/logger"
)

// errorCodeLimit is the width of the error_code column in the provider log tables.
const errorCodeLimit = 50

// operationLog writes one provider log row per call for the checkout and marketplace services:
// the request first, then the response or the error on the same row.
type operationLog struct {
	logger PaymentLogger
}

func (o operationLog) start(ctx context.Context, tenantID int, providerName, method, endpoint string, request any, clientIP string) (int64, time.Time) {
	started := time.Now()
	if o.logger == nil {
		return 0, started
	}
	logID, err := o.logger.LogRequest(ctx, tenantID, providerName, method, endpoint, request, "", clientIP)
	if err != nil {
		logger.Warn("Failed to log provider request", logger.LogContext{
			Provider: providerName,
			Fields:   map[string]any{"endpoint": endpoint, "error": err.Error()},
		})
	}
	return logID, started
}

func (o operationLog) finish(ctx context.Context, providerName string, logID int64, started time.Time, response any, opErr error) {
	if o.logger == nil || logID <= 0 {
		return
	}
	ms := time.Since(started).Milliseconds()

	var err error
	if opErr != nil {
		err = o.logger.LogError(ctx, logID, logErrorCode(opErr), opErr.Error(), ms)
	} else {
		err = o.logger.LogResponse(ctx, logID, response, ms)
	}
	if err != nil {
		logger.Warn("Failed to log provider response", logger.LogContext{
			Provider: providerName,
			Fields:   map[string]any{"log_id": logID, "error": err.Error()},
		})
	}
}

// logErrorCode names the failure class on the log row, so a refusal and an outage are told apart
// by a WHERE on error_code.
func logErrorCode(err error) string {
	var refusal *ProviderRefusal
	code := "PROVIDER_ERROR"
	switch {
	case errors.As(err, &refusal):
		code = "REFUSED_" + refusal.Code
	case errors.Is(err, ErrProviderUnavailable):
		code = "PROVIDER_UNAVAILABLE"
	case errors.Is(err, ErrInvalidRequest):
		code = "INVALID_REQUEST"
	}
	if len(code) > errorCodeLimit {
		code = code[:errorCodeLimit]
	}
	return code
}
