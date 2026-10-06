package handler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/mstgnz/gopay/infra/auth"
)

// Documentation addresses (RFC 5737): the consumer's server and an attacker.
const (
	consumerIP = "192.0.2.10"
	attackerIP = "198.51.100.7"
)

type fakeClock struct{ t time.Time }

func (c *fakeClock) now() time.Time { return c.t }

func newTestThrottle() (*loginThrottle, *fakeClock) {
	clock := &fakeClock{t: time.Date(2026, 10, 6, 12, 0, 0, 0, time.UTC)}
	th := newLoginThrottle()
	th.now = clock.now
	return th, clock
}

// failLogin runs one attempt that ends in a wrong password.
func failLogin(t *testing.T, th *loginThrottle, username, ip string) {
	t.Helper()
	attempt, _ := th.begin(username, ip)
	if attempt == nil {
		t.Fatalf("attempt for %q from %s was refused", username, ip)
	}
	attempt.failed()
	attempt.finish()
}

// succeedLogin runs one attempt that ends in a correct password.
func succeedLogin(t *testing.T, th *loginThrottle, username, ip string) {
	t.Helper()
	attempt, _ := th.begin(username, ip)
	if attempt == nil {
		t.Fatalf("attempt for %q from %s was refused", username, ip)
	}
	attempt.succeeded()
	attempt.finish()
}

// refused reports whether a new attempt would be refused. An accepted probe is finished as
// abandoned, which counts nothing.
func refused(th *loginThrottle, username, ip string) (bool, time.Duration) {
	attempt, retryAfter := th.begin(username, ip)
	if attempt == nil {
		return true, retryAfter
	}
	attempt.finish()
	return false, 0
}

func windowCount(th *loginThrottle) int {
	th.mu.Lock()
	defer th.mu.Unlock()
	return len(th.windows)
}

func TestLoginThrottle_BlocksAfterMaxFailures(t *testing.T) {
	th, _ := newTestThrottle()

	for i := 0; i < maxLoginFailures-1; i++ {
		failLogin(t, th, "tenant-a", attackerIP)
	}
	if blocked, _ := refused(th, "tenant-a", attackerIP); blocked {
		t.Fatal("blocked before reaching the limit")
	}

	failLogin(t, th, "tenant-a", attackerIP)
	blocked, retryAfter := refused(th, "tenant-a", attackerIP)
	if !blocked {
		t.Fatal("not blocked at the limit")
	}
	if retryAfter <= 0 || retryAfter > loginFailureWindow {
		t.Fatalf("retryAfter = %v, want within the window", retryAfter)
	}
}

func TestLoginThrottle_KeyIgnoresCaseAndSpace(t *testing.T) {
	th, _ := newTestThrottle()
	for i := 0; i < maxLoginFailures; i++ {
		failLogin(t, th, fmt.Sprintf(" Tenant-A%s", []string{"", " "}[i%2]), attackerIP)
	}
	if blocked, _ := refused(th, "tenant-a", attackerIP); !blocked {
		t.Fatal("variants of the same username were counted separately")
	}
}

func TestLoginThrottle_WindowExpires(t *testing.T) {
	th, clock := newTestThrottle()
	for i := 0; i < maxLoginFailures; i++ {
		failLogin(t, th, "tenant-b", attackerIP)
	}
	clock.t = clock.t.Add(loginFailureWindow)

	if blocked, _ := refused(th, "tenant-b", attackerIP); blocked {
		t.Fatal("still blocked after the window elapsed")
	}
	failLogin(t, th, "tenant-b", attackerIP)
	if blocked, _ := refused(th, "tenant-b", attackerIP); blocked {
		t.Fatal("a fresh window started at the old count")
	}
}

// Consumers log in on every call, so a success must clear earlier mistakes.
func TestLoginThrottle_ResetOnSuccess(t *testing.T) {
	th, _ := newTestThrottle()
	for i := 0; i < maxLoginFailures-1; i++ {
		failLogin(t, th, "tenant-c", consumerIP)
	}
	succeedLogin(t, th, "tenant-c", consumerIP)
	failLogin(t, th, "tenant-c", consumerIP)
	if blocked, _ := refused(th, "tenant-c", consumerIP); blocked {
		t.Fatal("success did not reset the counter")
	}
}

