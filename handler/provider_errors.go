package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/mstgnz/gopay/infra/logger"
	"github.com/mstgnz/gopay/infra/response"
	"github.com/mstgnz/gopay/provider"
)

// maxCapabilityBody bounds the checkout and marketplace request bodies.
const maxCapabilityBody = 64 << 10

// Failure kinds the checkout and marketplace routes return in data.kind. Only a 200 or a 422
// "refused" is an answer from the provider; "unavailable" and "internal" mean the outcome is unknown.
const (
	kindRefused     = "refused"
	kindUnavailable = "unavailable"
	kindInternal    = "internal"
)

// decodeStrict reads one JSON object and refuses unknown fields, so a misspelt or smuggled field
// is an error instead of a silently ignored value. allowEmpty accepts an empty body.
func decodeStrict(w http.ResponseWriter, r *http.Request, dst any, allowEmpty bool) error {
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxCapabilityBody))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		if allowEmpty && errors.Is(err, io.EOF) {
			return nil
		}
		return err
	}
	if err := dec.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return errors.New("body must hold a single JSON object")
	}
	return nil
}

// writeProviderError maps a checkout or marketplace service error to the HTTP contract in
// provider/iyzico/README.md.
func writeProviderError(w http.ResponseWriter, providerName, operation string, err error) {
	var refusal *provider.ProviderRefusal
	switch {
	case errors.Is(err, provider.ErrInvalidRequest):
		response.Error(w, http.StatusBadRequest, "Invalid request", err)
	case errors.Is(err, provider.ErrCheckoutUnsupported), errors.Is(err, provider.ErrMarketplaceUnsupported):
		response.Error(w, http.StatusBadRequest, "Provider does not support this operation", err)
	case errors.As(err, &refusal):
		response.Return(w, http.StatusUnprocessableEntity, false, "Provider refused the request", map[string]any{
			"kind": kindRefused, "providerErrorCode": refusal.Code,
			"providerErrorMessage": refusal.Message, "retryable": refusal.Retryable,
		})
	case errors.Is(err, provider.ErrProviderUnavailable), errors.Is(err, context.DeadlineExceeded):
		logger.Warn(fmt.Sprintf("%s: provider unavailable", operation), logger.LogContext{
			Provider: providerName, Fields: map[string]any{"error": err.Error()},
		})
		response.Return(w, http.StatusBadGateway, false, "Provider unavailable; the outcome is unknown",
			map[string]any{"kind": kindUnavailable})
	default:
		logger.Error(fmt.Sprintf("%s failed", operation), err, logger.LogContext{Provider: providerName})
		response.Return(w, http.StatusInternalServerError, false, "Internal error; the outcome is unknown",
			map[string]any{"kind": kindInternal})
	}
}
