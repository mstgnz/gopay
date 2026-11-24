package provider

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/mstgnz/gopay/infra/middle"
)

const (
	recTenant   = 990003
	recProvider = "stub-recorded"
	recEnv      = "sandbox"
)

// fakeStore is an in-memory PaymentStore.
type fakeStore struct {
	inserted  []NewPaymentRecord
	events    map[int64][]PaymentEvent
	insertErr error
	eventErr  error
	byID      map[string]int64 // provider payment id -> row
	byLog     map[int64]int64  // start log id -> row
	findErr   error

	ops      []*fakeOperation
	opErr    error // OpenOperation fails with it
	closeErr error
}

type fakeOperation struct {
	row int64
	PaymentOperation
}

func newFakeStore() *fakeStore {
	return &fakeStore{events: map[int64][]PaymentEvent{}, byID: map[string]int64{}, byLog: map[int64]int64{}}
}

// OpenOperation keeps the partial unique index's rule: one pending or unknown operation per row.
func (f *fakeStore) OpenOperation(_ context.Context, row int64, kind string, amount *float64, _ int64) (int64, error) {
	if f.opErr != nil {
		return 0, f.opErr
	}
	for _, op := range f.ops {
		if op.row == row && (op.Outcome == OutcomePending || op.Outcome == OutcomeUnknown) {
			return 0, ErrOperationInFlight
		}
	}
	f.ops = append(f.ops, &fakeOperation{row: row, PaymentOperation: PaymentOperation{
		ID: int64(len(f.ops) + 1), Kind: kind, Amount: amount, Outcome: OutcomePending,
	}})
	return int64(len(f.ops)), nil
}

func (f *fakeStore) CloseOperation(_ context.Context, id int64, outcome OperationOutcome) error {
	if f.closeErr != nil {
		return f.closeErr
	}
	f.ops[id-1].Outcome = outcome
	return nil
}

func (f *fakeStore) UnsettledOperation(_ context.Context, row int64) (*PaymentOperation, error) {
	for _, op := range f.ops {
		if op.row == row && (op.Outcome == OutcomePending || op.Outcome == OutcomeUnknown) {
			copied := op.PaymentOperation
			return &copied, nil
		}
	}
	return nil, nil
}

func (f *fakeStore) SucceededRefundTotal(_ context.Context, row int64) (float64, error) {
	total := 0.0
	for _, op := range f.ops {
		if op.row == row && op.Kind == OperationRefund && op.Outcome == OutcomeSucceeded && op.Amount != nil {
			total += *op.Amount
		}
	}
	return total, nil
}

func (f *fakeStore) InsertPayment(_ context.Context, rec NewPaymentRecord) (int64, error) {
	if f.insertErr != nil {
		return 0, f.insertErr
	}
	f.inserted = append(f.inserted, rec)
	return int64(len(f.inserted)), nil
}

// RecordEvent refuses a cancelled context like the database driver would.
func (f *fakeStore) RecordEvent(ctx context.Context, id int64, ev PaymentEvent) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	f.events[id] = append(f.events[id], ev)
	return f.eventErr
}

func (f *fakeStore) FindPayment(_ context.Context, key PaymentKey) (int64, error) {
	if f.findErr != nil {
		return 0, f.findErr
	}
	if key.TenantID != recTenant || key.Provider != recProvider || key.Environment != recEnv {
		return 0, ErrPaymentRowNotFound
	}
	if id, ok := f.byID[key.ProviderPaymentID]; ok {
		return id, nil
	}
	return 0, ErrPaymentRowNotFound
}

func (f *fakeStore) FindPaymentByStartLog(_ context.Context, tenantID int, providerName string, logID int64) (int64, error) {
	if id, ok := f.byLog[logID]; ok && providerName == recProvider && tenantID == recTenant {
		return id, nil
	}
	return 0, ErrPaymentRowNotFound
}

// scriptedProvider answers every core call with resp/err and counts the calls.
type scriptedProvider struct {
	stubProvider
	resp   *PaymentResponse
	refund *RefundResponse
	err    error
	calls  []string
}

