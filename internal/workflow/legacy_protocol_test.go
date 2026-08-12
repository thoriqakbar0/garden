package workflow_test

import (
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/thoriqakbar0/garden/internal/contracttest"
	"github.com/thoriqakbar0/garden/internal/discover"
	"github.com/thoriqakbar0/garden/internal/protocol"
	"github.com/thoriqakbar0/garden/internal/server"
	"github.com/thoriqakbar0/garden/internal/workflow"
	"github.com/thoriqakbar0/garden/internal/workflowtest"
)

func TestMigratedLegacyMessageHasCanonicalPublicLifecycle(t *testing.T) {
	root := t.TempDir()
	sessions := filepath.Join(root, "sessions")
	if err := os.MkdirAll(sessions, 0o700); err != nil {
		t.Fatal(err)
	}
	const sessionID = "session_legacy_protocol"
	legacy := strings.Join([]string{
		`{"index":0,"type":"session.started","at":"2026-01-01T00:00:00Z","sessionId":"session_legacy_protocol"}`,
		`{"index":1,"type":"turn.started","at":"2026-01-01T00:00:01Z","sessionId":"session_legacy_protocol","turnId":"turn_legacy","data":{"message":"old question"}}`,
		`{"index":2,"type":"message.completed","at":"2026-01-01T00:00:02Z","sessionId":"session_legacy_protocol","turnId":"turn_legacy","data":{"message":"old answer"}}`,
		`{"index":3,"type":"turn.completed","at":"2026-01-01T00:00:03Z","sessionId":"session_legacy_protocol","turnId":"turn_legacy"}`,
	}, "\n") + "\n"
	if err := os.WriteFile(filepath.Join(sessions, sessionID+".jsonl"), []byte(legacy), 0o600); err != nil {
		t.Fatal(err)
	}

	store, err := workflow.OpenRunner(root, workflowtest.EchoRunner())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	target := httptest.NewServer(server.Handler(discover.Manifest{}, store))
	t.Cleanup(target.Close)

	client := contracttest.NewClient(target.URL)
	stream := client.OpenStream(t, sessionID, nil)
	events := stream.ReadThroughBoundary(t)
	stream.Close()

	wantTypes := []protocol.EventType{
		protocol.SessionStarted,
		protocol.TurnStarted,
		protocol.MessageReceived,
		protocol.StepStarted,
		protocol.MessageAppended,
		protocol.MessageCompleted,
		protocol.StepCompleted,
		protocol.TurnCompleted,
		protocol.SessionWaiting,
	}
	gotTypes := make([]protocol.EventType, len(events))
	for index, event := range events {
		gotTypes[index] = event.Type
		if strings.HasPrefix(string(event.Type), "legacy.") {
			t.Fatalf("legacy internal event leaked publicly: %#v", event)
		}
	}
	if !reflect.DeepEqual(gotTypes, wantTypes) {
		t.Fatalf("migrated public lifecycle = %v, want %v", gotTypes, wantTypes)
	}

	var appended protocol.MessageAppendedData
	if err := json.Unmarshal(events[4].Data, &appended); err != nil {
		t.Fatal(err)
	}
	if appended.MessageDelta != "old answer" || appended.MessageSoFar != "old answer" {
		t.Fatalf("migrated message.appended = %#v", appended)
	}
	var completed protocol.MessageCompletedData
	if err := json.Unmarshal(events[5].Data, &completed); err != nil {
		t.Fatal(err)
	}
	if completed.Message != "old answer" || completed.FinishReason != "stop" {
		t.Fatalf("migrated message.completed = %#v", completed)
	}
}
