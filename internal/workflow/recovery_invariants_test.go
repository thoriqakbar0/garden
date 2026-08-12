package workflow_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/thoriqakbar0/garden/internal/workflow"
	"github.com/thoriqakbar0/garden/internal/workflowtest"
)

func TestOpenRejectsRecoveryInvariantViolations(t *testing.T) {
	const (
		sessionID = "ses_recovery_invariants"
		token     = "eve:00000000-0000-0000-0000-000000000006"
	)

	sessionStarted := func(index int) workflow.Event {
		return workflow.Event{
			Index: index, Type: "session.started",
			Meta: workflow.EventMeta{At: "2026-01-01T00:00:00Z"}, SessionID: sessionID,
			Data: json.RawMessage(`{"continuationToken":"` + token + `"}`),
		}
	}
	turnStarted := workflow.Event{
		Index: 1, Type: "turn.started",
		Meta: workflow.EventMeta{At: "2026-01-01T00:00:01Z"}, SessionID: sessionID,
		TurnID: "turn_0",
		Data:   json.RawMessage(`{"sequence":0,"turnId":"turn_0","nextContinuationToken":"` + token + `"}`),
	}
	turnCompleted := workflow.Event{
		Index: 2, Type: "turn.completed",
		Meta: workflow.EventMeta{At: "2026-01-01T00:00:02Z"}, SessionID: sessionID,
		TurnID: "turn_0", Data: json.RawMessage(`{"sequence":0,"turnId":"turn_0"}`),
	}

	tests := []struct {
		name   string
		events []workflow.Event
	}{
		{
			name: "terminal sequence mismatch",
			events: []workflow.Event{
				sessionStarted(0),
				turnStarted,
				{
					Index: 2, Type: "turn.completed",
					Meta: workflow.EventMeta{At: "2026-01-01T00:00:02Z"}, SessionID: sessionID,
					TurnID: "turn_0", Data: json.RawMessage(`{"sequence":1,"turnId":"turn_0"}`),
				},
			},
		},
		{
			name: "cancellation intent sequence mismatch",
			events: []workflow.Event{
				sessionStarted(0),
				turnStarted,
				{
					Index: 2, Type: "turn.cancellation.requested",
					Meta: workflow.EventMeta{At: "2026-01-01T00:00:02Z"}, SessionID: sessionID,
					TurnID: "turn_0", Data: json.RawMessage(`{"sequence":1,"turnId":"turn_0"}`),
				},
			},
		},
		{
			name: "waiting turn mismatch",
			events: []workflow.Event{
				sessionStarted(0),
				turnStarted,
				turnCompleted,
				{
					Index: 3, Type: "session.waiting",
					Meta: workflow.EventMeta{At: "2026-01-01T00:00:03Z"}, SessionID: sessionID,
					TurnID: "turn_1", Data: json.RawMessage(`{"continuationToken":"` + token + `","wait":"next-user-message"}`),
				},
			},
		},
		{
			name: "waiting turn empty",
			events: []workflow.Event{
				sessionStarted(0),
				turnStarted,
				turnCompleted,
				{
					Index: 3, Type: "session.waiting",
					Meta: workflow.EventMeta{At: "2026-01-01T00:00:03Z"}, SessionID: sessionID,
					Data: json.RawMessage(`{"continuationToken":"` + token + `","wait":"next-user-message"}`),
				},
			},
		},
		{
			name: "known event envelope turn mismatch",
			events: []workflow.Event{
				sessionStarted(0),
				turnStarted,
				{
					Index: 2, Type: "message.received",
					Meta: workflow.EventMeta{At: "2026-01-01T00:00:02Z"}, SessionID: sessionID,
					TurnID: "turn_1",
					Data:   json.RawMessage(`{"message":"wrong turn","parts":[{"text":"wrong turn","type":"text"}],"sequence":0,"turnId":"turn_1"}`),
				},
			},
		},
		{
			name: "known event payload sequence mismatch",
			events: []workflow.Event{
				sessionStarted(0),
				turnStarted,
				{
					Index: 2, Type: "message.received",
					Meta: workflow.EventMeta{At: "2026-01-01T00:00:02Z"}, SessionID: sessionID,
					TurnID: "turn_0",
					Data:   json.RawMessage(`{"message":"wrong sequence","parts":[{"text":"wrong sequence","type":"text"}],"sequence":1,"turnId":"turn_0"}`),
				},
			},
		},
		{
			name: "known event payload sequence missing",
			events: []workflow.Event{
				sessionStarted(0),
				turnStarted,
				{
					Index: 2, Type: "message.received",
					Meta: workflow.EventMeta{At: "2026-01-01T00:00:02Z"}, SessionID: sessionID,
					TurnID: "turn_0",
					Data:   json.RawMessage(`{"message":"missing sequence","parts":[{"text":"missing sequence","type":"text"}],"turnId":"turn_0"}`),
				},
			},
		},
		{
			name: "known event payload turn mismatch",
			events: []workflow.Event{
				sessionStarted(0),
				turnStarted,
				{
					Index: 2, Type: "message.received",
					Meta: workflow.EventMeta{At: "2026-01-01T00:00:02Z"}, SessionID: sessionID,
					TurnID: "turn_0",
					Data:   json.RawMessage(`{"message":"wrong payload turn","parts":[{"text":"wrong payload turn","type":"text"}],"sequence":0,"turnId":"turn_1"}`),
				},
			},
		},
		{
			name: "known event payload turn missing",
			events: []workflow.Event{
				sessionStarted(0),
				turnStarted,
				{
					Index: 2, Type: "message.received",
					Meta: workflow.EventMeta{At: "2026-01-01T00:00:02Z"}, SessionID: sessionID,
					TurnID: "turn_0",
					Data:   json.RawMessage(`{"message":"missing payload turn","parts":[{"text":"missing payload turn","type":"text"}],"sequence":0}`),
				},
			},
		},
		{
			name: "known event outside active turn",
			events: []workflow.Event{
				sessionStarted(0),
				{
					Index: 1, Type: "step.started",
					Meta: workflow.EventMeta{At: "2026-01-01T00:00:01Z"}, SessionID: sessionID,
					TurnID: "turn_0",
					Data:   json.RawMessage(`{"sequence":0,"stepIndex":0,"turnId":"turn_0"}`),
				},
			},
		},
		{
			name: "known event after durable cancellation intent",
			events: []workflow.Event{
				sessionStarted(0),
				turnStarted,
				{
					Index: 2, Type: "turn.cancellation.requested",
					Meta: workflow.EventMeta{At: "2026-01-01T00:00:02Z"}, SessionID: sessionID,
					TurnID: "turn_0", Data: json.RawMessage(`{"sequence":0,"turnId":"turn_0"}`),
				},
				{
					Index: 3, Type: "message.completed",
					Meta: workflow.EventMeta{At: "2026-01-01T00:00:03Z"}, SessionID: sessionID,
					TurnID: "turn_0",
					Data:   json.RawMessage(`{"finishReason":"stop","message":"late","sequence":0,"stepIndex":0,"turnId":"turn_0"}`),
				},
			},
		},
		{
			name: "step completion before start",
			events: []workflow.Event{
				sessionStarted(0),
				turnStarted,
				{
					Index: 2, Type: "step.completed",
					Meta: workflow.EventMeta{At: "2026-01-01T00:00:02Z"}, SessionID: sessionID,
					TurnID: "turn_0",
					Data:   json.RawMessage(`{"finishReason":"stop","sequence":0,"stepIndex":0,"turnId":"turn_0"}`),
				},
			},
		},
		{
			name: "action result without request",
			events: []workflow.Event{
				sessionStarted(0),
				turnStarted,
				{
					Index: 2, Type: "step.started",
					Meta: workflow.EventMeta{At: "2026-01-01T00:00:02Z"}, SessionID: sessionID,
					TurnID: "turn_0",
					Data:   json.RawMessage(`{"sequence":0,"stepIndex":0,"turnId":"turn_0"}`),
				},
				{
					Index: 3, Type: "action.result",
					Meta: workflow.EventMeta{At: "2026-01-01T00:00:03Z"}, SessionID: sessionID,
					TurnID: "turn_0",
					Data:   json.RawMessage(`{"result":{"callId":"call-1","kind":"tool-result","output":{},"toolName":"test"},"sequence":0,"status":"completed","stepIndex":0,"turnId":"turn_0"}`),
				},
			},
		},
		{
			name: "reused turn id",
			events: []workflow.Event{
				sessionStarted(0),
				turnStarted,
				turnCompleted,
				{
					Index: 3, Type: "session.waiting",
					Meta: workflow.EventMeta{At: "2026-01-01T00:00:03Z"}, SessionID: sessionID,
					TurnID: "turn_0", Data: json.RawMessage(`{"continuationToken":"` + token + `","wait":"next-user-message"}`),
				},
				{
					Index: 4, Type: "turn.started",
					Meta: workflow.EventMeta{At: "2026-01-01T00:00:04Z"}, SessionID: sessionID,
					TurnID: "turn_0",
					Data:   json.RawMessage(`{"sequence":1,"turnId":"turn_0","nextContinuationToken":"` + token + `"}`),
				},
			},
		},
		{
			name: "duplicate session started",
			events: []workflow.Event{
				sessionStarted(0),
				sessionStarted(1),
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			writeRecoveryInvariantLog(t, root, sessionID, test.events)

			store, err := workflow.OpenRunner(root, workflowtest.EchoRunner())
			if store != nil {
				_ = store.Close()
				t.Fatalf("Open returned a store for corrupt recovery history: %v", err)
			}
			if err == nil {
				t.Fatal("Open accepted corrupt recovery history")
			}
		})
	}
}

func writeRecoveryInvariantLog(t *testing.T, root, sessionID string, events []workflow.Event) {
	t.Helper()
	sessions := filepath.Join(root, "sessions")
	if err := os.MkdirAll(sessions, 0o700); err != nil {
		t.Fatal(err)
	}
	file, err := os.Create(filepath.Join(sessions, sessionID+".jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	for _, event := range events {
		if err := json.NewEncoder(file).Encode(event); err != nil {
			_ = file.Close()
			t.Fatal(err)
		}
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
}
