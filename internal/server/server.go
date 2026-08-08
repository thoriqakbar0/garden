// Package server exposes Garden's local runtime through Eve's HTTP protocol.
package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/thoriqakbar0/garden/internal/discover"
	"github.com/thoriqakbar0/garden/internal/protocol"
	"github.com/thoriqakbar0/garden/internal/workflow"
)

var decimalInteger = regexp.MustCompile(`^-?\d+$`)

const maxRequestBodyBytes = 1 << 20

// Handler constructs the Eve-compatible local HTTP surface.
func Handler(app discover.Application, store *workflow.Store) http.Handler {
	server := &api{app: app, store: store}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", server.health)
	mux.HandleFunc("GET /eve/v1/health", server.health)
	mux.HandleFunc("GET /eve/v1/info", server.info)
	mux.HandleFunc("POST /eve/v1/session", server.createSession)
	mux.HandleFunc("POST /eve/v1/session/{sessionID}", server.continueSession)
	mux.HandleFunc("GET /eve/v1/session/{sessionID}/stream", server.stream)
	mux.HandleFunc("POST /eve/v1/session/{sessionID}/cancel", server.cancel)
	mux.HandleFunc("POST /eve/v1/schedules/{scheduleID}/dispatch", server.dispatchSchedule)
	return mux
}

type api struct {
	app   discover.Application
	store *workflow.Store
}

func (a *api) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (a *api) info(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	schedules := make([]publicSchedule, len(a.app.Schedules))
	for index, schedule := range a.app.Schedules {
		schedules[index] = publicSchedule{ID: schedule.ID, Cron: schedule.Cron}
	}
	writeJSON(w, http.StatusOK, publicApplication{
		Model:       a.app.Model,
		Tools:       a.app.Tools,
		Skills:      a.app.Skills,
		Channels:    a.app.Channels,
		Connections: a.app.Connections,
		Subagents:   a.app.Subagents,
		Schedules:   schedules,
		Evals:       a.app.Evals,
	})
}

type publicApplication struct {
	Model       string           `json:"model,omitempty"`
	Tools       []string         `json:"tools"`
	Skills      []string         `json:"skills"`
	Channels    []string         `json:"channels"`
	Connections []string         `json:"connections"`
	Subagents   []string         `json:"subagents"`
	Schedules   []publicSchedule `json:"schedules"`
	Evals       []string         `json:"evals"`
}

type publicSchedule struct {
	ID   string `json:"id"`
	Cron string `json:"cron,omitempty"`
}

func (a *api) createSession(w http.ResponseWriter, r *http.Request) {
	var input protocol.CreateSessionRequest
	if err := decodeJSON(r, &input); err != nil {
		writeProtocolError(w, http.StatusBadRequest, err.Error())
		return
	}
	if strings.TrimSpace(input.Message) == "" {
		writeProtocolError(w, http.StatusBadRequest, "Missing or empty 'message' field.")
		return
	}

	result, err := a.store.StartSession(input.Message)
	if err != nil {
		writeServerError(w)
		return
	}

	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set(protocol.SessionIDHeader, result.SessionID)
	writeJSON(w, http.StatusAccepted, protocol.SessionResponse{
		ContinuationToken: result.ContinuationToken,
		OK:                true,
		SessionID:         result.SessionID,
	})
}

func (a *api) continueSession(w http.ResponseWriter, r *http.Request) {
	sessionID := r.PathValue("sessionID")
	var input protocol.ContinueSessionRequest
	if err := decodeJSON(r, &input); err != nil {
		writeProtocolError(w, http.StatusBadRequest, err.Error())
		return
	}
	if input.ContinuationToken == "" {
		writeProtocolError(w, http.StatusBadRequest, "Missing or empty 'continuationToken' field.")
		return
	}
	if strings.TrimSpace(input.Message) == "" {
		writeProtocolError(
			w,
			http.StatusBadRequest,
			"Expected a non-empty 'message', a non-empty 'inputResponses' array, or both.",
		)
		return
	}
	result, err := a.store.Continue(
		sessionID,
		input.ContinuationToken,
		input.Message,
	)
	if err != nil {
		switch {
		case errors.Is(err, workflow.ErrInvalidContinuation):
			writeProtocolError(w, http.StatusBadRequest, "Invalid 'continuationToken' field.")
		case errors.Is(err, workflow.ErrInvalidSessionID):
			writeProtocolError(w, http.StatusBadRequest, "Invalid session ID.")
		case errors.Is(err, workflow.ErrSessionBusy):
			writeProtocolError(w, http.StatusConflict, "Session already has an active turn.")
		default:
			writeServerError(w)
		}
		return
	}

	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set(protocol.SessionIDHeader, result.SessionID)
	writeJSON(w, http.StatusOK, protocol.SessionResponse{OK: true, SessionID: result.SessionID})
}

