package server_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/thoriqakbar0/garden/internal/discover"
	"github.com/thoriqakbar0/garden/internal/server"
	"github.com/thoriqakbar0/garden/internal/workflow"
)

func TestSessionHTTPFlowAndReplay(t *testing.T) {
	store, err := workflow.Open(t.TempDir(), workflow.EchoResponder)
	if err != nil {
		t.Fatal(err)
	}
	handler := server.Handler(discover.Application{}, store)
	create := request(t, handler, http.MethodPost, "/eve/v1/session", nil)
	if create.Code != http.StatusCreated {
		t.Fatalf("create status = %d: %s", create.Code, create.Body.String())
	}
	var created struct {
		SessionID string `json:"sessionId"`
	}
	decode(t, create, &created)

	turn := request(t, handler, http.MethodPost, "/eve/v1/session/"+created.SessionID+"/turn", map[string]string{"message": "hello"})
	if turn.Code != http.StatusOK {
		t.Fatalf("turn status = %d: %s", turn.Code, turn.Body.String())
	}
	replay := request(t, handler, http.MethodGet, "/eve/v1/session/"+created.SessionID+"/stream?startIndex=2", nil)
	if replay.Code != http.StatusOK {
		t.Fatalf("replay status = %d: %s", replay.Code, replay.Body.String())
	}
	var events []workflow.Event
	decode(t, replay, &events)
	if len(events) != 2 || events[0].Index != 2 || events[1].Type != "turn.completed" {
		t.Fatalf("events = %#v", events)
	}
}

func TestScheduleDispatchUsesDiscoveredIdentifier(t *testing.T) {
	store, err := workflow.Open(t.TempDir(), workflow.EchoResponder)
	if err != nil {
		t.Fatal(err)
	}
	app := discover.Application{Schedules: []discover.Schedule{{ID: "heartbeat"}}}
	response := request(t, server.Handler(app, store), http.MethodGet, "/eve/v1/schedules/heartbeat/dispatch", nil)
	if response.Code != http.StatusAccepted {
		t.Fatalf("status = %d: %s", response.Code, response.Body.String())
	}
}

func request(t *testing.T, handler http.Handler, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var encoded bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&encoded).Encode(body); err != nil {
			t.Fatal(err)
		}
	}
	request := httptest.NewRequestWithContext(context.Background(), method, path, &encoded)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}

func decode(t *testing.T, response *httptest.ResponseRecorder, target any) {
	t.Helper()
	if err := json.Unmarshal(response.Body.Bytes(), target); err != nil {
		t.Fatal(err)
	}
}
