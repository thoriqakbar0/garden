// Package workflow provides a crash-safe local event log for agent sessions.
package workflow

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

var sessionIDPattern = regexp.MustCompile(`^(?:ses|session)_[A-Za-z0-9_-]{1,128}$`)
var turnIDPattern = regexp.MustCompile(`^turn_[A-Za-z0-9_-]{1,128}$`)
var continuationTokenPattern = regexp.MustCompile(`^eve:[A-Za-z0-9][A-Za-z0-9._:-]{0,255}$`)

var (
	// ErrSessionNotFound means the addressed durable session does not exist.
	ErrSessionNotFound = errors.New("session does not exist")
	// ErrSessionBusy means the session already has an active turn.
	ErrSessionBusy = errors.New("session already has an active turn")
	// ErrInvalidContinuation means a continuation token is malformed.
	ErrInvalidContinuation = errors.New("invalid continuation token")
	// ErrInvalidSessionID means a session identifier is unsafe or malformed.
	ErrInvalidSessionID = errors.New("invalid session id")
	// ErrStoreInUse means another Garden process owns the workflow directory.
	ErrStoreInUse = errors.New("workflow store is already in use")
	// ErrStoreClosed means the store no longer accepts work.
	ErrStoreClosed = errors.New("workflow store is closed")
)

// Event is one immutable, ordered fact in a session history.
//
// Meta plus public Type/Data values match Eve's NDJSON envelope. Garden may
// persist richer internal Type/Data values; the HTTP server projects only the
// pinned public allowlist. Index, SessionID, and TurnID remain local fields.
type Event struct {
	Index     int             `json:"index"`
	Type      string          `json:"type"`
	Meta      EventMeta       `json:"meta"`
	SessionID string          `json:"sessionId"`
	TurnID    string          `json:"turnId,omitempty"`
	Data      json.RawMessage `json:"data,omitempty"`
}

type diskEvent struct {
	Index     int             `json:"index"`
	Type      string          `json:"type"`
	Meta      EventMeta       `json:"meta"`
	LegacyAt  string          `json:"at"`
	SessionID string          `json:"sessionId"`
	TurnID    string          `json:"turnId,omitempty"`
	Data      json.RawMessage `json:"data,omitempty"`
}

// EventMeta is durable metadata stamped when an event is appended.
type EventMeta struct {
	At string `json:"at"`
}

// Turn describes one model/tool loop invocation.
type Turn struct {
	SessionID string
	TurnID    string
	Message   string
	Sequence  int
	History   []Event
}

// Emit persists one turn-scoped runtime event before returning.
type Emit func(eventType string, data any) error

// Runner owns the model and tool execution loop for one turn.
type Runner interface {
	Run(context.Context, Turn, Emit) (string, error)
}

// RunnerFunc adapts a function to Runner.
type RunnerFunc func(context.Context, Turn, Emit) (string, error)

// Run invokes f.
func (f RunnerFunc) Run(ctx context.Context, turn Turn, emit Emit) (string, error) {
	return f(ctx, turn, emit)
}

// Responder is the legacy deterministic response seam used by compatibility tests.
type Responder func(context.Context, string, []Event) (string, error)

// TurnResult identifies a completed durable turn.
type TurnResult struct {
	SessionID string `json:"sessionId"`
	TurnID    string `json:"turnId"`
	Message   string `json:"message"`
}

// StartResult identifies an accepted asynchronous turn.
type StartResult struct {
	OK                bool   `json:"ok"`
	SessionID         string `json:"sessionId"`
	TurnID            string `json:"turnId"`
	ContinuationToken string `json:"continuationToken"`
}

// CancelResult reports whether an active turn accepted cancellation.
type CancelResult string

const (
	// CancelAccepted means the addressed active turn was cancelled.
	CancelAccepted CancelResult = "accepted"
	// CancelNoActiveTurn means no active turn matched the request.
	CancelNoActiveTurn CancelResult = "no_active_turn"
)

type activeTurn struct {
	id                  string
	sequence            int
	cancel              context.CancelFunc
	done                chan turnOutcome
	cancelRequested     bool
	cancelIntentDurable bool
	settling            bool
}

type turnOutcome struct {
	result TurnResult
	err    error
}

type sessionState struct {
	mu        sync.Mutex
	notify    chan struct{}
	nextIndex int
	events    []Event
	poisoned  error
}

// Store owns durable session logs, live-event notification, and turn cancellation.
type Store struct {
	rootFiles    *os.Root
	sessionFiles *os.Root
	runner       Runner
	lock         *os.File
	lifecycle    sync.RWMutex
	mu           sync.Mutex
	sessions     map[string]*sessionState
	active       map[string]*activeTurn
	tokens       map[string]string
	closed       bool
	workers      sync.WaitGroup
	closeOnce    sync.Once
	closeErr     error
}

// Open creates a store using the legacy responder seam.
func Open(root string, responder Responder) (*Store, error) {
	if responder == nil {
		responder = EchoResponder
	}
	return OpenRunner(root, ResponderRunner(responder))
}

// ResponderRunner adapts a one-shot responder to the same observable step and
// assistant-message event contract as a real model runner.
func ResponderRunner(responder Responder) Runner {
	return RunnerFunc(func(ctx context.Context, turn Turn, emit Emit) (string, error) {
		step := map[string]any{
			"sequence": turn.Sequence, "stepIndex": 0, "turnId": turn.TurnID,
		}
		if err := emit("step.started", step); err != nil {
			return "", err
		}
		message, err := responder(ctx, turn.Message, turn.History)
		if err != nil {
			return "", err
		}
		if err := emit("message.appended", map[string]any{
			"messageDelta": message, "messageSoFar": message,
			"sequence": turn.Sequence, "stepIndex": 0, "turnId": turn.TurnID,
		}); err != nil {
			return "", err
		}
		if err := emit("message.completed", map[string]any{
			"finishReason": "stop", "message": message,
			"sequence": turn.Sequence, "stepIndex": 0, "turnId": turn.TurnID,
		}); err != nil {
			return "", err
		}
		if err := emit("step.completed", map[string]any{
			"finishReason": "stop", "sequence": turn.Sequence,
			"stepIndex": 0, "turnId": turn.TurnID,
		}); err != nil {
			return "", err
		}
		return message, nil
	})
}

