package workflow

import (
	"encoding/json"
	"fmt"
	"strings"
)

// Step identifies one model step inside a durable turn.
type Step struct {
	Sequence  int
	StepIndex int
	TurnID    string
}

// ProviderMetadata is the safe provider identity persisted with model events.
type ProviderMetadata struct {
	API        string `json:"api"`
	Model      string `json:"model"`
	Provider   string `json:"provider"`
	ResponseID string `json:"responseId"`
}

// TokenUsage is provider-neutral token accounting persisted with model events.
type TokenUsage struct {
	InputTokens      int `json:"inputTokens"`
	OutputTokens     int `json:"outputTokens"`
	CacheReadTokens  int `json:"cacheReadTokens"`
	CacheWriteTokens int `json:"cacheWriteTokens"`
	TotalTokens      int `json:"totalTokens"`
}

// CompletionMetadata is optional model metadata for a completed message or step.
type CompletionMetadata struct {
	Provider *ProviderMetadata
	Usage    *TokenUsage
}

// ActionRequest is one durable tool request emitted by a runner.
type ActionRequest struct {
	CallID       string          `json:"callId"`
	Input        json.RawMessage `json:"input"`
	Kind         string          `json:"kind"`
	ProviderData json.RawMessage `json:"providerData,omitempty"`
	ToolName     string          `json:"toolName"`
}

// ActionResult is one durable tool result emitted by a runner.
type ActionResult struct {
	CallID   string          `json:"callId"`
	IsError  bool            `json:"isError,omitempty"`
	Kind     string          `json:"kind"`
	Output   json.RawMessage `json:"output"`
	ToolName string          `json:"toolName"`
}

// ActionFailure is the safe failure projection for a failed tool result.
type ActionFailure struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// RunnerEvent is an opaque, typed event that a Runner can emit.
//
// Callers construct values through the event constructors in this package.
type RunnerEvent struct {
	eventType string
	data      any
	step      Step
}

// Type returns the stable durable event discriminator.
func (event RunnerEvent) Type() string {
	return event.eventType
}

// Payload returns a detached JSON projection for tests and diagnostics.
func (event RunnerEvent) Payload() (json.RawMessage, error) {
	data, err := json.Marshal(event.data)
	if err != nil {
		return nil, err
	}
	return append(json.RawMessage(nil), data...), nil
}

type stepData struct {
	Sequence  int    `json:"sequence"`
	StepIndex int    `json:"stepIndex"`
	TurnID    string `json:"turnId"`
}

type stepFailedData struct {
	stepData
	Code    string `json:"code"`
	Message string `json:"message"`
}

type messageAppendedData struct {
	stepData
	MessageDelta string `json:"messageDelta"`
	MessageSoFar string `json:"messageSoFar"`
}

type messageCompletedData struct {
	stepData
	FinishReason     string            `json:"finishReason"`
	Message          string            `json:"message"`
	ProviderMetadata *ProviderMetadata `json:"providerMetadata,omitempty"`
	Usage            *TokenUsage       `json:"usage,omitempty"`
}

type actionsRequestedData struct {
	stepData
	Actions []ActionRequest `json:"actions"`
}

type stepCompletedData struct {
	stepData
	FinishReason     string            `json:"finishReason"`
	ProviderMetadata *ProviderMetadata `json:"providerMetadata,omitempty"`
	Usage            *TokenUsage       `json:"usage,omitempty"`
}

type actionResultData struct {
	stepData
	Error  *ActionFailure `json:"error,omitempty"`
	Result ActionResult   `json:"result"`
	Status string         `json:"status"`
}

type runnerStepPhase uint8

const (
	stepPhaseStarted runnerStepPhase = iota
	stepPhaseMessageStreaming
	stepPhaseMessageCompleted
	stepPhaseActionsRequested
	stepPhaseActionsRequestedMessageStreaming
	stepPhaseActionsRequestedMessageCompleted
	stepPhaseCompletedToolCalls
	stepPhaseCompletedFinal
	stepPhaseFailed
)