func TestLoginThrottle_OtherUsernamesUnaffected(t *testing.T) {
	th, _ := newTestThrottle()
	for i := 0; i < maxLoginFailures; i++ {
		failLogin(t, th, "attacked", attackerIP)
	}
	if blocked, _ := refused(th, "tenant-a", attackerIP); blocked {
		t.Fatal("throttling one username blocked another")
	}
}

// Someone who knows the username must not be able to lock the consumer out of payments, and
// the consumer's next successful login must not clear the attacker's count.
func TestLoginThrottle_AttackerCannotLockOutConsumer(t *testing.T) {
	th, _ := newTestThrottle()
	for i := 0; i < maxLoginFailures; i++ {
		failLogin(t, th, "tenant-a", attackerIP)
	}

	if blocked, _ := refused(th, "tenant-a", consumerIP); blocked {
		t.Fatal("failures from the attacker's IP blocked the consumer's IP")
	}
	succeedLogin(t, th, "tenant-a", consumerIP)
	if blocked, _ := refused(th, "tenant-a", attackerIP); !blocked {
		t.Fatal("the consumer's success cleared the attacker's block")
	}
}

func TestLoginThrottle_PrunesExpiredEntries(t *testing.T) {
	th, clock := newTestThrottle()
	for i := 0; i < 1000; i++ {
		failLogin(t, th, fmt.Sprintf("spray-%d", i), attackerIP)
	}

	// A sweep just before the spray expires removes nothing, and the next one is not due yet:
	// the expired spray stays until the interval has passed, so new keys never pay for a scan.
	clock.t = clock.t.Add(loginFailureWindow - loginThrottlePruneEvery/4)
	failLogin(t, th, "before-expiry", attackerIP)
	clock.t = clock.t.Add(loginThrottlePruneEvery / 2)
	failLogin(t, th, "inside-interval", attackerIP)
	if n := windowCount(th); n != 1002 {
		t.Fatalf("map size = %d inside the prune interval, want 1002 (swept too early)", n)
	}

	clock.t = clock.t.Add(loginThrottlePruneEvery)
	failLogin(t, th, "after-interval", attackerIP)
	if n := windowCount(th); n != 3 {
		t.Fatalf("map size = %d after the interval, want 3 (expired spray swept)", n)
	}
}

// A spray of one-failure junk keys must neither grow the map past its cap nor push out a key
// that is already blocked.
func TestLoginThrottle_CapKeepsBlockedKey(t *testing.T) {
	th, _ := newTestThrottle()
	th.maxKeys = loginThrottleEvictionSample
	for range maxLoginFailures {
		failLogin(t, th, "tenant-a", attackerIP)
	}

	for i := range 1000 {
		failLogin(t, th, fmt.Sprintf("junk-%d", i), attackerIP)
	}

	if n := windowCount(th); n > th.maxKeys {
		t.Fatalf("map size = %d, cap is %d", n, th.maxKeys)
	}
	if blocked, _ := refused(th, "tenant-a", attackerIP); !blocked {
		t.Fatal("the spray evicted the blocked key")
	}
}

// When every entry has an attempt in bcrypt there is nothing to evict; a new key must still get
// through rather than be refused, since it may be a consumer's.
func TestLoginThrottle_FullMapNeverRefusesNewKey(t *testing.T) {
	th, _ := newTestThrottle()
	th.maxKeys = 2
	held := []*loginAttempt{}
	for _, user := range []string{"a-user", "b-user"} {
		attempt, _ := th.begin(user, attackerIP)
		if attempt == nil {
			t.Fatalf("setup attempt for %s refused", user)
		}
		held = append(held, attempt)
	}

	attempt, _ := th.begin("tenant-a", consumerIP)
	if attempt == nil {
		t.Fatal("a new key was refused because the map was full")
	}
	attempt.succeeded()
	attempt.finish()
	for _, h := range held {
		h.finish()
	}
	if n := windowCount(th); n != 0 {
		t.Fatalf("%d entries left, want 0", n)
	}
}