// OpenRunner creates a store, repairs incomplete tail writes, and settles turns
// interrupted by a previous process as RUNTIME_RESTARTED failures.
func OpenRunner(root string, runner Runner) (*Store, error) {
	if runner == nil {
		return nil, errors.New("workflow runner is required")
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("resolve workflow store: %w", err)
	}
	root = filepath.Clean(absRoot)
	sessionsRoot := filepath.Join(root, "sessions")
	if err := os.MkdirAll(sessionsRoot, 0o700); err != nil {
		return nil, fmt.Errorf("create workflow store: %w", err)
	}
	resolvedRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return nil, fmt.Errorf("resolve workflow store links: %w", err)
	}
	root = resolvedRoot
	sessionsRoot = filepath.Join(root, "sessions")
	if err := requireRealDirectory(sessionsRoot); err != nil {
		return nil, err
	}
	lock, err := acquireStoreLock(root)
	if err != nil {
		return nil, err
	}
	rootFiles, err := os.OpenRoot(root)
	if err != nil {
		_ = releaseStoreLock(lock)
		return nil, fmt.Errorf("open workflow root: %w", err)
	}
	sessionFiles, err := rootFiles.OpenRoot("sessions")
	if err != nil {
		_ = rootFiles.Close()
		_ = releaseStoreLock(lock)
		return nil, fmt.Errorf("open workflow sessions root: %w", err)
	}
	store := &Store{
		rootFiles:    rootFiles,
		sessionFiles: sessionFiles,
		runner:       runner,
		lock:         lock,
		sessions:     make(map[string]*sessionState),
		active:       make(map[string]*activeTurn),
		tokens:       make(map[string]string),
	}
	directory, err := sessionFiles.Open(".")
	if err != nil {
		_ = store.closeFiles()
		return nil, fmt.Errorf("scan workflow store: %w", err)
	}
	entries, err := directory.ReadDir(-1)
	closeDirectoryErr := directory.Close()
	if err != nil || closeDirectoryErr != nil {
		_ = store.closeFiles()
		return nil, fmt.Errorf("scan workflow store: %w", errors.Join(err, closeDirectoryErr))
	}
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".jsonl" {
			continue
		}
		info, err := sessionFiles.Lstat(entry.Name())
		if err != nil {
			_ = store.closeFiles()
			return nil, fmt.Errorf("inspect workflow entry %q: %w", entry.Name(), err)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			_ = store.closeFiles()
			return nil, fmt.Errorf("workflow entry %q must not be a symlink", entry.Name())
		}
		if !info.Mode().IsRegular() {
			_ = store.closeFiles()
			return nil, fmt.Errorf("workflow entry %q is not a regular file", entry.Name())
		}
		id := strings.TrimSuffix(entry.Name(), ".jsonl")
		if err := validateSessionID(id); err != nil {
			_ = store.closeFiles()
			return nil, fmt.Errorf("unsafe session log %q: %w", entry.Name(), err)
		}
		if err := store.recoverSession(id); err != nil {
			_ = store.closeFiles()
			return nil, err
		}
		if err := store.registerRecoveredToken(id); err != nil {
			_ = store.closeFiles()
			return nil, err
		}
	}
	return store, nil
}

// EchoResponder provides deterministic Eve stress-fixture semantics.
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
	token, err := newContinuationToken()
	if err != nil {
		return "", err
	}
	return s.createSession(token, false)
}

func (s *Store) createSession(token string, reuseOwner bool) (string, error) {
	s.lifecycle.RLock()
	defer s.lifecycle.RUnlock()
	id, err := newID("ses")
	if err != nil {
		return "", err
	}
	state := &sessionState{notify: make(chan struct{})}
	state.mu.Lock()
	defer state.mu.Unlock()
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return "", ErrStoreClosed
	}
	if owner := s.tokens[token]; owner != "" {
		s.mu.Unlock()
		if reuseOwner {
			return owner, nil
		}
		return "", errors.New("generated continuation token is already owned")
	}
	s.sessions[id] = state
	s.tokens[token] = id
	s.mu.Unlock()
	if _, err := s.appendLocked(id, "", "session.started", map[string]any{
		"continuationToken": token,
	}); err != nil {
		s.discardSessionClaim(id, token)
		return "", err
	}
	if err := s.syncSessionDirectory(); err != nil {
		s.discardSessionClaim(id, token)
		return "", err
	}
	return id, nil
}

// StartSession creates a session and begins its first turn asynchronously.
func (s *Store) StartSession(message string) (StartResult, error) {
	id, err := s.CreateSession()
	if err != nil {
		return StartResult{}, err
	}
	return s.start(context.Background(), id, "", message)
}

// Continue begins a follow-up turn using the current continuation token.
func (s *Store) Continue(sessionID, continuationToken, message string) (StartResult, error) {
	if err := validateContinuationToken(continuationToken); err != nil {
		return StartResult{}, err
	}
	ownerID := s.continuationOwner(continuationToken)
	if ownerID == "" {
		var err error
		ownerID, err = s.createSession(continuationToken, true)
		if err != nil {
			return StartResult{}, err
		}
	}
	return s.start(context.Background(), ownerID, continuationToken, message)
}

