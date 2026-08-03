// Package contracttest provides one black-box Eve HTTP client and assertion
// suite that can run unchanged against Garden or an official Eve server.
package contracttest

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/thoriqakbar0/garden/internal/protocol"
)

const requestTimeout = 5 * time.Second

// Client targets one Eve-compatible HTTP origin.
type Client struct {
	BaseURL string
	HTTP    *http.Client
}

// Session is the accepted identity and continuation handle for a conversation.
type Session struct {
	ContinuationToken string
	SessionID         string
}

// Stream is one open NDJSON response.
type Stream struct {
	reader   *bufio.Reader
	response *http.Response
}

// NewClient constructs a black-box client for an Eve-compatible origin.
func NewClient(baseURL string) *Client {
	return &Client{
		BaseURL: strings.TrimRight(baseURL, "/"),
		HTTP:    &http.Client{},
	}
}

// Create starts a new session and asserts the exact accepted-response envelope.
func (c *Client) Create(t testing.TB, message string) Session {
	t.Helper()
	response := c.postJSON(t, "/eve/v1/session", protocol.CreateSessionRequest{Message: message})
	defer response.Body.Close()
	assertStatusAndContentType(t, response, http.StatusAccepted, "application/json")
	if got := response.Header.Get("cache-control"); got != "no-store" {
		t.Fatalf("create cache-control = %q, want no-store", got)
	}
	var envelope map[string]json.RawMessage
	decodeBody(t, response.Body, &envelope)
	assertKeys(t, envelope, "continuationToken", "ok", "sessionId")
	var result protocol.SessionResponse
	decodeRawMap(t, envelope, &result)
	if !result.OK || result.SessionID == "" || !strings.HasPrefix(result.ContinuationToken, "eve:") {
		t.Fatalf("create response = %#v", result)
	}
	if got := response.Header.Get(protocol.SessionIDHeader); got != result.SessionID {
		t.Fatalf("%s = %q, want %q", protocol.SessionIDHeader, got, result.SessionID)
	}
	return Session{
		ContinuationToken: result.ContinuationToken,
		SessionID:         result.SessionID,
	}
}

// Continue sends one follow-up through the current continuation token.
func (c *Client) Continue(t testing.TB, session Session, message string) {
	t.Helper()
	actualSessionID := c.ContinueWithToken(t, session.SessionID, session.ContinuationToken, message)
	if actualSessionID != session.SessionID {
		t.Fatalf("continued session = %q, want %q", actualSessionID, session.SessionID)
	}
}

// ContinueWithToken sends one follow-up and returns the token-selected session.
func (c *Client) ContinueWithToken(
	t testing.TB,
	requestedSessionID string,
	continuationToken string,
	message string,
) string {
	t.Helper()
	response := c.postJSON(
		t,
		"/eve/v1/session/"+url.PathEscape(requestedSessionID),
		protocol.ContinueSessionRequest{
			ContinuationToken: continuationToken,
			Message:           message,
		},
	)
	defer response.Body.Close()
	assertStatusAndContentType(t, response, http.StatusOK, "application/json")
	assertHeader(t, response, "cache-control", "no-store")
	var envelope map[string]json.RawMessage
	decodeBody(t, response.Body, &envelope)
	assertKeys(t, envelope, "ok", "sessionId")
	var result protocol.SessionResponse
	decodeRawMap(t, envelope, &result)
	if !result.OK || result.SessionID == "" {
		t.Fatalf("continue response = %#v", result)
	}
	if got := response.Header.Get(protocol.SessionIDHeader); got != result.SessionID {
		t.Fatalf("%s = %q, want %q", protocol.SessionIDHeader, got, result.SessionID)
	}
	return result.SessionID
}

// Cancel requests cancellation, optionally guarded by one observed turn ID.
func (c *Client) Cancel(t testing.TB, sessionID, turnID string) protocol.CancelTurnResponse {
	t.Helper()
	var body any = map[string]any{}
	if turnID != "" {
		body = protocol.CancelTurnRequest{TurnID: turnID}
	}
	response := c.postJSON(
		t,
		"/eve/v1/session/"+url.PathEscape(sessionID)+"/cancel",
		body,
	)
	defer response.Body.Close()
	assertStatusAndContentType(t, response, http.StatusAccepted, "application/json")
	assertHeader(t, response, "cache-control", "no-store")
	assertHeader(t, response, protocol.SessionIDHeader, sessionID)
	var envelope map[string]json.RawMessage
	decodeBody(t, response.Body, &envelope)
	assertKeys(t, envelope, "ok", "sessionId", "status")
	var result protocol.CancelTurnResponse
	decodeRawMap(t, envelope, &result)
	if !result.OK || result.SessionID != sessionID {
		t.Fatalf("cancel response = %#v", result)
	}
	return result
}