func (a *api) stream(w http.ResponseWriter, r *http.Request) {
	sessionID := r.PathValue("sessionID")
	startIndex, err := parseStartIndex(r)
	if err != nil {
		writeProtocolError(w, http.StatusBadRequest, "Expected startIndex to be an integer.")
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		writeProtocolError(w, http.StatusInternalServerError, "Streaming is unavailable.")
		return
	}
	w.Header().Set("Cache-Control", "no-store, no-transform")
	w.Header().Set("Content-Type", protocol.MessageStreamContentType)
	w.Header().Set("X-Accel-Buffering", "no")
	w.Header().Set(protocol.SessionIDHeader, sessionID)
	w.Header().Set(protocol.StreamFormatHeader, protocol.MessageStreamFormat)
	w.Header().Set(protocol.StreamVersionHeader, protocol.MessageStreamVersion)
	w.WriteHeader(http.StatusOK)
	_, _ = io.WriteString(w, "\n")
	flusher.Flush()

	events, replayErr := a.store.Replay(sessionID, 0)
	if replayErr != nil {
		return
	}
	public := make([]protocol.Event, 0, len(events))
	for _, event := range events {
		projected, visible, err := publicEvent(event)
		if err != nil {
			return
		}
		if visible {
			public = append(public, projected)
		}
	}
	cursor := startIndex
	if cursor < 0 {
		cursor = len(public) + cursor
		if cursor < 0 {
			cursor = 0
		}
	}

	encoder := json.NewEncoder(w)
	publicIndex := 0
	for _, event := range public {
		if publicIndex >= cursor {
			if err := encoder.Encode(event); err != nil {
				return
			}
			flusher.Flush()
		}
		publicIndex++
	}
	internalCursor := len(events)
	for {
		events, waitErr := a.store.WaitForEvents(r.Context(), sessionID, internalCursor)
		if waitErr != nil {
			return
		}
		for _, event := range events {
			internalCursor = event.Index + 1
			projected, visible, err := publicEvent(event)
			if err != nil {
				return
			}
			if visible {
				if publicIndex >= cursor {
					if err := encoder.Encode(projected); err != nil {
						return
					}
					flusher.Flush()
				}
				publicIndex++
			}
		}
	}
}

func publicEvent(event workflow.Event) (protocol.Event, bool, error) {
	projected := protocol.Event{
		Data: append(json.RawMessage(nil), event.Data...),
		Meta: protocol.EventMeta{At: event.Meta.At},
		Type: protocol.EventType(event.Type),
	}
	switch projected.Type {
	case protocol.SessionStarted:
		projected.Data = json.RawMessage(`{}`)
	case protocol.TurnStarted:
		var data protocol.TurnData
		if err := json.Unmarshal(event.Data, &data); err != nil {
			return protocol.Event{}, false, err
		}
		encoded, err := json.Marshal(data)
		if err != nil {
			return protocol.Event{}, false, err
		}
		projected.Data = encoded
	case protocol.MessageReceived:
		var data protocol.MessageReceivedData
		if err := json.Unmarshal(event.Data, &data); err != nil {
			return protocol.Event{}, false, err
		}
		data.Parts = []protocol.TextPart{{Text: data.Message, Type: "text"}}
		encoded, err := json.Marshal(data)
		if err != nil {
			return protocol.Event{}, false, err
		}
		projected.Data = encoded
	case protocol.MessageCompleted:
		var data protocol.MessageCompletedData
		if err := json.Unmarshal(event.Data, &data); err != nil {
			return protocol.Event{}, false, err
		}
		encoded, err := json.Marshal(data)
		if err != nil {
			return protocol.Event{}, false, err
		}
		projected.Data = encoded
	case protocol.StepCompleted:
		var data protocol.StepCompletedData
		if err := json.Unmarshal(event.Data, &data); err != nil {
			return protocol.Event{}, false, err
		}
		encoded, err := json.Marshal(data)
		if err != nil {
			return protocol.Event{}, false, err
		}
		projected.Data = encoded
	case protocol.StepStarted,
		protocol.MessageAppended,
		protocol.TurnCompleted,
		protocol.TurnCancelled,
		protocol.TurnFailed,
		protocol.SessionWaiting:
		// These event types already use the pinned Eve envelope and payload.
	default:
		return protocol.Event{}, false, nil
	}
	return projected, true, nil
}