// Send runs one turn and waits for its result. It remains for the local CLI and
// deterministic compatibility tests; HTTP clients should use StartSession or Continue.
func (s *Store) Send(ctx context.Context, sessionID, message string) (TurnResult, error) {
	started, done, err := s.startWithDone(ctx, sessionID, "", message)
	if err != nil {
		return TurnResult{}, err
	}
	select {
	case outcome := <-done:
		return outcome.result, outcome.err
	case <-ctx.Done():
		_, _ = s.Cancel(context.Background(), sessionID, started.TurnID)
		outcome := <-done
		if outcome.err != nil {
			return TurnResult{}, outcome.err
		}
		return outcome.result, ctx.Err()
	}
}

func (s *Store) start(ctx context.Context, sessionID, token, message string) (StartResult, error) {
	started, _, err := s.startWithDone(ctx, sessionID, token, message)
	return started, err
}

func (s *Store) startWithDone(
	parent context.Context,
	sessionID string,
	token string,
	message string,
) (StartResult, <-chan turnOutcome, error) {
	s.lifecycle.RLock()
	defer s.lifecycle.RUnlock()
	if err := validateSessionID(sessionID); err != nil {
		return StartResult{}, nil, err
	}
	if strings.TrimSpace(message) == "" {
		return StartResult{}, nil, errors.New("message must not be empty")
	}
	state, ok := s.existingSession(sessionID)
	if !ok {
		return StartResult{}, nil, ErrSessionNotFound
	}
	state.mu.Lock()
	defer state.mu.Unlock()
	if state.poisoned != nil {
		return StartResult{}, nil, state.poisoned
	}

	events, err := s.replayLocked(sessionID, 0)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return StartResult{}, nil, ErrSessionNotFound
		}
		return StartResult{}, nil, err
	}
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return StartResult{}, nil, ErrStoreClosed
	}
	_, busy := s.active[sessionID]
	s.mu.Unlock()
	if busy {
		return StartResult{}, nil, ErrSessionBusy
	}
	expectedToken, err := continuationToken(events)
	if err != nil {
		return StartResult{}, nil, err
	}
	if token != "" && token != expectedToken {
		return StartResult{}, nil, ErrInvalidContinuation
	}
	sequence := turnSequence(events)
	turnID := fmt.Sprintf("turn_%d", sequence)
	if _, err := s.appendLocked(sessionID, turnID, "turn.started", map[string]any{
		"sequence":              sequence,
		"turnId":                turnID,
		"nextContinuationToken": expectedToken,
	}); err != nil {
		return StartResult{}, nil, err
	}
	if _, err := s.appendLocked(sessionID, turnID, "message.received", map[string]any{
		"message": message,
		"parts": []map[string]string{{
			"text": message,
			"type": "text",
		}},
		"sequence": sequence,
		"turnId":   turnID,
	}); err != nil {
		return StartResult{}, nil, err
	}

	runContext, cancel := context.WithCancel(parent)
	done := make(chan turnOutcome, 1)
	active := &activeTurn{
		id: turnID, sequence: sequence, cancel: cancel, done: done,
	}
	s.mu.Lock()
	s.active[sessionID] = active
	s.workers.Add(1)
	s.mu.Unlock()
	started := StartResult{
		OK: true, SessionID: sessionID, TurnID: turnID, ContinuationToken: expectedToken,
	}
	history := append([]Event(nil), events...)
	go s.execute(runContext, Turn{
		SessionID: sessionID,
		TurnID:    turnID,
		Message:   message,
		Sequence:  sequence,
		History:   history,
	}, expectedToken, active)
	return started, done, nil
}

func (s *Store) execute(ctx context.Context, turn Turn, nextToken string, active *activeTurn) {
	defer s.workers.Done()
	result := TurnResult{SessionID: turn.SessionID, TurnID: turn.TurnID}
	emit := func(eventType string, data any) error {
		state, exists := s.existingSession(turn.SessionID)
		if !exists {
			return ErrSessionNotFound
		}
		state.mu.Lock()
		defer state.mu.Unlock()
		if state.poisoned != nil {
			return state.poisoned
		}
		s.mu.Lock()
		current, owned := s.active[turn.SessionID]
		cancelled := !owned || current != active || active.cancelRequested || active.settling
		s.mu.Unlock()
		if cancelled {
			return context.Canceled
		}
		_, err := s.appendLocked(turn.SessionID, turn.TurnID, eventType, data)
		return err
	}
	message, runErr := s.runner.Run(ctx, turn, emit)
	result.Message = message

	s.mu.Lock()
	current, owned := s.active[turn.SessionID]
	cancelled := errors.Is(runErr, context.Canceled)
	if owned && current == active {
		cancelled = cancelled || active.cancelRequested
		active.settling = true
	}
	s.mu.Unlock()
	if cancelled && runErr == nil {
		runErr = context.Canceled
	}

	var settlementErr error
	state, exists := s.existingSession(turn.SessionID)
	if !exists {
		settlementErr = ErrSessionNotFound
	} else {
		state.mu.Lock()
		if runErr == nil {
			_, settlementErr = s.appendLocked(turn.SessionID, turn.TurnID, "turn.completed", map[string]any{
				"sequence": turn.Sequence,
				"turnId":   turn.TurnID,
			})
		} else if cancelled {
			_, appendErr := s.appendLocked(turn.SessionID, turn.TurnID, "turn.cancelled", map[string]any{
				"sequence": turn.Sequence,
				"turnId":   turn.TurnID,
			})
			settlementErr = errors.Join(settlementErr, appendErr)
		} else {
			_, appendErr := s.appendLocked(turn.SessionID, turn.TurnID, "turn.failed", map[string]any{
				"code":     "TURN_FAILED",
				"message":  "The turn failed.",
				"sequence": turn.Sequence,
				"turnId":   turn.TurnID,
			})
			settlementErr = errors.Join(settlementErr, appendErr)
		}
		if settlementErr == nil {
			_, settlementErr = s.appendLocked(turn.SessionID, turn.TurnID, "session.waiting", map[string]any{
				"continuationToken": nextToken,
				"wait":              "next-user-message",
			})
		}
		s.mu.Lock()
		if settlementErr == nil {
			current, ok := s.active[turn.SessionID]
			if ok && current == active {
				delete(s.active, turn.SessionID)
			}
		}
		s.mu.Unlock()
		state.mu.Unlock()
	}
	active.done <- turnOutcome{result: result, err: errors.Join(runErr, settlementErr)}
	close(active.done)
}