// OpenStream opens and validates one live Eve NDJSON stream. The first blank
// line must arrive as the body prelude before any durable event.
func (c *Client) OpenStream(t testing.TB, sessionID string, startIndex *int) *Stream {
	t.Helper()
	path := "/eve/v1/session/" + url.PathEscape(sessionID) + "/stream"
	if startIndex != nil {
		path += "?startIndex=" + strconvItoa(*startIndex)
	}
	ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
	t.Cleanup(cancel)
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL+path, nil)
	if err != nil {
		t.Fatal(err)
	}
	response, err := c.HTTP.Do(request)
	if err != nil {
		t.Fatalf("open stream: %v", err)
	}
	assertStatusAndContentType(t, response, http.StatusOK, protocol.MessageStreamContentType)
	assertHeader(t, response, "cache-control", "no-store, no-transform")
	assertHeader(t, response, "x-accel-buffering", "no")
	assertHeader(t, response, protocol.SessionIDHeader, sessionID)
	assertHeader(t, response, protocol.StreamFormatHeader, protocol.MessageStreamFormat)
	assertHeader(t, response, protocol.StreamVersionHeader, protocol.MessageStreamVersion)

	stream := &Stream{reader: bufio.NewReader(response.Body), response: response}
	first, err := stream.reader.ReadString('\n')
	if err != nil {
		response.Body.Close()
		t.Fatalf("read NDJSON prelude: %v", err)
	}
	if first != "\n" {
		response.Body.Close()
		t.Fatalf("NDJSON prelude = %q, want one blank line", first)
	}
	return stream
}

// Next reads and validates the next durable event.
func (s *Stream) Next(t testing.TB) protocol.Event {
	t.Helper()
	line, err := s.reader.ReadBytes('\n')
	if err != nil {
		t.Fatalf("read NDJSON event: %v", err)
	}
	var envelope map[string]json.RawMessage
	if err := json.Unmarshal(bytes.TrimSpace(line), &envelope); err != nil {
		t.Fatalf("decode NDJSON envelope %q: %v", line, err)
	}
	assertKeys(t, envelope, "data", "meta", "type")
	var meta map[string]json.RawMessage
	if err := json.Unmarshal(envelope["meta"], &meta); err != nil {
		t.Fatalf("decode NDJSON event meta: %v", err)
	}
	assertKeys(t, meta, "at")
	var event protocol.Event
	if err := json.Unmarshal(bytes.TrimSpace(line), &event); err != nil {
		t.Fatalf("decode NDJSON event %q: %v", line, err)
	}
	assertEventShape(t, event)
	return event
}

// Close simulates an HTTP disconnect.
func (s *Stream) Close() {
	_ = s.response.Body.Close()
}

// ReadThroughBoundary reads until session.waiting.
func (s *Stream) ReadThroughBoundary(t testing.TB) []protocol.Event {
	t.Helper()
	var events []protocol.Event
	for {
		event := s.Next(t)
		events = append(events, event)
		if event.Type == protocol.SessionWaiting {
			return events
		}
	}
}

