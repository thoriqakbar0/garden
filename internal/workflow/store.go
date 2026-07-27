// Package workflow provides a small durable event log for agent sessions.
package workflow

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Event is one immutable, ordered fact in a session history.
type Event struct {
	Index     int             `json:"index"`
	Type      string          `json:"type"`
	At        time.Time       `json:"at"`
	SessionID string          `json:"sessionId"`
	TurnID    string          `json:"turnId,omitempty"`
	Data      json.RawMessage `json:"data,omitempty"`
}

// TurnResult identifies a completed durable turn.
type TurnResult struct {
	SessionID string `json:"sessionId"`
	TurnID    string `json:"turnId"`
	Message   string `json:"message"`
}

// CancelResult reports whether an active turn accepted cancellation.
type CancelResult string

const (
	// CancelAccepted means the addressed active turn was cancelled.
	CancelAccepted CancelResult = "accepted"
	// CancelNoActiveTurn means no active turn matched the request.
	CancelNoActiveTurn CancelResult = "no_active_turn"
)

// Responder computes an assistant message. It must honor context cancellation.
type Responder func(context.Context, string, []Event) (string, error)

type stepRecorderKey struct{}

// RecordStep durably records internal execution state when called from a responder.
func RecordStep(ctx context.Context, eventType string, data any) error {
	recorder, ok := ctx.Value(stepRecorderKey{}).(func(string, any) error)
	if !ok {
		return errors.New("workflow step recorder is unavailable")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	return recorder(eventType, data)
}

// Store owns durable session logs and in-process turn concurrency.
type Store struct {
	root      string
	mu        sync.Mutex
	sessions  map[string]*sync.Mutex
	active    map[string]activeTurn
	responder Responder
}

type activeTurn struct {
	id     string
	cancel context.CancelFunc
}

// Open creates or opens a workflow store.
func Open(root string, responder Responder) (*Store, error) {
	if responder == nil {
		return nil, errors.New("workflow responder is required")
	}
	if err := os.MkdirAll(filepath.Join(root, "sessions"), 0o700); err != nil {
		return nil, fmt.Errorf("create workflow store: %w", err)
	}
	return &Store{
		root:      root,
		sessions:  make(map[string]*sync.Mutex),
		active:    make(map[string]activeTurn),
		responder: responder,
	}, nil
}

// EchoResponder is an explicit deterministic workflow diagnostic and test seam.
func EchoResponder(_ context.Context, message string, history []Event) (string, error) {
	turn := 1
	for _, event := range history {
		if event.Type == "turn.completed" {
			turn++
		}
	}
	return fmt.Sprintf("stress-ack:%d:%s", turn, message), nil
}

// CreateSession creates a session and persists its initial event before returning.
func (s *Store) CreateSession() (string, error) {
	id, err := newID("session")
	if err != nil {
		return "", err
	}
	lock := s.sessionLock(id)
	lock.Lock()
	defer lock.Unlock()
	if _, err := s.append(id, "", "session.started", nil); err != nil {
		return "", err
	}
	return id, nil
}

// Send serializes turns within a session while allowing independent sessions to run concurrently.
func (s *Store) Send(ctx context.Context, sessionID, message string) (TurnResult, error) {
	if message == "" {
		return TurnResult{}, errors.New("message must not be empty")
	}
	lock := s.sessionLock(sessionID)
	lock.Lock()
	defer lock.Unlock()
	if _, err := os.Stat(s.sessionPath(sessionID)); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return TurnResult{}, fmt.Errorf("session %q does not exist", sessionID)
		}
		return TurnResult{}, fmt.Errorf("inspect session: %w", err)
	}
	turnID, err := newID("turn")
	if err != nil {
		return TurnResult{}, err
	}
	turnContext, cancel := context.WithCancel(ctx)
	defer cancel()
	s.setActive(sessionID, activeTurn{id: turnID, cancel: cancel})
	defer s.clearActive(sessionID, turnID)

	if _, err := s.append(sessionID, turnID, "turn.started", map[string]string{"message": message}); err != nil {
		return TurnResult{}, err
	}
	turnContext = context.WithValue(turnContext, stepRecorderKey{}, func(eventType string, data any) error {
		_, err := s.append(sessionID, turnID, eventType, data)
		return err
	})
	history, err := s.Replay(sessionID, 0)
	if err != nil {
		return TurnResult{}, err
	}
	response, err := s.responder(turnContext, message, history)
	if err != nil {
		eventType := "turn.failed"
		if errors.Is(err, context.Canceled) {
			eventType = "turn.cancelled"
		}
		_, appendErr := s.append(sessionID, turnID, eventType, map[string]string{"error": safeError(err)})
		if appendErr != nil {
			return TurnResult{}, errors.Join(err, appendErr)
		}
		return TurnResult{}, err
	}
	if _, err := s.append(sessionID, turnID, "message.completed", map[string]string{"message": response}); err != nil {
		return TurnResult{}, err
	}
	if _, err := s.append(sessionID, turnID, "turn.completed", nil); err != nil {
		return TurnResult{}, err
	}
	return TurnResult{SessionID: sessionID, TurnID: turnID, Message: response}, nil
}