// Cancel durably records cancellation intent for the active turn when turnID
// is empty or matches it. Terminal settlement continues asynchronously.
func (s *Store) Cancel(ctx context.Context, sessionID, turnID string) (CancelResult, error) {
	if validateSessionID(sessionID) != nil {
		return CancelNoActiveTurn, nil
	}
	if err := ctx.Err(); err != nil {
		return CancelNoActiveTurn, err
	}
	state, ok := s.existingSession(sessionID)
	if !ok {
		return CancelNoActiveTurn, nil
	}
	state.mu.Lock()
	defer state.mu.Unlock()
	s.mu.Lock()
	active, ok := s.active[sessionID]
	if !ok || active.settling {
		s.mu.Unlock()
		return CancelNoActiveTurn, nil
	}
	// Eve treats a stale guarded cancel as a consumed no-op.
	if turnID != "" && turnID != active.id {
		s.mu.Unlock()
		return CancelAccepted, nil
	}
	if active.cancelIntentDurable {
		s.mu.Unlock()
		return CancelAccepted, nil
	}
	active.cancelRequested = true
	active.cancel()
	s.mu.Unlock()
	if _, err := s.appendLocked(sessionID, active.id, "turn.cancellation.requested", map[string]any{
		"sequence": active.sequence,
		"turnId":   active.id,
	}); err != nil {
		return CancelAccepted, err
	}
	s.mu.Lock()
	active.cancelIntentDurable = true
	s.mu.Unlock()
	return CancelAccepted, nil
}

// Replay returns persisted events beginning at an absolute or tail-relative cursor.
func (s *Store) Replay(sessionID string, startIndex int) ([]Event, error) {
	if err := validateSessionID(sessionID); err != nil {
		return nil, err
	}
	state, ok := s.existingSession(sessionID)
	if !ok {
		return nil, ErrSessionNotFound
	}
	state.mu.Lock()
	defer state.mu.Unlock()
	events, err := s.replayLocked(sessionID, startIndex)
	if errors.Is(err, os.ErrNotExist) {
		return nil, ErrSessionNotFound
	}
	return events, err
}

// WaitForEvents returns available events at startIndex or waits for an append.
func (s *Store) WaitForEvents(
	ctx context.Context,
	sessionID string,
	startIndex int,
) ([]Event, error) {
	if err := validateSessionID(sessionID); err != nil {
		return nil, err
	}
	if startIndex < 0 {
		return s.Replay(sessionID, startIndex)
	}
	state, ok := s.existingSession(sessionID)
	if !ok {
		return nil, ErrSessionNotFound
	}
	for {
		state.mu.Lock()
		events, err := s.replayLocked(sessionID, startIndex)
		if err != nil {
			state.mu.Unlock()
			if errors.Is(err, os.ErrNotExist) {
				return nil, ErrSessionNotFound
			}
			return nil, err
		}
		if len(events) > 0 {
			state.mu.Unlock()
			return events, nil
		}
		if state.poisoned != nil {
			err := state.poisoned
			state.mu.Unlock()
			return nil, err
		}
		s.mu.Lock()
		closed := s.closed
		s.mu.Unlock()
		if closed {
			state.mu.Unlock()
			return nil, ErrStoreClosed
		}
		wait := state.notify
		state.mu.Unlock()
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-wait:
		}
	}
}

func (s *Store) append(sessionID, turnID, eventType string, data any) (Event, error) {
	state, ok := s.existingSession(sessionID)
	if !ok {
		return Event{}, ErrSessionNotFound
	}
	state.mu.Lock()
	defer state.mu.Unlock()
	if state.poisoned != nil {
		return Event{}, state.poisoned
	}
	return s.appendLocked(sessionID, turnID, eventType, data)
}

func (s *Store) appendLocked(
	sessionID string,
	turnID string,
	eventType string,
	data any,
) (Event, error) {
	state, ok := s.existingSession(sessionID)
	if !ok {
		return Event{}, ErrSessionNotFound
	}
	if state.poisoned != nil {
		return Event{}, state.poisoned
	}
	var err error
	var raw json.RawMessage
	if data != nil {
		raw, err = json.Marshal(data)
		if err != nil {
			return Event{}, fmt.Errorf("encode event data: %w", err)
		}
	}
	event := Event{
		Index: state.nextIndex, Type: eventType, Meta: EventMeta{At: time.Now().UTC().Format(time.RFC3339Nano)},
		SessionID: sessionID, TurnID: turnID, Data: raw,
	}
	line, err := json.Marshal(event)
	if err != nil {
		return Event{}, fmt.Errorf("encode session event: %w", err)
	}
	line = append(line, '\n')
	file, err := s.openSessionFile(
		sessionID,
		os.O_CREATE|os.O_APPEND|os.O_WRONLY,
		0o600,
	)
	if err != nil {
		return Event{}, poisonSession(state, fmt.Errorf("open session log: %w", err))
	}
	if _, err := file.Write(line); err != nil {
		_ = file.Close()
		return Event{}, poisonSession(state, fmt.Errorf("append session event: %w", err))
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		return Event{}, poisonSession(state, fmt.Errorf("sync session event: %w", err))
	}
	if err := file.Close(); err != nil {
		return Event{}, poisonSession(state, fmt.Errorf("close session event: %w", err))
	}
	state.nextIndex++
	state.events = append(state.events, cloneEvent(event))
	close(state.notify)
	state.notify = make(chan struct{})
	return event, nil
}