// RunConversationContract exercises create, continuation, event ordering,
// disconnect/reconnect, absolute resume, and tail-relative resume.
func RunConversationContract(t *testing.T, baseURL string) {
	t.Helper()
	client := NewClient(baseURL)
	firstMessage := "contract-first"
	session := client.Create(t, firstMessage)
	stream := client.OpenStream(t, session.SessionID, nil)

	head := make([]protocol.Event, 4)
	for index := range head {
		head[index] = stream.Next(t)
	}
	stream.Close()

	resumeAt := len(head)
	resumed := client.OpenStream(t, session.SessionID, &resumeAt)
	tail := resumed.ReadThroughBoundary(t)
	resumed.Close()
	firstTurn := append(head, tail...)
	assertSuccessfulTurn(
		t,
		firstTurn,
		0,
		firstMessage,
		"stress-ack:1:"+firstMessage,
		session.ContinuationToken,
		true,
	)

	client.Continue(t, session, "contract-second")
	secondStart := len(firstTurn)
	secondStream := client.OpenStream(t, session.SessionID, &secondStart)
	secondTurn := secondStream.ReadThroughBoundary(t)
	secondStream.Close()
	assertSuccessfulTurn(
		t,
		secondTurn,
		1,
		"contract-second",
		"stress-ack:2:contract-second",
		session.ContinuationToken,
		false,
	)

	ownerRoutedID := client.ContinueWithToken(
		t,
		"ses_contract_missing",
		session.ContinuationToken,
		"contract-owner-token",
	)
	if ownerRoutedID != session.SessionID {
		t.Fatalf("owned token selected session %q, want %q", ownerRoutedID, session.SessionID)
	}
	ownerStart := len(firstTurn) + len(secondTurn)
	ownerStream := client.OpenStream(t, session.SessionID, &ownerStart)
	ownerTurn := ownerStream.ReadThroughBoundary(t)
	ownerStream.Close()
	assertSuccessfulTurn(
		t,
		ownerTurn,
		2,
		"contract-owner-token",
		"stress-ack:3:contract-owner-token",
		session.ContinuationToken,
		false,
	)

	unownedToken := freshUnownedToken(t)
	reboundID := client.ContinueWithToken(
		t,
		"ses_contract_missing",
		unownedToken,
		"contract-unowned-token",
	)
	if reboundID == session.SessionID {
		t.Fatal("an unowned continuation token resumed the URL session")
	}
	reboundStream := client.OpenStream(t, reboundID, nil)
	reboundTurn := reboundStream.ReadThroughBoundary(t)
	reboundStream.Close()
	assertSuccessfulTurn(
		t,
		reboundTurn,
		0,
		"contract-unowned-token",
		"stress-ack:1:contract-unowned-token",
		unownedToken,
		true,
	)

	all := append(append([]protocol.Event(nil), firstTurn...), secondTurn...)
	all = append(all, ownerTurn...)
	cutoff := 3
	replay := client.OpenStream(t, session.SessionID, &cutoff)
	for index, want := range all[cutoff:] {
		got := replay.Next(t)
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("absolute replay event %d differs:\ngot  %#v\nwant %#v", index, got, want)
		}
	}
	replay.Close()

	tailIndex := -1
	tailStream := client.OpenStream(t, session.SessionID, &tailIndex)
	gotTail := tailStream.Next(t)
	tailStream.Close()
	if !reflect.DeepEqual(gotTail, all[len(all)-1]) {
		t.Fatalf("tail-relative replay differs:\ngot  %#v\nwant %#v", gotTail, all[len(all)-1])
	}

	invalid := client.get(t, "/eve/v1/session/"+url.PathEscape(session.SessionID)+"/stream?startIndex=1.5")
	defer invalid.Body.Close()
	assertStatusAndContentType(t, invalid, http.StatusBadRequest, "application/json")
	problem := decodeProblem(t, invalid.Body)
	if problem.OK || problem.Error != "Expected startIndex to be an integer." {
		t.Fatalf("invalid cursor response = %#v", problem)
	}

	missing := client.OpenStream(t, "ses_contract_missing", nil)
	missing.Close()

	empty := client.postJSON(t, "/eve/v1/session", protocol.CreateSessionRequest{})
	defer empty.Body.Close()
	assertStatusAndContentType(t, empty, http.StatusBadRequest, "application/json")
	problem = decodeProblem(t, empty.Body)
	if problem.OK || problem.Error != "Missing or empty 'message' field." {
		t.Fatalf("empty create response = %#v", problem)
	}
	whitespace := client.postJSON(t, "/eve/v1/session", protocol.CreateSessionRequest{Message: " \t\n"})
	defer whitespace.Body.Close()
	assertStatusAndContentType(t, whitespace, http.StatusBadRequest, "application/json")
	problem = decodeProblem(t, whitespace.Body)
	if problem.OK || problem.Error != "Missing or empty 'message' field." {
		t.Fatalf("whitespace create response = %#v", problem)
	}

	invalidToken := client.postJSON(
		t,
		"/eve/v1/session/"+url.PathEscape(session.SessionID),
		protocol.ContinueSessionRequest{
			ContinuationToken: "not-an-eve-token",
			Message:           "contract-invalid-token",
		},
	)
	defer invalidToken.Body.Close()
	assertStatusAndContentType(t, invalidToken, http.StatusBadRequest, "application/json")
	_ = decodeProblem(t, invalidToken.Body)

	malformed := client.postBody(t, "/eve/v1/session", "not-json")
	defer malformed.Body.Close()
	assertStatusAndContentType(t, malformed, http.StatusBadRequest, "application/json")
	_ = decodeProblem(t, malformed.Body)

	unknownField := client.postJSON(t, "/eve/v1/session", map[string]any{
		"message": "contract-invalid", "unexpected": true,
	})
	defer unknownField.Body.Close()
	assertStatusAndContentType(t, unknownField, http.StatusBadRequest, "application/json")
	_ = decodeProblem(t, unknownField.Body)

	oversized := client.postJSON(t, "/eve/v1/session", protocol.CreateSessionRequest{
		Message: strings.Repeat("x", (1<<20)+1),
	})
	defer oversized.Body.Close()
	assertStatusAndContentType(t, oversized, http.StatusBadRequest, "application/json")
	_ = decodeProblem(t, oversized.Body)

	unknownCancel := client.Cancel(t, "ses_contract_missing", "")
	if unknownCancel.Status != "no_active_turn" {
		t.Fatalf("unknown-session cancel = %#v", unknownCancel)
	}
}