type runnerStepState struct {
	phase runnerStepPhase
}

type runnerEventSequence struct {
	nextStep    int
	pending     map[string]pendingAction
	seenActions map[string]struct{}
	steps       map[int]*runnerStepState
}

type pendingAction struct {
	stepIndex int
	toolName  string
}

func newRunnerEventSequence() *runnerEventSequence {
	return &runnerEventSequence{
		pending:     make(map[string]pendingAction),
		seenActions: make(map[string]struct{}),
		steps:       make(map[int]*runnerStepState),
	}
}

func (sequence *runnerEventSequence) accept(event RunnerEvent, turn Turn) bool {
	if !event.validFor(turn) {
		return false
	}
	step, exists := sequence.steps[event.step.StepIndex]
	switch data := event.data.(type) {
	case stepData:
		if exists || event.step.StepIndex != sequence.nextStep {
			return false
		}
		if sequence.nextStep > 0 {
			previous := sequence.steps[sequence.nextStep-1]
			if previous.phase != stepPhaseCompletedToolCalls {
				return false
			}
		}
		sequence.steps[event.step.StepIndex] = &runnerStepState{phase: stepPhaseStarted}
		sequence.nextStep++
		return true
	case stepFailedData:
		if !exists || (step.phase != stepPhaseStarted &&
			step.phase != stepPhaseMessageStreaming &&
			step.phase != stepPhaseMessageCompleted) {
			return false
		}
		step.phase = stepPhaseFailed
		return true
	case messageAppendedData:
		if !exists {
			return false
		}
		switch step.phase {
		case stepPhaseStarted, stepPhaseMessageStreaming:
			step.phase = stepPhaseMessageStreaming
		case stepPhaseActionsRequested, stepPhaseActionsRequestedMessageStreaming:
			step.phase = stepPhaseActionsRequestedMessageStreaming
		default:
			return false
		}
		return true
	case messageCompletedData:
		if !exists {
			return false
		}
		switch step.phase {
		case stepPhaseMessageStreaming:
			step.phase = stepPhaseMessageCompleted
		case stepPhaseActionsRequestedMessageStreaming:
			step.phase = stepPhaseActionsRequestedMessageCompleted
		default:
			return false
		}
		return true
	case actionsRequestedData:
		if !exists {
			return false
		}
		var nextPhase runnerStepPhase
		switch step.phase {
		case stepPhaseStarted:
			nextPhase = stepPhaseActionsRequested
		case stepPhaseMessageStreaming:
			nextPhase = stepPhaseActionsRequestedMessageStreaming
		case stepPhaseMessageCompleted:
			nextPhase = stepPhaseActionsRequestedMessageCompleted
		default:
			return false
		}
		batch := make(map[string]ActionRequest, len(data.Actions))
		for _, action := range data.Actions {
			if _, seen := sequence.seenActions[action.CallID]; seen {
				return false
			}
			if _, duplicate := batch[action.CallID]; duplicate {
				return false
			}
			batch[action.CallID] = action
		}
		for callID, action := range batch {
			sequence.pending[callID] = pendingAction{
				stepIndex: event.step.StepIndex,
				toolName:  action.ToolName,
			}
			sequence.seenActions[callID] = struct{}{}
		}
		step.phase = nextPhase
		return true
	case stepCompletedData:
		if !exists {
			return false
		}
		if data.FinishReason == "tool-calls" {
			if step.phase != stepPhaseActionsRequested &&
				step.phase != stepPhaseActionsRequestedMessageStreaming &&
				step.phase != stepPhaseActionsRequestedMessageCompleted {
				return false
			}
			step.phase = stepPhaseCompletedToolCalls
		} else if step.phase == stepPhaseMessageCompleted {
			step.phase = stepPhaseCompletedFinal
		} else {
			return false
		}
		return true
	case actionResultData:
		if !exists || step.phase != stepPhaseCompletedToolCalls {
			return false
		}
		request, requested := sequence.pending[data.Result.CallID]
		if !requested || request.stepIndex != event.step.StepIndex || request.toolName != data.Result.ToolName {
			return false
		}
		delete(sequence.pending, data.Result.CallID)
		return true
	default:
		return false
	}
}