// Cancel cancels the active turn when turnID is empty or matches the observed active turn.
func (s *Store) Cancel(sessionID, turnID string) CancelResult {
	s.mu.Lock()
	defer s.mu.Unlock()
	active, ok := s.active[sessionID]
	if !ok || (turnID != "" && turnID != active.id) {
		return CancelNoActiveTurn
	}
	active.cancel()
	return CancelAccepted
}

// Replay returns persisted events beginning at startIndex.
func (s *Store) Replay(sessionID string, startIndex int) ([]Event, error) {
	if startIndex < 0 {
		return nil, errors.New("start index must not be negative")
	}
	file, err := os.Open(s.sessionPath(sessionID))
	if err != nil {
		return nil, fmt.Errorf("open session %q: %w", sessionID, err)
	}
	defer file.Close()
	var events []Event
	decoder := json.NewDecoder(bufio.NewReader(file))
	for {
		var event Event
		if err := decoder.Decode(&event); errors.Is(err, io.EOF) {
			break
		} else if err != nil {
			return nil, fmt.Errorf("decode session %q: %w", sessionID, err)
		}
		if event.Index >= startIndex {
			events = append(events, event)
		}
	}
	return events, nil
}

func (s *Store) append(sessionID, turnID, eventType string, data any) (Event, error) {
	events, err := s.Replay(sessionID, 0)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		var pathErr *os.PathError
		if !errors.As(err, &pathErr) || !errors.Is(pathErr.Err, os.ErrNotExist) {
			return Event{}, err
		}
		events = nil
	}
	var raw json.RawMessage
	if data != nil {
		raw, err = json.Marshal(data)
		if err != nil {
			return Event{}, fmt.Errorf("encode event data: %w", err)
		}
	}
	event := Event{
		Index: len(events), Type: eventType, At: time.Now().UTC(),
		SessionID: sessionID, TurnID: turnID, Data: raw,
	}
	file, err := os.OpenFile(s.sessionPath(sessionID), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return Event{}, fmt.Errorf("open session log: %w", err)
	}
	encoder := json.NewEncoder(file)
	if err := encoder.Encode(event); err != nil {
		file.Close()
		return Event{}, fmt.Errorf("append session event: %w", err)
	}
	if err := file.Sync(); err != nil {
		file.Close()
		return Event{}, fmt.Errorf("sync session event: %w", err)
	}
	if err := file.Close(); err != nil {
		return Event{}, fmt.Errorf("close session event: %w", err)
	}
	return event, nil
}

func (s *Store) sessionLock(id string) *sync.Mutex {
	s.mu.Lock()
	defer s.mu.Unlock()
	lock := s.sessions[id]
	if lock == nil {
		lock = &sync.Mutex{}
		s.sessions[id] = lock
	}
	return lock
}

func (s *Store) setActive(sessionID string, turn activeTurn) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.active[sessionID] = turn
}

func (s *Store) clearActive(sessionID, turnID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if active, ok := s.active[sessionID]; ok && active.id == turnID {
		delete(s.active, sessionID)
	}
}

func (s *Store) sessionPath(id string) string {
	return filepath.Join(s.root, "sessions", id+".jsonl")
}

func newID(prefix string) (string, error) {
	var value [12]byte
	if _, err := rand.Read(value[:]); err != nil {
		return "", fmt.Errorf("generate %s id: %w", prefix, err)
	}
	return prefix + "_" + hex.EncodeToString(value[:]), nil
}

func safeError(err error) string {
	if errors.Is(err, context.Canceled) {
		return "cancelled"
	}
	return "responder failed"
}
