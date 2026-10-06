package handler

import (
	"strings"
	"sync"
	"time"
)

const (
	// Failed logins allowed per username inside loginFailureWindow before login is refused.
	// Successful logins are never counted, so consumers that log in on every call are unaffected.
	maxLoginFailures   = 10
	loginFailureWindow = 15 * time.Minute
	// Attempts still inside bcrypt count toward this ceiling together with the failures, so a
	// parallel burst cannot run past maxLoginFailures unchecked. It sits far above the consumers'
	// measured peak (4 logins/s across all of them), so concurrent successful
	// logins are not refused even when the database is slow.
	maxLoginInFlight = 50
	// Hard cap on tracked keys: 179 bytes per entry measured with an IPv6 address and a
	// 50-character username, so about 18 MB per replica at the cap.
	loginThrottleMaxKeys = 100_000
	// At the cap, this many random entries are sampled and the one with the fewest failures is
	// evicted, so a spray of one-failure junk keys cannot push out a blocked key.
	loginThrottleEvictionSample = 64
	// Expired idle entries are swept at most this often, never on every new key: a full scan
	// runs under the lock every login waits on.
	loginThrottlePruneEvery = time.Minute
)

type failureWindow struct {
	failures int
	inFlight int
	started  time.Time
}

type attemptResult int

const (
	// attemptAbandoned covers anything that says nothing about the password: a database
	// error or a panic. It releases the reservation without counting a failure.
	attemptAbandoned attemptResult = iota
	attemptFailed
	attemptSucceeded
)

// loginAttempt is a reservation taken before the password is checked. finish must run exactly
// once, deferred, so a panic cannot leak the reservation. A nil throttle marks an attempt let
// through untracked because the map was full.
type loginAttempt struct {
	throttle *loginThrottle
	key      string
	result   attemptResult
}

func (a *loginAttempt) failed()    { a.result = attemptFailed }
func (a *loginAttempt) succeeded() { a.result = attemptSucceeded }

func (a *loginAttempt) finish() {
	if a.throttle != nil {
		a.throttle.finish(a.key, a.result)
	}
}

// loginThrottle counts failed logins per username and client IP. Keying on the username alone
// let anyone who knew it lock out a consumer that logs in before every payment call.
// It is per process: with N replicas an attacker gets at most N times maxLoginInFlight guesses
// per window from one IP (maxLoginFailures when sent one at a time), which still ends online
// guessing.
type loginThrottle struct {
	mu        sync.Mutex
	windows   map[string]*failureWindow
	maxKeys   int
	lastPrune time.Time
	now       func() time.Time
}

func newLoginThrottle() *loginThrottle {
	return &loginThrottle{
		windows: make(map[string]*failureWindow),
		maxKeys: loginThrottleMaxKeys,
		now:     time.Now,
	}
}

// throttleKey puts the IP first: it never contains "|", so no username can forge another pair.
func throttleKey(username, clientIP string) string {
	return clientIP + "|" + strings.ToLower(strings.TrimSpace(username))
}

// begin reserves an attempt for the username from clientIP, checking and reserving under one
// lock. It returns nil and how long to wait when the attempt must be refused.
func (t *loginThrottle) begin(username, clientIP string) (*loginAttempt, time.Duration) {
	t.mu.Lock()
	defer t.mu.Unlock()

	now := t.now()
	t.maybePruneLocked(now)

	key := throttleKey(username, clientIP)
	w, ok := t.windows[key]
	if ok {
		expireLocked(w, now)
	} else {
		if len(t.windows) >= t.maxKeys && !t.evictLocked() {
			// Every sampled entry has an attempt in bcrypt. Let this one through untracked
			// rather than refuse a login that may be a consumer's.
			return &loginAttempt{}, 0
		}
		w = &failureWindow{started: now}
		t.windows[key] = w
	}
	if w.failures >= maxLoginFailures {
		return nil, loginFailureWindow - now.Sub(w.started)
	}
	if w.failures+w.inFlight >= maxLoginInFlight {
		// Released as soon as the attempts in bcrypt finish, so the wait is short.
		return nil, time.Second
	}
	w.inFlight++
	return &loginAttempt{throttle: t, key: key}, 0
}

func (t *loginThrottle) finish(key string, result attemptResult) {
	t.mu.Lock()
	defer t.mu.Unlock()

	// Eviction and pruning skip entries with attempts in flight, so a tracked key is still here.
	w, ok := t.windows[key]
	if !ok {
		return
	}
	expireLocked(w, t.now())
	if w.inFlight > 0 {
		w.inFlight--
	}
	switch result {
	case attemptFailed:
		w.failures++
	case attemptSucceeded:
		w.failures = 0
	}
	if w.failures == 0 && w.inFlight == 0 {
		delete(t.windows, key)
	}
}

// expireLocked starts a fresh window once the old one has run out. Attempts still in flight
// carry over into it.
func expireLocked(w *failureWindow, now time.Time) {
	if now.Sub(w.started) >= loginFailureWindow {
		w.failures = 0
		w.started = now
	}
}

// evictLocked drops the sampled entry with the fewest failures, the oldest on a tie. Entries
// with attempts in flight are skipped: their finish still has to find them.
func (t *loginThrottle) evictLocked() bool {
	victim, sampled := "", 0
	var victimWindow *failureWindow
	for key, w := range t.windows {
		if sampled == loginThrottleEvictionSample {
			break
		}
		sampled++
		if w.inFlight > 0 {
			continue
		}
		if victimWindow == nil || w.failures < victimWindow.failures ||
			(w.failures == victimWindow.failures && w.started.Before(victimWindow.started)) {
			victim, victimWindow = key, w
		}
	}
	if victimWindow == nil {
		return false
	}
	delete(t.windows, victim)
	return true
}

func (t *loginThrottle) maybePruneLocked(now time.Time) {
	if now.Sub(t.lastPrune) < loginThrottlePruneEvery {
		return
	}
	t.lastPrune = now
	for key, w := range t.windows {
		if w.inFlight == 0 && now.Sub(w.started) >= loginFailureWindow {
			delete(t.windows, key)
		}
	}
}