// RunCancellationContract proves the stream is live before turn completion,
// cancellation settles as turn.cancelled -> session.waiting, stale guards
// cannot cancel the observed turn, and late cancels are benign.
func RunCancellationContract(t *testing.T, baseURL string) {
	t.Helper()
	client := NewClient(baseURL)
	session := client.Create(t, "Please wait for cancellation.")
	stream := client.OpenStream(t, session.SessionID, nil)

	var events []protocol.Event
	var turnID string
	for turnID == "" {
		event := stream.Next(t)
		events = append(events, event)
		if event.Type == protocol.StepStarted {
			var data protocol.StepData
			decodeEventData(t, event, &data)
			turnID = data.TurnID
		}
	}

	stale := client.Cancel(t, session.SessionID, "turn_stale")
	if stale.Status != string(workflowCancelAccepted) {
		t.Fatalf("stale guarded cancel status = %q, want accepted", stale.Status)
	}
	cancelled := client.Cancel(t, session.SessionID, turnID)
	if cancelled.Status != string(workflowCancelAccepted) {
		t.Fatalf("matching cancel status = %q, want accepted", cancelled.Status)
	}

	for {
		event := stream.Next(t)
		events = append(events, event)
		if event.Type == protocol.SessionWaiting {
			break
		}
	}
	stream.Close()
	assertCancellationTail(t, events, turnID, session.ContinuationToken)

	deadline := time.Now().Add(requestTimeout)
	for {
		late := client.Cancel(t, session.SessionID, "")
		if late.Status == "no_active_turn" {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("late cancel remained %q", late.Status)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

type cancelStatus string

const workflowCancelAccepted cancelStatus = "accepted"

func assertSuccessfulTurn(
	t testing.TB,
	events []protocol.Event,
	sequence int,
	input string,
	output string,
	continuationToken string,
	first bool,
) {
	t.Helper()
	wantTypes := []protocol.EventType{
		protocol.TurnStarted,
		protocol.MessageReceived,
		protocol.StepStarted,
		protocol.MessageAppended,
		protocol.MessageCompleted,
		protocol.StepCompleted,
		protocol.TurnCompleted,
		protocol.SessionWaiting,
	}
	if first {
		wantTypes = append([]protocol.EventType{protocol.SessionStarted}, wantTypes...)
	}
	if got := eventTypes(events); !reflect.DeepEqual(got, wantTypes) {
		t.Fatalf("event order = %v, want %v", got, wantTypes)
	}

	offset := 0
	if first {
		assertKeysInData(t, events[0], "invocation?", "runtime?")
		offset = 1
	}
	turnID := fmt.Sprintf("turn_%d", sequence)
	assertDataEquals(t, events[offset], protocol.TurnData{Sequence: sequence, TurnID: turnID})
	assertDataEquals(t, events[offset+1], protocol.MessageReceivedData{
		Message:  input,
		Parts:    []protocol.TextPart{{Text: input, Type: "text"}},
		Sequence: sequence,
		TurnID:   turnID,
	})
	assertDataEquals(t, events[offset+2], protocol.StepData{
		Sequence: sequence, StepIndex: 0, TurnID: turnID,
	})
	assertDataEquals(t, events[offset+3], protocol.MessageAppendedData{
		MessageDelta: output, MessageSoFar: output,
		Sequence: sequence, StepIndex: 0, TurnID: turnID,
	})
	assertDataEquals(t, events[offset+4], protocol.MessageCompletedData{
		FinishReason: "stop", Message: output,
		Sequence: sequence, StepIndex: 0, TurnID: turnID,
	})
	assertStepCompleted(t, events[offset+5], sequence, turnID)
	assertDataEquals(t, events[offset+6], protocol.TurnData{Sequence: sequence, TurnID: turnID})
	assertDataEquals(t, events[offset+7], protocol.SessionWaitingData{
		ContinuationToken: continuationToken,
		Wait:              "next-user-message",
	})
}

func assertCancellationTail(
	t testing.TB,
	events []protocol.Event,
	turnID string,
	continuationToken string,
) {
	t.Helper()
	for _, event := range events {
		switch event.Type {
		case protocol.TurnFailed:
			t.Fatalf("cancellation emitted failure event %#v", event)
		}
	}
	if len(events) < 2 ||
		events[len(events)-2].Type != protocol.TurnCancelled ||
		events[len(events)-1].Type != protocol.SessionWaiting {
		t.Fatalf("cancellation tail = %v", eventTypes(events))
	}
	assertDataEquals(t, events[len(events)-2], protocol.TurnData{
		Sequence: 0,
		TurnID:   turnID,
	})
	assertDataEquals(t, events[len(events)-1], protocol.SessionWaitingData{
		ContinuationToken: continuationToken,
		Wait:              "next-user-message",
	})
}

func assertEventShape(t testing.TB, event protocol.Event) {
	t.Helper()
	if event.Type == "" {
		t.Fatal("event type is empty")
	}
	if _, err := time.Parse(time.RFC3339Nano, event.Meta.At); err != nil {
		t.Fatalf("event meta.at = %q: %v", event.Meta.At, err)
	}
	if event.Type != "session.completed" && len(event.Data) == 0 {
		t.Fatalf("%s event has no data object", event.Type)
	}
}

func assertStepCompleted(t testing.TB, event protocol.Event, sequence int, turnID string) {
	t.Helper()
	var object map[string]json.RawMessage
	decodeEventData(t, event, &object)
	for key := range object {
		switch key {
		case "finishReason", "providerMetadata", "sequence", "stepIndex", "turnId", "usage":
		default:
			t.Fatalf("step.completed has unexpected data key %q", key)
		}
	}
	var core protocol.StepCompletedData
	decodeEventData(t, event, &core)
	if core != (protocol.StepCompletedData{
		FinishReason: "stop",
		Sequence:     sequence,
		StepIndex:    0,
		TurnID:       turnID,
	}) {
		t.Fatalf("step.completed data = %#v", core)
	}
}

func assertKeysInData(t testing.TB, event protocol.Event, allowed ...string) {
	t.Helper()
	var object map[string]json.RawMessage
	decodeEventData(t, event, &object)
	allowedSet := make(map[string]struct{}, len(allowed))
	for _, key := range allowed {
		allowedSet[strings.TrimSuffix(key, "?")] = struct{}{}
	}
	for key := range object {
		if _, ok := allowedSet[key]; !ok {
			t.Fatalf("%s has unexpected data key %q", event.Type, key)
		}
	}
}

func assertDataEquals(t testing.TB, event protocol.Event, want any) {
	t.Helper()
	expected, err := json.Marshal(want)
	if err != nil {
		t.Fatal(err)
	}
	var got any
	var normalizedWant any
	if err := json.Unmarshal(event.Data, &got); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(expected, &normalizedWant); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, normalizedWant) {
		t.Fatalf("%s data = %#v, want %#v", event.Type, got, normalizedWant)
	}
}

func decodeEventData(t testing.TB, event protocol.Event, target any) {
	t.Helper()
	if err := json.Unmarshal(event.Data, target); err != nil {
		t.Fatalf("decode %s data: %v", event.Type, err)
	}
}

func eventTypes(events []protocol.Event) []protocol.EventType {
	types := make([]protocol.EventType, len(events))
	for index, event := range events {
		types[index] = event.Type
	}
	return types
}

func (c *Client) postJSON(t testing.TB, path string, body any) *http.Response {
	t.Helper()
	encoded, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	return c.postBody(t, path, string(encoded))
}

func (c *Client) postBody(t testing.TB, path, body string) *http.Response {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
	t.Cleanup(cancel)
	request, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		c.BaseURL+path,
		strings.NewReader(body),
	)
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("content-type", "application/json")
	response, err := c.HTTP.Do(request)
	if err != nil {
		t.Fatalf("POST %s: %v", path, err)
	}
	return response
}

func decodeProblem(t testing.TB, body io.Reader) protocol.ErrorResponse {
	t.Helper()
	var envelope map[string]json.RawMessage
	decodeBody(t, body, &envelope)
	assertKeys(t, envelope, "error", "ok")
	var problem protocol.ErrorResponse
	decodeRawMap(t, envelope, &problem)
	if problem.OK || problem.Error == "" {
		t.Fatalf("invalid error response = %#v", problem)
	}
	return problem
}

func (c *Client) get(t testing.TB, path string) *http.Response {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
	t.Cleanup(cancel)
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL+path, nil)
	if err != nil {
		t.Fatal(err)
	}
	response, err := c.HTTP.Do(request)
	if err != nil {
		t.Fatalf("GET %s: %v", path, err)
	}
	return response
}

