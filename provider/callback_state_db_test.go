package provider

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mstgnz/gopay/infra/config"
)

// insertCallbackRow writes a callbacks row whose expires_at is now() plus expiresIn on the database
// clock; a negative interval makes it already expired.
func insertCallbackRow(t *testing.T, providerName, paymentID string, used bool, expiresIn string) int64 {
	t.Helper()
	var id int64
	err := config.App().DB.QueryRow(
		`INSERT INTO callbacks (tenant_id, provider, payment_id, state_data, expires_at, used)
		 VALUES (1, $1, $2, jsonb_build_object('paymentId', $2::text), now() + $3::interval, $4) RETURNING id`,
		providerName, paymentID, expiresIn, used,
	).Scan(&id)
	if err != nil {
		t.Fatalf("insert callbacks row: %v", err)
	}
	deleteCallbackRowOnCleanup(t, id)
	return id
}

func deleteCallbackRowOnCleanup(t *testing.T, id int64) {
	t.Cleanup(func() {
		if _, err := config.App().DB.Exec(`DELETE FROM callbacks WHERE id = $1`, id); err != nil {
			t.Errorf("delete callbacks row %d: %v", id, err)
		}
	})
}

func storeTestCallbackState(t *testing.T, paymentID string) string {
	t.Helper()
	stateID, err := StoreCallbackState(context.Background(), CallbackState{
		TenantID: 1, Provider: "paycell", PaymentID: paymentID, Amount: 10, Environment: "sandbox",
	})
	if err != nil {
		t.Fatalf("StoreCallbackState: %v", err)
	}
	id, err := strconv.ParseInt(stateID, 10, 64)
	if err != nil {
		t.Fatalf("state id %q: %v", stateID, err)
	}
	deleteCallbackRowOnCleanup(t, id)
	return stateID
}

func TestStoreCallbackState_ExpiresOnDatabaseClock(t *testing.T) {
	requireTestDB(t)
	stateID := storeTestCallbackState(t, fmt.Sprintf("cbstore-%d", time.Now().UnixNano()))

	var seconds float64
	err := config.App().DB.QueryRow(
		`SELECT EXTRACT(EPOCH FROM expires_at - now())::float8 FROM callbacks WHERE id = $1`, stateID,
	).Scan(&seconds)
	if err != nil {
		t.Fatalf("read expires_at: %v", err)
	}
	if ttl := CallbackStateTTL.Seconds(); seconds > ttl || seconds < ttl-5 {
		t.Errorf("expires in %.0fs, want about %.0fs", seconds, ttl)
	}
}

func TestRetrieveCallbackState_AcceptsOnce(t *testing.T) {
	requireTestDB(t)
	paymentID := fmt.Sprintf("cbonce-%d", time.Now().UnixNano())
	stateID := storeTestCallbackState(t, paymentID)
	ctx := context.Background()

	state, err := RetrieveCallbackState(ctx, stateID)
	if err != nil {
		t.Fatalf("first RetrieveCallbackState: %v", err)
	}
	if state.PaymentID != paymentID {
		t.Errorf("paymentID = %q, want %q", state.PaymentID, paymentID)
	}

	if _, err := RetrieveCallbackState(ctx, stateID); err == nil || !strings.Contains(err.Error(), "already used") {
		t.Errorf("second RetrieveCallbackState: err = %v, want already used", err)
	}
}

// The expiry is the database's: before this check moved into SQL, lib/pq read the zoneless
// expires_at as UTC and a callback was accepted hours after it expired.
func TestRetrieveCallbackState_RefusesExpired(t *testing.T) {
	requireTestDB(t)
	id := insertCallbackRow(t, "paycell", fmt.Sprintf("cbexpired-%d", time.Now().UnixNano()), false, "-1 second")
	ctx := context.Background()

	if _, err := RetrieveCallbackState(ctx, strconv.FormatInt(id, 10)); err == nil || !strings.Contains(err.Error(), "expired") {
		t.Fatalf("err = %v, want expired", err)
	}
	var used bool
	if err := config.App().DB.QueryRow(`SELECT used IS TRUE FROM callbacks WHERE id = $1`, id).Scan(&used); err != nil {
		t.Fatalf("read used: %v", err)
	}
	if used {
		t.Error("a refused callback marked the state used")
	}

	if _, err := RetrieveCallbackState(ctx, "2147483647"); err == nil || !strings.Contains(err.Error(), "not found") {
		t.Errorf("unknown id: err = %v, want not found", err)
	}
}

