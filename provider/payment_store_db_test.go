package provider

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/mstgnz/gopay/infra/config"
	"github.com/mstgnz/gopay/infra/middle"
)

// paycell is an active provider in gopay.sql; tenants 1 and 2 come from .github/ci-seed.sql.
func insertTestPayment(t *testing.T, store *SQLPaymentStore, tenantID int, logID int64) int64 {
	t.Helper()
	id, err := store.InsertPayment(context.Background(), NewPaymentRecord{
		TenantID: tenantID, Provider: "paycell", Environment: "sandbox", Channel: ChannelCard3D,
		ConsumerReference: "c-1", Amount: 12.34, Currency: "TRY", Installment: 1, StartLogID: logID,
	})
	if err != nil {
		t.Fatalf("InsertPayment: %v", err)
	}
	t.Cleanup(func() {
		if _, err := config.App().DB.Exec(`DELETE FROM payments WHERE id = $1`, id); err != nil {
			t.Errorf("delete payment %d: %v", id, err)
		}
	})
	return id
}

func paymentRow(t *testing.T, id int64) (status, providerPaymentID, errorCode string) {
	t.Helper()
	var ppid, code sql.NullString
	if err := config.App().DB.QueryRow(`SELECT status, provider_payment_id, error_code FROM payments WHERE id = $1`, id).
		Scan(&status, &ppid, &code); err != nil {
		t.Fatalf("read payment %d: %v", id, err)
	}
	return status, ppid.String, code.String
}

func eventCount(t *testing.T, id int64) int {
	t.Helper()
	var n int
	if err := config.App().DB.QueryRow(`SELECT count(*) FROM payment_events WHERE payment_id = $1`, id).Scan(&n); err != nil {
		t.Fatalf("count events: %v", err)
	}
	return n
}

func uniqueLogID() int64 { return time.Now().UnixNano() % 1e15 }

func TestPaymentStoreRecordsEvents(t *testing.T) {
	requireTestDB(t)
	store := NewPaymentStore(config.App().DB.DB)
	ctx := context.Background()
	id := insertTestPayment(t, store, 1, uniqueLogID())

	if status, _, _ := paymentRow(t, id); status != "initiated" {
		t.Fatalf("new row status %s", status)
	}

	err := store.RecordEvent(ctx, id, PaymentEvent{Kind: EventStart, Status: StatusFailed, ErrorCode: "0051",
		PaymentStatus: StatusFailed, ProviderPaymentID: "PX"})
	if err != nil {
		t.Fatalf("RecordEvent: %v", err)
	}
	if status, ppid, code := paymentRow(t, id); status != "failed" || ppid != "PX" || code != "0051" {
		t.Errorf("row %s %s %s", status, ppid, code)
	}

	// A status poll repeating the stored status writes nothing.
	if err := store.RecordEvent(ctx, id, PaymentEvent{Kind: EventStatus, Status: StatusFailed, PaymentStatus: StatusFailed, OnlyOnChange: true}); err != nil {
		t.Fatalf("unchanged status: %v", err)
	}
	if n := eventCount(t, id); n != 1 {
		t.Errorf("unchanged status wrote an event: %d events", n)
	}

	// A changed status writes, and clears the old error code.
	if err := store.RecordEvent(ctx, id, PaymentEvent{Kind: EventStatus, Status: StatusSuccessful, PaymentStatus: StatusSuccessful, OnlyOnChange: true}); err != nil {
		t.Fatalf("changed status: %v", err)
	}
	if status, ppid, code := paymentRow(t, id); status != "successful" || ppid != "PX" || code != "" || eventCount(t, id) != 2 {
		t.Errorf("row %s %s %q, %d events", status, ppid, code, eventCount(t, id))
	}

	// A failed refund records its own result and keeps the payment's status.
	amount := 5.0
	if err := store.RecordEvent(ctx, id, PaymentEvent{Kind: EventRefund, Status: StatusFailed, ErrorCode: "0411", Amount: &amount}); err != nil {
		t.Fatalf("refund event: %v", err)
	}
	if status, _, code := paymentRow(t, id); status != "successful" || code != "" {
		t.Errorf("a failed refund changed the payment: %s %q", status, code)
	}

	if err := store.RecordEvent(ctx, 0, PaymentEvent{Kind: EventStart}); !errors.Is(err, ErrPaymentRowNotFound) {
		t.Errorf("missing row: err = %v", err)
	}
	if err := store.RecordEvent(ctx, id, PaymentEvent{Kind: "bogus"}); err == nil {
		t.Error("an unknown event kind passed the CHECK")
	}
}

