package provider

import (
	"context"
	"testing"
	"time"
)

func TestWaitForBackgroundTasksReturnsWhenIdle(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	if !WaitForBackgroundTasks(ctx) {
		t.Error("WaitForBackgroundTasks reported unfinished work with nothing running")
	}
}

// A wait that gives up must leave nothing behind for the next task. This is the shape that made
// the sync.WaitGroup version race: the abandoned Wait collided with the following Add.
func TestWaitForBackgroundTasksSurvivesATimedOutWait(t *testing.T) {
	for range 3 {
		release := make(chan struct{})

		BackgroundTaskStarted()
		go func() {
			defer BackgroundTaskDone()
			<-release
		}()

		shortCtx, cancelShort := context.WithTimeout(context.Background(), 20*time.Millisecond)
		finished := WaitForBackgroundTasks(shortCtx)
		cancelShort()

		if finished {
			t.Fatal("WaitForBackgroundTasks returned true while a task was still running")
		}

		close(release)

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		finished = WaitForBackgroundTasks(ctx)
		cancel()

		if !finished {
			t.Fatal("a timed out wait left the tracker unusable for the next task")
		}
	}
}

func TestBackgroundTaskDonePanicsWhenUnpaired(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("BackgroundTaskDone without a matching Started should panic")
		}
	}()

	BackgroundTaskDone()
}

func TestWaitForBackgroundTasksWaitsForRunningWork(t *testing.T) {
	release := make(chan struct{})
	finished := make(chan struct{})

	BackgroundTaskStarted()
	go func() {
		defer BackgroundTaskDone()
		defer close(finished)
		<-release
	}()

	// While the task runs, a wait with a short budget must report that it did not finish.
	shortCtx, cancelShort := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancelShort()

	if WaitForBackgroundTasks(shortCtx) {
		t.Error("WaitForBackgroundTasks returned true while a task was still running")
	}

	close(release)
	<-finished

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if !WaitForBackgroundTasks(ctx) {
		t.Error("WaitForBackgroundTasks did not return after the task finished")
	}
}
