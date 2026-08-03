package workflow

import (
	"context"
	"errors"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestAcceptedCancelRejectsEmitAlreadyWaitingToAppend(t *testing.T) {
	emitReady := make(chan Emit, 1)
	releaseRunner := make(chan struct{})
	runner := RunnerFunc(func(ctx context.Context, _ Turn, emit Emit) (string, error) {
		emitReady <- emit
		<-releaseRunner
		return "", ctx.Err()
	})
	store, err := OpenRunner(t.TempDir(), runner)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		close(releaseRunner)
		_ = store.Close()
	})
	started, err := store.StartSession("cancel while emitting")
	if err != nil {
		t.Fatal(err)
	}
	emit := <-emitReady
	state, ok := store.existingSession(started.SessionID)
	if !ok {
		t.Fatal("started session is missing")
	}

	state.mu.Lock()
	locked := true
	defer func() {
		if locked {
			state.mu.Unlock()
		}
	}()

	type cancelOutcome struct {
		result CancelResult
		err    error
	}
	cancelled := make(chan cancelOutcome, 1)
	go func() {
		result, err := store.Cancel(context.Background(), started.SessionID, started.TurnID)
		cancelled <- cancelOutcome{result: result, err: err}
	}()
	waitForBlockedStack(t, "internal/workflow.(*Store).Cancel", "sync.(*Mutex).Lock")

	emitted := make(chan error, 1)
	go func() {
		emitted <- emit("message.completed", map[string]any{
			"finishReason": "stop",
			"message":      "must not persist",
			"sequence":     0,
			"stepIndex":    0,
			"turnId":       started.TurnID,
		})
	}()
	waitForBlockedStack(t, "internal/workflow.(*Store).execute.func1", "sync.(*Mutex).Lock")

	state.mu.Unlock()
	locked = false
	outcome := <-cancelled
	if outcome.err != nil || outcome.result != CancelAccepted {
		t.Fatalf("cancel = %q, %v", outcome.result, outcome.err)
	}
	if err := <-emitted; !errors.Is(err, context.Canceled) {
		t.Fatalf("emit after accepted cancellation = %v, want context.Canceled", err)
	}

	events, err := store.Replay(started.SessionID, 0)
	if err != nil {
		t.Fatal(err)
	}
	for _, event := range events {
		if event.Type == "message.completed" {
			t.Fatalf("runner emit appended after durable cancellation intent: %#v", event)
		}
	}
}

func waitForBlockedStack(t *testing.T, markers ...string) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		buffer := make([]byte, 1<<20)
		length := runtime.Stack(buffer, true)
		for _, stack := range strings.Split(string(buffer[:length]), "\n\n") {
			matched := true
			for _, marker := range markers {
				matched = matched && strings.Contains(stack, marker)
			}
			if matched {
				return
			}
		}
		runtime.Gosched()
	}
	t.Fatalf("no blocked goroutine contained stack markers %q", markers)
}
