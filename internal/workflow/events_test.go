package workflow

import (
	"encoding/json"
	"testing"
)

func TestRunnerEventSequenceAcceptsReachableStepPhases(t *testing.T) {
	turn := Turn{Sequence: 3, TurnID: "turn-3"}
	step := func(index int) Step {
		return Step{Sequence: turn.Sequence, StepIndex: index, TurnID: turn.TurnID}
	}
	action := ActionRequest{
		CallID: "call-1", Input: json.RawMessage(`{}`), Kind: "tool-call", ToolName: "lookup",
	}
	result := ActionResult{
		CallID: action.CallID, Kind: "tool-result", Output: json.RawMessage(`{}`), ToolName: action.ToolName,
	}

	tests := []struct {
		name         string
		events       []RunnerEvent
		wantComplete bool
		wantPhase    runnerStepPhase
	}{
		{
			name: "final message",
			events: []RunnerEvent{
				StepStarted(step(0)),
				MessageAppended(step(0), "done", "done"),
				MessageCompleted(step(0), "done", "stop", CompletionMetadata{}),
				StepCompleted(step(0), "stop", CompletionMetadata{}),
			},
			wantComplete: true,
			wantPhase:    stepPhaseCompletedFinal,
		},
		{
			name: "message before tool request",
			events: []RunnerEvent{
				StepStarted(step(0)),
				MessageAppended(step(0), "thinking", "thinking"),
				MessageCompleted(step(0), "thinking", "tool-calls", CompletionMetadata{}),
				ActionsRequested(step(0), []ActionRequest{action}),
				StepCompleted(step(0), "tool-calls", CompletionMetadata{}),
			},
			wantPhase: stepPhaseCompletedToolCalls,
		},
		{
			name: "message after tool request and next step",
			events: []RunnerEvent{
				StepStarted(step(0)),
				ActionsRequested(step(0), []ActionRequest{action}),
				MessageAppended(step(0), "thinking", "thinking"),
				MessageCompleted(step(0), "thinking", "tool-calls", CompletionMetadata{}),
				StepCompleted(step(0), "tool-calls", CompletionMetadata{}),
				ActionCompleted(step(0), result),
				StepStarted(step(1)),
				MessageAppended(step(1), "done", "done"),
				MessageCompleted(step(1), "done", "stop", CompletionMetadata{}),
				StepCompleted(step(1), "stop", CompletionMetadata{}),
			},
			wantComplete: true,
			wantPhase:    stepPhaseCompletedFinal,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			sequence := newRunnerEventSequence()
			for index, event := range test.events {
				if !sequence.accept(event, turn) {
					t.Fatalf("event %d (%s) rejected", index, event.Type())
				}
			}
			if got := sequence.complete(); got != test.wantComplete {
				t.Fatalf("complete = %v, want %v", got, test.wantComplete)
			}
			last := sequence.steps[sequence.nextStep-1]
			if last.phase != test.wantPhase {
				t.Fatalf("last phase = %v, want %v", last.phase, test.wantPhase)
			}
		})
	}
}

func TestRunnerEventSequenceRejectsIllegalPhaseTransitions(t *testing.T) {
	turn := Turn{Sequence: 4, TurnID: "turn-4"}
	step := Step{Sequence: turn.Sequence, StepIndex: 0, TurnID: turn.TurnID}
	action := ActionRequest{
		CallID: "call-1", Input: json.RawMessage(`{}`), Kind: "tool-call", ToolName: "lookup",
	}

	tests := []struct {
		name   string
		before []RunnerEvent
		event  RunnerEvent
	}{
		{
			name:   "message completion before a delta",
			before: []RunnerEvent{StepStarted(step)},
			event:  MessageCompleted(step, "done", "stop", CompletionMetadata{}),
		},
		{
			name: "message delta after message completion",
			before: []RunnerEvent{
				StepStarted(step),
				MessageAppended(step, "done", "done"),
				MessageCompleted(step, "done", "stop", CompletionMetadata{}),
			},
			event: MessageAppended(step, "late", "done late"),
		},
		{
			name: "failure after tool request",
			before: []RunnerEvent{
				StepStarted(step),
				ActionsRequested(step, []ActionRequest{action}),
			},
			event: StepFailed(step, "MODEL_FAILED", "failed"),
		},
		{
			name: "final completion with tool request",
			before: []RunnerEvent{
				StepStarted(step),
				ActionsRequested(step, []ActionRequest{action}),
			},
			event: StepCompleted(step, "stop", CompletionMetadata{}),
		},
		{
			name: "tool completion without tool request",
			before: []RunnerEvent{
				StepStarted(step),
				MessageAppended(step, "done", "done"),
				MessageCompleted(step, "done", "stop", CompletionMetadata{}),
			},
			event: StepCompleted(step, "tool-calls", CompletionMetadata{}),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			sequence := newRunnerEventSequence()
			for index, event := range test.before {
				if !sequence.accept(event, turn) {
					t.Fatalf("setup event %d (%s) rejected", index, event.Type())
				}
			}
			if sequence.accept(test.event, turn) {
				t.Fatalf("illegal event %s accepted", test.event.Type())
			}
		})
	}
}

func TestRunnerEventSequenceAcceptsFailureBeforeToolRequest(t *testing.T) {
	turn := Turn{Sequence: 5, TurnID: "turn-5"}
	step := Step{Sequence: turn.Sequence, StepIndex: 0, TurnID: turn.TurnID}

	tests := []struct {
		name   string
		before []RunnerEvent
	}{
		{name: "started", before: []RunnerEvent{StepStarted(step)}},
		{
			name: "message streaming",
			before: []RunnerEvent{
				StepStarted(step),
				MessageAppended(step, "partial", "partial"),
				MessageAppended(step, " response", "partial response"),
			},
		},
		{
			name: "message completed",
			before: []RunnerEvent{
				StepStarted(step),
				MessageAppended(step, "done", "done"),
				MessageCompleted(step, "done", "stop", CompletionMetadata{}),
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			sequence := newRunnerEventSequence()
			for index, event := range test.before {
				if !sequence.accept(event, turn) {
					t.Fatalf("setup event %d (%s) rejected", index, event.Type())
				}
			}
			if !sequence.accept(StepFailed(step, "MODEL_FAILED", "failed"), turn) {
				t.Fatal("step failure rejected")
			}
			if got := sequence.steps[0].phase; got != stepPhaseFailed {
				t.Fatalf("phase = %v, want %v", got, stepPhaseFailed)
			}
		})
	}
}
