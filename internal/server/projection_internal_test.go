package server

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/thoriqakbar0/garden/internal/protocol"
	"github.com/thoriqakbar0/garden/internal/workflow"
)

func TestPublicEventStripsGardenProviderMetadata(t *testing.T) {
	messageEvent, visible, err := publicEvent(workflow.Event{
		Type: "message.completed",
		Data: json.RawMessage(`{"finishReason":"stop","message":"ok","sequence":1,"stepIndex":0,"turnId":"turn_1","providerMetadata":{"provider":"anthropic"},"usage":{"inputTokens":2,"totalTokens":3}}`),
	})
	if err != nil || !visible {
		t.Fatalf("message projection: visible=%v error=%v", visible, err)
	}
	if strings.Contains(string(messageEvent.Data), "providerMetadata") || strings.Contains(string(messageEvent.Data), "usage") {
		t.Fatalf("message.completed leaked internal metadata: %s", messageEvent.Data)
	}

	stepEvent, visible, err := publicEvent(workflow.Event{
		Type: "step.completed",
		Data: json.RawMessage(`{"finishReason":"stop","sequence":1,"stepIndex":0,"turnId":"turn_1","providerMetadata":{"provider":"anthropic","model":"claude"},"usage":{"inputTokens":11,"outputTokens":1,"cacheReadTokens":4,"cacheWriteTokens":5,"totalTokens":12}}`),
	})
	if err != nil || !visible {
		t.Fatalf("step projection: visible=%v error=%v", visible, err)
	}
	if strings.Contains(string(stepEvent.Data), "providerMetadata") || strings.Contains(string(stepEvent.Data), "totalTokens") {
		t.Fatalf("step.completed leaked internal metadata: %s", stepEvent.Data)
	}
	var projected protocol.StepCompletedData
	if err := json.Unmarshal(stepEvent.Data, &projected); err != nil {
		t.Fatal(err)
	}
	if projected.Usage == nil || projected.Usage.InputTokens != 11 || projected.Usage.OutputTokens != 1 ||
		projected.Usage.CacheReadTokens != 4 || projected.Usage.CacheWriteTokens != 5 {
		t.Fatalf("public usage = %#v", projected.Usage)
	}
}
