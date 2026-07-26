package workflow_test

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/thoriqakbar0/garden/internal/workflow"
)

func TestSequentialTurnsPersistAndReplay(t *testing.T) {
	store := open(t, workflow.EchoResponder)
	sessionID, err := store.CreateSession()
	if err != nil {
		t.Fatal(err)
	}
	for turn := 1; turn <= 100; turn++ {
		message := fmt.Sprintf("sequential-turn-%03d", turn)
		result, err := store.Send(context.Background(), sessionID, message)
		if err != nil {
			t.Fatal(err)
		}
		want := fmt.Sprintf("stress-ack:%d:%s", turn, message)
		if result.Message != want {
			t.Fatalf("turn %d message = %q, want %q", turn, result.Message, want)
		}
	}
	events, err := store.Replay(sessionID, 250)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 51 || events[0].Index != 250 {
		t.Fatalf("replayed %d events beginning at %#v", len(events), events[0])
	}
}

func TestConcurrentSessionsRemainIsolated(t *testing.T) {
	store := open(t, workflow.EchoResponder)
	const count = 50
	var wait sync.WaitGroup
	errs := make(chan error, count)
	ids := make(chan string, count)
	for index := range count {
		wait.Add(1)
		go func() {
			defer wait.Done()
			id, err := store.CreateSession()
			if err != nil {
				errs <- err
				return
			}
			result, err := store.Send(context.Background(), id, fmt.Sprintf("session-%02d", index))
			if err != nil {
				errs <- err
				return
			}
			ids <- result.SessionID
		}()
	}
	wait.Wait()
	close(errs)
	close(ids)
	for err := range errs {
		t.Error(err)
	}
	unique := map[string]struct{}{}
	for id := range ids {
		unique[id] = struct{}{}
	}
	if len(unique) != count {
		t.Fatalf("unique sessions = %d, want %d", len(unique), count)
	}
}

func TestCancellationRejectsStaleTurnAndCancelsActiveTurn(t *testing.T) {
	started := make(chan struct{})
	store := open(t, func(ctx context.Context, _ string, _ []workflow.Event) (string, error) {
		close(started)
		<-ctx.Done()
		return "", ctx.Err()
	})
	id, err := store.CreateSession()
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() {
		_, err := store.Send(context.Background(), id, "wait")
		done <- err
	}()
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("turn did not start")
	}
	if got := store.Cancel(id, "stale"); got != workflow.CancelNoActiveTurn {
		t.Fatalf("stale cancellation = %q", got)
	}
	if got := store.Cancel(id, ""); got != workflow.CancelAccepted {
		t.Fatalf("cancellation = %q", got)
	}
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Fatalf("send error = %v", err)
	}
	events, err := store.Replay(id, 0)
	if err != nil {
		t.Fatal(err)
	}
	if events[len(events)-1].Type != "turn.cancelled" {
		t.Fatalf("last event = %q", events[len(events)-1].Type)
	}
}

func open(t *testing.T, responder workflow.Responder) *workflow.Store {
	t.Helper()
	store, err := workflow.Open(t.TempDir(), responder)
	if err != nil {
		t.Fatal(err)
	}
	return store
}
