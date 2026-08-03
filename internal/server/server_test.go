package server_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/thoriqakbar0/garden/internal/contracttest"
	"github.com/thoriqakbar0/garden/internal/discover"
	"github.com/thoriqakbar0/garden/internal/protocol"
	"github.com/thoriqakbar0/garden/internal/server"
	"github.com/thoriqakbar0/garden/internal/workflow"
)

func TestGardenPassesSharedEveConversationContract(t *testing.T) {
	target := serve(t, workflow.EchoResponder)
	contracttest.RunConversationContract(t, target.URL)
}

func TestGardenPassesSharedEveCancellationContract(t *testing.T) {
	target := serve(t, func(ctx context.Context, _ string, _ []workflow.Event) (string, error) {
		<-ctx.Done()
		return "", ctx.Err()
	})
	contracttest.RunCancellationContract(t, target.URL)
}

func TestToolInternalsStayOffPublicStreamAndCursor(t *testing.T) {
	runner := workflow.RunnerFunc(func(_ context.Context, turn workflow.Turn, emit workflow.Emit) (string, error) {
		step := func(index int) map[string]any {
			return map[string]any{"sequence": turn.Sequence, "stepIndex": index, "turnId": turn.TurnID}
		}
		if err := emit("step.started", step(0)); err != nil {
			return "", err
		}
		if err := emit("actions.requested", map[string]any{"private": "tool arguments"}); err != nil {
			return "", err
		}
		first := step(0)
		first["finishReason"] = "tool-calls"
		if err := emit("step.completed", first); err != nil {
			return "", err
		}
		if err := emit("action.result", map[string]any{"private": "tool output"}); err != nil {
			return "", err
		}
		if err := emit("step.started", step(1)); err != nil {
			return "", err
		}
		if err := emit("message.appended", map[string]any{
			"messageDelta": "done", "messageSoFar": "done",
			"sequence": turn.Sequence, "stepIndex": 1, "turnId": turn.TurnID,
		}); err != nil {
			return "", err
		}
		if err := emit("message.completed", map[string]any{
			"finishReason": "stop", "message": "done",
			"sequence": turn.Sequence, "stepIndex": 1, "turnId": turn.TurnID,
		}); err != nil {
			return "", err
		}
		last := step(1)
		last["finishReason"] = "stop"
		if err := emit("step.completed", last); err != nil {
			return "", err
		}
		return "done", nil
	})
	target := serveRunner(t, runner)
	client := contracttest.NewClient(target.URL)
	session := client.Create(t, "tool turn")
	stream := client.OpenStream(t, session.SessionID, nil)
	for range 4 {
		_ = stream.Next(t)
	}
	stream.Close()

	cursor := 4
	resumed := client.OpenStream(t, session.SessionID, &cursor)
	events := resumed.ReadThroughBoundary(t)
	resumed.Close()
	if len(events) == 0 || events[0].Type != protocol.StepCompleted {
		t.Fatalf("first resumed event = %#v", events)
	}
	for _, event := range events {
		if event.Type == "actions.requested" || event.Type == "action.result" || event.Type == "step.failed" {
			t.Fatalf("Garden-only event leaked publicly: %#v", event)
		}
	}
}

func TestStreamDisconnectDoesNotCancelActiveTurn(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	runner := workflow.RunnerFunc(func(_ context.Context, turn workflow.Turn, emit workflow.Emit) (string, error) {
		step := map[string]any{"sequence": turn.Sequence, "stepIndex": 0, "turnId": turn.TurnID}
		if err := emit("step.started", step); err != nil {
			return "", err
		}
		close(started)
		<-release
		if err := emit("message.appended", map[string]any{
			"messageDelta": "done", "messageSoFar": "done",
			"sequence": turn.Sequence, "stepIndex": 0, "turnId": turn.TurnID,
		}); err != nil {
			return "", err
		}
		if err := emit("message.completed", map[string]any{
			"finishReason": "stop", "message": "done",
			"sequence": turn.Sequence, "stepIndex": 0, "turnId": turn.TurnID,
		}); err != nil {
			return "", err
		}
		step["finishReason"] = "stop"
		if err := emit("step.completed", step); err != nil {
			return "", err
		}
		return "done", nil
	})
	target := serveRunner(t, runner)
	client := contracttest.NewClient(target.URL)
	session := client.Create(t, "disconnect")
	stream := client.OpenStream(t, session.SessionID, nil)
	seen := 0
	for {
		event := stream.Next(t)
		seen++
		if event.Type == protocol.StepStarted {
			break
		}
	}
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("runner did not reach blocking step")
	}
	stream.Close()
	close(release)
	resumed := client.OpenStream(t, session.SessionID, &seen)
	events := resumed.ReadThroughBoundary(t)
	resumed.Close()
	foundCompleted := false
	for _, event := range events {
		if event.Type == protocol.TurnCancelled {
			t.Fatal("stream disconnect cancelled the turn")
		}
		if event.Type == protocol.TurnCompleted {
			foundCompleted = true
		}
	}
	if !foundCompleted {
		t.Fatalf("resumed events omitted turn.completed: %#v", events)
	}
}