func (s *Store) replayLocked(sessionID string, startIndex int) ([]Event, error) {
	state, ok := s.existingSession(sessionID)
	if !ok {
		return nil, ErrSessionNotFound
	}
	all := state.events
	if startIndex < 0 {
		startIndex = len(all) + startIndex
		if startIndex < 0 {
			startIndex = 0
		}
	}
	if startIndex >= len(all) {
		return []Event{}, nil
	}
	return cloneEvents(all[startIndex:]), nil
}

func (s *Store) recoverSession(sessionID string) error {
	file, err := s.openSessionFile(sessionID, os.O_RDWR, 0)
	if err != nil {
		return fmt.Errorf("open session %q for recovery: %w", sessionID, err)
	}
	data, err := io.ReadAll(file)
	if err != nil {
		_ = file.Close()
		return fmt.Errorf("read session %q: %w", sessionID, err)
	}
	lastNewline := bytes.LastIndexByte(data, '\n')
	complete := data
	if lastNewline != len(data)-1 {
		tail := data[lastNewline+1:]
		var event Event
		if len(bytes.TrimSpace(tail)) > 0 && json.Unmarshal(tail, &event) == nil {
			complete = append(append([]byte(nil), data...), '\n')
		} else {
			complete = append([]byte(nil), data[:lastNewline+1]...)
		}
		if err := file.Truncate(int64(len(complete))); err != nil {
			_ = file.Close()
			return fmt.Errorf("truncate session %q tail: %w", sessionID, err)
		}
		if len(complete) > len(data) {
			if _, err := file.WriteAt(complete[len(data):], int64(len(data))); err != nil {
				_ = file.Close()
				return fmt.Errorf("complete session %q tail: %w", sessionID, err)
			}
		}
		if err := file.Sync(); err != nil {
			_ = file.Close()
			return fmt.Errorf("sync repaired session %q: %w", sessionID, err)
		}
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close recovered session %q: %w", sessionID, err)
	}
	stored := make([]diskEvent, 0)
	decoder := json.NewDecoder(bytes.NewReader(complete))
	for {
		var event diskEvent
		if err := decoder.Decode(&event); errors.Is(err, io.EOF) {
			break
		} else if err != nil {
			return fmt.Errorf("decode session %q: %w", sessionID, err)
		}
		stored = append(stored, event)
	}
	events, legacy, err := normalizeDiskEvents(sessionID, stored)
	if err != nil {
		return fmt.Errorf("normalize session %q: %w", sessionID, err)
	}
	if len(events) == 0 || events[0].Type != "session.started" {
		return fmt.Errorf("session %q is missing session.started", sessionID)
	}
	for index, event := range events {
		if event.Index != index || event.SessionID != sessionID || event.Type == "" {
			return fmt.Errorf("session %q has corrupt event at index %d", sessionID, index)
		}
		if err := validateRecoveredEvent(event); err != nil {
			return fmt.Errorf("session %q has corrupt event at index %d: %w", sessionID, index, err)
		}
	}
	recovery, err := recoveryStateFor(events)
	if err != nil {
		return fmt.Errorf("session %q has corrupt lifecycle: %w", sessionID, err)
	}
	if legacy {
		if err := rewriteSessionLog(s.sessionFiles, s.sessionName(sessionID), events); err != nil {
			return fmt.Errorf("migrate legacy session %q: %w", sessionID, err)
		}
		if err := s.syncSessionDirectory(); err != nil {
			return err
		}
	}
	state := s.session(sessionID)
	state.mu.Lock()
	defer state.mu.Unlock()
	state.events = cloneEvents(events)
	state.nextIndex = len(events)
	if !recovery.active && !recovery.needsWaiting {
		return nil
	}
	if recovery.active {
		eventType := "turn.failed"
		data := map[string]any{
			"code":     "RUNTIME_RESTARTED",
			"message":  "The runtime restarted before the turn settled.",
			"sequence": recovery.sequence,
			"turnId":   recovery.turnID,
		}
		if recovery.cancelRequested {
			eventType = "turn.cancelled"
			data = map[string]any{
				"sequence": recovery.sequence,
				"turnId":   recovery.turnID,
			}
		}
		if _, err := s.appendLocked(sessionID, recovery.turnID, eventType, data); err != nil {
			return err
		}
	}
	if _, err := s.appendLocked(sessionID, recovery.turnID, "session.waiting", map[string]any{
		"continuationToken": recovery.token,
		"wait":              "next-user-message",
	}); err != nil {
		return err
	}
	return nil
}

type recoveryState struct {
	turnID          string
	sequence        int
	token           string
	active          bool
	cancelRequested bool
	needsWaiting    bool
}