func TestPaymentStoreFinds(t *testing.T) {
	requireTestDB(t)
	store := NewPaymentStore(config.App().DB.DB)
	ctx := context.Background()
	ppid := fmt.Sprintf("find-%d", time.Now().UnixNano())

	older := insertTestPayment(t, store, 1, uniqueLogID())
	logID := uniqueLogID() + 1
	newer := insertTestPayment(t, store, 1, logID)
	other := insertTestPayment(t, store, 2, uniqueLogID()+2)
	for _, id := range []int64{older, newer, other} {
		if err := store.RecordEvent(ctx, id, PaymentEvent{Kind: EventStart, ProviderPaymentID: ppid}); err != nil {
			t.Fatalf("RecordEvent: %v", err)
		}
	}

	got, err := store.FindPayment(ctx, PaymentKey{TenantID: 1, Provider: "paycell", Environment: "sandbox", ProviderPaymentID: ppid})
	if err != nil || got != newer {
		t.Errorf("FindPayment = %d, %v; want the newest row %d", got, err, newer)
	}
	if got, err := store.FindPayment(ctx, PaymentKey{TenantID: 2, Provider: "paycell", Environment: "sandbox", ProviderPaymentID: ppid}); err != nil || got != other {
		t.Errorf("tenant 2 = %d, %v; want %d", got, err, other)
	}
	for _, key := range []PaymentKey{
		{TenantID: 1, Provider: "nkolay", Environment: "sandbox", ProviderPaymentID: ppid},
		{TenantID: 1, Provider: "paycell", Environment: "production", ProviderPaymentID: ppid},
		{TenantID: 1, Provider: "paycell", Environment: "sandbox", ProviderPaymentID: ppid + "-none"},
	} {
		if _, err := store.FindPayment(ctx, key); !errors.Is(err, ErrPaymentRowNotFound) {
			t.Errorf("%+v: err = %v, want not found", key, err)
		}
	}

	if got, err := store.FindPaymentByStartLog(ctx, 1, "paycell", logID); err != nil || got != newer {
		t.Errorf("FindPaymentByStartLog = %d, %v; want %d", got, err, newer)
	}
	if _, err := store.FindPaymentByStartLog(ctx, 1, "nkolay", logID); !errors.Is(err, ErrPaymentRowNotFound) {
		t.Errorf("other provider's log id: err = %v", err)
	}
	if _, err := store.FindPaymentByStartLog(ctx, 2, "paycell", logID); !errors.Is(err, ErrPaymentRowNotFound) {
		t.Errorf("other tenant's log id: err = %v", err)
	}
}

func TestPaymentStoreRefusesBadInserts(t *testing.T) {
	requireTestDB(t)
	store := NewPaymentStore(config.App().DB.DB)
	ctx := context.Background()
	base := NewPaymentRecord{TenantID: 1, Provider: "paycell", Environment: "sandbox", Channel: ChannelCard, Amount: 1, Currency: "TRY"}

	for name, mutate := range map[string]func(r *NewPaymentRecord){
		"unknown provider": func(r *NewPaymentRecord) { r.Provider = "no-such-provider" },
		"bad environment":  func(r *NewPaymentRecord) { r.Environment = "test" },
		"bad channel":      func(r *NewPaymentRecord) { r.Channel = "cash" },
		"no tenant":        func(r *NewPaymentRecord) { r.TenantID = 0 },
	} {
		rec := base
		mutate(&rec)
		if id, err := store.InsertPayment(ctx, rec); err == nil {
			_, _ = config.App().DB.Exec(`DELETE FROM payments WHERE id = $1`, id)
			t.Errorf("%s: insert accepted", name)
		}
	}
}

// The callback is matched by the START's log id, which Complete3DPayment must read from the state
// before it overwrites LogID with the completion's own log id.
func TestComplete3DPaymentRecordsOnTheStartRow(t *testing.T) {
	requireTestDB(t)
	const logID = int64(7_000_001)
	stateID, err := StoreCallbackState(context.Background(), CallbackState{
		TenantID: 1, Provider: "paycell", PaymentID: "X1", Amount: 10, Environment: "sandbox", LogID: logID,
	})
	if err != nil {
		t.Fatalf("StoreCallbackState: %v", err)
	}
	id, _ := strconv.ParseInt(stateID, 10, 64)
	deleteCallbackRowOnCleanup(t, id)

	p := &scriptedProvider{resp: &PaymentResponse{Success: true, Status: StatusSuccessful, PaymentID: "X1"}}
	GetProviderCache().Set(1, "paycell", "sandbox", p)
	t.Cleanup(func() { GetProviderCache().Delete(1, "paycell", "sandbox") })

	store := newFakeStore()
	store.byID["X1"] = 9 // the payment-id fallback must not be what matched
	s := NewPaymentService(&recLogger{}, startLogStore{fakeStore: store, logID: logID, row: 3})
	if _, err := s.Complete3DPayment(context.Background(), "paycell", stateID, map[string]string{}); err != nil {
		t.Fatalf("Complete3DPayment: %v", err)
	}
	if len(store.events[3]) != 1 || len(store.events[9]) != 0 {
		t.Errorf("events %+v; want one on the start row 3", store.events)
	}
}

