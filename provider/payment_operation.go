package provider

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/lib/pq"
)

// Operation kinds recorded in payment_operations.
const (
	OperationCancel = "cancel"
	OperationRefund = "refund"
)

// OperationOutcome is how a cancel or refund ended. Unknown means the provider call failed after it
// may have reached the bank.
type OperationOutcome string

const (
	OutcomePending   OperationOutcome = "pending"
	OutcomeSucceeded OperationOutcome = "succeeded"
	OutcomeRefused   OperationOutcome = "refused"
	OutcomeUnknown   OperationOutcome = "unknown"
)

// operationSettleAfter is how old an open operation must be before the provider's ledger may call it
// not landed: the handler gives up after 30 s and the provider client after 45 s, so by then a call
// has ended, and a pending one lost its close.
const operationSettleAfter = 2 * time.Minute

var (
	// ErrOperationInFlight refuses a cancel or refund while another one for the payment is running.
	ErrOperationInFlight = errors.New("another cancel or refund for this payment is in progress; retry later")
	// ErrOperationUnresolved refuses a cancel or refund while an earlier one has no known outcome: it
	// may have reached the bank, so sending another could return the money twice.
	ErrOperationUnresolved = errors.New("an earlier cancel or refund for this payment has no known outcome yet; retry later")
	// ErrOperationNotRecorded refuses a cancel or refund that could not be recorded first.
	ErrOperationNotRecorded = errors.New("cancel or refund could not be recorded")
)

// PaymentOperation is a payment's open cancel or refund.
type PaymentOperation struct {
	ID      int64
	Kind    string
	Amount  *float64 // nil for a cancel or a full refund
	Outcome OperationOutcome
	// Aged is an operation last written longer than operationSettleAfter ago.
	Aged bool
}

// OpenOperation records a cancel or refund before the provider is called. While another operation
// for the payment is open, the partial unique index refuses it with ErrOperationInFlight.
func (s *SQLPaymentStore) OpenOperation(ctx context.Context, paymentRowID int64, kind string, amount *float64, logID int64) (int64, error) {
	var id int64
	// The amount is stored as the minor units sent to the provider, so the ledger comparison matches.
	var stored any
	if amount != nil {
		stored = float64(MinorUnits(*amount)) / 100
	}
	err := s.db.QueryRowContext(ctx, `
		INSERT INTO payment_operations (payment_id, kind, amount, outcome, log_id)
		VALUES ($1, $2, $3::numeric, $4, $5::bigint)
		RETURNING id`,
		paymentRowID, kind, stored, string(OutcomePending), nullInt64(logID),
	).Scan(&id)
	var dbErr *pq.Error
	if errors.As(err, &dbErr) && dbErr.Code == "23505" {
		return 0, ErrOperationInFlight
	}
	if err != nil {
		return 0, fmt.Errorf("failed to open payment operation: %w", err)
	}
	return id, nil
}

// CloseOperation sets the outcome of an open operation.
func (s *SQLPaymentStore) CloseOperation(ctx context.Context, opID int64, outcome OperationOutcome) error {
	res, err := s.db.ExecContext(ctx, `
		UPDATE payment_operations SET outcome = $2, updated_at = now()
		WHERE id = $1 AND outcome IN ('pending', 'unknown')`, opID, string(outcome))
	if err != nil {
		return fmt.Errorf("failed to close payment operation: %w", err)
	}
	if n, err := res.RowsAffected(); err != nil || n != 1 {
		return fmt.Errorf("payment operation %d was not open (%v)", opID, err)
	}
	return nil
}

// UnsettledOperation returns the payment's open operation, or nil when there is none.
func (s *SQLPaymentStore) UnsettledOperation(ctx context.Context, paymentRowID int64) (*PaymentOperation, error) {
	var op PaymentOperation
	var amount sql.NullFloat64
	var outcome string
	err := s.db.QueryRowContext(ctx, `
		SELECT id, kind, amount, outcome, updated_at < now() - make_interval(secs => $2)
		FROM payment_operations
		WHERE payment_id = $1 AND outcome IN ('pending', 'unknown')`,
		paymentRowID, operationSettleAfter.Seconds(),
	).Scan(&op.ID, &op.Kind, &amount, &outcome, &op.Aged)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to read open payment operation: %w", err)
	}
	op.Outcome = OperationOutcome(outcome)
	if amount.Valid {
		op.Amount = &amount.Float64
	}
	return &op, nil
}

// SucceededRefundTotal is what GoPay recorded as refunded for the payment.
func (s *SQLPaymentStore) SucceededRefundTotal(ctx context.Context, paymentRowID int64) (float64, error) {
	var total float64
	err := s.db.QueryRowContext(ctx, `
		SELECT COALESCE(sum(amount), 0)::float8 FROM payment_operations
		WHERE payment_id = $1 AND kind = 'refund' AND outcome = 'succeeded'`, paymentRowID,
	).Scan(&total)
	if err != nil {
		return 0, fmt.Errorf("failed to sum refunds: %w", err)
	}
	return total, nil
}
