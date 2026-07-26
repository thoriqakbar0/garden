// Package server exposes the durable runtime through an HTTP API.
package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/thoriqakbar0/garden/internal/discover"
	"github.com/thoriqakbar0/garden/internal/workflow"
)

// Handler constructs the eve-compatible HTTP surface.
func Handler(app discover.Application, store *workflow.Store) http.Handler {
	server := &api{app: app, store: store}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", server.health)
	mux.HandleFunc("GET /eve/v1/info", server.info)
	mux.HandleFunc("POST /eve/v1/session", server.createSession)
	mux.HandleFunc("POST /eve/v1/session/{sessionID}/turn", server.send)
	mux.HandleFunc("GET /eve/v1/session/{sessionID}/stream", server.stream)
	mux.HandleFunc("POST /eve/v1/session/{sessionID}/cancel", server.cancel)
	mux.HandleFunc("POST /eve/v1/schedules/{scheduleID}/dispatch", server.dispatchSchedule)
	mux.HandleFunc("GET /eve/v1/schedules/{scheduleID}/dispatch", server.dispatchSchedule)
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
	writeJSON(w, http.StatusOK, a.app)
}

func (a *api) createSession(w http.ResponseWriter, _ *http.Request) {
	id, err := a.store.CreateSession()
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"sessionId": id})
}

func (a *api) send(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Message string `json:"message"`
	}
	if err := decodeJSON(r, &input); err != nil {
		writeClientError(w, err)
		return
	}
	result, err := a.store.Send(r.Context(), r.PathValue("sessionID"), input.Message)
	if err != nil {
		if errors.Is(err, r.Context().Err()) {
			writeJSON(w, 499, map[string]string{"error": "request cancelled"})
			return
		}
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (a *api) stream(w http.ResponseWriter, r *http.Request) {
	startIndex := 0
	if raw := r.URL.Query().Get("startIndex"); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil || value < 0 {
			writeClientError(w, errors.New("startIndex must be a non-negative integer"))
			return
		}
		startIndex = value
	}
	events, err := a.store.Replay(r.PathValue("sessionID"), startIndex)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, events)
}

func (a *api) cancel(w http.ResponseWriter, r *http.Request) {
	var input struct {
		TurnID string `json:"turnId"`
	}
	if r.ContentLength != 0 {
		if err := decodeJSON(r, &input); err != nil {
			writeClientError(w, err)
			return
		}
	}
	result := a.store.Cancel(r.PathValue("sessionID"), input.TurnID)
	writeJSON(w, http.StatusOK, map[string]workflow.CancelResult{"status": result})
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
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{
		"scheduleId": id,
		"sessionIds": []string{sessionID},
	})
}

func decodeJSON(r *http.Request, target any) error {
	decoder := json.NewDecoder(io.LimitReader(r.Body, (1<<20)+1))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return fmt.Errorf("invalid JSON body: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return errors.New("invalid JSON body: expected exactly one value")
	}
	return nil
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeClientError(w http.ResponseWriter, err error) {
	writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
}

func writeError(w http.ResponseWriter, err error) {
	status := http.StatusInternalServerError
	if strings.Contains(err.Error(), "does not exist") || errors.Is(err, errors.ErrUnsupported) {
		status = http.StatusNotFound
	}
	writeJSON(w, status, map[string]string{"error": err.Error()})
}
