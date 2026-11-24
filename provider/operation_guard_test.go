package provider

import (
	"context"
	"errors"
	"testing"
)

// ledgerProvider is a provider that can read its refunds back from the bank.
type ledgerProvider struct {
	scriptedProvider
	ledger      RefundLedger
	ledgerErr   error
	ledgerCalls int
}

func (p *ledgerProvider) Clone() PaymentProvider { return p }

func (p *ledgerProvider) RefundLedger(context.Context, string, int64) (RefundLedger, error) {
	p.ledgerCalls++
	return p.ledger, p.ledgerErr
}

// setupGuarded is a service over a ledger-capable provider and a payments row for "P1".
func setupGuarded(t *testing.T) (*PaymentService, *fakeStore, *ledgerProvider, *recLogger) {
	t.Helper()
	p := &ledgerProvider{}
	GetProviderCache().Set(recTenant, recProvider, recEnv, p)
	t.Cleanup(func() { GetProviderCache().Delete(recTenant, recProvider, recEnv) })
	store, log := newFakeStore(), &recLogger{}
	store.byID["P1"] = 1
	return NewPaymentService(log, store), store, p, log
}

func refundP1(s *PaymentService, amount float64) (*RefundResponse, error) {
	return s.RefundPayment(recCtx(), recEnv, recProvider, RefundRequest{PaymentID: "P1", RefundAmount: amount})
}

func refundCalls(p *ledgerProvider) int {
	n := 0
	for _, c := range p.calls {
		if c == "refund" || c == "cancel" {
			n++
		}
	}
	return n
}

func outcomes(store *fakeStore) []OperationOutcome {
	var out []OperationOutcome
	for _, op := range store.ops {
		out = append(out, op.Outcome)
	}
	return out
}

func TestRefundIsRecordedAsAnOperation(t *testing.T) {
	s, store, p, _ := setupGuarded(t)
	p.refund = &RefundResponse{Success: true, PaymentID: "P1"}

	if _, err := refundP1(s, 100); err != nil {
		t.Fatal(err)
	}
	if len(store.ops) != 1 || store.ops[0].Outcome != OutcomeSucceeded || *store.ops[0].Amount != 100 {
		t.Fatalf("operations %+v", store.ops)
	}

	// A non-success answer may be "still processing" (Paycell 3023) rather than a decline, so it is
	// left for the ledger to settle.
	p.refund = &RefundResponse{Success: false, ErrorCode: "3023"}
	if _, err := refundP1(s, 50); err != nil {
		t.Fatal(err)
	}
	if got := outcomes(store); len(got) != 2 || got[1] != OutcomeUnknown {
		t.Fatalf("a non-success answer closed as %v, want unknown", got)
	}
}

// A retry seconds after the call may come before the ledger lists it; only a landed refund is
// certain then, so "not landed" waits until the operation has aged.
func TestImmediateRetryOfAnUnlandedRefundWaits(t *testing.T) {
	s, store, p, _ := setupGuarded(t)
	p.refund = &RefundResponse{Success: false, ErrorCode: "3023"}
	_, _ = refundP1(s, 100)

	p.ledger = RefundLedger{RefundedMinor: 0}
	if _, err := refundP1(s, 100); !errors.Is(err, ErrOperationInFlight) {
		t.Fatalf("immediate retry: err %v, want ErrOperationInFlight", err)
	}
	if refundCalls(p) != 1 {
		t.Fatalf("bank calls %d, want 1", refundCalls(p))
	}

	store.ops[0].Aged = true
	p.refund = &RefundResponse{Success: true}
	if resp, err := refundP1(s, 100); err != nil || !resp.Success || refundCalls(p) != 2 {
		t.Fatalf("aged retry: %+v, %v, bank calls %d", resp, err, refundCalls(p))
	}
}

// A refund without an amount cannot be placed in the ledger; recording it would block the payment
// for good, so it stays unguarded and later refunds still go through.
func TestRefundWithoutAmountDoesNotBlockThePayment(t *testing.T) {
	s, store, p, _ := setupGuarded(t)
	p.err = errors.New("paycell: refund amount must be greater than 0")
	if _, err := refundP1(s, 0); err == nil {
		t.Fatal("want the provider error")
	}
	if len(store.ops) != 0 {
		t.Fatalf("an amountless refund was recorded: %v", outcomes(store))
	}

	p.err = nil
	p.refund = &RefundResponse{Success: true}
	if _, err := refundP1(s, 25); err != nil {
		t.Fatalf("next refund: %v", err)
	}
}