func (sequence *runnerEventSequence) complete() bool {
	if sequence.nextStep == 0 || len(sequence.pending) != 0 {
		return false
	}
	last := sequence.steps[sequence.nextStep-1]
	return last.phase == stepPhaseCompletedFinal
}

func (sequence *runnerEventSequence) empty() bool {
	return sequence.nextStep == 0
}

func dataForStep(step Step) stepData {
	return stepData{Sequence: step.Sequence, StepIndex: step.StepIndex, TurnID: step.TurnID}
}

func (event RunnerEvent) validFor(turn Turn) bool {
	if event.data == nil || event.step.Sequence != turn.Sequence || event.step.TurnID != turn.TurnID ||
		event.step.StepIndex < 0 {
		return false
	}
	nonempty := func(values ...string) bool {
		for _, value := range values {
			if strings.TrimSpace(value) == "" {
				return false
			}
		}
		return true
	}
	switch data := event.data.(type) {
	case stepData:
		return event.eventType == "step.started"
	case stepFailedData:
		return event.eventType == "step.failed" && nonempty(data.Code, data.Message)
	case messageAppendedData:
		return event.eventType == "message.appended" && nonempty(data.MessageDelta, data.MessageSoFar)
	case messageCompletedData:
		return event.eventType == "message.completed" && nonempty(data.Message, data.FinishReason)
	case actionsRequestedData:
		if event.eventType != "actions.requested" || len(data.Actions) == 0 {
			return false
		}
		for _, action := range data.Actions {
			if !nonempty(action.CallID, action.ToolName) || action.Kind != "tool-call" || !json.Valid(action.Input) ||
				(len(action.ProviderData) > 0 && !json.Valid(action.ProviderData)) {
				return false
			}
		}
		return true
	case stepCompletedData:
		return event.eventType == "step.completed" && nonempty(data.FinishReason)
	case actionResultData:
		if event.eventType != "action.result" || !nonempty(
			data.Status,
			data.Result.CallID,
			data.Result.ToolName,
		) || data.Result.Kind != "tool-result" || !json.Valid(data.Result.Output) {
			return false
		}
		if data.Error == nil {
			return data.Status == "completed" && !data.Result.IsError
		}
		return data.Status != "completed" && data.Result.IsError &&
			nonempty(data.Error.Code, data.Error.Message)
	default:
		return false
	}
}

func runnerEventFromStored(event Event) (RunnerEvent, error) {
	var data any
	var identity stepData
	switch event.Type {
	case "step.started":
		var decoded stepData
		if err := json.Unmarshal(event.Data, &decoded); err != nil {
			return RunnerEvent{}, err
		}
		data, identity = decoded, decoded
	case "step.failed":
		var decoded stepFailedData
		if err := json.Unmarshal(event.Data, &decoded); err != nil {
			return RunnerEvent{}, err
		}
		data, identity = decoded, decoded.stepData
	case "message.appended":
		var decoded messageAppendedData
		if err := json.Unmarshal(event.Data, &decoded); err != nil {
			return RunnerEvent{}, err
		}
		data, identity = decoded, decoded.stepData
	case "message.completed":
		var decoded messageCompletedData
		if err := json.Unmarshal(event.Data, &decoded); err != nil {
			return RunnerEvent{}, err
		}
		data, identity = decoded, decoded.stepData
	case "actions.requested":
		var decoded actionsRequestedData
		if err := json.Unmarshal(event.Data, &decoded); err != nil {
			return RunnerEvent{}, err
		}
		data, identity = decoded, decoded.stepData
	case "step.completed":
		var decoded stepCompletedData
		if err := json.Unmarshal(event.Data, &decoded); err != nil {
			return RunnerEvent{}, err
		}
		data, identity = decoded, decoded.stepData
	case "action.result":
		var decoded actionResultData
		if err := json.Unmarshal(event.Data, &decoded); err != nil {
			return RunnerEvent{}, err
		}
		if decoded.Error == nil && decoded.Result.IsError && decoded.Status != "completed" {
			decoded.Error = &ActionFailure{
				Code: "LEGACY_ACTION_FAILED", Message: "The action failed.",
			}
		}
		data, identity = decoded, decoded.stepData
	default:
		return RunnerEvent{}, fmt.Errorf("unsupported runner event type %q", event.Type)
	}
	return RunnerEvent{
		eventType: event.Type,
		data:      data,
		step: Step{
			Sequence: identity.Sequence, StepIndex: identity.StepIndex, TurnID: identity.TurnID,
		},
	}, nil
}

