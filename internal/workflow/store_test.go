package workflow_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
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
	if len(events) != 551 || events[0].Index != 250 {
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

func TestCancellationConsumesStaleGuardWithoutCancellingActiveTurn(t *testing.T) {
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
	if got, err := store.Cancel(context.Background(), id, "stale"); err != nil || got != workflow.CancelAccepted {
		t.Fatalf("stale cancellation = %q", got)
	}
	select {
	case err := <-done:
		t.Fatalf("stale guard cancelled active turn: %v", err)
	default:
	}
	if got, err := store.Cancel(context.Background(), id, ""); err != nil || got != workflow.CancelAccepted {
		t.Fatalf("cancellation = %q", got)
	}
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Fatalf("send error = %v", err)
	}
	events, err := store.Replay(id, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) < 2 ||
		events[len(events)-2].Type != "turn.cancelled" ||
		events[len(events)-1].Type != "session.waiting" {
		t.Fatalf("last events = %#v", events[len(events)-2:])
	}
}

func TestSessionIDCannotEscapeStore(t *testing.T) {
	store := open(t, workflow.EchoResponder)
	for _, id := range []string{"../../outside", "ses_ok/../../outside", "not-a-session"} {
		if _, err := store.Replay(id, 0); !errors.Is(err, workflow.ErrInvalidSessionID) {
			t.Fatalf("Replay(%q) error = %v", id, err)
		}
	}
}

func TestContinuationTokenSelectsOwnerAndCreatesUnownedSession(t *testing.T) {
	store := open(t, workflow.EchoResponder)
	first, err := store.StartSession("first")
	if err != nil {
		t.Fatal(err)
	}
	waitForBoundary(t, store, first.SessionID)

	owned, err := store.Continue("ses_missing", first.ContinuationToken, "owned")
	if err != nil {
		t.Fatal(err)
	}
	if owned.SessionID != first.SessionID {
		t.Fatalf("owned token selected %q, want %q", owned.SessionID, first.SessionID)
	}
	waitForBoundary(t, store, owned.SessionID)

	unownedToken := "eve:00000000-0000-0000-0000-000000000099"
	unowned, err := store.Continue("ses_missing", unownedToken, "unowned")
	if err != nil {
		t.Fatal(err)
	}
	if unowned.SessionID == first.SessionID || unowned.ContinuationToken != unownedToken {
		t.Fatalf("unowned token result = %#v", unowned)
	}
	if _, err := store.Continue(first.SessionID, "not-an-eve-token", "invalid"); !errors.Is(err, workflow.ErrInvalidContinuation) {
		t.Fatalf("invalid token error = %v", err)
	}
}

func TestReplaySupportsTailRelativeCursor(t *testing.T) {
	store := open(t, workflow.EchoResponder)
	id, err := store.CreateSession()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Send(context.Background(), id, "hello"); err != nil {
		t.Fatal(err)
	}
	events, err := store.Replay(id, -1)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || events[0].Type != "session.waiting" {
		t.Fatalf("tail replay = %#v", events)
	}
}