func (p *scriptedProvider) Clone() PaymentProvider { return p } // shared on purpose: the test reads calls

func (p *scriptedProvider) CreatePayment(context.Context, PaymentRequest) (*PaymentResponse, error) {
	p.calls = append(p.calls, "pay")
	return p.resp, p.err
}

func (p *scriptedProvider) Create3DPayment(context.Context, PaymentRequest) (*PaymentResponse, error) {
	p.calls = append(p.calls, "pay3d")
	return p.resp, p.err
}

func (p *scriptedProvider) Complete3DPayment(context.Context, *CallbackState, map[string]string) (*PaymentResponse, error) {
	p.calls = append(p.calls, "complete3d")
	return p.resp, p.err
}

func (p *scriptedProvider) GetPaymentStatus(context.Context, GetPaymentStatusRequest) (*PaymentResponse, error) {
	p.calls = append(p.calls, "status")
	return p.resp, p.err
}

func (p *scriptedProvider) CancelPayment(context.Context, CancelRequest) (*PaymentResponse, error) {
	p.calls = append(p.calls, "cancel")
	return p.resp, p.err
}

func (p *scriptedProvider) RefundPayment(context.Context, RefundRequest) (*RefundResponse, error) {
	p.calls = append(p.calls, "refund")
	return p.refund, p.err
}

// recLogger is a PaymentLogger that hands out increasing log ids.
type recLogger struct {
	next       int64
	errorCodes []string
}

func (l *recLogger) LogRequest(context.Context, int, string, string, string, any, string, string) (int64, error) {
	l.next++
	return l.next + 100, nil
}
func (l *recLogger) LogResponse(context.Context, int64, any, int64) error { return nil }
func (l *recLogger) LogError(_ context.Context, _ int64, code, _ string, _ int64) error {
	l.errorCodes = append(l.errorCodes, code)
	return nil
}

func recCtx() context.Context {
	return context.WithValue(context.Background(), middle.TenantIDKey, fmt.Sprint(recTenant))
}

func setupRecorded(t *testing.T) (*PaymentService, *fakeStore, *scriptedProvider, *recLogger) {
	t.Helper()
	p := &scriptedProvider{}
	GetProviderCache().Set(recTenant, recProvider, recEnv, p)
	t.Cleanup(func() { GetProviderCache().Delete(recTenant, recProvider, recEnv) })
	store, log := newFakeStore(), &recLogger{}
	return NewPaymentService(log, store), store, p, log
}

func payRequest(use3D bool) PaymentRequest {
	// Installments need at least 1000 (PaymentService rule).
	return PaymentRequest{ID: "consumer-7", Amount: 1500.5, Currency: "TRY", InstallmentCount: 3, Use3D: use3D}
}

func TestCreatePaymentRefusedWhenRowCannotBeWritten(t *testing.T) {
	for _, use3D := range []bool{false, true} {
		s, store, p, log := setupRecorded(t)
		store.insertErr = errors.New("db down")

		resp, err := s.CreatePayment(recCtx(), recEnv, recProvider, payRequest(use3D))
		if !errors.Is(err, ErrPaymentNotRecorded) || resp != nil {
			t.Fatalf("use3D=%v: got %+v, %v; want ErrPaymentNotRecorded", use3D, resp, err)
		}
		if len(p.calls) != 0 {
			t.Errorf("use3D=%v: the provider was called without a payments row: %v", use3D, p.calls)
		}
		if len(log.errorCodes) != 1 || log.errorCodes[0] != "PAYMENT_RECORD_ERROR" {
			t.Errorf("use3D=%v: log row not closed with the record error: %v", use3D, log.errorCodes)
		}
	}
}

func TestCreatePaymentWithoutStoreIsRefused(t *testing.T) {
	s, _, p, _ := setupRecorded(t)
	s.recorder.store = nil
	if _, err := s.CreatePayment(recCtx(), recEnv, recProvider, payRequest(false)); !errors.Is(err, ErrPaymentNotRecorded) || len(p.calls) != 0 {
		t.Errorf("err %v, calls %v", err, p.calls)
	}
}

