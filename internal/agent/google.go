package agent

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// googleModel implements Google's native generateContent API. Its endpoint is
// the complete :generateContent URL; model selection is therefore normally
// encoded in the endpoint as required by the Google API.
type googleModel struct {
	client   *http.Client
	baseURL  string
	endpoint string // optional direct endpoint, primarily for focused tests
	apiKey   string
	headers  map[string]string
}

func newGoogleModel(client *http.Client, baseURL, apiKey string) *googleModel {
	return &googleModel{client: client, baseURL: strings.TrimRight(baseURL, "/"), apiKey: apiKey}
}

type googleContent struct {
	Role  string       `json:"role"`
	Parts []googlePart `json:"parts"`
}

type googlePart struct {
	Text             string                  `json:"text,omitempty"`
	ThoughtSignature string                  `json:"thoughtSignature,omitempty"`
	FunctionCall     *googleFunctionCall     `json:"functionCall,omitempty"`
	FunctionResponse *googleFunctionResponse `json:"functionResponse,omitempty"`
}

type googleFunctionCall struct {
	ID   string          `json:"id,omitempty"`
	Name string          `json:"name"`
	Args json.RawMessage `json:"args"`
}

type googleFunctionResponse struct {
	ID       string          `json:"id,omitempty"`
	Name     string          `json:"name"`
	Response json.RawMessage `json:"response"`
}

type googleFunctionDeclaration struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Parameters  json.RawMessage `json:"parameters"`
}

type googleTool struct {
	FunctionDeclarations []googleFunctionDeclaration `json:"functionDeclarations"`
}

type googleGenerateRequest struct {
	SystemInstruction *googleContent  `json:"systemInstruction,omitempty"`
	Contents          []googleContent `json:"contents"`
	Tools             []googleTool    `json:"tools,omitempty"`
}

