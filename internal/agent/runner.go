package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/thoriqakbar0/garden/internal/discover"
	"github.com/thoriqakbar0/garden/internal/workflow"
)

const modelStepTimeout = 60 * time.Second

// Runner owns one model -> tool -> model execution loop.
type Runner struct {
	backend      model
	modelName    string
	instructions string
	tools        map[string]Tool
	definitions  []ToolDefinition
}

// NewRunner validates an application and constructs its native workflow runner.
func NewRunner(app discover.Application, backend model, modelName string, manifest []Tool) (*Runner, error) {
	if backend == nil {
		return nil, errors.New("model backend is required")
	}
	if strings.TrimSpace(modelName) == "" {
		return nil, errors.New("model identifier is required; set GARDEN_MODEL or declare one in agent/agent.ts")
	}
	available := make(map[string]Tool, len(manifest))
	for _, tool := range manifest {
		if tool == nil {
			return nil, errors.New("native tool manifest contains a nil implementation")
		}
		definition := tool.Definition()
		if err := validateDefinition(definition); err != nil {
			return nil, err
		}
		if _, exists := available[definition.Name]; exists {
			return nil, fmt.Errorf("native tool manifest contains duplicate %q", definition.Name)
		}
		available[definition.Name] = tool
	}

	tools := make(map[string]Tool, len(app.Tools))
	definitions := make([]ToolDefinition, 0, len(app.Tools))
	for _, name := range app.Tools {
		tool, exists := available[name]
		if !exists {
			return nil, fmt.Errorf("declared tool %q has no native manifest implementation", name)
		}
		tools[name] = tool
		definitions = append(definitions, tool.Definition())
	}
	return &Runner{
		backend:      backend,
		modelName:    modelName,
		instructions: app.Instructions,
		tools:        tools,
		definitions:  definitions,
	}, nil
}

// NewResponder preserves the deterministic responder seam for tests and callers
// that do not consume intermediate workflow events.
func NewResponder(app discover.Application, backend model, modelName string, manifest []Tool) (workflow.Responder, error) {
	runner, err := NewRunner(app, backend, modelName, manifest)
	if err != nil {
		return nil, err
	}
	return runner.Respond, nil
}

// Run executes one turn and emits each model and tool boundary before continuing.
func (r *Runner) Run(ctx context.Context, turn workflow.Turn, emit workflow.Emit) (string, error) {
	if emit == nil {
		return "", errors.New("workflow event emitter is required")
	}
	messages, err := conversation(turn.History)
	if err != nil {
		return "", err
	}
	messages = append(messages, message{Role: "user", Content: turn.Message})
	seenCallIDs := make(map[string]struct{})

	for stepIndex := 0; stepIndex < maxModelRounds; stepIndex++ {
		step := map[string]any{
			"sequence": turn.Sequence, "stepIndex": stepIndex, "turnId": turn.TurnID,
		}
		if err := emit("step.started", step); err != nil {
			return "", err
		}

		stepContext, cancel := context.WithTimeout(ctx, modelStepTimeout)
		result, modelErr := r.backend.Complete(stepContext, modelRequest{
			Instructions: r.instructions,
			Model:        r.modelName,
			Messages:     messages,
			Tools:        r.definitions,
		})
		cancel()
		if modelErr != nil {
			_ = emit("step.failed", map[string]any{
				"code": "MODEL_CALL_FAILED", "message": "The model call failed.",
				"sequence": turn.Sequence, "stepIndex": stepIndex, "turnId": turn.TurnID,
			})
			return "", modelErr
		}
		if err := validateAssistant(result); err != nil {
			_ = emit("step.failed", map[string]any{
				"code": "MODEL_RESPONSE_INVALID", "message": "The model response was invalid.",
				"sequence": turn.Sequence, "stepIndex": stepIndex, "turnId": turn.TurnID,
			})
			return "", err
		}
		for _, call := range result.ToolCalls {
			if _, exists := seenCallIDs[call.ID]; exists {
				return "", errors.New("model reused a tool-call ID")
			}
			seenCallIDs[call.ID] = struct{}{}
		}

		if result.Content != "" {
			if err := emit("message.appended", map[string]any{
				"messageDelta": result.Content, "messageSoFar": result.Content,
				"sequence": turn.Sequence, "stepIndex": stepIndex, "turnId": turn.TurnID,
			}); err != nil {
				return "", err
			}
			if err := emit("message.completed", map[string]any{
				"finishReason": "stop", "message": result.Content,
				"sequence": turn.Sequence, "stepIndex": stepIndex, "turnId": turn.TurnID,
			}); err != nil {
				return "", err
			}
			if err := emit("step.completed", map[string]any{
				"finishReason": "stop", "sequence": turn.Sequence,
				"stepIndex": stepIndex, "turnId": turn.TurnID,
			}); err != nil {
				return "", err
			}
			return result.Content, nil
		}

		actions := make([]actionRequest, 0, len(result.ToolCalls))
		for _, call := range result.ToolCalls {
			if _, exists := r.tools[call.Name]; !exists {
				return "", fmt.Errorf("model requested undeclared tool %q", call.Name)
			}
			actions = append(actions, actionRequest{
				CallID: call.ID, Input: call.Arguments, Kind: "tool-call", ToolName: call.Name,
			})
		}
		if err := emit("actions.requested", map[string]any{
			"actions": actions, "sequence": turn.Sequence,
			"stepIndex": stepIndex, "turnId": turn.TurnID,
		}); err != nil {
			return "", err
		}
		if err := emit("step.completed", map[string]any{
			"finishReason": "tool-calls", "sequence": turn.Sequence,
			"stepIndex": stepIndex, "turnId": turn.TurnID,
		}); err != nil {
			return "", err
		}
		messages = append(messages, result)

		for _, call := range result.ToolCalls {
			output, toolErr := r.executeTool(ctx, r.tools[call.Name], call.Arguments)
			if errors.Is(toolErr, context.Canceled) || errors.Is(toolErr, context.DeadlineExceeded) {
				return "", toolErr
			}
			status := "completed"
			action := actionResult{
				CallID: call.ID, Kind: "tool-result", Output: output, ToolName: call.Name,
			}
			if toolErr != nil {
				status = "failed"
				action.IsError = true
				action.Output = json.RawMessage(`{"code":"TOOL_EXECUTION_FAILED","message":"Tool execution failed."}`)
			}
			if len(action.Output) == 0 || len(action.Output) > maxPayloadBytes || !json.Valid(action.Output) {
				return "", fmt.Errorf("native tool %q returned invalid JSON", call.Name)
			}
			data := map[string]any{
				"result": action, "sequence": turn.Sequence, "status": status,
				"stepIndex": stepIndex, "turnId": turn.TurnID,
			}
			if toolErr != nil {
				data["error"] = map[string]string{
					"code": "TOOL_EXECUTION_FAILED", "message": "Tool execution failed.",
				}
			}
			if err := emit("action.result", data); err != nil {
				return "", err
			}
			messages = append(messages, message{
				Role: "tool", Content: string(action.Output), ToolCallID: call.ID,
			})
		}
	}
	return "", fmt.Errorf("model exceeded the maximum of %d rounds", maxModelRounds)
}

