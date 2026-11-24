package provider

import (
	"context"
	"errors"
	"time"

	"github.com/mstgnz/gopay/infra/logger"
)

// ErrPaymentNotRecorded is returned when the payments row cannot be written; the payment is then
// refused before any provider call, so nothing is charged without a record.
var ErrPaymentNotRecorded = errors.New("payment could not be recorded")

// paymentRecorder writes the payments rows for the services. Opening a row is fail-closed; every
// later write is best-effort, because by then money may have moved and the answer must still reach
// the consumer.
type paymentRecorder struct {
	store PaymentStore
}

// recordTimeout bounds a write made after the provider answered.
const recordTimeout = 5 * time.Second

// afterProvider detaches ctx from the request for writes made after the provider answered: the
// consumer may disconnect or the handler deadline pass once money moved, and the record must land.
func afterProvider(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.WithoutCancel(ctx), recordTimeout)
}

// open stores the attempt before the provider is called. The consumer gets only
// ErrPaymentNotRecorded; the database detail goes to the system log.
func (r paymentRecorder) open(ctx context.Context, rec NewPaymentRecord) (int64, error) {
	if r.store == nil {
		logger.Error("Payment refused: no payment store", ErrPaymentNotRecorded, logger.LogContext{Provider: rec.Provider})
		return 0, ErrPaymentNotRecorded
	}
	id, err := r.store.InsertPayment(ctx, rec)
	if err != nil {
		logger.Error("Payment refused: payments row not written", err, logger.LogContext{
			Provider: rec.Provider,
			Fields:   map[string]any{"tenant_id": rec.TenantID, "channel": rec.Channel},
		})
		return 0, ErrPaymentNotRecorded
	}
	return id, nil
}

// record writes an event for a row; a failure is logged, never returned.
func (r paymentRecorder) record(ctx context.Context, providerName string, rowID int64, ev PaymentEvent) {
	if r.store == nil || rowID <= 0 {
		return
	}
	ctx, cancel := afterProvider(ctx)
	defer cancel()
	if err := r.store.RecordEvent(ctx, rowID, ev); err != nil {
		logger.Error("Failed to record payment event", err, logger.LogContext{
			Provider: providerName,
			Fields:   map[string]any{"payment_row": rowID, "kind": ev.Kind},
		})
	}
}

// find returns the row for a consumer-held payment id, or 0 when there is none (payments started
// before the table existed) or the lookup failed (logged).
func (r paymentRecorder) find(ctx context.Context, key PaymentKey) int64 {
	if r.store == nil || key.ProviderPaymentID == "" {
		return 0
	}
	ctx, cancel := afterProvider(ctx)
	defer cancel()
	id, err := r.store.FindPayment(ctx, key)
	if err != nil {
		if !errors.Is(err, ErrPaymentRowNotFound) {
			logger.Error("Failed to find payment row", err, logger.LogContext{
				Provider: key.Provider,
				Fields:   map[string]any{"payment_id": key.ProviderPaymentID},
			})
		}
		return 0
	}
	return id
}

// findForCallback returns the row a 3D callback belongs to: by the start's log id carried in the
// state, else by the payment id in the state.
func (r paymentRecorder) findForCallback(ctx context.Context, providerName string, state *CallbackState, startLogID int64) int64 {
	if r.store == nil {
		return 0
	}
	if startLogID > 0 {
		lookupCtx, cancel := afterProvider(ctx)
		id, err := r.store.FindPaymentByStartLog(lookupCtx, state.TenantID, providerName, startLogID)
		cancel()
		if err == nil {
			return id
		}
		if !errors.Is(err, ErrPaymentRowNotFound) {
			logger.Error("Failed to find payment row by start log", err, logger.LogContext{
				Provider: providerName,
				Fields:   map[string]any{"start_log_id": startLogID},
			})
		}
	}
	return r.find(ctx, PaymentKey{
		TenantID: state.TenantID, Provider: providerName, Environment: state.Environment, ProviderPaymentID: state.PaymentID,
	})
}

// recordCallback records a 3D completion on the row its start opened.
func (s *PaymentService) recordCallback(ctx context.Context, providerName string, state *CallbackState, startLogID, logID int64, resp *PaymentResponse, err error) {
	rowID := s.recorder.findForCallback(ctx, providerName, state, startLogID)
	if rowID == 0 {
		return
	}
	status, code := paymentOutcome(resp, err)
	s.recorder.record(ctx, providerName, rowID, PaymentEvent{
		Kind: EventCallback, Status: status, ErrorCode: code, LogID: logID,
		PaymentStatus: status, ProviderPaymentID: responsePaymentID(resp),
	})
}

// recordStatus mirrors a status answer when it changes the row. An error or an unknown status is
// not an answer about the payment, so nothing is written.
func (s *PaymentService) recordStatus(ctx context.Context, tenantID int, providerName, environment, paymentID string, logID int64, resp *PaymentResponse, err error) {
	if err != nil || resp == nil || !recordedStatuses[resp.Status] {
		return
	}
	rowID := s.recorder.find(ctx, PaymentKey{TenantID: tenantID, Provider: providerName, Environment: environment, ProviderPaymentID: paymentID})
	if rowID == 0 {
		return
	}
	s.recorder.record(ctx, providerName, rowID, PaymentEvent{
		Kind: EventStatus, Status: resp.Status, ErrorCode: resp.ErrorCode, LogID: logID,
		PaymentStatus: resp.Status, OnlyOnChange: true,
	})
}

// recordedStatuses are the provider statuses a payments row mirrors.
var recordedStatuses = map[PaymentStatus]bool{
	StatusPending: true, StatusProcessing: true, StatusSuccessful: true,
	StatusFailed: true, StatusCancelled: true, StatusRefunded: true,
}

// paymentOutcome maps a provider answer to the status and error code to record. A Go error is
// "error"; a status the row does not know is left unrecorded rather than guessed.
func paymentOutcome(resp *PaymentResponse, err error) (PaymentStatus, string) {
	if err != nil || resp == nil {
		return paymentError, "PROVIDER_ERROR"
	}
	if !recordedStatuses[resp.Status] {
		return "", resp.ErrorCode
	}
	return resp.Status, resp.ErrorCode
}

// responsePaymentID is the id the consumer gets back, empty when there is no answer.
func responsePaymentID(resp *PaymentResponse) string {
	if resp == nil {
		return ""
	}
	return resp.PaymentID
}

// startEvent is the event recorded after the provider answered a payment start.
func startEvent(resp *PaymentResponse, err error, logID int64) PaymentEvent {
	status, code := paymentOutcome(resp, err)
	return PaymentEvent{
		Kind: EventStart, Status: status, ErrorCode: code, LogID: logID,
		PaymentStatus: status, ProviderPaymentID: responsePaymentID(resp),
	}
}

// operationEvent is the event for a cancel or refund: its own result, and the payment's new status
// only when the operation succeeded. A Go error leaves the payment as it is: the outcome is unknown.
func operationEvent(kind string, done PaymentStatus, success bool, errorCode string, amount *float64, logID int64, err error) PaymentEvent {
	ev := PaymentEvent{Kind: kind, ErrorCode: errorCode, Amount: amount, LogID: logID}
	switch {
	case err != nil:
		ev.Status, ev.ErrorCode = paymentError, "PROVIDER_ERROR"
	case success:
		ev.Status, ev.PaymentStatus = done, done
	default:
		ev.Status = StatusFailed
	}
	return ev
}
