package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
)

const (
	anthropicVersion   = "2023-06-01"
	anthropicMaxTokens = 4096
)

type anthropicModel struct {
	client   *http.Client
	endpoint string
	apiKey   string
	headers  map[string]string
}

type anthropicMessage struct {
	Role    string                  `json:"role"`
	Content []anthropicContentBlock `json:"content"`
}

type anthropicContentBlock struct {
	Type      string          `json:"type"`
	Text      string          `json:"text,omitempty"`
	ID        string          `json:"id,omitempty"`
	Name      string          `json:"name,omitempty"`
	Input     json.RawMessage `json:"input,omitempty"`
	ToolUseID string          `json:"tool_use_id,omitempty"`
	Content   string          `json:"content,omitempty"`
}

type anthropicTool struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"input_schema"`
}

func (m *anthropicModel) Complete(ctx context.Context, request modelRequest) (message, error) {
	messages, err := anthropicMessages(request.Messages)
	if err != nil {
		return message{}, err
	}
	tools := make([]anthropicTool, 0, len(request.Tools))
	for _, definition := range request.Tools {
		tools = append(tools, anthropicTool{
			Name: definition.Name, Description: definition.Description,
			InputSchema: definition.Parameters,
		})
	}
	payload := struct {
		Model     string             `json:"model"`
		MaxTokens int                `json:"max_tokens"`
		System    string             `json:"system,omitempty"`
		Messages  []anthropicMessage `json:"messages"`
		Tools     []anthropicTool    `json:"tools,omitempty"`
	}{
		Model: request.Model, MaxTokens: anthropicMaxTokens,
		System: request.Instructions, Messages: messages, Tools: tools,
	}

	headers := make(map[string]string, len(m.headers)+2)
	for name, value := range m.headers {
		if !strings.EqualFold(name, "authorization") &&
			!strings.EqualFold(name, "x-api-key") &&
			!strings.EqualFold(name, "anthropic-version") {
			headers[name] = value
		}
	}
	if m.apiKey != "" {
		headers["x-api-key"] = m.apiKey
	}
	headers["anthropic-version"] = anthropicVersion

	data, _, status, err := postJSON(ctx, m.client, m.endpoint, "", headers, payload)
	if err != nil {
		return message{}, err
	}
	if status < 200 || status >= 300 {
		return message{}, fmt.Errorf("model endpoint returned HTTP %d", status)
	}
	return decodeAnthropicMessage(data, request.Model)
}

func anthropicMessages(source []message) ([]anthropicMessage, error) {
	converted := make([]anthropicMessage, 0, len(source))
	for _, item := range source {
		switch item.Role {
		case "user":
			converted = append(converted, anthropicMessage{Role: "user", Content: []anthropicContentBlock{{
				Type: "text", Text: item.Content,
			}}})
		case "assistant":
			blocks := make([]anthropicContentBlock, 0, len(item.ToolCalls)+1)
			if item.Content != "" {
				blocks = append(blocks, anthropicContentBlock{Type: "text", Text: item.Content})
			}
			for _, call := range item.ToolCalls {
				if !json.Valid(call.Arguments) {
					return nil, errors.New("encode model request")
				}
				blocks = append(blocks, anthropicContentBlock{
					Type: "tool_use", ID: call.ID, Name: call.Name, Input: call.Arguments,
				})
			}
			converted = append(converted, anthropicMessage{Role: "assistant", Content: blocks})
		case "tool":
			block := anthropicContentBlock{
				Type: "tool_result", ToolUseID: item.ToolCallID, Content: item.Content,
			}
			// Anthropic expects all results for one assistant tool-use message in a
			// single following user message.
			if len(converted) > 0 && converted[len(converted)-1].Role == "user" &&
				len(converted[len(converted)-1].Content) > 0 &&
				converted[len(converted)-1].Content[0].Type == "tool_result" {
				converted[len(converted)-1].Content = append(converted[len(converted)-1].Content, block)
			} else {
				converted = append(converted, anthropicMessage{Role: "user", Content: []anthropicContentBlock{block}})
			}
		default:
			return nil, errors.New("model request contains an unsupported message role")
		}
	}
	return converted, nil
}

func decodeAnthropicMessage(data []byte, requestedModel string) (message, error) {
	var response struct {
		ID         string            `json:"id"`
		Type       string            `json:"type"`
		Role       string            `json:"role"`
		Model      string            `json:"model"`
		Content    []json.RawMessage `json:"content"`
		StopReason string            `json:"stop_reason"`
		Usage      struct {
			Input      int `json:"input_tokens"`
			Output     int `json:"output_tokens"`
			CacheRead  int `json:"cache_read_input_tokens"`
			CacheWrite int `json:"cache_creation_input_tokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(data, &response); err != nil {
		return message{}, errors.New("model endpoint returned malformed JSON")
	}
	if response.Type != "message" || response.Role != "assistant" {
		return message{}, errors.New("model response must contain one assistant message")
	}
	result := message{Role: "assistant"}
	var text strings.Builder
	for _, raw := range response.Content {
		var kind struct {
			Type string `json:"type"`
		}
		if err := json.Unmarshal(raw, &kind); err != nil || kind.Type == "" {
			return message{}, errors.New("model endpoint returned malformed content")
		}
		switch kind.Type {
		case "text":
			var block struct {
				Text string `json:"text"`
			}
			if err := json.Unmarshal(raw, &block); err != nil {
				return message{}, errors.New("model endpoint returned malformed content")
			}
			text.WriteString(block.Text)
		case "thinking", "redacted_thinking":
			// Garden does not persist provider reasoning blocks. Preserve only the
			// semantic text/tool result boundary shared by every provider.
			continue
		case "tool_use":
			var block struct {
				ID    string          `json:"id"`
				Name  string          `json:"name"`
				Input json.RawMessage `json:"input"`
			}
			if err := json.Unmarshal(raw, &block); err != nil || !json.Valid(block.Input) {
				return message{}, errors.New("model returned malformed tool arguments")
			}
			result.ToolCalls = append(result.ToolCalls, toolCall{
				ID: block.ID, Name: block.Name, Arguments: block.Input,
			})
		default:
			return message{}, errors.New("model returned an unsupported content block")
		}
	}
	result.Content = text.String()
	metadata := metadataFor(providerAnthropic, requestedModel)
	metadata.ResponseID = response.ID
	metadata.Usage = modelUsage{
		Input: nonnegative(response.Usage.Input), Output: nonnegative(response.Usage.Output),
		CacheRead: nonnegative(response.Usage.CacheRead), CacheWrite: nonnegative(response.Usage.CacheWrite),
	}
	if response.Model != "" {
		metadata.Model = response.Model
	}
	stopReason, err := normalizeStopReason(providerAnthropic, response.StopReason, len(result.ToolCalls) > 0)
	if err != nil {
		return message{}, err
	}
	metadata.StopReason = stopReason
	result.Metadata = metadata
	return result, nil
}