// The case the guard exists for: the first refund reached the bank but GoPay answered with an error,
// and the consumer retries. The bank's ledger shows it landed, so nothing is sent again.
func TestRetriedRefundThatLandedIsNotSentTwice(t *testing.T) {
	s, store, p, _ := setupGuarded(t)
	p.err = errors.New("read timeout")
	if _, err := refundP1(s, 100); err == nil {
		t.Fatal("want the provider error")
	}
	if got := outcomes(store); len(got) != 1 || got[0] != OutcomeUnknown {
		t.Fatalf("a failed call closed as %v, want unknown", got)
	}

	p.err = nil
	p.ledger = RefundLedger{RefundedMinor: 10000} // 100.00 TRY landed
	resp, err := refundP1(s, 100)
	if err != nil || resp == nil || !resp.Success {
		t.Fatalf("retry: %+v, %v; want the earlier refund reported as done", resp, err)
	}
	if refundCalls(p) != 1 {
		t.Fatalf("the bank was asked to refund %d times, want 1", refundCalls(p))
	}
	if got := outcomes(store); len(got) != 1 || got[0] != OutcomeSucceeded {
		t.Errorf("operations %v, want the unknown one settled as succeeded", got)
	}
}

func TestRetriedRefundThatDidNotLandIsSent(t *testing.T) {
	s, store, p, _ := setupGuarded(t)
	p.err = errors.New("connection reset")
	_, _ = refundP1(s, 100)
	store.ops[0].Aged = true

	p.err = nil
	p.ledger = RefundLedger{RefundedMinor: 0}
	p.refund = &RefundResponse{Success: true, PaymentID: "P1"}
	if resp, err := refundP1(s, 100); err != nil || !resp.Success {
		t.Fatalf("retry: %+v, %v", resp, err)
	}
	if refundCalls(p) != 2 {
		t.Errorf("bank calls %d, want 2", refundCalls(p))
	}
	if got := outcomes(store); len(got) != 2 || got[0] != OutcomeRefused || got[1] != OutcomeSucceeded {
		t.Errorf("operations %v, want the first refused and the retry succeeded", got)
	}
}

// A landed refund of a different amount is a new request: the earlier one is settled and the new one sent.
func TestNewRefundAfterALandedOneIsSent(t *testing.T) {
	s, store, p, _ := setupGuarded(t)
	p.err = errors.New("timeout")
	_, _ = refundP1(s, 100)

	p.err = nil
	p.ledger = RefundLedger{RefundedMinor: 10000}
	p.refund = &RefundResponse{Success: true}
	if _, err := refundP1(s, 40); err != nil {
		t.Fatal(err)
	}
	if refundCalls(p) != 2 || len(store.ops) != 2 || store.ops[0].Outcome != OutcomeSucceeded {
		t.Errorf("calls %d, operations %v", refundCalls(p), outcomes(store))
	}
}

func TestUnresolvableRefundBlocksTheNextOne(t *testing.T) {
	for name, ledger := range map[string]struct {
		led RefundLedger
		err error
	}{
		"ledger does not match": {led: RefundLedger{RefundedMinor: 5000}},
		"ledger unreadable":     {err: errors.New("paycell 3023")},
	} {
		t.Run(name, func(t *testing.T) {
			s, store, p, log := setupGuarded(t)
			p.err = errors.New("timeout")
			_, _ = refundP1(s, 100)

			p.err, p.ledger, p.ledgerErr = nil, ledger.led, ledger.err
			if _, err := refundP1(s, 100); !errors.Is(err, ErrOperationUnresolved) {
				t.Fatalf("err %v, want ErrOperationUnresolved", err)
			}
			if refundCalls(p) != 1 {
				t.Errorf("bank calls %d, want 1", refundCalls(p))
			}
			if got := outcomes(store); len(got) != 1 || got[0] != OutcomeUnknown {
				t.Errorf("operations %v, want the unknown one left open", got)
			}
			if last := log.errorCodes[len(log.errorCodes)-1]; last != "REFUND_REFUSED" {
				t.Errorf("log row closed with %q", last)
			}
		})
	}
}