func TestCreatePaymentRecordsTheAttemptAndItsOutcome(t *testing.T) {
	cases := []struct {
		name    string
		use3D   bool
		resp    *PaymentResponse
		err     error
		channel string
		want    PaymentEvent
	}{
		{"paid", false, &PaymentResponse{Success: true, Status: StatusSuccessful, PaymentID: "P1"}, nil, ChannelCard,
			PaymentEvent{Kind: EventStart, Status: StatusSuccessful, PaymentStatus: StatusSuccessful, ProviderPaymentID: "P1", LogID: 101}},
		{"3D started", true, &PaymentResponse{Success: true, Status: StatusPending, PaymentID: "X1"}, nil, ChannelCard3D,
			PaymentEvent{Kind: EventStart, Status: StatusPending, PaymentStatus: StatusPending, ProviderPaymentID: "X1", LogID: 101}},
		{"declined", false, &PaymentResponse{Status: StatusFailed, ErrorCode: "0051", PaymentID: "P2"}, nil, ChannelCard,
			PaymentEvent{Kind: EventStart, Status: StatusFailed, ErrorCode: "0051", PaymentStatus: StatusFailed, ProviderPaymentID: "P2", LogID: 101}},
		{"provider error", false, nil, errors.New("timeout"), ChannelCard,
			PaymentEvent{Kind: EventStart, Status: paymentError, ErrorCode: "PROVIDER_ERROR", PaymentStatus: paymentError, LogID: 101}},
		{"unknown status left unrecorded", false, &PaymentResponse{Success: true, PaymentID: "P3"}, nil, ChannelCard,
			PaymentEvent{Kind: EventStart, ProviderPaymentID: "P3", LogID: 101}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s, store, p, _ := setupRecorded(t)
			p.resp, p.err = tc.resp, tc.err

			resp, err := s.CreatePayment(recCtx(), recEnv, recProvider, payRequest(tc.use3D))
			if resp != tc.resp || !errors.Is(err, tc.err) {
				t.Errorf("the consumer answer changed: %+v, %v", resp, err)
			}

			want := NewPaymentRecord{TenantID: recTenant, Provider: recProvider, Environment: recEnv, Channel: tc.channel,
				ConsumerReference: "consumer-7", Amount: 1500.5, Currency: "TRY", Installment: 3, StartLogID: 101}
			if len(store.inserted) != 1 || store.inserted[0] != want {
				t.Fatalf("inserted %+v, want %+v", store.inserted, want)
			}
			if evs := store.events[1]; len(evs) != 1 || fmt.Sprint(evs[0]) != fmt.Sprint(tc.want) {
				t.Errorf("events %+v, want %+v", evs, tc.want)
			}
		})
	}
}

// The consumer may disconnect or the handler deadline pass after the provider took the money.
func TestEventIsWrittenAfterTheRequestContextEnds(t *testing.T) {
	s, store, _, _ := setupRecorded(t)
	store.byID["P1"] = 6
	ctx, cancel := context.WithCancel(recCtx())
	cancel()

	s.recordStatus(ctx, recTenant, recProvider, recEnv, "P1", 101, &PaymentResponse{Status: StatusSuccessful}, nil)
	if len(store.events[6]) != 1 {
		t.Errorf("event lost with the request context: %+v", store.events)
	}
}

func TestRecordRefusalHidesTheDatabaseDetail(t *testing.T) {
	s, store, _, _ := setupRecorded(t)
	store.insertErr = errors.New(`pq: relation "payments" does not exist`)
	_, err := s.CreatePayment(recCtx(), recEnv, recProvider, payRequest(false))
	if err == nil || err.Error() != ErrPaymentNotRecorded.Error() {
		t.Errorf("consumer error %q, want only %q", err, ErrPaymentNotRecorded)
	}
}

func TestEventWriteFailureDoesNotChangeTheAnswer(t *testing.T) {
	s, store, p, _ := setupRecorded(t)
	store.eventErr = errors.New("db down")
	p.resp = &PaymentResponse{Success: true, Status: StatusSuccessful, PaymentID: "P1"}

	resp, err := s.CreatePayment(recCtx(), recEnv, recProvider, payRequest(false))
	if err != nil || resp != p.resp {
		t.Errorf("a failed event write changed the answer: %+v, %v", resp, err)
	}
}