// StepStarted constructs a typed model-step start event.
func StepStarted(step Step) RunnerEvent {
	return RunnerEvent{eventType: "step.started", data: dataForStep(step), step: step}
}

// StepFailed constructs a typed model-step failure event.
func StepFailed(step Step, code, message string) RunnerEvent {
	return RunnerEvent{
		eventType: "step.failed",
		step:      step,
		data: stepFailedData{
			stepData: dataForStep(step),
			Code:     code,
			Message:  message,
		},
	}
}

// MessageAppended constructs a typed assistant-message delta event.
func MessageAppended(step Step, delta, message string) RunnerEvent {
	return RunnerEvent{
		eventType: "message.appended",
		step:      step,
		data: messageAppendedData{
			stepData:     dataForStep(step),
			MessageDelta: delta,
			MessageSoFar: message,
		},
	}
}

// MessageCompleted constructs a typed assistant-message completion event.
func MessageCompleted(
	step Step,
	message string,
	finishReason string,
	metadata CompletionMetadata,
) RunnerEvent {
	return RunnerEvent{
		eventType: "message.completed",
		step:      step,
		data: messageCompletedData{
			stepData:         dataForStep(step),
			FinishReason:     finishReason,
			Message:          message,
			ProviderMetadata: metadata.Provider,
			Usage:            metadata.Usage,
		},
	}
}

// ActionsRequested constructs a typed tool-request event.
func ActionsRequested(step Step, actions []ActionRequest) RunnerEvent {
	return RunnerEvent{
		eventType: "actions.requested",
		step:      step,
		data: actionsRequestedData{
			stepData: dataForStep(step),
			Actions:  append([]ActionRequest(nil), actions...),
		},
	}
}

// StepCompleted constructs a typed model-step completion event.
func StepCompleted(step Step, finishReason string, metadata CompletionMetadata) RunnerEvent {
	return RunnerEvent{
		eventType: "step.completed",
		step:      step,
		data: stepCompletedData{
			stepData:         dataForStep(step),
			FinishReason:     finishReason,
			ProviderMetadata: metadata.Provider,
			Usage:            metadata.Usage,
		},
	}
}

// ActionCompleted constructs a typed successful tool-result event.
func ActionCompleted(step Step, result ActionResult) RunnerEvent {
	result.IsError = false
	return RunnerEvent{
		eventType: "action.result",
		step:      step,
		data: actionResultData{
			stepData: dataForStep(step),
			Result:   result,
			Status:   "completed",
		},
	}
}

// ActionFailed constructs a typed failed tool-result event.
func ActionFailed(
	step Step,
	status string,
	result ActionResult,
	failure ActionFailure,
) RunnerEvent {
	result.IsError = true
	return RunnerEvent{
		eventType: "action.result",
		step:      step,
		data: actionResultData{
			stepData: dataForStep(step),
			Error:    &failure,
			Result:   result,
			Status:   status,
		},
	}
}