func TestStoreAllowsOnlyOneWriter(t *testing.T) {
	root := t.TempDir()
	first, err := workflow.Open(root, workflow.EchoResponder)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := workflow.Open(root, workflow.EchoResponder); !errors.Is(err, workflow.ErrStoreInUse) {
		t.Fatalf("second writer error = %v", err)
	}
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}
	third, err := workflow.Open(root, workflow.EchoResponder)
	if err != nil {
		t.Fatalf("open after close: %v", err)
	}
	if err := third.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestCloseWakesEventWaiters(t *testing.T) {
	store, err := workflow.Open(t.TempDir(), workflow.EchoResponder)
	if err != nil {
		t.Fatal(err)
	}
	id, err := store.CreateSession()
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() {
		_, err := store.WaitForEvents(context.Background(), id, 1)
		done <- err
	}()
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		if !errors.Is(err, workflow.ErrStoreClosed) {
			t.Fatalf("wait error = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("event waiter remained blocked after Close")
	}
}

func TestOpenSessionRootCannotBeRedirectedAfterStartup(t *testing.T) {
	root := t.TempDir()
	store, err := workflow.Open(root, workflow.EchoResponder)
	if err != nil {
		t.Fatal(err)
	}
	id, err := store.CreateSession()
	if err != nil {
		t.Fatal(err)
	}
	original := filepath.Join(root, "sessions")
	anchored := filepath.Join(root, "sessions-anchored")
	if err := os.Rename(original, anchored); err != nil {
		t.Fatal(err)
	}
	outside := t.TempDir()
	if err := os.Symlink(outside, original); err != nil {
		t.Fatal(err)
	}
	result, err := store.Send(context.Background(), id, "anchored")
	if err != nil {
		t.Fatal(err)
	}
	if result.Message != "stress-ack:1:anchored" {
		t.Fatalf("message = %q", result.Message)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	outsideEntries, err := os.ReadDir(outside)
	if err != nil {
		t.Fatal(err)
	}
	if len(outsideEntries) != 0 {
		t.Fatalf("redirected directory received files: %v", outsideEntries)
	}
	if _, err := os.Stat(filepath.Join(anchored, id+".jsonl")); err != nil {
		t.Fatalf("anchored session log missing: %v", err)
	}
}

func TestConcurrentUnownedContinuationHasOneOwner(t *testing.T) {
	root := t.TempDir()
	store, err := workflow.Open(root, workflow.EchoResponder)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })

	const count = 24
	token := "eve:00000000-0000-0000-0000-000000000024"
	start := make(chan struct{})
	results := make(chan workflow.StartResult, count)
	errs := make(chan error, count)
	var wait sync.WaitGroup
	for index := range count {
		wait.Add(1)
		go func() {
			defer wait.Done()
			<-start
			result, err := store.Continue("ses_missing", token, fmt.Sprintf("message-%d", index))
			if err != nil {
				errs <- err
				return
			}
			results <- result
		}()
	}
	close(start)
	wait.Wait()
	close(results)
	close(errs)

	owner := ""
	for result := range results {
		if owner == "" {
			owner = result.SessionID
		}
		if result.SessionID != owner {
			t.Fatalf("token owners include %q and %q", owner, result.SessionID)
		}
	}
	if owner == "" {
		t.Fatal("no continuation request succeeded")
	}
	for err := range errs {
		if !errors.Is(err, workflow.ErrSessionBusy) {
			t.Errorf("continuation error = %v", err)
		}
	}
	entries, err := os.ReadDir(filepath.Join(root, "sessions"))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("session log count = %d, want 1", len(entries))
	}
}

func TestAcceptedCancellationWinsAgainstLateRunnerSuccess(t *testing.T) {
	started := make(chan struct{})
	runner := workflow.RunnerFunc(func(ctx context.Context, _ workflow.Turn, _ workflow.Emit) (string, error) {
		close(started)
		<-ctx.Done()
		return "late success", nil
	})
	store, err := workflow.OpenRunner(t.TempDir(), runner)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	result, err := store.StartSession("cancel me")
	if err != nil {
		t.Fatal(err)
	}
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("runner did not start")
	}
	status, err := store.Cancel(context.Background(), result.SessionID, result.TurnID)
	if err != nil || status != workflow.CancelAccepted {
		t.Fatalf("cancel = %q, %v", status, err)
	}
	waitForBoundary(t, store, result.SessionID)
	events, err := store.Replay(result.SessionID, 0)
	if err != nil {
		t.Fatal(err)
	}
	var completed, cancelled int
	for _, event := range events {
		switch event.Type {
		case "turn.completed":
			completed++
		case "turn.cancelled":
			cancelled++
		}
	}
	if completed != 0 || cancelled != 1 || events[len(events)-1].Type != "session.waiting" {
		t.Fatalf("terminal events: completed=%d cancelled=%d tail=%q", completed, cancelled, events[len(events)-1].Type)
	}
}

func TestOpenRejectsSymlinkedSessionLog(t *testing.T) {
	root := t.TempDir()
	sessions := filepath.Join(root, "sessions")
	if err := os.MkdirAll(sessions, 0o700); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(t.TempDir(), "outside.jsonl")
	const contents = "outside must stay unchanged"
	if err := os.WriteFile(outside, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(sessions, "ses_escape.jsonl")); err != nil {
		t.Fatal(err)
	}
	store, err := workflow.Open(root, workflow.EchoResponder)
	if store != nil || err == nil {
		t.Fatalf("open symlinked log = %#v, %v", store, err)
	}
	got, err := os.ReadFile(outside)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != contents {
		t.Fatalf("outside file changed to %q", got)
	}
}