func normalizeDiskEvents(sessionID string, stored []diskEvent) ([]Event, bool, error) {
	legacy := false
	modern := false
	for index, event := range stored {
		if event.Index != index || event.SessionID != sessionID || event.Type == "" {
			return nil, false, fmt.Errorf("corrupt stored envelope at index %d", index)
		}
		legacy = legacy || event.LegacyAt != ""
		modern = modern || event.Meta.At != ""
	}
	if legacy && modern {
		return nil, false, errors.New("legacy and current event formats are mixed")
	}
	if !legacy {
		events := make([]Event, len(stored))
		for index, event := range stored {
			events[index] = Event{
				Index: event.Index, Type: event.Type, Meta: event.Meta,
				SessionID: event.SessionID, TurnID: event.TurnID,
				Data: append(json.RawMessage(nil), event.Data...),
			}
		}
		return events, false, nil
	}

	token, err := newContinuationToken()
	if err != nil {
		return nil, false, err
	}
	events := make([]Event, 0, len(stored)*2)
	sequences := make(map[string]int)
	nextSequence := 0
	appendEvent := func(event Event) {
		event.Index = len(events)
		event.SessionID = sessionID
		events = append(events, event)
	}
	for _, storedEvent := range stored {
		meta := EventMeta{At: storedEvent.LegacyAt}
		switch storedEvent.Type {
		case "session.started":
			appendEvent(Event{
				Type: "session.started", Meta: meta,
				Data: mustJSON(map[string]any{"continuationToken": token}),
			})
		case "turn.started":
			if !turnIDPattern.MatchString(storedEvent.TurnID) {
				return nil, false, errors.New("legacy turn ID is invalid")
			}
			var data struct {
				Message string `json:"message"`
			}
			if err := json.Unmarshal(storedEvent.Data, &data); err != nil || strings.TrimSpace(data.Message) == "" {
				return nil, false, errors.New("legacy turn message is invalid")
			}
			sequence := nextSequence
			nextSequence++
			sequences[storedEvent.TurnID] = sequence
			appendEvent(Event{
				Type: "turn.started", Meta: meta, TurnID: storedEvent.TurnID,
				Data: mustJSON(map[string]any{
					"sequence": sequence, "turnId": storedEvent.TurnID,
					"nextContinuationToken": token,
				}),
			})
			appendEvent(Event{
				Type: "message.received", Meta: meta, TurnID: storedEvent.TurnID,
				Data: mustJSON(map[string]any{
					"message":  data.Message,
					"parts":    []map[string]string{{"text": data.Message, "type": "text"}},
					"sequence": sequence, "turnId": storedEvent.TurnID,
				}),
			})
		case "message.completed":
			sequence, ok := sequences[storedEvent.TurnID]
			if !ok {
				return nil, false, errors.New("legacy message has no turn")
			}
			var data struct {
				Message string `json:"message"`
			}
			if err := json.Unmarshal(storedEvent.Data, &data); err != nil {
				return nil, false, errors.New("legacy assistant message is invalid")
			}
			step := map[string]any{
				"sequence": sequence, "stepIndex": 0, "turnId": storedEvent.TurnID,
			}
			appendEvent(Event{
				Type: "step.started", Meta: meta, TurnID: storedEvent.TurnID,
				Data: mustJSON(step),
			})
			appendEvent(Event{
				Type: "message.appended", Meta: meta, TurnID: storedEvent.TurnID,
				Data: mustJSON(map[string]any{
					"messageDelta": data.Message, "messageSoFar": data.Message,
					"sequence": sequence, "stepIndex": 0, "turnId": storedEvent.TurnID,
				}),
			})
			appendEvent(Event{
				Type: "message.completed", Meta: meta, TurnID: storedEvent.TurnID,
				Data: mustJSON(map[string]any{
					"finishReason": "stop", "message": data.Message,
					"sequence": sequence, "stepIndex": 0, "turnId": storedEvent.TurnID,
				}),
			})
			appendEvent(Event{
				Type: "step.completed", Meta: meta, TurnID: storedEvent.TurnID,
				Data: mustJSON(map[string]any{
					"finishReason": "stop", "sequence": sequence,
					"stepIndex": 0, "turnId": storedEvent.TurnID,
				}),
			})
		case "turn.completed", "turn.failed", "turn.cancelled":
			sequence, ok := sequences[storedEvent.TurnID]
			if !ok {
				return nil, false, errors.New("legacy terminal event has no turn")
			}
			data := map[string]any{"sequence": sequence, "turnId": storedEvent.TurnID}
			if storedEvent.Type == "turn.failed" {
				data["code"] = "LEGACY_TURN_FAILED"
				data["message"] = "The legacy turn failed."
			}
			appendEvent(Event{
				Type: storedEvent.Type, Meta: meta, TurnID: storedEvent.TurnID,
				Data: mustJSON(data),
			})
			appendEvent(Event{
				Type: "session.waiting", Meta: meta, TurnID: storedEvent.TurnID,
				Data: mustJSON(map[string]any{
					"continuationToken": token, "wait": "next-user-message",
				}),
			})
		default:
			var legacyData any
			if len(bytes.TrimSpace(storedEvent.Data)) > 0 {
				if err := json.Unmarshal(storedEvent.Data, &legacyData); err != nil {
					return nil, false, errors.New("legacy internal event data is invalid")
				}
			}
			appendEvent(Event{
				Type: "legacy." + storedEvent.Type, Meta: meta,
				TurnID: storedEvent.TurnID,
				Data:   mustJSON(map[string]any{"legacyData": legacyData}),
			})
		}
	}
	return events, true, nil
}

func mustJSON(value any) json.RawMessage {
	encoded, err := json.Marshal(value)
	if err != nil {
		panic("workflow migration payload is not JSON serializable: " + err.Error())
	}
	return encoded
}