func TestRefundWhileAnotherIsRunningIsRefused(t *testing.T) {
	s, store, p, _ := setupGuarded(t)
	if _, err := store.OpenOperation(recCtx(), 1, OperationRefund, nil, 0); err != nil {
		t.Fatal(err)
	}
	if _, err := refundP1(s, 100); !errors.Is(err, ErrOperationInFlight) {
		t.Fatalf("err %v, want ErrOperationInFlight", err)
	}
	if refundCalls(p) != 0 || p.ledgerCalls != 0 {
		t.Errorf("bank calls %d, ledger reads %d; want none", refundCalls(p), p.ledgerCalls)
	}
}

// A pending operation that outlived every timeout lost its close; it is settled like an unknown one.
func TestStalePendingOperationIsSettledFromTheLedger(t *testing.T) {
	s, store, p, _ := setupGuarded(t)
	amount := 100.0
	if _, err := store.OpenOperation(recCtx(), 1, OperationRefund, &amount, 0); err != nil {
		t.Fatal(err)
	}
	store.ops[0].Aged = true
	p.ledger = RefundLedger{RefundedMinor: 10000}

	if resp, err := refundP1(s, 100); err != nil || !resp.Success || refundCalls(p) != 0 {
		t.Fatalf("resp %+v, err %v, bank calls %d; want a replay", resp, err, refundCalls(p))
	}
}

func TestRefundThatCannotBeRecordedIsNotSent(t *testing.T) {
	s, store, p, _ := setupGuarded(t)
	store.opErr = errors.New("db down")
	if _, err := refundP1(s, 100); !errors.Is(err, ErrOperationNotRecorded) {
		t.Fatalf("err %v, want ErrOperationNotRecorded", err)
	}
	if refundCalls(p) != 0 {
		t.Errorf("the bank was called without a record")
	}

	store.opErr, store.findErr = nil, errors.New("db down")
	if _, err := refundP1(s, 100); !errors.Is(err, ErrOperationNotRecorded) || refundCalls(p) != 0 {
		t.Errorf("payment lookup failure: err %v, bank calls %d", err, refundCalls(p))
	}
}

func TestCancelThatLandedIsNotSentTwice(t *testing.T) {
	s, store, p, _ := setupGuarded(t)
	p.err = errors.New("timeout")
	if _, err := s.CancelPayment(recCtx(), recEnv, recProvider, CancelRequest{PaymentID: "P1"}); err == nil {
		t.Fatal("want the provider error")
	}

	p.err = nil
	p.ledger = RefundLedger{Reversed: true}
	resp, err := s.CancelPayment(recCtx(), recEnv, recProvider, CancelRequest{PaymentID: "P1"})
	if err != nil || !resp.Success || resp.Status != StatusCancelled {
		t.Fatalf("retry: %+v, %v", resp, err)
	}
	if refundCalls(p) != 1 || store.ops[0].Outcome != OutcomeSucceeded {
		t.Errorf("bank calls %d, operations %v", refundCalls(p), outcomes(store))
	}
}

// Providers that cannot read their ledger, and payments started before the payments table, keep the
// behaviour they had: no operation is recorded and nothing is refused.
func TestUnguardedRefundsAreUnchanged(t *testing.T) {
	s, store, p, _ := setupRecorded(t) // scriptedProvider has no ledger
	store.byID["P1"] = 1
	p.refund = &RefundResponse{Success: true}
	if _, err := refundP1(s, 100); err != nil || len(store.ops) != 0 || len(p.calls) != 1 {
		t.Errorf("no ledger: err %v, operations %d, calls %v", err, len(store.ops), p.calls)
	}

	gs, gstore, gp, _ := setupGuarded(t)
	gp.refund = &RefundResponse{Success: true}
	if _, err := gs.RefundPayment(recCtx(), recEnv, recProvider, RefundRequest{PaymentID: "OLD", RefundAmount: 10}); err != nil ||
		len(gstore.ops) != 0 || refundCalls(gp) != 1 {
		t.Errorf("no payments row: err %v, operations %d, calls %d", err, len(gstore.ops), refundCalls(gp))
	}
}
