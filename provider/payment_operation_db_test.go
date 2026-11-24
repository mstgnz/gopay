package provider

import (
	"context"
	"errors"
	"testing"

	"github.com/mstgnz/gopay/infra/config"
)

func paymentWithOperations(t *testing.T) (*SQLPaymentStore, int64) {
	t.Helper()
	requireTestDB(t)
	store := NewPaymentStore(config.App().DB.DB)
	row := insertTestPayment(t, store, 1, uniqueLogID())
	t.Cleanup(func() { // before the payment's own cleanup: the FK is RESTRICT
		if _, err := config.App().DB.Exec(`DELETE FROM payment_operations WHERE payment_id = $1`, row); err != nil {
			t.Errorf("delete operations: %v", err)
		}
	})
	return store, row
}

// The partial unique index is what serializes two cancels or refunds of one payment.
func TestOperationStoreAllowsOneOpenOperation(t *testing.T) {
	store, row := paymentWithOperations(t)
	ctx := context.Background()
	amount := 5.0

	first, err := store.OpenOperation(ctx, row, OperationRefund, &amount, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.OpenOperation(ctx, row, OperationCancel, nil, 0); !errors.Is(err, ErrOperationInFlight) {
		t.Fatalf("second open: err %v, want ErrOperationInFlight", err)
	}
	if err := store.CloseOperation(ctx, first, OutcomeUnknown); err != nil {
		t.Fatal(err)
	}
	if _, err := store.OpenOperation(ctx, row, OperationRefund, &amount, 0); !errors.Is(err, ErrOperationInFlight) {
		t.Fatalf("open beside an unknown one: err %v, want ErrOperationInFlight", err)
	}

	open, err := store.UnsettledOperation(ctx, row)
	if err != nil || open == nil || open.ID != first || open.Outcome != OutcomeUnknown || open.Aged || *open.Amount != 5 {
		t.Fatalf("unsettled %+v, %v", open, err)
	}

	if err := store.CloseOperation(ctx, first, OutcomeSucceeded); err != nil {
		t.Fatal(err)
	}
	if err := store.CloseOperation(ctx, first, OutcomeRefused); err == nil {
		t.Error("a closed operation was closed again")
	}
	if open, err := store.UnsettledOperation(ctx, row); err != nil || open != nil {
		t.Fatalf("after close: %+v, %v", open, err)
	}
	second, err := store.OpenOperation(ctx, row, OperationRefund, &amount, 0)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.CloseOperation(ctx, second, OutcomeSucceeded); err != nil {
		t.Fatal(err)
	}
	if total, err := store.SucceededRefundTotal(ctx, row); err != nil || total != 10 {
		t.Errorf("refund total %v, %v; want 10", total, err)
	}
}

func TestOperationStoreMarksAStalePendingOperation(t *testing.T) {
	store, row := paymentWithOperations(t)
	ctx := context.Background()
	id, err := store.OpenOperation(ctx, row, OperationCancel, nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := config.App().DB.Exec(`UPDATE payment_operations SET updated_at = now() - interval '3 minutes' WHERE id = $1`, id); err != nil {
		t.Fatal(err)
	}
	open, err := store.UnsettledOperation(ctx, row)
	if err != nil || open == nil || !open.Aged || open.Amount != nil {
		t.Fatalf("unsettled %+v, %v; want an aged cancel", open, err)
	}
}

// The ledger compares minor units; a sub-kuruş amount is stored as the kuruş sent to the bank.
func TestOperationStoreKeepsTheMinorUnitsSent(t *testing.T) {
	store, row := paymentWithOperations(t)
	ctx := context.Background()
	amount := 10.005 // float64 10.00499..., sent as 1000
	id, err := store.OpenOperation(ctx, row, OperationRefund, &amount, 0)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.CloseOperation(ctx, id, OutcomeSucceeded); err != nil {
		t.Fatal(err)
	}
	if total, err := store.SucceededRefundTotal(ctx, row); err != nil || MinorUnits(total) != MinorUnits(amount) {
		t.Errorf("stored %v (%d kuruş), sent %d kuruş; %v", total, MinorUnits(total), MinorUnits(amount), err)
	}
}