func rewriteSessionLog(root *os.Root, name string, events []Event) error {
	temporaryID, err := newID("migrate")
	if err != nil {
		return err
	}
	temporary := ".garden-" + temporaryID
	file, err := root.OpenFile(temporary, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer root.Remove(temporary)
	encoder := json.NewEncoder(file)
	for _, event := range events {
		if err := encoder.Encode(event); err != nil {
			_ = file.Close()
			return err
		}
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	return root.Rename(temporary, name)
}

func recoveryStateFor(events []Event) (recoveryState, error) {
	var state recoveryState
	sessionStarted := false
	nextSequence := 0
	seenTurnIDs := make(map[string]struct{})
	for index, event := range events {
		switch event.Type {
		case "session.started":
			if sessionStarted || index != 0 || event.TurnID != "" {
				return recoveryState{}, errors.New("session.started must be the first and only session start")
			}
			var data struct {
				ContinuationToken string `json:"continuationToken"`
			}
			if err := json.Unmarshal(event.Data, &data); err != nil {
				return recoveryState{}, err
			}
			state.token = data.ContinuationToken
			sessionStarted = true
		case "session.waiting":
			if !sessionStarted || !state.needsWaiting {
				return recoveryState{}, errors.New("session.waiting has no unsettled turn boundary")
			}
			if event.TurnID != state.turnID {
				return recoveryState{}, errors.New("session.waiting does not match the settled turn")
			}
			var data struct {
				ContinuationToken string `json:"continuationToken"`
			}
			if err := json.Unmarshal(event.Data, &data); err != nil {
				return recoveryState{}, err
			}
			if data.ContinuationToken != state.token {
				return recoveryState{}, errors.New("session.waiting changes the continuation token")
			}
			state.needsWaiting = false
		case "turn.started":
			if !sessionStarted || state.active || state.needsWaiting {
				return recoveryState{}, errors.New("turn started before the previous boundary settled")
			}
			if _, seen := seenTurnIDs[event.TurnID]; seen {
				return recoveryState{}, errors.New("turn ID is reused")
			}
			var data struct {
				NextContinuationToken string `json:"nextContinuationToken"`
			}
			if err := json.Unmarshal(event.Data, &data); err != nil {
				return recoveryState{}, err
			}
			sequence, err := recoveredTurnSequence(event)
			if err != nil {
				return recoveryState{}, err
			}
			if sequence != nextSequence {
				return recoveryState{}, errors.New("turn sequence is not contiguous")
			}
			if data.NextContinuationToken != state.token {
				return recoveryState{}, errors.New("turn changes the continuation token")
			}
			state.turnID = event.TurnID
			state.sequence = sequence
			state.active = true
			state.cancelRequested = false
			state.needsWaiting = false
			seenTurnIDs[event.TurnID] = struct{}{}
			nextSequence++
		case "turn.cancellation.requested":
			if !state.active || event.TurnID != state.turnID {
				return recoveryState{}, errors.New("cancellation intent does not match the active turn")
			}
			sequence, err := recoveredTurnSequence(event)
			if err != nil {
				return recoveryState{}, err
			}
			if sequence != state.sequence || state.cancelRequested {
				return recoveryState{}, errors.New("cancellation intent does not match the active sequence")
			}
			state.cancelRequested = true
		case "turn.completed", "turn.failed", "turn.cancelled":
			if !state.active || event.TurnID != state.turnID {
				return recoveryState{}, errors.New("terminal event does not match the active turn")
			}
			sequence, err := recoveredTurnSequence(event)
			if err != nil {
				return recoveryState{}, err
			}
			if sequence != state.sequence {
				return recoveryState{}, errors.New("terminal event does not match the active sequence")
			}
			if state.cancelRequested && event.Type != "turn.cancelled" {
				return recoveryState{}, errors.New("cancelled turn has a non-cancellation terminal event")
			}
			state.active = false
			state.needsWaiting = true
		case "message.received", "step.started", "message.appended", "message.completed",
			"step.completed", "step.failed", "actions.requested", "action.result":
			if !state.active || state.cancelRequested || event.TurnID != state.turnID {
				return recoveryState{}, errors.New("turn event does not match the active turn")
			}
			sequence, err := recoveredTurnSequence(event)
			if err != nil {
				return recoveryState{}, err
			}
			if sequence != state.sequence {
				return recoveryState{}, errors.New("turn event does not match the active sequence")
			}
		default:
			if event.TurnID != "" &&
				(!state.active || state.cancelRequested || event.TurnID != state.turnID) {
				return recoveryState{}, errors.New("internal event does not match the active turn")
			}
		}
	}
	if !sessionStarted {
		return recoveryState{}, errors.New("session has no start event")
	}
	return state, nil
}

func validateRecoveredEvent(event Event) error {
	if _, err := time.Parse(time.RFC3339Nano, event.Meta.At); err != nil {
		return errors.New("event timestamp is invalid")
	}
	trimmed := bytes.TrimSpace(event.Data)
	if len(trimmed) < 2 || trimmed[0] != '{' || trimmed[len(trimmed)-1] != '}' || !json.Valid(trimmed) {
		return errors.New("event data is not a JSON object")
	}
	switch event.Type {
	case "session.started", "session.waiting":
		var data struct {
			ContinuationToken string `json:"continuationToken"`
		}
		if err := json.Unmarshal(event.Data, &data); err != nil {
			return err
		}
		return validateContinuationToken(data.ContinuationToken)
	case "turn.started":
		var data struct {
			NextContinuationToken string `json:"nextContinuationToken"`
		}
		if err := json.Unmarshal(event.Data, &data); err != nil {
			return err
		}
		if _, err := recoveredTurnSequence(event); err != nil {
			return err
		}
		return validateContinuationToken(data.NextContinuationToken)
	case "turn.completed", "turn.failed", "turn.cancelled", "turn.cancellation.requested",
		"message.received", "step.started", "message.appended", "message.completed",
		"step.completed", "step.failed", "actions.requested", "action.result":
		_, err := recoveredTurnSequence(event)
		return err
	}
	return nil
}

func recoveredTurnSequence(event Event) (int, error) {
	var identity struct {
		Sequence *int   `json:"sequence"`
		TurnID   string `json:"turnId"`
	}
	if err := json.Unmarshal(event.Data, &identity); err != nil {
		return 0, err
	}
	if identity.Sequence == nil || *identity.Sequence < 0 ||
		identity.TurnID != event.TurnID || !turnIDPattern.MatchString(identity.TurnID) {
		return 0, errors.New("turn event identity is invalid")
	}
	return *identity.Sequence, nil
}

func continuationToken(events []Event) (string, error) {
	var token string
	for _, event := range events {
		if event.Type != "session.started" && event.Type != "session.waiting" {
			continue
		}
		var data struct {
			ContinuationToken string `json:"continuationToken"`
		}
		if err := json.Unmarshal(event.Data, &data); err != nil {
			return "", fmt.Errorf("decode %s continuation token: %w", event.Type, err)
		}
		if err := validateContinuationToken(data.ContinuationToken); err != nil {
			return "", fmt.Errorf("decode %s continuation token: %w", event.Type, err)
		}
		token = data.ContinuationToken
	}
	if token == "" {
		return "", errors.New("session has no continuation token")
	}
	return token, nil
}

func turnSequence(events []Event) int {
	sequence := 0
	for _, event := range events {
		if event.Type == "turn.started" {
			sequence++
		}
	}
	return sequence
}

func cloneEvent(event Event) Event {
	event.Data = append(json.RawMessage(nil), event.Data...)
	return event
}

func cloneEvents(events []Event) []Event {
	cloned := make([]Event, len(events))
	for index, event := range events {
		cloned[index] = cloneEvent(event)
	}
	return cloned
}

func poisonSession(state *sessionState, err error) error {
	if state.poisoned == nil {
		state.poisoned = err
		close(state.notify)
		state.notify = make(chan struct{})
	}
	return state.poisoned
}

func (s *Store) session(id string) *sessionState {
	s.mu.Lock()
	defer s.mu.Unlock()
	state := s.sessions[id]
	if state == nil {
		state = &sessionState{notify: make(chan struct{})}
		s.sessions[id] = state
	}
	return state
}

func (s *Store) existingSession(id string) (*sessionState, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	state, ok := s.sessions[id]
	return state, ok
}

func (s *Store) continuationOwner(token string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.tokens[token]
}

func (s *Store) registerRecoveredToken(sessionID string) error {
	state, ok := s.existingSession(sessionID)
	if !ok {
		return ErrSessionNotFound
	}
	state.mu.Lock()
	events, err := s.replayLocked(sessionID, 0)
	state.mu.Unlock()
	if err != nil {
		return err
	}
	token, err := continuationToken(events)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if owner := s.tokens[token]; owner != "" && owner != sessionID {
		return fmt.Errorf("continuation token is owned by both %q and %q", owner, sessionID)
	}
	s.tokens[token] = sessionID
	return nil
}

func (s *Store) rollbackSessionClaim(sessionID, token string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.sessions, sessionID)
	if s.tokens[token] == sessionID {
		delete(s.tokens, token)
	}
}

func (s *Store) discardSessionClaim(sessionID, token string) {
	_ = s.sessionFiles.Remove(s.sessionName(sessionID))
	s.rollbackSessionClaim(sessionID, token)
}

func (s *Store) sessionName(id string) string {
	return id + ".jsonl"
}

func (s *Store) openSessionFile(id string, flags int, perm os.FileMode) (*os.File, error) {
	if err := validateSessionID(id); err != nil {
		return nil, err
	}
	name := s.sessionName(id)
	info, err := s.sessionFiles.Lstat(name)
	if err == nil {
		if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
			return nil, fmt.Errorf("session log %q is not a regular file", id)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	file, err := s.sessionFiles.OpenFile(name, flags, perm)
	if err != nil {
		return nil, err
	}
	opened, err := file.Stat()
	if err != nil {
		_ = file.Close()
		return nil, err
	}
	if !opened.Mode().IsRegular() {
		_ = file.Close()
		return nil, fmt.Errorf("session log %q is not a regular file", id)
	}
	return file, nil
}

func (s *Store) syncSessionDirectory() error {
	directory, err := s.sessionFiles.Open(".")
	if err != nil {
		return fmt.Errorf("open workflow directory: %w", err)
	}
	if err := directory.Sync(); err != nil {
		_ = directory.Close()
		return fmt.Errorf("sync workflow directory: %w", err)
	}
	if err := directory.Close(); err != nil {
		return fmt.Errorf("close workflow directory: %w", err)
	}
	return nil
}

func (s *Store) closeFiles() error {
	return errors.Join(
		s.sessionFiles.Close(),
		s.rootFiles.Close(),
		releaseStoreLock(s.lock),
	)
}

func validateSessionID(id string) error {
	if !sessionIDPattern.MatchString(id) {
		return fmt.Errorf("%w: %q", ErrInvalidSessionID, id)
	}
	return nil
}

func validateContinuationToken(token string) error {
	if !continuationTokenPattern.MatchString(token) {
		return fmt.Errorf("%w: malformed token", ErrInvalidContinuation)
	}
	return nil
}

func newID(prefix string) (string, error) {
	var value [12]byte
	if _, err := rand.Read(value[:]); err != nil {
		return "", fmt.Errorf("generate %s id: %w", prefix, err)
	}
	return prefix + "_" + hex.EncodeToString(value[:]), nil
}

func newContinuationToken() (string, error) {
	var value [16]byte
	if _, err := rand.Read(value[:]); err != nil {
		return "", fmt.Errorf("generate continuation token: %w", err)
	}
	hexValue := hex.EncodeToString(value[:])
	return fmt.Sprintf(
		"eve:%s-%s-%s-%s-%s",
		hexValue[0:8],
		hexValue[8:12],
		hexValue[12:16],
		hexValue[16:20],
		hexValue[20:32],
	), nil
}

// Close cancels owned turns, waits for them to settle, and releases the writer lock.
func (s *Store) Close() error {
	s.closeOnce.Do(func() {
		s.lifecycle.Lock()
		defer s.lifecycle.Unlock()
		s.mu.Lock()
		s.closed = true
		active := make([]*activeTurn, 0, len(s.active))
		for _, turn := range s.active {
			turn.cancelRequested = true
			active = append(active, turn)
		}
		s.mu.Unlock()
		for _, turn := range active {
			turn.cancel()
		}
		s.workers.Wait()
		s.mu.Lock()
		states := make([]*sessionState, 0, len(s.sessions))
		for _, state := range s.sessions {
			states = append(states, state)
		}
		s.mu.Unlock()
		for _, state := range states {
			state.mu.Lock()
			close(state.notify)
			state.notify = make(chan struct{})
			state.mu.Unlock()
		}
		s.closeErr = s.closeFiles()
	})
	return s.closeErr
}

func requireRealDirectory(path string) error {
	info, err := os.Lstat(path)
	if err != nil {
		return fmt.Errorf("inspect workflow directory: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return fmt.Errorf("workflow sessions path %q must be a real directory", path)
	}
	return nil
}
