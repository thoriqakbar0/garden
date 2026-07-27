package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/thoriqakbar0/garden/internal/discover"
	"github.com/thoriqakbar0/garden/internal/workflow"
)

// NewResponder validates an application and creates its model-backed workflow responder.
func NewResponder(app discover.Application, backend model, modelName string, manifest []Tool) (workflow.Responder, error) {
	if backend == nil {
		return nil, errors.New("model backend is required")
	}
	if modelName == "" {
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

	return func(ctx context.Context, current string, events []workflow.Event) (string, error) {
		messages, err := completedHistory(events)
		if err != nil {
			return "", err
		}
		messages = append(messages, message{Role: "user", Content: current})
		seenCallIDs := make(map[string]struct{})
		for round := 0; round < maxModelRounds; round++ {
			stepContext, cancel := context.WithTimeout(ctx, 60*time.Second)
			result, modelErr := backend.Complete(stepContext, modelRequest{
				Instructions: app.Instructions,
				Model:        modelName,
				Messages:     messages,
				Tools:        definitions,
			})
			cancel()
			if modelErr != nil {
				return "", modelErr
			}
			if err := validateAssistant(result); err != nil {
				return "", err
			}
			for _, call := range result.ToolCalls {
				if _, exists := seenCallIDs[call.ID]; exists {
					return "", errors.New("model reused a tool-call ID")
				}
				seenCallIDs[call.ID] = struct{}{}
			}
			if result.Content != "" {
				return result.Content, nil
			}
			for _, call := range result.ToolCalls {
				if _, exists := tools[call.Name]; !exists {
					return "", fmt.Errorf("model requested undeclared tool %q", call.Name)
				}
			}
			recordedCalls := make([]map[string]any, 0, len(result.ToolCalls))
			for _, call := range result.ToolCalls {
				recordedCalls = append(recordedCalls, map[string]any{
					"id": call.ID, "name": call.Name, "arguments": call.Arguments,
				})
			}
			if err := workflow.RecordStep(ctx, "assistant.tool_calls", recordedCalls); err != nil {
				return "", err
			}
			messages = append(messages, result)
			for _, call := range result.ToolCalls {
				stepContext, cancel := context.WithTimeout(ctx, 60*time.Second)
				output, toolErr := tools[call.Name].Execute(stepContext, call.Arguments)
				cancel()
				if toolErr != nil {
					if errors.Is(toolErr, context.Canceled) || errors.Is(toolErr, context.DeadlineExceeded) {
						return "", toolErr
					}
					return "", fmt.Errorf("native tool %q failed", call.Name)
				}
				if len(output) == 0 || len(output) > maxPayloadBytes || !json.Valid(output) {
					return "", fmt.Errorf("native tool %q returned invalid JSON", call.Name)
				}
				if err := workflow.RecordStep(ctx, "tool.completed", map[string]any{
					"toolCallId": call.ID,
					"name":       call.Name,
					"result":     json.RawMessage(output),
				}); err != nil {
					return "", err
				}
				messages = append(messages, message{
					Role:       "tool",
					Content:    string(output),
					ToolCallID: call.ID,
				})
			}
		}
		return "", fmt.Errorf("model exceeded the maximum of %d rounds", maxModelRounds)
	}, nil
}

func completedHistory(events []workflow.Event) ([]message, error) {
	type turn struct {
		user      string
		assistant string
	}
	turns := make(map[string]*turn)
	var messages []message
	for _, event := range events {
		switch event.Type {
		case "turn.started":
			var data struct {
				Message string `json:"message"`
			}
			if err := json.Unmarshal(event.Data, &data); err != nil {
				return nil, errors.New("invalid persisted turn history")
			}
			turns[event.TurnID] = &turn{user: data.Message}
		case "message.completed":
			var data struct {
				Message string `json:"message"`
			}
			if err := json.Unmarshal(event.Data, &data); err != nil {
				return nil, errors.New("invalid persisted message history")
			}
			if value := turns[event.TurnID]; value != nil {
				value.assistant = data.Message
			}
		case "turn.completed":
			if value := turns[event.TurnID]; value != nil && value.user != "" && value.assistant != "" {
				messages = append(messages,
					message{Role: "user", Content: value.user},
					message{Role: "assistant", Content: value.assistant},
				)
			}
		}
	}
	return messages, nil
}
