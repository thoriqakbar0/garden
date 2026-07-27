package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
)

type openAIModel struct {
	client   *http.Client
	endpoint string
	apiKey   string
}

type chatMessage struct {
	Role       string         `json:"role"`
	Content    *string        `json:"content,omitempty"`
	ToolCalls  []chatToolCall `json:"tool_calls,omitempty"`
	ToolCallID string         `json:"tool_call_id,omitempty"`
}

type chatToolCall struct {
	ID       string       `json:"id"`
	Type     string       `json:"type"`
	Function chatFunction `json:"function"`
}

type chatFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

func (m *openAIModel) Complete(ctx context.Context, request modelRequest) (message, error) {
	messages := make([]chatMessage, 0, len(request.Messages)+1)
	instructions := request.Instructions
	messages = append(messages, chatMessage{Role: "system", Content: &instructions})
	for _, item := range request.Messages {
		converted := chatMessage{Role: item.Role, ToolCallID: item.ToolCallID}
		if item.Content != "" {
			content := item.Content
			converted.Content = &content
		}
		for _, call := range item.ToolCalls {
			converted.ToolCalls = append(converted.ToolCalls, chatToolCall{
				ID:   call.ID,
				Type: "function",
				Function: chatFunction{
					Name:      call.Name,
					Arguments: string(call.Arguments),
				},
			})
		}
		messages = append(messages, converted)
	}
	tools := make([]map[string]any, 0, len(request.Tools))
	for _, definition := range request.Tools {
		tools = append(tools, map[string]any{
			"type": "function",
			"function": map[string]any{
				"name":        definition.Name,
				"description": definition.Description,
				"parameters":  definition.Parameters,
				"strict":      true,
			},
		})
	}
	payload := map[string]any{
		"model":    request.Model,
		"messages": messages,
	}
	if len(tools) > 0 {
		payload["tools"] = tools
		payload["tool_choice"] = "auto"
	}
	data, _, status, err := postJSON(ctx, m.client, m.endpoint, m.apiKey, nil, payload)
	if err != nil {
		return message{}, err
	}
	if status < 200 || status >= 300 {
		return message{}, fmt.Errorf("model endpoint returned HTTP %d", status)
	}
	var response struct {
		Choices []struct {
			Message struct {
				Role      string         `json:"role"`
				Content   *string        `json:"content"`
				ToolCalls []chatToolCall `json:"tool_calls"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(data, &response); err != nil {
		return message{}, errors.New("model endpoint returned malformed JSON")
	}
	if len(response.Choices) != 1 {
		return message{}, errors.New("model endpoint must return exactly one choice")
	}
	result := message{Role: response.Choices[0].Message.Role}
	if response.Choices[0].Message.Content != nil {
		result.Content = *response.Choices[0].Message.Content
	}
	for _, call := range response.Choices[0].Message.ToolCalls {
		if call.Type != "function" {
			return message{}, errors.New("model returned an unsupported tool-call type")
		}
		result.ToolCalls = append(result.ToolCalls, toolCall{
			ID:        call.ID,
			Name:      call.Function.Name,
			Arguments: json.RawMessage(call.Function.Arguments),
		})
	}
	return result, nil
}
