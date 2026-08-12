// Package workflowtest provides deterministic adapters for workflow tests.
package workflowtest

import (
	"context"
	"fmt"

	"github.com/thoriqakbar0/garden/internal/workflow"
)

// Response computes one deterministic answer for a test runner.
type Response func(context.Context, string, []workflow.Event) (string, error)

// Runner adapts a deterministic response function to the production workflow seam.
func Runner(response Response) workflow.Runner {
	return workflow.RunnerFunc(func(ctx context.Context, turn workflow.Turn, emit workflow.Emit) (string, error) {
		step := workflow.Step{Sequence: turn.Sequence, StepIndex: 0, TurnID: turn.TurnID}
		if err := emit(workflow.StepStarted(step)); err != nil {
			return "", err
		}
		message, err := response(ctx, turn.Message, turn.History)
		if err != nil {
			return "", err
		}
		if err := emit(workflow.MessageAppended(step, message, message)); err != nil {
			return "", err
		}
		if err := emit(workflow.MessageCompleted(
			step,
			message,
			"stop",
			workflow.CompletionMetadata{},
		)); err != nil {
			return "", err
		}
		if err := emit(workflow.StepCompleted(step, "stop", workflow.CompletionMetadata{})); err != nil {
			return "", err
		}
		return message, nil
	})
}

// EchoRunner returns the deterministic runner used by shared conversation tests.
func EchoRunner() workflow.Runner {
	return Runner(func(_ context.Context, message string, history []workflow.Event) (string, error) {
		turn := 1
		for _, event := range history {
			if event.Type == "turn.completed" {
				turn++
			}
		}
		return fmt.Sprintf("stress-ack:%d:%s", turn, message), nil
	})
}