// startLogStore answers FindPaymentByStartLog for tenant 1, provider paycell.
type startLogStore struct {
	*fakeStore
	logID int64
	row   int64
}

func (s startLogStore) FindPaymentByStartLog(_ context.Context, tenantID int, providerName string, logID int64) (int64, error) {
	if tenantID == 1 && providerName == "paycell" && logID == s.logID {
		return s.row, nil
	}
	return 0, ErrPaymentRowNotFound
}

func (s startLogStore) FindPayment(context.Context, PaymentKey) (int64, error) {
	return s.byID["X1"], nil
}

// cardStorageStub is a saved-card provider whose wallet calls are counted.
type cardStorageStub struct {
	stubProvider
	pays int
}

func (c *cardStorageStub) Clone() PaymentProvider { return c }
func (c *cardStorageStub) SendCardOTP(context.Context, CardOTPSendRequest) (*CardOTPSendResponse, error) {
	return nil, nil
}
func (c *cardStorageStub) ValidateCardOTP(context.Context, CardOTPValidateRequest) (*CardOTPValidateResponse, error) {
	return nil, nil
}
func (c *cardStorageStub) RegisterCard(context.Context, RegisterCardRequest) (*RegisterCardResponse, error) {
	return nil, nil
}
func (c *cardStorageStub) ListProviderCards(context.Context, ListCardsRequest) (*ListCardsResponse, error) {
	return nil, nil
}
func (c *cardStorageStub) DeleteProviderCard(context.Context, DeleteCardRequest) (*DeleteCardResponse, error) {
	return nil, nil
}
func (c *cardStorageStub) PayWithSavedCard(context.Context, SavedCardPaymentRequest) (*PaymentResponse, error) {
	c.pays++
	return &PaymentResponse{Success: true, Status: StatusSuccessful, PaymentID: "SC1"}, nil
}
func (c *cardStorageStub) Create3DPaymentWithSavedCard(context.Context, SavedCardPaymentRequest) (*PaymentResponse, error) {
	c.pays++
	return &PaymentResponse{Success: true, Status: StatusPending, PaymentID: "SC3D"}, nil
}

func TestPaySavedCardRecordsOrRefuses(t *testing.T) {
	requireTestDB(t)
	ctx := context.WithValue(context.Background(), middle.TenantIDKey, "1")
	repo := NewSavedCardRepository(config.App().DB.DB)

	var providerID int
	if err := config.App().DB.QueryRow(`SELECT id FROM providers WHERE name = 'paycell'`).Scan(&providerID); err != nil {
		t.Fatalf("paycell provider id: %v", err)
	}
	cardID, err := repo.Create(ctx, &SavedCard{TenantID: 1, ProviderID: providerID, Environment: "sandbox",
		MSISDN: "5000000009", ProviderCardID: fmt.Sprintf("card-%d", time.Now().UnixNano())})
	if err != nil {
		t.Fatalf("create saved card: %v", err)
	}
	t.Cleanup(func() { _, _ = config.App().DB.Exec(`DELETE FROM saved_cards WHERE id = $1`, cardID) })

	stub := &cardStorageStub{}
	GetProviderCache().Set(1, "paycell", "sandbox", stub)
	t.Cleanup(func() { GetProviderCache().Delete(1, "paycell", "sandbox") })

	req := SavedCardPaymentRequest{Amount: 25, Currency: "TRY", ConversationID: "conv-1"}

	// The payments row cannot be written: the wallet is never charged, with or without 3D.
	failing := newFakeStore()
	failing.insertErr = errors.New("db down")
	s := NewCardService(&recLogger{}, repo, nil, failing)
	for _, use3D := range []bool{false, true} {
		if _, err := s.PaySavedCard(ctx, "sandbox", "paycell", cardID, req, use3D); !errors.Is(err, ErrPaymentNotRecorded) || stub.pays != 0 {
			t.Fatalf("use3D=%v: err %v, pays %d; want refused before the provider", use3D, err, stub.pays)
		}
	}

	for _, use3D := range []bool{false, true} {
		store := newFakeStore()
		s := NewCardService(&recLogger{}, repo, nil, store)
		resp, err := s.PaySavedCard(ctx, "sandbox", "paycell", cardID, req, use3D)
		if err != nil || resp == nil {
			t.Fatalf("use3D=%v: %v", use3D, err)
		}
		want := ChannelSavedCard
		if use3D {
			want = ChannelSavedCard3D
		}
		if len(store.inserted) != 1 || store.inserted[0].Channel != want || store.inserted[0].ConsumerReference != "conv-1" || store.inserted[0].Amount != 25 {
			t.Errorf("use3D=%v: inserted %+v", use3D, store.inserted)
		}
		if evs := store.events[1]; len(evs) != 1 || evs[0].ProviderPaymentID != resp.PaymentID {
			t.Errorf("use3D=%v: events %+v", use3D, evs)
		}
	}
}