func (m *googleModel) Complete(ctx context.Context, request modelRequest) (message, error) {
	payload, err := googleRequest(request)
	if err != nil {
		return message{}, err
	}

	// Do not put the API key in the URL. Apart from avoiding query-string
	// logging, a private copy also makes concurrent Complete calls race-free.
	headers := make(map[string]string, len(m.headers)+1)
	for name, value := range m.headers {
		headers[name] = value
	}
	if m.apiKey != "" {
		headers["x-goog-api-key"] = m.apiKey
	}
	client := m.client
	if client == nil {
		client = http.DefaultClient
	}
	endpoint, err := m.generateContentEndpoint(request.Model)
	if err != nil {
		return message{}, err
	}
	data, _, status, err := postJSON(ctx, client, endpoint, "", headers, payload)
	if err != nil {
		return message{}, err
	}
	if status < http.StatusOK || status >= http.StatusMultipleChoices {
		return message{}, fmt.Errorf("model endpoint returned HTTP %d", status)
	}

	var response struct {
		ResponseID   string `json:"responseId"`
		ModelVersion string `json:"modelVersion"`
		Candidates   []struct {
			Content      googleContent `json:"content"`
			FinishReason string        `json:"finishReason"`
		} `json:"candidates"`
		Usage struct {
			PromptTokens    int `json:"promptTokenCount"`
			CandidateTokens int `json:"candidatesTokenCount"`
			CachedTokens    int `json:"cachedContentTokenCount"`
		} `json:"usageMetadata"`
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	if err := decoder.Decode(&response); err != nil {
		return message{}, errors.New("model endpoint returned malformed JSON")
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return message{}, errors.New("model endpoint returned malformed JSON")
	}
	if len(response.Candidates) != 1 {
		return message{}, errors.New("model endpoint must return exactly one candidate")
	}
	candidate := response.Candidates[0]
	if candidate.Content.Role != "" && candidate.Content.Role != "model" {
		return message{}, errors.New("model endpoint returned an invalid candidate role")
	}

	metadata := metadataFor(providerGoogle, request.Model)
	metadata.ResponseID = response.ResponseID
	stopReason, err := normalizeStopReason(
		providerGoogle, candidate.FinishReason, hasGoogleFunctionCall(candidate.Content.Parts),
	)
	if err != nil {
		return message{}, err
	}
	metadata.StopReason = stopReason
	metadata.Usage.Output = nonnegative(response.Usage.CandidateTokens)
	metadata.Usage.CacheRead = nonnegative(response.Usage.CachedTokens)
	metadata.Usage.Input = nonnegative(response.Usage.PromptTokens - metadata.Usage.CacheRead)
	if response.ModelVersion != "" {
		metadata.Model = response.ModelVersion
	}
	result := message{Role: "assistant", Metadata: metadata}
	var text strings.Builder
	for _, part := range candidate.Content.Parts {
		if part.Text != "" {
			text.WriteString(part.Text)
		}
		if part.FunctionCall == nil {
			continue
		}
		call := part.FunctionCall
		if !toolNamePattern.MatchString(call.Name) {
			return message{}, errors.New("model returned a malformed tool name")
		}
		if !validJSONObject(call.Args) {
			return message{}, fmt.Errorf("model returned invalid arguments for tool %q", call.Name)
		}
		callID := call.ID
		if callID == "" {
			callID, err = nextGoogleToolCallID()
			if err != nil {
				return message{}, err
			}
		} else if !callIDPattern.MatchString(callID) {
			return message{}, errors.New("model returned a malformed tool-call ID")
		}
		var providerData json.RawMessage
		if part.ThoughtSignature != "" {
			providerData, err = json.Marshal(map[string]string{
				"thoughtSignature": part.ThoughtSignature,
			})
			if err != nil {
				return message{}, errors.New("encode Google continuation metadata")
			}
		}
		result.ToolCalls = append(result.ToolCalls, toolCall{
			ID: callID, Name: call.Name,
			Arguments: append(json.RawMessage(nil), call.Args...), ProviderData: providerData,
		})
	}
	result.Content = text.String()
	return result, nil
}

func (m *googleModel) generateContentEndpoint(model string) (string, error) {
	if m.endpoint != "" {
		return m.endpoint, nil
	}
	model = strings.TrimPrefix(strings.TrimSpace(model), "models/")
	if m.baseURL == "" || model == "" || strings.ContainsAny(model, "/?#") {
		return "", errors.New("invalid Google generateContent endpoint")
	}
	return m.baseURL + "/models/" + model + ":generateContent", nil
}

func googleRequest(request modelRequest) (googleGenerateRequest, error) {
	payload := googleGenerateRequest{
		Contents: make([]googleContent, 0, len(request.Messages)),
	}
	if request.Instructions != "" {
		payload.SystemInstruction = &googleContent{
			Parts: []googlePart{{Text: request.Instructions}},
		}
	}
	if len(request.Tools) > 0 {
		declarations := make([]googleFunctionDeclaration, 0, len(request.Tools))
		for _, definition := range request.Tools {
			declarations = append(declarations, googleFunctionDeclaration{
				Name: definition.Name, Description: definition.Description,
				Parameters: definition.Parameters,
			})
		}
		payload.Tools = []googleTool{{FunctionDeclarations: declarations}}
	}

	callNames := make(map[string]string)
	for _, item := range request.Messages {
		switch item.Role {
		case "user":
			payload.Contents = append(payload.Contents, googleContent{
				Role: "user", Parts: []googlePart{{Text: item.Content}},
			})
		case "assistant":
			content := googleContent{Role: "model"}
			if item.Content != "" {
				content.Parts = append(content.Parts, googlePart{Text: item.Content})
			}
			for _, call := range item.ToolCalls {
				if call.ID == "" || call.Name == "" || !validJSONObject(call.Arguments) {
					return googleGenerateRequest{}, errors.New("invalid assistant tool-call history")
				}
				if _, duplicate := callNames[call.ID]; duplicate {
					return googleGenerateRequest{}, errors.New("duplicate tool-call ID in history")
				}
				callNames[call.ID] = call.Name
				thoughtSignature, err := googleThoughtSignature(call.ProviderData)
				if err != nil {
					return googleGenerateRequest{}, err
				}
				content.Parts = append(content.Parts, googlePart{
					ThoughtSignature: thoughtSignature,
					FunctionCall: &googleFunctionCall{
						ID: call.ID, Name: call.Name, Args: call.Arguments,
					},
				})
			}
			if len(content.Parts) == 0 {
				return googleGenerateRequest{}, errors.New("invalid empty assistant message")
			}
			payload.Contents = append(payload.Contents, content)
		case "tool":
			name, ok := callNames[item.ToolCallID]
			if !ok {
				return googleGenerateRequest{}, errors.New("tool result has no matching tool call")
			}
			response, err := googleFunctionResult(item.Content)
			if err != nil {
				return googleGenerateRequest{}, err
			}
			part := googlePart{FunctionResponse: &googleFunctionResponse{
				ID: item.ToolCallID, Name: name, Response: response,
			}}
			if len(payload.Contents) > 0 && payload.Contents[len(payload.Contents)-1].Role == "user" &&
				len(payload.Contents[len(payload.Contents)-1].Parts) > 0 &&
				payload.Contents[len(payload.Contents)-1].Parts[0].FunctionResponse != nil {
				payload.Contents[len(payload.Contents)-1].Parts = append(
					payload.Contents[len(payload.Contents)-1].Parts, part,
				)
			} else {
				payload.Contents = append(payload.Contents, googleContent{
					Role: "user", Parts: []googlePart{part},
				})
			}
		default:
			return googleGenerateRequest{}, fmt.Errorf("unsupported message role %q", item.Role)
		}
	}
	return payload, nil
}

func googleFunctionResult(content string) (json.RawMessage, error) {
	raw := json.RawMessage(content)
	if len(raw) == 0 || len(raw) > maxPayloadBytes || !json.Valid(raw) {
		return nil, errors.New("tool result contains invalid JSON")
	}
	var object map[string]json.RawMessage
	if err := json.Unmarshal(raw, &object); err == nil && object != nil {
		return raw, nil
	}
	// FunctionResponse.response is a protobuf Struct and must be an object.
	wrapped, err := json.Marshal(map[string]json.RawMessage{"result": raw})
	if err != nil {
		return nil, errors.New("encode tool result")
	}
	return wrapped, nil
}

func validJSONObject(raw json.RawMessage) bool {
	if len(raw) == 0 || len(raw) > maxPayloadBytes || !json.Valid(raw) {
		return false
	}
	var object map[string]json.RawMessage
	return json.Unmarshal(raw, &object) == nil && object != nil
}

func hasGoogleFunctionCall(parts []googlePart) bool {
	for _, part := range parts {
		if part.FunctionCall != nil {
			return true
		}
	}
	return false
}

func nextGoogleToolCallID() (string, error) {
	var random [12]byte
	if _, err := rand.Read(random[:]); err != nil {
		return "", errors.New("generate Google tool-call ID")
	}
	return "google-call-" + hex.EncodeToString(random[:]), nil
}

func googleThoughtSignature(providerData json.RawMessage) (string, error) {
	if len(providerData) == 0 {
		return "", nil
	}
	var data struct {
		ThoughtSignature string `json:"thoughtSignature"`
	}
	if err := json.Unmarshal(providerData, &data); err != nil {
		return "", errors.New("invalid Google continuation metadata")
	}
	return data.ThoughtSignature, nil
}