// Respond runs the model loop without exposing its intermediate event stream.
func (r *Runner) Respond(ctx context.Context, current string, events []workflow.Event) (string, error) {
	return r.Run(ctx, workflow.Turn{Message: current, History: events}, func(string, any) error {
		return nil
	})
}

func (r *Runner) executeTool(ctx context.Context, tool Tool, arguments json.RawMessage) (json.RawMessage, error) {
	stepContext, cancel := context.WithTimeout(ctx, modelStepTimeout)
	defer cancel()
	output, err := tool.Execute(stepContext, arguments)
	if err != nil && stepContext.Err() != nil {
		return nil, stepContext.Err()
	}
	return output, err
}

type actionRequest struct {
	CallID   string          `json:"callId"`
	Input    json.RawMessage `json:"input"`
	Kind     string          `json:"kind"`
	ToolName string          `json:"toolName"`
}

type actionResult struct {
	CallID   string          `json:"callId"`
	IsError  bool            `json:"isError,omitempty"`
	Kind     string          `json:"kind"`
	Output   json.RawMessage `json:"output"`
	ToolName string          `json:"toolName"`
}

func conversation(events []workflow.Event) ([]message, error) {
	messages := make([]message, 0, len(events))
	pending := make(map[string][]message)
	for _, event := range events {
		switch event.Type {
		case "message.received":
			var data struct {
				Message string `json:"message"`
			}
			if err := json.Unmarshal(event.Data, &data); err != nil || strings.TrimSpace(data.Message) == "" {
				return nil, fmt.Errorf("parse message.received event %d", event.Index)
			}
			pending[event.TurnID] = append(pending[event.TurnID], message{Role: "user", Content: data.Message})
		case "message.completed":
			var data struct {
				Message string `json:"message"`
			}
			if err := json.Unmarshal(event.Data, &data); err != nil {
				return nil, fmt.Errorf("parse message.completed event %d", event.Index)
			}
			if strings.TrimSpace(data.Message) != "" {
				pending[event.TurnID] = append(pending[event.TurnID], message{Role: "assistant", Content: data.Message})
			}
		case "actions.requested":
			var data struct {
				Actions []actionRequest `json:"actions"`
			}
			if err := json.Unmarshal(event.Data, &data); err != nil || len(data.Actions) == 0 {
				return nil, fmt.Errorf("parse actions.requested event %d", event.Index)
			}
			calls := make([]toolCall, 0, len(data.Actions))
			for _, action := range data.Actions {
				calls = append(calls, toolCall{
					ID: action.CallID, Name: action.ToolName, Arguments: action.Input,
				})
			}
			pending[event.TurnID] = append(pending[event.TurnID], message{Role: "assistant", ToolCalls: calls})
		case "action.result":
			var data struct {
				Result actionResult `json:"result"`
			}
			if err := json.Unmarshal(event.Data, &data); err != nil ||
				data.Result.CallID == "" || !json.Valid(data.Result.Output) {
				return nil, fmt.Errorf("parse action.result event %d", event.Index)
			}
			pending[event.TurnID] = append(pending[event.TurnID], message{
				Role: "tool", ToolCallID: data.Result.CallID, Content: string(data.Result.Output),
			})
		case "turn.completed":
			messages = append(messages, pending[event.TurnID]...)
			delete(pending, event.TurnID)
		case "turn.failed", "turn.cancelled":
			delete(pending, event.TurnID)
		}
	}
	return messages, nil
}