func assertStatusAndContentType(
	t testing.TB,
	response *http.Response,
	status int,
	contentType string,
) {
	t.Helper()
	if response.StatusCode != status {
		body, _ := io.ReadAll(response.Body)
		t.Fatalf("status = %d, want %d: %s", response.StatusCode, status, body)
	}
	if got := response.Header.Get("content-type"); got != contentType {
		t.Fatalf("content-type = %q, want %q", got, contentType)
	}
}

func assertHeader(t testing.TB, response *http.Response, name, want string) {
	t.Helper()
	if got := response.Header.Get(name); got != want {
		t.Fatalf("%s = %q, want %q", name, got, want)
	}
}

func assertKeys(t testing.TB, object map[string]json.RawMessage, want ...string) {
	t.Helper()
	got := make(map[string]struct{}, len(object))
	for key := range object {
		got[key] = struct{}{}
	}
	for _, key := range want {
		if _, ok := got[key]; !ok {
			t.Fatalf("response is missing key %q: %v", key, mapKeys(object))
		}
		delete(got, key)
	}
	if len(got) != 0 {
		t.Fatalf("response has unexpected keys: %v", mapSetKeys(got))
	}
}

func decodeBody(t testing.TB, body io.Reader, target any) {
	t.Helper()
	decoder := json.NewDecoder(body)
	if err := decoder.Decode(target); err != nil {
		t.Fatal(err)
	}
	if err := decoder.Decode(&struct{}{}); !errorsIsEOF(err) {
		t.Fatalf("response contains more than one JSON value: %v", err)
	}
}

func decodeRawMap(t testing.TB, object map[string]json.RawMessage, target any) {
	t.Helper()
	encoded, err := json.Marshal(object)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(encoded, target); err != nil {
		t.Fatal(err)
	}
}

func errorsIsEOF(err error) bool {
	return err == io.EOF
}

func mapKeys(object map[string]json.RawMessage) []string {
	keys := make([]string, 0, len(object))
	for key := range object {
		keys = append(keys, key)
	}
	return keys
}

func mapSetKeys(object map[string]struct{}) []string {
	keys := make([]string, 0, len(object))
	for key := range object {
		keys = append(keys, key)
	}
	return keys
}

func strconvItoa(value int) string {
	return fmt.Sprintf("%d", value)
}

func freshUnownedToken(t testing.TB) string {
	t.Helper()
	var value [16]byte
	if _, err := rand.Read(value[:]); err != nil {
		t.Fatal(err)
	}
	return fmt.Sprintf(
		"eve:%08x-%04x-%04x-%04x-%012x",
		value[0:4],
		value[4:6],
		value[6:8],
		value[8:10],
		value[10:16],
	)
}
