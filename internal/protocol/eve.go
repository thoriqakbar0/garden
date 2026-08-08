// Package protocol defines the public Eve HTTP and event-stream contract used
// by Garden's local runtime.
package protocol

import (
	"encoding/json"
	"time"
)

const (
	// SessionIDHeader identifies the durable session accepted by a request.
	SessionIDHeader = "x-eve-session-id"
	// StreamFormatHeader identifies the event-stream framing.
	StreamFormatHeader = "x-eve-stream-format"
	// StreamVersionHeader identifies the Eve event protocol revision.
	StreamVersionHeader = "x-eve-stream-version"
	// MessageStreamContentType is Eve's canonical NDJSON media type.
	MessageStreamContentType = "application/x-ndjson; charset=utf-8"
	// MessageStreamFormat is the value of StreamFormatHeader.
	MessageStreamFormat = "ndjson"
	// MessageStreamVersion matches the official Eve baseline in UPSTREAM.md.
	MessageStreamVersion = "19"
)

// EventType is one public Eve workflow event discriminator.
type EventType string

const (
	SessionStarted   EventType = "session.started"
	TurnStarted      EventType = "turn.started"
	MessageReceived  EventType = "message.received"
	StepStarted      EventType = "step.started"
	MessageAppended  EventType = "message.appended"
	MessageCompleted EventType = "message.completed"
	StepCompleted    EventType = "step.completed"
	TurnCompleted    EventType = "turn.completed"
	TurnCancelled    EventType = "turn.cancelled"
	TurnFailed       EventType = "turn.failed"
	SessionWaiting   EventType = "session.waiting"
)

// Event is one immutable public fact in an Eve session stream.
type Event struct {
	Data json.RawMessage `json:"data,omitempty"`
	Meta EventMeta       `json:"meta"`
	Type EventType       `json:"type"`
}

// EventMeta is durable event metadata. Replays preserve the original value.
type EventMeta struct {
	At string `json:"at"`
}

// CreateSessionRequest starts a conversation-mode session.
type CreateSessionRequest struct {
	Message string `json:"message"`
}

// ContinueSessionRequest resumes a session through its channel-owned token.
type ContinueSessionRequest struct {
	ContinuationToken string `json:"continuationToken"`
	Message           string `json:"message"`
}

// SessionResponse is returned when a create or continuation is accepted.
type SessionResponse struct {
	ContinuationToken string `json:"continuationToken,omitempty"`
	OK                bool   `json:"ok"`
	SessionID         string `json:"sessionId"`
}

// CancelTurnRequest optionally guards cancellation with an observed turn ID.
type CancelTurnRequest struct {
	TurnID string `json:"turnId,omitempty"`
}

// CancelTurnResponse reports whether a cancellation hook was available.
type CancelTurnResponse struct {
	OK        bool   `json:"ok"`
	SessionID string `json:"sessionId"`
	Status    string `json:"status"`
}

// ErrorResponse is the stable JSON error envelope used by Eve routes.
type ErrorResponse struct {
	Error string `json:"error"`
	OK    bool   `json:"ok"`
}

// NewEvent constructs a timestamped event from a JSON-safe payload.
func NewEvent(eventType EventType, data any) Event {
	var raw json.RawMessage
	if data != nil {
		encoded, err := json.Marshal(data)
		if err != nil {
			panic("protocol event payload is not JSON serializable: " + err.Error())
		}
		raw = encoded
	}
	return Event{
		Data: raw,
		Meta: EventMeta{At: time.Now().UTC().Format(time.RFC3339Nano)},
		Type: eventType,
	}
}

// TurnData is shared by turn lifecycle events.
type TurnData struct {
	Sequence int    `json:"sequence"`
	TurnID   string `json:"turnId"`
}

// StepData is shared by model-step lifecycle events.
type StepData struct {
	Sequence  int    `json:"sequence"`
	StepIndex int    `json:"stepIndex"`
	TurnID    string `json:"turnId"`
}

// MessageReceivedData is the public projection of one inbound text message.
type MessageReceivedData struct {
	Message  string     `json:"message"`
	Parts    []TextPart `json:"parts"`
	Sequence int        `json:"sequence"`
	TurnID   string     `json:"turnId"`
}

// TextPart is one structured inbound text part.
type TextPart struct {
	Text string `json:"text"`
	Type string `json:"type"`
}

// MessageAppendedData carries one incremental assistant text delta.
type MessageAppendedData struct {
	MessageDelta string `json:"messageDelta"`
	MessageSoFar string `json:"messageSoFar"`
	Sequence     int    `json:"sequence"`
	StepIndex    int    `json:"stepIndex"`
	TurnID       string `json:"turnId"`
}

// MessageCompletedData carries the finalized assistant text block.
type MessageCompletedData struct {
	FinishReason string `json:"finishReason"`
	Message      string `json:"message"`
	Sequence     int    `json:"sequence"`
	StepIndex    int    `json:"stepIndex"`
	TurnID       string `json:"turnId"`
}

// StepCompletedData marks a successfully completed deterministic step.
type StepCompletedData struct {
	FinishReason string     `json:"finishReason"`
	Sequence     int        `json:"sequence"`
	StepIndex    int        `json:"stepIndex"`
	TurnID       string     `json:"turnId"`
	Usage        *StepUsage `json:"usage,omitempty"`
}

// StepUsage is the provider-neutral token accounting allowed by Eve v19.
type StepUsage struct {
	InputTokens      int `json:"inputTokens,omitempty"`
	OutputTokens     int `json:"outputTokens,omitempty"`
	CacheReadTokens  int `json:"cacheReadTokens,omitempty"`
	CacheWriteTokens int `json:"cacheWriteTokens,omitempty"`
}

// SessionWaitingData carries the token for the next user message.
type SessionWaitingData struct {
	ContinuationToken string `json:"continuationToken"`
	Wait              string `json:"wait"`
}
