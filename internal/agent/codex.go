package agent

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type codexModel struct {
	client    *http.Client
	endpoint  string
	token     string
	accountID string
	chatGPT   bool
}

type responsesItem struct {
	Type      string             `json:"type"`
	Role      string             `json:"role,omitempty"`
	Content   []responsesContent `json:"content,omitempty"`
	CallID    string             `json:"call_id,omitempty"`
	Name      string             `json:"name,omitempty"`
	Arguments string             `json:"arguments,omitempty"`
	Output    string             `json:"output,omitempty"`
}

type responsesContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

func (m *codexModel) Complete(ctx context.Context, request modelRequest) (message, error) {
	input := make([]responsesItem, 0, len(request.Messages))
	for _, item := range request.Messages {
		switch item.Role {
		case "user":
			input = append(input, responsesItem{
				Type: "message", Role: "user",
				Content: []responsesContent{{Type: "input_text", Text: item.Content}},
			})
		case "assistant":
			if item.Content != "" {
				input = append(input, responsesItem{
					Type: "message", Role: "assistant",
					Content: []responsesContent{{Type: "output_text", Text: item.Content}},
				})
			}
			for _, call := range item.ToolCalls {
				input = append(input, responsesItem{
					Type: "function_call", CallID: call.ID, Name: call.Name,
					Arguments: string(call.Arguments),
				})
			}
		case "tool":
			input = append(input, responsesItem{
				Type: "function_call_output", CallID: item.ToolCallID, Output: item.Content,
			})
		default:
			return message{}, errors.New("unsupported conversation role")
		}
	}
	tools := make([]map[string]any, 0, len(request.Tools))
	for _, definition := range request.Tools {
		tools = append(tools, map[string]any{
			"type":        "function",
			"name":        definition.Name,
			"description": definition.Description,
			"parameters":  definition.Parameters,
			"strict":      true,
		})
	}
	payload := map[string]any{
		"model":        request.Model,
		"instructions": request.Instructions,
		"input":        input,
		"store":        false,
		"stream":       true,
	}
	if len(tools) > 0 {
		payload["tools"] = tools
		payload["tool_choice"] = "auto"
		payload["parallel_tool_calls"] = false
	}
	headers := map[string]string{
		"ChatGPT-Account-Id": m.accountID,
		"Accept":             "text/event-stream, application/json",
		"originator":         "eve",
		"User-Agent":         "garden/0.1",
	}
	data, contentType, status, err := postJSON(ctx, m.client, m.endpoint, m.token, headers, payload)
	if err != nil {
		return message{}, err
	}
	if status < 200 || status >= 300 {
		if m.chatGPT && (status == http.StatusUnauthorized || status == http.StatusForbidden) {
			return message{}, errors.New("Codex ChatGPT token is expired or rejected; run `codex login`")
		}
		return message{}, fmt.Errorf("Codex Responses endpoint returned HTTP %d", status)
	}
	if strings.Contains(contentType, "text/event-stream") || bytes.HasPrefix(bytes.TrimSpace(data), []byte("data:")) {
		data, err = completedSSE(data)
		if err != nil {
			return message{}, err
		}
	}
	return decodeResponses(data)
}

func completedSSE(data []byte) ([]byte, error) {
	scanner := bufio.NewScanner(bytes.NewReader(data))
	scanner.Buffer(make([]byte, 64*1024), maxPayloadBytes)
	var completed []byte
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		value := bytes.TrimSpace([]byte(strings.TrimPrefix(line, "data:")))
		if bytes.Equal(value, []byte("[DONE]")) {
			continue
		}
		var event struct {
			Type     string          `json:"type"`
			Response json.RawMessage `json:"response"`
		}
		if err := json.Unmarshal(value, &event); err != nil {
			return nil, errors.New("Codex endpoint returned malformed event data")
		}
		if event.Type == "response.completed" {
			completed = append([]byte(nil), event.Response...)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, errors.New("Codex event exceeded payload limit")
	}
	if len(completed) == 0 {
		return nil, errors.New("Codex endpoint omitted a completed response")
	}
	return completed, nil
}

func decodeResponses(data []byte) (message, error) {
	var response struct {
		Output []responsesItem `json:"output"`
	}
	if err := json.Unmarshal(data, &response); err != nil {
		return message{}, errors.New("Codex endpoint returned malformed JSON")
	}
	result := message{Role: "assistant"}
	for _, item := range response.Output {
		switch item.Type {
		case "message":
			if item.Role != "" && item.Role != "assistant" {
				return message{}, errors.New("Codex endpoint returned a non-assistant message")
			}
			for _, content := range item.Content {
				if content.Type != "output_text" || content.Text == "" {
					return message{}, errors.New("Codex endpoint returned invalid message content")
				}
				result.Content += content.Text
			}
		case "function_call":
			result.ToolCalls = append(result.ToolCalls, toolCall{
				ID: item.CallID, Name: item.Name, Arguments: json.RawMessage(item.Arguments),
			})
		case "reasoning":
			continue
		default:
			return message{}, fmt.Errorf("Codex endpoint returned unsupported output type %q", item.Type)
		}
	}
	return result, nil
}

func tokenExpired(token string, now time.Time) bool {
	expires, ok := tokenExpiration(token)
	return ok && !now.Before(expires)
}

func tokenExpiration(token string) (time.Time, bool) {
	claims, ok := tokenClaims(token)
	if !ok {
		return time.Time{}, false
	}
	expires, ok := claims["exp"].(float64)
	if !ok || expires <= 0 {
		return time.Time{}, false
	}
	return time.Unix(int64(expires), 0), true
}

func accountIDFromToken(token string) string {
	claims, ok := tokenClaims(token)
	if !ok {
		return ""
	}
	if accountID := nonEmptyClaim(claims["chatgpt_account_id"]); accountID != "" {
		return accountID
	}
	if auth, ok := claims["https://api.openai.com/auth"].(map[string]any); ok {
		if accountID := nonEmptyClaim(auth["chatgpt_account_id"]); accountID != "" {
			return accountID
		}
	}
	if organizations, ok := claims["organizations"].([]any); ok && len(organizations) > 0 {
		if organization, ok := organizations[0].(map[string]any); ok {
			return nonEmptyClaim(organization["id"])
		}
	}
	return ""
}

func tokenClaims(token string) (map[string]any, bool) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, false
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, false
	}
	var claims map[string]any
	if json.Unmarshal(payload, &claims) != nil {
		return nil, false
	}
	return claims, true
}

func nonEmptyClaim(value any) string {
	text, ok := value.(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(text)
}