func TestRetrieveCallbackState_ConcurrentCallbacksAcceptOne(t *testing.T) {
	requireTestDB(t)
	stateID := storeTestCallbackState(t, fmt.Sprintf("cbrace-%d", time.Now().UnixNano()))

	const callers = 8
	var wg sync.WaitGroup
	var mu sync.Mutex
	accepted := 0
	for range callers {
		wg.Go(func() {
			if _, err := RetrieveCallbackState(context.Background(), stateID); err == nil {
				mu.Lock()
				accepted++
				mu.Unlock()
			}
		})
	}
	wg.Wait()

	if accepted != 1 {
		t.Errorf("%d of %d concurrent callbacks accepted, want 1", accepted, callers)
	}
}

func TestCleanupExpiredCallbackStates_KeepsRetention(t *testing.T) {
	requireTestDB(t)
	session := fmt.Sprintf("cbclean-%d", time.Now().UnixNano())
	beyond := fmt.Sprintf("-%d seconds", int(CallbackStateRetention.Seconds())+3600)

	kept := map[string]int64{
		"open":           insertCallbackRow(t, "paycell", session+"-open", false, "10 minutes"),
		"expired unused": insertCallbackRow(t, "paycell", session+"-unused", false, "-1 minute"),
		"expired used":   insertCallbackRow(t, "paycell", session+"-used", true, "-1 minute"),
	}
	gone := insertCallbackRow(t, "paycell", session+"-old", true, beyond)

	if err := CleanupExpiredCallbackStates(context.Background()); err != nil {
		t.Fatalf("CleanupExpiredCallbackStates: %v", err)
	}

	exists := func(id int64) bool {
		var ok bool
		if err := config.App().DB.QueryRow(`SELECT EXISTS (SELECT 1 FROM callbacks WHERE id = $1)`, id).Scan(&ok); err != nil {
			t.Fatalf("read callbacks row %d: %v", id, err)
		}
		return ok
	}
	for name, id := range kept {
		if !exists(id) {
			t.Errorf("%s row deleted inside the retention window", name)
		}
	}
	if exists(gone) {
		t.Error("row past the retention window kept")
	}
}

func TestCallbackStateOf(t *testing.T) {
	requireTestDB(t)
	session := fmt.Sprintf("cbphase-%d", time.Now().UnixNano())

	insertCallbackRow(t, "paycell", session+"-open", false, "10 minutes")
	insertCallbackRow(t, "paycell", session+"-closed", false, "-1 minute")
	insertCallbackRow(t, "paycell", session+"-accepted", true, "-1 minute")
	// A retried start leaves a second row; one accepted row decides.
	insertCallbackRow(t, "paycell", session+"-accepted", false, "10 minutes")
	insertCallbackRow(t, "nkolay", session+"-other", true, "10 minutes")

	ctx := context.Background()
	for _, tt := range []struct {
		provider, paymentID string
		want                CallbackPhase
	}{
		{"paycell", session + "-open", CallbackOpen},
		{"paycell", session + "-closed", CallbackClosed},
		{"paycell", session + "-accepted", CallbackAccepted},
		{"paycell", session + "-none", CallbackClosed},
		{"paycell", session + "-other", CallbackClosed},
	} {
		got, err := CallbackStateOf(ctx, tt.provider, tt.paymentID)
		if err != nil {
			t.Fatalf("CallbackStateOf(%s, %s): %v", tt.provider, tt.paymentID, err)
		}
		if got != tt.want {
			t.Errorf("CallbackStateOf(%s, %s) = %v, want %v", tt.provider, tt.paymentID, got, tt.want)
		}
	}
}