// Check and reservation happen under one lock, so a parallel burst gets at most
// maxLoginInFlight attempts in, however many requests arrive before any finishes.
func TestLoginThrottle_ParallelBurstIsBounded(t *testing.T) {
	th, _ := newTestThrottle()
	const burst = 500

	var (
		wg       sync.WaitGroup
		mu       sync.Mutex
		accepted []*loginAttempt
	)
	for range burst {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if attempt, _ := th.begin("tenant-a", attackerIP); attempt != nil {
				mu.Lock()
				accepted = append(accepted, attempt)
				mu.Unlock()
			}
		}()
	}
	wg.Wait()

	if len(accepted) != maxLoginInFlight {
		t.Fatalf("%d of %d parallel attempts got in, want %d", len(accepted), burst, maxLoginInFlight)
	}
	for _, attempt := range accepted {
		attempt.failed()
		attempt.finish()
	}
	if blocked, _ := refused(th, "tenant-a", attackerIP); !blocked {
		t.Fatal("not blocked after the burst failed")
	}
}

// A consumer's concurrent logins are all correct; more of them than maxLoginFailures in flight
// at once must still get through, and leave nothing behind once they succeed.
func TestLoginThrottle_ConcurrentSuccessesAboveFailureLimit(t *testing.T) {
	th, _ := newTestThrottle()
	var inFlight []*loginAttempt
	for range maxLoginFailures * 2 {
		attempt, _ := th.begin("tenant-a", consumerIP)
		if attempt == nil {
			t.Fatalf("concurrent login %d refused", len(inFlight)+1)
		}
		inFlight = append(inFlight, attempt)
	}
	for _, attempt := range inFlight {
		attempt.succeeded()
		attempt.finish()
	}
	if n := windowCount(th); n != 0 {
		t.Fatalf("%d entries left after every login succeeded, want 0", n)
	}
}

// A database error says nothing about the password, so it must not lock the consumer out.
func TestLoginThrottle_AbandonedAttemptsDoNotCount(t *testing.T) {
	th, _ := newTestThrottle()
	for range maxLoginFailures * 2 {
		attempt, _ := th.begin("tenant-b", consumerIP)
		if attempt == nil {
			t.Fatal("abandoned attempts were counted as failures")
		}
		attempt.finish()
	}
	if n := windowCount(th); n != 0 {
		t.Fatalf("%d entries left after abandoned attempts, want 0", n)
	}
}

// A zero TenantService panics on its nil database inside Login; the deferred finish must still
// release the reservation, or every panic would eat one slot until the key locks up.
func TestAuthHandler_Login_PanicReleasesReservation(t *testing.T) {
	h := NewAuthHandler(&auth.TenantService{}, &auth.JWTService{}, validator.New())
	body, _ := json.Marshal(LoginRequest{Username: "tenant-a", Password: "whatever1"})
	req := httptest.NewRequest(http.MethodPost, "/v1/auth/login", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Real-IP", consumerIP)

	func() {
		defer func() {
			if recover() == nil {
				t.Fatal("expected Login to panic on the nil database")
			}
		}()
		h.Login(httptest.NewRecorder(), req)
	}()

	if n := windowCount(h.throttle); n != 0 {
		t.Fatalf("%d reservations leaked after a panic, want 0", n)
	}
}

// The throttled path must answer before the tenant service is reached: a zero TenantService
// has no database, so reaching Login would panic. The IP comes from X-Real-IP, not from a
// True-Client-IP the caller can set.
func TestAuthHandler_Login_ThrottledReturns429(t *testing.T) {
	h := NewAuthHandler(&auth.TenantService{}, &auth.JWTService{}, validator.New())
	for i := 0; i < maxLoginFailures; i++ {
		failLogin(t, h.throttle, "tenant-a", attackerIP)
	}

	body, _ := json.Marshal(LoginRequest{Username: "tenant-a", Password: "whatever1"})
	req := httptest.NewRequest(http.MethodPost, "/v1/auth/login", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Real-IP", attackerIP)
	req.Header.Set("True-Client-IP", consumerIP)
	rr := httptest.NewRecorder()
	h.Login(rr, req)

	if rr.Code != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want 429", rr.Code)
	}
	if rr.Header().Get("Retry-After") == "" {
		t.Fatal("Retry-After header missing")
	}
}