func TestCancelValidationMatchesEve(t *testing.T) {
	target := serve(t, workflow.EchoResponder)
	for _, testCase := range []struct {
		name string
		body string
	}{
		{name: "malformed", body: "not-json"},
		{name: "array", body: "[]"},
		{name: "number turn id", body: `{"turnId":7}`},
		{name: "empty turn id", body: `{"turnId":""}`},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			response, err := http.Post(
				target.URL+"/eve/v1/session/unknown/cancel",
				"application/json",
				strings.NewReader(testCase.body),
			)
			if err != nil {
				t.Fatal(err)
			}
			defer response.Body.Close()
			if response.StatusCode != http.StatusBadRequest {
				t.Fatalf("status = %d", response.StatusCode)
			}
			var problem struct {
				OK bool `json:"ok"`
			}
			if err := json.NewDecoder(response.Body).Decode(&problem); err != nil {
				t.Fatal(err)
			}
			if problem.OK {
				t.Fatal("validation error reported ok")
			}
		})
	}
}

func TestScheduleDispatchUsesDiscoveredIdentifier(t *testing.T) {
	store, err := workflow.Open(t.TempDir(), workflow.EchoResponder)
	if err != nil {
		t.Fatal(err)
	}
	app := discover.Application{Schedules: []discover.Schedule{{ID: "heartbeat"}}}
	target := httptest.NewServer(server.Handler(app, store))
	t.Cleanup(target.Close)
	response, err := http.Post(
		target.URL+"/eve/v1/schedules/heartbeat/dispatch",
		"application/json",
		http.NoBody,
	)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusAccepted {
		t.Fatalf("status = %d", response.StatusCode)
	}

	getResponse, err := http.Get(target.URL + "/eve/v1/schedules/heartbeat/dispatch")
	if err != nil {
		t.Fatal(err)
	}
	defer getResponse.Body.Close()
	if getResponse.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("GET status = %d, want %d", getResponse.StatusCode, http.StatusMethodNotAllowed)
	}
	if got := getResponse.Header.Get("Allow"); got != http.MethodPost {
		t.Fatalf("Allow = %q, want POST", got)
	}
}

func TestInfoRedactsProjectRootAndInstructions(t *testing.T) {
	store, err := workflow.Open(t.TempDir(), workflow.EchoResponder)
	if err != nil {
		t.Fatal(err)
	}
	app := discover.Application{
		Root:         "/private/project",
		Instructions: "api-key-secret",
		Model:        "provider/model",
		Tools:        []string{"weather"},
		Schedules:    []discover.Schedule{{ID: "hourly", Cron: "0 * * * *", Path: "/private/project/hourly.ts"}},
	}
	target := httptest.NewServer(server.Handler(app, store))
	t.Cleanup(target.Close)

	response, err := http.Get(target.URL + "/eve/v1/info")
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", response.StatusCode)
	}
	if got := response.Header.Get("Cache-Control"); got != "no-store" {
		t.Fatalf("Cache-Control = %q, want no-store", got)
	}
	var info map[string]json.RawMessage
	if err := json.NewDecoder(response.Body).Decode(&info); err != nil {
		t.Fatal(err)
	}
	if _, exposed := info["root"]; exposed {
		t.Fatal("info exposed project root")
	}
	if _, exposed := info["instructions"]; exposed {
		t.Fatal("info exposed full instructions")
	}
	encoded, err := json.Marshal(info)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), "/private/project") || strings.Contains(string(encoded), "api-key-secret") {
		t.Fatalf("info exposed private application data: %s", encoded)
	}
	if got := string(info["model"]); got != `"provider/model"` {
		t.Fatalf("model = %s", got)
	}
}

func serve(t *testing.T, responder workflow.Responder) *httptest.Server {
	t.Helper()
	store, err := workflow.Open(t.TempDir(), responder)
	if err != nil {
		t.Fatal(err)
	}
	target := httptest.NewServer(server.Handler(discover.Application{}, store))
	t.Cleanup(target.Close)
	t.Cleanup(func() { _ = store.Close() })
	return target
}

func serveRunner(t *testing.T, runner workflow.Runner) *httptest.Server {
	t.Helper()
	store, err := workflow.OpenRunner(t.TempDir(), runner)
	if err != nil {
		t.Fatal(err)
	}
	target := httptest.NewServer(server.Handler(discover.Application{}, store))
	t.Cleanup(target.Close)
	t.Cleanup(func() { _ = store.Close() })
	return target
}