func TestOpenRejectsSymlinkedSessionsDirectory(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(root, "sessions")); err != nil {
		t.Fatal(err)
	}
	store, err := workflow.Open(root, workflow.EchoResponder)
	if store != nil || err == nil {
		t.Fatalf("open symlinked sessions directory = %#v, %v", store, err)
	}
}

func TestOpenRejectsCorruptLifecyclePayload(t *testing.T) {
	root := t.TempDir()
	sessions := filepath.Join(root, "sessions")
	if err := os.MkdirAll(sessions, 0o700); err != nil {
		t.Fatal(err)
	}
	id := "ses_corrupt"
	events := []workflow.Event{
		{
			Index: 0, Type: "session.started",
			Meta: workflow.EventMeta{At: "2026-01-01T00:00:00Z"}, SessionID: id,
			Data: json.RawMessage(`{"continuationToken":"eve:00000000-0000-0000-0000-000000000003"}`),
		},
		{
			Index: 1, Type: "turn.started",
			Meta: workflow.EventMeta{At: "2026-01-01T00:00:01Z"}, SessionID: id,
			TurnID: "turn_0", Data: json.RawMessage(`{"sequence":0,"turnId":"turn_0"}`),
		},
	}
	file, err := os.Create(filepath.Join(sessions, id+".jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	for _, event := range events {
		if err := json.NewEncoder(file).Encode(event); err != nil {
			t.Fatal(err)
		}
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	store, err := workflow.Open(root, workflow.EchoResponder)
	if store != nil || err == nil {
		t.Fatalf("open corrupt lifecycle = %#v, %v", store, err)
	}
}

func TestOpenRejectsCompletionAfterDurableCancellationIntent(t *testing.T) {
	root := t.TempDir()
	sessions := filepath.Join(root, "sessions")
	if err := os.MkdirAll(sessions, 0o700); err != nil {
		t.Fatal(err)
	}
	id := "ses_cancelled_complete"
	events := []workflow.Event{
		{
			Index: 0, Type: "session.started",
			Meta: workflow.EventMeta{At: "2026-01-01T00:00:00Z"}, SessionID: id,
			Data: json.RawMessage(`{"continuationToken":"eve:00000000-0000-0000-0000-000000000005"}`),
		},
		{
			Index: 1, Type: "turn.started",
			Meta: workflow.EventMeta{At: "2026-01-01T00:00:01Z"}, SessionID: id,
			TurnID: "turn_0",
			Data:   json.RawMessage(`{"sequence":0,"turnId":"turn_0","nextContinuationToken":"eve:00000000-0000-0000-0000-000000000005"}`),
		},
		{
			Index: 2, Type: "turn.cancellation.requested",
			Meta: workflow.EventMeta{At: "2026-01-01T00:00:02Z"}, SessionID: id,
			TurnID: "turn_0", Data: json.RawMessage(`{"sequence":0,"turnId":"turn_0"}`),
		},
		{
			Index: 3, Type: "turn.completed",
			Meta: workflow.EventMeta{At: "2026-01-01T00:00:03Z"}, SessionID: id,
			TurnID: "turn_0", Data: json.RawMessage(`{"sequence":0,"turnId":"turn_0"}`),
		},
	}
	file, err := os.Create(filepath.Join(sessions, id+".jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	for _, event := range events {
		if err := json.NewEncoder(file).Encode(event); err != nil {
			t.Fatal(err)
		}
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	store, err := workflow.Open(root, workflow.EchoResponder)
	if store != nil || err == nil {
		t.Fatalf("open cancelled completion = %#v, %v", store, err)
	}
}

func TestOpenAtomicallyMigratesLegacySessionLog(t *testing.T) {
	root := t.TempDir()
	sessions := filepath.Join(root, "sessions")
	if err := os.MkdirAll(sessions, 0o700); err != nil {
		t.Fatal(err)
	}
	id := "session_legacy"
	path := filepath.Join(sessions, id+".jsonl")
	legacy := strings.Join([]string{
		`{"index":0,"type":"session.started","at":"2026-01-01T00:00:00Z","sessionId":"session_legacy"}`,
		`{"index":1,"type":"turn.started","at":"2026-01-01T00:00:01Z","sessionId":"session_legacy","turnId":"turn_legacy","data":{"message":"old question"}}`,
		`{"index":2,"type":"message.completed","at":"2026-01-01T00:00:02Z","sessionId":"session_legacy","turnId":"turn_legacy","data":{"message":"old answer"}}`,
		`{"index":3,"type":"turn.completed","at":"2026-01-01T00:00:03Z","sessionId":"session_legacy","turnId":"turn_legacy"}`,
	}, "\n") + "\n"
	if err := os.WriteFile(path, []byte(legacy), 0o600); err != nil {
		t.Fatal(err)
	}

	store, err := workflow.Open(root, workflow.EchoResponder)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	events, err := store.Replay(id, 0)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"session.started", "turn.started", "message.received",
		"step.started", "message.appended", "message.completed", "step.completed",
		"turn.completed", "session.waiting",
	}
	if len(events) != len(want) {
		t.Fatalf("migrated event count = %d, want %d", len(events), len(want))
	}
	for index, eventType := range want {
		if events[index].Type != eventType {
			t.Fatalf("migrated event %d = %q, want %q", index, events[index].Type, eventType)
		}
	}
	result, err := store.Send(context.Background(), id, "new question")
	if err != nil {
		t.Fatal(err)
	}
	if result.Message != "stress-ack:2:new question" {
		t.Fatalf("post-migration response = %q", result.Message)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	firstLine := strings.SplitN(string(data), "\n", 2)[0]
	var envelope map[string]json.RawMessage
	if err := json.Unmarshal([]byte(firstLine), &envelope); err != nil {
		t.Fatal(err)
	}
	if _, ok := envelope["meta"]; !ok {
		t.Fatalf("legacy envelope was not replaced: %s", firstLine)
	}
	if _, ok := envelope["at"]; ok {
		t.Fatalf("legacy top-level timestamp remains: %s", firstLine)
	}
}

func TestOpenRepairsPartialTailAndSettlesInterruptedTurn(t *testing.T) {
	root := t.TempDir()
	sessions := filepath.Join(root, "sessions")
	if err := os.MkdirAll(sessions, 0o700); err != nil {
		t.Fatal(err)
	}
	id := "ses_recovery"
	token := "eve:00000000-0000-0000-0000-000000000001"
	events := []workflow.Event{
		{
			Index: 0, Type: "session.started",
			Meta: workflow.EventMeta{At: "2026-01-01T00:00:00Z"}, SessionID: id,
			Data: json.RawMessage(`{"continuationToken":"eve:00000000-0000-0000-0000-000000000001"}`),
		},
		{
			Index: 1, Type: "turn.started",
			Meta: workflow.EventMeta{At: "2026-01-01T00:00:01Z"}, SessionID: id,
			TurnID: "turn_interrupted",
			Data:   json.RawMessage(`{"sequence":0,"turnId":"turn_interrupted","nextContinuationToken":"eve:00000000-0000-0000-0000-000000000001"}`),
		},
		{
			Index: 2, Type: "message.received",
			Meta: workflow.EventMeta{At: "2026-01-01T00:00:02Z"}, SessionID: id,
			TurnID: "turn_interrupted",
			Data:   json.RawMessage(`{"message":"hello","sequence":0,"turnId":"turn_interrupted"}`),
		},
	}
	path := filepath.Join(sessions, id+".jsonl")
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, event := range events {
		if err := json.NewEncoder(file).Encode(event); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := file.WriteString(`{"index":3,"type":"step.started"`); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}

	store, err := workflow.Open(root, workflow.EchoResponder)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	replayed, err := store.Replay(id, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(replayed) != 5 ||
		replayed[3].Type != "turn.failed" ||
		replayed[4].Type != "session.waiting" {
		t.Fatalf("recovered events = %#v", replayed)
	}
	var failed struct {
		Code string `json:"code"`
	}
	if err := json.Unmarshal(replayed[3].Data, &failed); err != nil {
		t.Fatal(err)
	}
	if failed.Code != "RUNTIME_RESTARTED" {
		t.Fatalf("recovery code = %q", failed.Code)
	}
	started, err := store.Continue(id, token, "continue")
	if err != nil {
		t.Fatal(err)
	}
	if started.SessionID != id {
		t.Fatalf("continued session = %q", started.SessionID)
	}
	waitForBoundary(t, store, id)
}

func TestOpenRepairsMissingWaitingAfterDurableTerminal(t *testing.T) {
	root := t.TempDir()
	sessions := filepath.Join(root, "sessions")
	if err := os.MkdirAll(sessions, 0o700); err != nil {
		t.Fatal(err)
	}
	id := "ses_terminal_recovery"
	token := "eve:00000000-0000-0000-0000-000000000002"
	events := []workflow.Event{
		{
			Index: 0, Type: "session.started",
			Meta: workflow.EventMeta{At: "2026-01-01T00:00:00Z"}, SessionID: id,
			Data: json.RawMessage(`{"continuationToken":"eve:00000000-0000-0000-0000-000000000002"}`),
		},
		{
			Index: 1, Type: "turn.started",
			Meta: workflow.EventMeta{At: "2026-01-01T00:00:01Z"}, SessionID: id,
			TurnID: "turn_0",
			Data:   json.RawMessage(`{"sequence":0,"turnId":"turn_0","nextContinuationToken":"eve:00000000-0000-0000-0000-000000000002"}`),
		},
		{
			Index: 2, Type: "turn.completed",
			Meta: workflow.EventMeta{At: "2026-01-01T00:00:02Z"}, SessionID: id,
			TurnID: "turn_0", Data: json.RawMessage(`{"sequence":0,"turnId":"turn_0"}`),
		},
	}
	path := filepath.Join(sessions, id+".jsonl")
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, event := range events {
		if err := json.NewEncoder(file).Encode(event); err != nil {
			t.Fatal(err)
		}
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}

	store, err := workflow.Open(root, workflow.EchoResponder)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	replayed, err := store.Replay(id, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(replayed) != 4 || replayed[2].Type != "turn.completed" || replayed[3].Type != "session.waiting" {
		t.Fatalf("recovered events = %#v", replayed)
	}
	continued, err := store.Continue(id, token, "next")
	if err != nil {
		t.Fatal(err)
	}
	if continued.SessionID != id {
		t.Fatalf("continued session = %q", continued.SessionID)
	}
}

func TestOpenFinishesDurableCancellationIntent(t *testing.T) {
	root := t.TempDir()
	sessions := filepath.Join(root, "sessions")
	if err := os.MkdirAll(sessions, 0o700); err != nil {
		t.Fatal(err)
	}
	id := "ses_cancel_recovery"
	token := "eve:00000000-0000-0000-0000-000000000004"
	events := []workflow.Event{
		{
			Index: 0, Type: "session.started",
			Meta: workflow.EventMeta{At: "2026-01-01T00:00:00Z"}, SessionID: id,
			Data: json.RawMessage(`{"continuationToken":"eve:00000000-0000-0000-0000-000000000004"}`),
		},
		{
			Index: 1, Type: "turn.started",
			Meta: workflow.EventMeta{At: "2026-01-01T00:00:01Z"}, SessionID: id,
			TurnID: "turn_0",
			Data:   json.RawMessage(`{"sequence":0,"turnId":"turn_0","nextContinuationToken":"eve:00000000-0000-0000-0000-000000000004"}`),
		},
		{
			Index: 2, Type: "turn.cancellation.requested",
			Meta: workflow.EventMeta{At: "2026-01-01T00:00:02Z"}, SessionID: id,
			TurnID: "turn_0", Data: json.RawMessage(`{"sequence":0,"turnId":"turn_0"}`),
		},
	}
	path := filepath.Join(sessions, id+".jsonl")
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, event := range events {
		if err := json.NewEncoder(file).Encode(event); err != nil {
			t.Fatal(err)
		}
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	store, err := workflow.Open(root, workflow.EchoResponder)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	replayed, err := store.Replay(id, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(replayed) != 5 || replayed[3].Type != "turn.cancelled" || replayed[4].Type != "session.waiting" {
		t.Fatalf("recovered cancellation = %#v", replayed)
	}
	for _, event := range replayed {
		if event.Type == "turn.failed" {
			t.Fatal("durable cancellation recovered as failure")
		}
	}
	continued, err := store.Continue(id, token, "next")
	if err != nil {
		t.Fatal(err)
	}
	if continued.SessionID != id {
		t.Fatalf("continued session = %q", continued.SessionID)
	}
	waitForBoundary(t, store, id)
}

func open(t *testing.T, responder workflow.Responder) *workflow.Store {
	t.Helper()
	store, err := workflow.Open(t.TempDir(), responder)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store
}

func waitForBoundary(t *testing.T, store *workflow.Store, sessionID string) {
	t.Helper()
	cursor := 0
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	for {
		events, err := store.WaitForEvents(ctx, sessionID, cursor)
		if err != nil {
			t.Fatal(err)
		}
		for _, event := range events {
			cursor = event.Index + 1
			if event.Type == "session.waiting" {
				return
			}
		}
	}
}
