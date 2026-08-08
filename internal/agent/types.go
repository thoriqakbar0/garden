package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strings"
)

const (
	maxPayloadBytes = 1 << 20
	maxModelRounds  = 8
)

var (
	toolNamePattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)
	callIDPattern   = regexp.MustCompile(`^[A-Za-z0-9_.:-]{1,128}$`)
)

// ToolDefinition is the native schema advertised to a model.
type ToolDefinition struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Parameters  json.RawMessage `json:"parameters"`
}

// Tool is one build-time native capability.
type Tool interface {
	Definition() ToolDefinition
	Execute(context.Context, json.RawMessage) (json.RawMessage, error)
}

type message struct {
	Role       string
	Content    string
	ToolCalls  []toolCall
	ToolCallID string
	Metadata   modelMetadata
}

type toolCall struct {
	ID           string
	Name         string
	Arguments    json.RawMessage
	ProviderData json.RawMessage
}

type modelRequest struct {
	Instructions string
	Model        string
	Messages     []message
	Tools        []ToolDefinition
}

type model interface {
	Complete(context.Context, modelRequest) (message, error)
}

func validateDefinition(definition ToolDefinition) error {
	if !toolNamePattern.MatchString(definition.Name) {
		return fmt.Errorf("native tool has invalid name %q", definition.Name)
	}
	if definition.Description == "" {
		return fmt.Errorf("native tool %q has an empty description", definition.Name)
	}
	if len(definition.Parameters) == 0 || len(definition.Parameters) > maxPayloadBytes {
		return fmt.Errorf("native tool %q has an invalid schema size", definition.Name)
	}
	var schema map[string]json.RawMessage
	decoder := json.NewDecoder(bytes.NewReader(definition.Parameters))
	if err := decoder.Decode(&schema); err != nil {
		return fmt.Errorf("native tool %q has invalid JSON Schema", definition.Name)
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return fmt.Errorf("native tool %q has invalid JSON Schema", definition.Name)
	}
	var rootType string
	if err := json.Unmarshal(schema["type"], &rootType); err != nil || rootType != "object" {
		return fmt.Errorf("native tool %q schema root must have type object", definition.Name)
	}
	return nil
}

func validateAssistant(result message) error {
	if result.Role != "assistant" {
		return errors.New("model response must contain one assistant message")
	}
	hasContent := strings.TrimSpace(result.Content) != ""
	hasCalls := len(result.ToolCalls) > 0
	if !hasContent && !hasCalls {
		return errors.New("model assistant message must contain text or tool calls")
	}
	seen := make(map[string]struct{}, len(result.ToolCalls))
	for _, call := range result.ToolCalls {
		if !callIDPattern.MatchString(call.ID) {
			return errors.New("model returned a malformed tool-call ID")
		}
		if _, exists := seen[call.ID]; exists {
			return errors.New("model returned duplicate tool-call IDs")
		}
		seen[call.ID] = struct{}{}
		if !toolNamePattern.MatchString(call.Name) {
			return errors.New("model returned a malformed tool name")
		}
		if len(call.Arguments) == 0 || len(call.Arguments) > maxPayloadBytes || !json.Valid(call.Arguments) {
			return fmt.Errorf("model returned invalid arguments for tool %q", call.Name)
		}
		if len(call.ProviderData) > maxPayloadBytes ||
			(len(call.ProviderData) > 0 && !json.Valid(call.ProviderData)) {
			return fmt.Errorf("model returned invalid provider data for tool %q", call.Name)
		}
		var object map[string]json.RawMessage
		if err := json.Unmarshal(call.Arguments, &object); err != nil || object == nil {
			return fmt.Errorf("model returned non-object arguments for tool %q", call.Name)
		}
	}
	return nil
}