func (a *api) cancel(w http.ResponseWriter, r *http.Request) {
	input, err := parseCancelTurn(r)
	if err != nil {
		writeProtocolError(w, http.StatusBadRequest, err.Error())
		return
	}
	sessionID := r.PathValue("sessionID")
	result, err := a.store.Cancel(r.Context(), sessionID, input.TurnID)
	if err != nil {
		writeServerError(w)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set(protocol.SessionIDHeader, sessionID)
	writeJSON(w, http.StatusAccepted, protocol.CancelTurnResponse{
		OK:        true,
		SessionID: sessionID,
		Status:    string(result),
	})
}

func (a *api) dispatchSchedule(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("scheduleID")
	found := false
	for _, schedule := range a.app.Schedules {
		if schedule.ID == id {
			found = true
			break
		}
	}
	if !found {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": fmt.Sprintf("schedule %q not found", id)})
		return
	}
	sessionID, err := a.store.CreateSession()
	if err != nil {
		writeServerError(w)
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{
		"scheduleId": id,
		"sessionIds": []string{sessionID},
	})
}

func parseStartIndex(r *http.Request) (int, error) {
	values, present := r.URL.Query()["startIndex"]
	if !present {
		return 0, nil
	}
	if len(values) != 1 || !decimalInteger.MatchString(values[0]) {
		return 0, errors.New("startIndex is not a decimal integer")
	}
	value, err := strconv.ParseInt(values[0], 10, 64)
	if err != nil || int64(int(value)) != value {
		return 0, errors.New("startIndex is outside the supported integer range")
	}
	return int(value), nil
}

func parseCancelTurn(r *http.Request) (protocol.CancelTurnRequest, error) {
	body, err := io.ReadAll(io.LimitReader(r.Body, maxRequestBodyBytes+1))
	if err != nil {
		return protocol.CancelTurnRequest{}, errors.New("Unreadable request body.")
	}
	if len(body) > maxRequestBodyBytes {
		return protocol.CancelTurnRequest{}, errors.New("Request body is too large.")
	}
	if strings.TrimSpace(string(body)) == "" {
		return protocol.CancelTurnRequest{}, nil
	}
	var object map[string]json.RawMessage
	if err := json.Unmarshal(body, &object); err != nil || object == nil {
		return protocol.CancelTurnRequest{}, errors.New("Expected a JSON object.")
	}
	raw, present := object["turnId"]
	if !present {
		return protocol.CancelTurnRequest{}, nil
	}
	var turnID string
	if err := json.Unmarshal(raw, &turnID); err != nil || turnID == "" {
		return protocol.CancelTurnRequest{}, errors.New("Expected 'turnId' to be a non-empty string.")
	}
	return protocol.CancelTurnRequest{TurnID: turnID}, nil
}

func decodeJSON(r *http.Request, target any) error {
	body, err := io.ReadAll(io.LimitReader(r.Body, maxRequestBodyBytes+1))
	if err != nil {
		return errors.New("Invalid JSON body.")
	}
	if len(body) > maxRequestBodyBytes {
		return errors.New("Request body is too large.")
	}
	decoder := json.NewDecoder(strings.NewReader(string(body)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return errors.New("Invalid JSON body.")
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return errors.New("Invalid JSON body.")
	}
	return nil
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeProtocolError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, protocol.ErrorResponse{Error: message, OK: false})
}

func writeServerError(w http.ResponseWriter) {
	writeProtocolError(w, http.StatusInternalServerError, "Local runtime request failed.")
}
