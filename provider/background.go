package provider

import (
	"context"
	"sync"
)

// Tracking for work that outlives the request that started it: today the Paycell provision
// compensation. Shutdown waits for it, because a deploy that kills such a task kills exactly the
// compensation the deploy itself caused.
//
// This is deliberately not a sync.WaitGroup. A Wait that gives up on a timeout leaves its
// goroutine parked on the group, and the next Add then races with it, which the race detector
// reports and the sync docs forbid. A counter plus a channel closed when it reaches zero has no
// such rule: a waiter that times out simply stops selecting.
var (
	backgroundMu    sync.Mutex
	backgroundCount int
	backgroundIdle  chan struct{} // non-nil only while tasks are running; closed when the last finishes
)

// BackgroundTaskStarted must be called before the goroutine is started, not inside it, so the
// count is already up when shutdown begins to wait.
func BackgroundTaskStarted() {
	backgroundMu.Lock()
	defer backgroundMu.Unlock()

	if backgroundCount == 0 {
		backgroundIdle = make(chan struct{})
	}
	backgroundCount++
}

// BackgroundTaskDone marks one task finished. Pair it with a defer.
func BackgroundTaskDone() {
	backgroundMu.Lock()
	defer backgroundMu.Unlock()

	if backgroundCount == 0 {
		panic("provider: BackgroundTaskDone called more times than BackgroundTaskStarted")
	}

	backgroundCount--
	if backgroundCount == 0 {
		close(backgroundIdle)
		backgroundIdle = nil
	}
}

// WaitForBackgroundTasks blocks until every task finishes or ctx expires, and reports whether
// they all finished.
func WaitForBackgroundTasks(ctx context.Context) bool {
	backgroundMu.Lock()
	idle := backgroundIdle
	running := backgroundCount > 0
	backgroundMu.Unlock()

	if !running {
		return true
	}

	select {
	case <-idle:
		return true
	case <-ctx.Done():
		return false
	}
}
