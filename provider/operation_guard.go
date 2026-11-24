package provider

import (
	"context"
	"errors"
	"time"

	"github.com/mstgnz/gopay/infra/logger"
)

// RefundLedger is what the provider itself recorded as returned for a payment.
type RefundLedger struct {
	RefundedMinor int64 // successful refunds, in minor units
	Reversed      bool  // a successful cancel
}

// RefundLedgerReader is implemented by providers that can read a payment's refunds and cancel back
// from the bank. Cancels and refunds are guarded against a duplicate only for them.
type RefundLedgerReader interface {
	RefundLedger(ctx context.Context, paymentID string, logID int64) (RefundLedger, error)
}

// beginOperation opens a cancel or refund before the provider call. opID 0 with no error means the
// payment is not guarded (its provider cannot read its ledger, or it has no payments row). replay
// reports that an earlier, identical operation already reached the bank, so nothing is to be sent.
func (s *PaymentService) beginOperation(ctx context.Context, p PaymentProvider, key PaymentKey, kind string, amount *float64, logID int64) (opID int64, replay bool, err error) {
	ledger, ok := p.(RefundLedgerReader)
	store := s.recorder.store
	if !ok || store == nil || key.ProviderPaymentID == "" {
		return 0, false, nil
	}
	if kind == OperationRefund && amount == nil {
		// The ledger cannot place a refund without an amount, so recording one would block the
		// payment for good. A ledger provider refuses it anyway (Paycell needs an amount).
		return 0, false, nil
	}

	rowID, err := store.FindPayment(ctx, key)
	if errors.Is(err, ErrPaymentRowNotFound) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, operationNotRecorded(key, "payment lookup", err)
	}

	open, err := store.UnsettledOperation(ctx, rowID)
	if err != nil {
		return 0, false, operationNotRecorded(key, "open operation lookup", err)
	}
	if open != nil {
		if open.Outcome == OutcomePending && !open.Aged {
			return 0, false, ErrOperationInFlight
		}
		landed, err := s.settleOperation(ctx, ledger, store, rowID, open, key, logID)
		if err != nil {
			return 0, false, err
		}
		if landed && open.Kind == kind && sameAmount(open.Amount, amount) {
			return 0, true, nil
		}
	}

	opID, err = store.OpenOperation(ctx, rowID, kind, amount, logID)
	if errors.Is(err, ErrOperationInFlight) {
		return 0, false, err
	}
	if err != nil {
		return 0, false, operationNotRecorded(key, "open operation", err)
	}
	return opID, false, nil
}

// settleOperation decides an operation whose outcome is unknown from the provider's own ledger and
// closes it. It answers whether the operation reached the bank; anything it cannot decide is
// ErrOperationUnresolved, and the operation stays open.
func (s *PaymentService) settleOperation(ctx context.Context, ledger RefundLedgerReader, store PaymentStore, rowID int64, op *PaymentOperation, key PaymentKey, logID int64) (bool, error) {
	led, err := ledger.RefundLedger(ctx, key.ProviderPaymentID, logID)
	if err != nil {
		return false, operationUnresolved(key, op, "ledger read failed: "+err.Error())
	}

	landed, known := false, false
	switch op.Kind {
	case OperationCancel:
		landed, known = led.Reversed, true
	case OperationRefund:
		if op.Amount == nil {
			break // a full refund's amount is not recorded, so the ledger cannot place it
		}
		recorded, err := store.SucceededRefundTotal(ctx, rowID)
		if err != nil {
			return false, operationUnresolved(key, op, "refund total read failed: "+err.Error())
		}
		switch led.RefundedMinor {
		case MinorUnits(recorded) + MinorUnits(*op.Amount):
			landed, known = true, true
		case MinorUnits(recorded):
			known = true
		}
	}
	if !known {
		return false, operationUnresolved(key, op, "the provider's refunds do not match the recorded ones")
	}
	if !landed && !op.Aged {
		// A ledger read seconds after the call may not list it yet; only a landed one is certain now.
		return false, ErrOperationInFlight
	}

	outcome := OutcomeRefused
	if landed {
		outcome = OutcomeSucceeded
	}
	if err := store.CloseOperation(ctx, op.ID, outcome); err != nil {
		return false, operationUnresolved(key, op, "close failed: "+err.Error())
	}
	logger.Info("Unknown payment operation settled from the provider ledger", logger.LogContext{
		Provider: key.Provider,
		Fields:   map[string]any{"payment_id": key.ProviderPaymentID, "operation": op.ID, "outcome": string(outcome)},
	})
	return landed, nil
}

// finishOperation closes an operation with the provider's answer. Anything but a success is unknown:
// a Go error may have reached the bank, and a provider answer may be "still processing" rather than a
// decline. The next operation settles it from the ledger. A failed close leaves it pending, and it is
// settled once aged.
func (s *PaymentService) finishOperation(ctx context.Context, providerName string, opID int64, success bool) {
	if opID == 0 {
		return
	}
	outcome := OutcomeUnknown
	if success {
		outcome = OutcomeSucceeded
	}
	ctx, cancel := afterProvider(ctx)
	defer cancel()
	if err := s.recorder.store.CloseOperation(ctx, opID, outcome); err != nil {
		logger.Error("Failed to close payment operation", err, logger.LogContext{
			Provider: providerName,
			Fields:   map[string]any{"operation": opID, "outcome": string(outcome)},
		})
	}
}

// logOperationRefusal closes the request's log row for a cancel or refund the guard refused.
func (s *PaymentService) logOperationRefusal(ctx context.Context, logID int64, code string, refusal error, start time.Time) {
	if logID <= 0 || s.logger == nil {
		return
	}
	if err := s.logger.LogError(ctx, logID, code, refusal.Error(), time.Since(start).Milliseconds()); err != nil {
		logger.Warn("Failed to log a refused cancel or refund", logger.LogContext{Fields: map[string]any{"log_id": logID, "error": err.Error()}})
	}
}

// logReplay closes the request's log row with the answer given for an operation that had already
// reached the bank.
func (s *PaymentService) logReplay(ctx context.Context, logID int64, resp any, start time.Time) {
	if logID <= 0 || s.logger == nil {
		return
	}
	if err := s.logger.LogResponse(ctx, logID, resp, time.Since(start).Milliseconds()); err != nil {
		logger.Warn("Failed to log a replayed cancel or refund", logger.LogContext{Fields: map[string]any{"log_id": logID, "error": err.Error()}})
	}
}

func operationNotRecorded(key PaymentKey, step string, err error) error {
	logger.Error("Cancel or refund refused: "+step+" failed", err, logger.LogContext{
		Provider: key.Provider,
		Fields:   map[string]any{"payment_id": key.ProviderPaymentID},
	})
	return ErrOperationNotRecorded
}

func operationUnresolved(key PaymentKey, op *PaymentOperation, why string) error {
	logger.Warn("Cancel or refund refused: an earlier one is unresolved", logger.LogContext{
		Provider: key.Provider,
		Fields:   map[string]any{"payment_id": key.ProviderPaymentID, "operation": op.ID, "kind": op.Kind, "reason": why},
	})
	return ErrOperationUnresolved
}

func sameAmount(a, b *float64) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return MinorUnits(*a) == MinorUnits(*b)
}