func TestStatusIsRecordedOnlyAsAnAnswer(t *testing.T) {
	cases := []struct {
		name   string
		resp   *PaymentResponse
		err    error
		known  bool
		record bool
	}{
		{"answer for a known payment", &PaymentResponse{Status: StatusSuccessful}, nil, true, true},
		{"provider error", nil, errors.New("down"), true, false},
		{"unknown status", &PaymentResponse{Status: "weird"}, nil, true, false},
		{"payment from before the table", &PaymentResponse{Status: StatusSuccessful}, nil, false, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s, store, p, _ := setupRecorded(t)
			p.resp, p.err = tc.resp, tc.err
			if tc.known {
				store.byID["P1"] = 7
			}
			_, _ = s.GetPaymentStatus(recCtx(), recEnv, recProvider, GetPaymentStatusRequest{PaymentID: "P1"})

			evs := store.events[7]
			if !tc.record {
				if len(evs) != 0 {
					t.Errorf("recorded %+v", evs)
				}
				return
			}
			want := PaymentEvent{Kind: EventStatus, Status: StatusSuccessful, PaymentStatus: StatusSuccessful, OnlyOnChange: true, LogID: 101}
			if len(evs) != 1 || fmt.Sprint(evs[0]) != fmt.Sprint(want) {
				t.Errorf("events %+v, want %+v", evs, want)
			}
		})
	}
}

func TestStatusLookupFailureIsNotFatal(t *testing.T) {
	s, store, p, _ := setupRecorded(t)
	p.resp = &PaymentResponse{Status: StatusSuccessful}
	store.findErr = errors.New("db down")
	if resp, err := s.GetPaymentStatus(recCtx(), recEnv, recProvider, GetPaymentStatusRequest{PaymentID: "P1"}); err != nil || resp != p.resp {
		t.Errorf("got %+v, %v", resp, err)
	}
}

func TestCancelAndRefundEvents(t *testing.T) {
	amount := 40.0
	cases := []struct {
		name string
		run  func(s *PaymentService, p *scriptedProvider)
		want PaymentEvent
	}{
		{"cancelled", func(s *PaymentService, p *scriptedProvider) {
			p.resp = &PaymentResponse{Success: true, Status: StatusCancelled}
			_, _ = s.CancelPayment(recCtx(), recEnv, recProvider, CancelRequest{PaymentID: "P1"})
		}, PaymentEvent{Kind: EventCancel, Status: StatusCancelled, PaymentStatus: StatusCancelled, LogID: 101}},
		{"cancel refused", func(s *PaymentService, p *scriptedProvider) {
			p.resp = &PaymentResponse{Status: StatusSuccessful, ErrorCode: "0211"}
			_, _ = s.CancelPayment(recCtx(), recEnv, recProvider, CancelRequest{PaymentID: "P1"})
		}, PaymentEvent{Kind: EventCancel, Status: StatusFailed, ErrorCode: "0211", LogID: 101}},
		{"cancel unknown", func(s *PaymentService, p *scriptedProvider) {
			p.err = errors.New("timeout")
			_, _ = s.CancelPayment(recCtx(), recEnv, recProvider, CancelRequest{PaymentID: "P1"})
		}, PaymentEvent{Kind: EventCancel, Status: paymentError, ErrorCode: "PROVIDER_ERROR", LogID: 101}},
		{"refunded", func(s *PaymentService, p *scriptedProvider) {
			p.refund = &RefundResponse{Success: true}
			_, _ = s.RefundPayment(recCtx(), recEnv, recProvider, RefundRequest{PaymentID: "P1", RefundAmount: amount})
		}, PaymentEvent{Kind: EventRefund, Status: StatusRefunded, PaymentStatus: StatusRefunded, Amount: &amount, LogID: 101}},
		{"full refund records no amount", func(s *PaymentService, p *scriptedProvider) {
			p.refund = &RefundResponse{Success: true}
			_, _ = s.RefundPayment(recCtx(), recEnv, recProvider, RefundRequest{PaymentID: "P1"})
		}, PaymentEvent{Kind: EventRefund, Status: StatusRefunded, PaymentStatus: StatusRefunded, LogID: 101}},
		{"refund refused", func(s *PaymentService, p *scriptedProvider) {
			p.refund = &RefundResponse{ErrorCode: "0411"}
			_, _ = s.RefundPayment(recCtx(), recEnv, recProvider, RefundRequest{PaymentID: "P1", RefundAmount: amount})
		}, PaymentEvent{Kind: EventRefund, Status: StatusFailed, ErrorCode: "0411", Amount: &amount, LogID: 101}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s, store, p, _ := setupRecorded(t)
			store.byID["P1"] = 4
			tc.run(s, p)
			evs := store.events[4]
			if len(evs) != 1 {
				t.Fatalf("events %+v", evs)
			}
			got := evs[0]
			if got.Amount != nil && tc.want.Amount != nil && *got.Amount == *tc.want.Amount {
				got.Amount = tc.want.Amount
			}
			if fmt.Sprint(got) != fmt.Sprint(tc.want) {
				t.Errorf("event %+v, want %+v", got, tc.want)
			}
		})
	}
}

