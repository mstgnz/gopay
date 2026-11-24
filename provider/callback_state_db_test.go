package provider

import (
	"context"
	"errors"
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
		 VALUES (1, $1, $2::text, jsonb_build_object('paymentId', $2::text), now() + $3::interval, $4) RETURNING id`,
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

// callbackRowID finds the row a token was issued for.
func callbackRowID(t *testing.T, token string) int64 {
	t.Helper()
	var id int64
	if err := config.App().DB.QueryRow(`SELECT id FROM callbacks WHERE token_hash = $1`, callbackTokenHash(token)).Scan(&id); err != nil {
		t.Fatalf("callbacks row for token: %v", err)
	}
	return id
}

func storeTestCallbackState(t *testing.T, paymentID string) string {
	t.Helper()
	token, err := StoreCallbackState(context.Background(), CallbackState{
		TenantID: 1, Provider: "paycell", PaymentID: paymentID, Amount: 10, Environment: "sandbox",
	})
	if err != nil {
		t.Fatalf("StoreCallbackState: %v", err)
	}
	deleteCallbackRowOnCleanup(t, callbackRowID(t, token))
	return token
}

func TestStoreCallbackState_ExpiresOnDatabaseClock(t *testing.T) {
	requireTestDB(t)
	token := storeTestCallbackState(t, fmt.Sprintf("cbstore-%d", time.Now().UnixNano()))

	var seconds float64
	err := config.App().DB.QueryRow(
		`SELECT EXTRACT(EPOCH FROM expires_at - now())::float8 FROM callbacks WHERE id = $1`, callbackRowID(t, token),
	).Scan(&seconds)
	if err != nil {
		t.Fatalf("read expires_at: %v", err)
	}
	if ttl := CallbackStateTTL.Seconds(); seconds > ttl || seconds < ttl-5 {
		t.Errorf("expires in %.0fs, want about %.0fs", seconds, ttl)
	}
}

// The state in a callback URL is a random token; the row's numeric id must not open it, or recent
// ids could be walked to spend payers' 3D sessions and read their redirect pages.
func TestCallbackStateTokenCannotBeWalkedByID(t *testing.T) {
	requireTestDB(t)
	ctx := context.Background()
	token := storeTestCallbackState(t, fmt.Sprintf("cbtoken-%d", time.Now().UnixNano()))

	if !callbackTokenPattern.MatchString(token) {
		t.Fatalf("issued state %q is not a token", token)
	}
	id := callbackRowID(t, token)
	if _, err := RetrieveCallbackState(ctx, "paycell", strconv.FormatInt(id, 10)); err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("numeric id of a token row: err = %v, want not found", err)
	}
	var stored string
	if err := config.App().DB.QueryRow(`SELECT token_hash FROM callbacks WHERE id = $1`, id).Scan(&stored); err != nil ||
		stored == token || stored != callbackTokenHash(token) {
		t.Errorf("stored %q: want the token's hash, never the token (%v)", stored, err)
	}
	if _, err := RetrieveCallbackState(ctx, "paycell", token); err != nil {
		t.Fatalf("token after the id was refused: %v", err)
	}
}

// A 3D payment started by the previous release has a numeric state and no token; its callback must
// still be accepted after the deploy, once.
func TestRetrieveCallbackState_AcceptsLegacyNumericStateOnce(t *testing.T) {
	requireTestDB(t)
	ctx := context.Background()
	id := insertCallbackRow(t, "paycell", fmt.Sprintf("cblegacy-%d", time.Now().UnixNano()), false, "10 minutes")
	state := strconv.FormatInt(id, 10)

	if _, err := HandleCallbackState(ctx, "paycell", state); err != nil {
		t.Fatalf("legacy numeric state: %v", err)
	}
	if _, err := RetrieveCallbackState(ctx, "paycell", state); err == nil || !strings.Contains(err.Error(), "already used") {
		t.Errorf("second legacy callback: err = %v, want already used", err)
	}
}

// Nkolay writes its reference code into the state after storing it, addressing it by the URL's state.
func TestUpdateCallbackState_ByTokenAndLegacyID(t *testing.T) {
	requireTestDB(t)
	ctx := context.Background()
	token := storeTestCallbackState(t, fmt.Sprintf("cbupd-%d", time.Now().UnixNano()))
	legacy := insertCallbackRow(t, "nkolay", fmt.Sprintf("cbupd-legacy-%d", time.Now().UnixNano()), false, "10 minutes")

	for _, tc := range []struct {
		state string
		id    int64
	}{{token, callbackRowID(t, token)}, {strconv.FormatInt(legacy, 10), legacy}} {
		ref := fmt.Sprintf("REF-%d", tc.id)
		if err := UpdateCallbackState(ctx, tc.state, ref); err != nil {
			t.Fatalf("UpdateCallbackState: %v", err)
		}
		var paymentID, inState string
		if err := config.App().DB.QueryRow(`SELECT payment_id, state_data->>'paymentId' FROM callbacks WHERE id = $1`, tc.id).
			Scan(&paymentID, &inState); err != nil {
			t.Fatalf("read row: %v", err)
		}
		if paymentID != ref || inState != ref {
			t.Errorf("row %d: payment_id %q, state paymentId %q; want %q", tc.id, paymentID, inState, ref)
		}
	}
	if err := UpdateCallbackState(ctx, "12abc", "REF"); !errors.Is(err, ErrInvalidCallbackState) {
		t.Errorf("malformed state: err = %v", err)
	}
}

func TestRetrieveCallbackState_AcceptsOnce(t *testing.T) {
	requireTestDB(t)
	paymentID := fmt.Sprintf("cbonce-%d", time.Now().UnixNano())
	stateID := storeTestCallbackState(t, paymentID)
	ctx := context.Background()

	state, err := RetrieveCallbackState(ctx, "paycell", stateID)
	if err != nil {
		t.Fatalf("first RetrieveCallbackState: %v", err)
	}
	if state.PaymentID != paymentID {
		t.Errorf("paymentID = %q, want %q", state.PaymentID, paymentID)
	}

	if _, err := RetrieveCallbackState(ctx, "paycell", stateID); err == nil || !strings.Contains(err.Error(), "already used") {
		t.Errorf("second RetrieveCallbackState: err = %v, want already used", err)
	}
}

// The expiry is the database's: before this check moved into SQL, lib/pq read the zoneless
// expires_at as UTC and a callback was accepted hours after it expired.
func TestRetrieveCallbackState_RefusesExpired(t *testing.T) {
	requireTestDB(t)
	id := insertCallbackRow(t, "paycell", fmt.Sprintf("cbexpired-%d", time.Now().UnixNano()), false, "-1 second")
	ctx := context.Background()

	if _, err := RetrieveCallbackState(ctx, "paycell", strconv.FormatInt(id, 10)); err == nil || !strings.Contains(err.Error(), "expired") {
		t.Fatalf("err = %v, want expired", err)
	}
	var used bool
	if err := config.App().DB.QueryRow(`SELECT used IS TRUE FROM callbacks WHERE id = $1`, id).Scan(&used); err != nil {
		t.Fatalf("read used: %v", err)
	}
	if used {
		t.Error("a refused callback marked the state used")
	}

	if _, err := RetrieveCallbackState(ctx, "paycell", "2147483647"); err == nil || !strings.Contains(err.Error(), "not found") {
		t.Errorf("unknown id: err = %v, want not found", err)
	}
}

// A state completed through another provider's callback route must be refused without spending it,
// so the payer's real callback still completes it.
func TestRetrieveCallbackState_RefusesAnotherProvider(t *testing.T) {
	requireTestDB(t)
	stateID := storeTestCallbackState(t, fmt.Sprintf("cbprovider-%d", time.Now().UnixNano()))
	ctx := context.Background()

	if _, err := RetrieveCallbackState(ctx, "ziraat", stateID); err == nil || !strings.Contains(err.Error(), "another provider") {
		t.Fatalf("other provider: err = %v, want another provider", err)
	}
	if _, err := RetrieveCallbackState(ctx, "paycell", stateID); err != nil {
		t.Errorf("own provider after a refused attempt: %v", err)
	}

	// The provider check comes first, so a wrong route learns nothing about used or expired rows.
	spent := insertCallbackRow(t, "paycell", fmt.Sprintf("cbprovider-spent-%d", time.Now().UnixNano()), true, "-1 minute")
	if _, err := RetrieveCallbackState(ctx, "ziraat", strconv.FormatInt(spent, 10)); err == nil || !strings.Contains(err.Error(), "another provider") {
		t.Errorf("spent row on another route: err = %v, want another provider", err)
	}
}

// The service passes the route's provider, not the state, as the provider: two adjacent string
// parameters that would still compile if swapped.
func TestComplete3DPayment_RefusesStateOfAnotherProvider(t *testing.T) {
	requireTestDB(t)
	stateID := storeTestCallbackState(t, fmt.Sprintf("cbservice-%d", time.Now().UnixNano()))

	_, err := NewPaymentService(nil, nil).Complete3DPayment(context.Background(), "ziraat", stateID, map[string]string{})
	if err == nil || !strings.Contains(err.Error(), "another provider") {
		t.Errorf("err = %v, want the state refused as another provider's", err)
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
			if _, err := RetrieveCallbackState(context.Background(), "paycell", stateID); err == nil {
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