func TestCancelOfUnknownPaymentWritesNothing(t *testing.T) {
	s, store, p, _ := setupRecorded(t)
	p.resp = &PaymentResponse{Success: true, Status: StatusCancelled}
	_, _ = s.CancelPayment(recCtx(), recEnv, recProvider, CancelRequest{PaymentID: "old"})
	if len(store.events) != 0 {
		t.Errorf("events %+v", store.events)
	}
}

func TestRecordCallbackFindsTheRow(t *testing.T) {
	state := &CallbackState{TenantID: recTenant, Environment: recEnv, PaymentID: "X1"}
	paid := &PaymentResponse{Success: true, Status: StatusSuccessful, PaymentID: "X1"}

	t.Run("by start log id", func(t *testing.T) {
		s, store, _, _ := setupRecorded(t)
		store.byLog[55], store.byID["X1"] = 3, 9
		s.recordCallback(context.Background(), recProvider, state, 55, 200, paid, nil)
		want := PaymentEvent{Kind: EventCallback, Status: StatusSuccessful, PaymentStatus: StatusSuccessful, ProviderPaymentID: "X1", LogID: 200}
		if evs := store.events[3]; len(evs) != 1 || fmt.Sprint(evs[0]) != fmt.Sprint(want) {
			t.Errorf("events %+v", store.events)
		}
	})
	t.Run("by payment id when the start log is unknown", func(t *testing.T) {
		s, store, _, _ := setupRecorded(t)
		store.byID["X1"] = 9
		s.recordCallback(context.Background(), recProvider, state, 55, 200, paid, nil)
		if len(store.events[9]) != 1 {
			t.Errorf("events %+v", store.events)
		}
	})
	t.Run("state without a log id", func(t *testing.T) {
		s, store, _, _ := setupRecorded(t)
		store.byID["X1"] = 9
		s.recordCallback(context.Background(), recProvider, state, 0, 200, nil, errors.New("bank down"))
		if evs := store.events[9]; len(evs) != 1 || evs[0].Status != paymentError {
			t.Errorf("events %+v", store.events)
		}
	})
	t.Run("payment from before the table", func(t *testing.T) {
		s, store, _, _ := setupRecorded(t)
		s.recordCallback(context.Background(), recProvider, state, 55, 200, paid, nil)
		if len(store.events) != 0 {
			t.Errorf("events %+v", store.events)
		}
	})
}

func TestRecorderWithoutStoreIsANoOp(t *testing.T) {
	var r paymentRecorder
	r.record(context.Background(), recProvider, 1, PaymentEvent{Kind: EventStart})
	if r.find(context.Background(), PaymentKey{ProviderPaymentID: "P"}) != 0 {
		t.Error("find without a store")
	}
	if r.findForCallback(context.Background(), recProvider, &CallbackState{}, 1) != 0 {
		t.Error("findForCallback without a store")
	}
}
