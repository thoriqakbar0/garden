package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestAnthropicMessagesToolRoundTripAndMetadata(t *testing.T) {
	var payload map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("x-api-key"); got != "anthropic-secret" {
			t.Errorf("x-api-key = %q", got)
		}
		if got := r.Header.Get("anthropic-version"); got != anthropicVersion {
			t.Errorf("anthropic-version = %q", got)
		}
		if got := r.Header.Get("Authorization"); got != "" {
			t.Errorf("Authorization = %q, want empty", got)
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Error(err)
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{
			"id":"msg_123","type":"message","role":"assistant","model":"claude-sonnet-response",
			"content":[{"type":"tool_use","id":"toolu_1","name":"weather","input":{"city":"Jakarta"}}],
			"stop_reason":"tool_use","usage":{"input_tokens":12,"output_tokens":7,
			"cache_read_input_tokens":5,"cache_creation_input_tokens":3}
		}`)
	}))
	defer server.Close()

	backend := &anthropicModel{
		client: server.Client(), endpoint: server.URL, apiKey: "anthropic-secret",
		headers: map[string]string{"Authorization": "must-not-be-sent"},
	}
	result, err := backend.Complete(context.Background(), modelRequest{
		Instructions: "Be concise.", Model: "claude-request",
		Messages: []message{
			{Role: "user", Content: "Weather?"},
			{Role: "assistant", ToolCalls: []toolCall{
				{ID: "old_1", Name: "weather", Arguments: json.RawMessage(`{"city":"A"}`)},
				{ID: "old_2", Name: "weather", Arguments: json.RawMessage(`{"city":"B"}`)},
			}},
			{Role: "tool", ToolCallID: "old_1", Content: `{"temperature":30}`},
			{Role: "tool", ToolCallID: "old_2", Content: `{"temperature":31}`},
		},
		Tools: []ToolDefinition{{
			Name: "weather", Description: "Get weather.",
			Parameters: json.RawMessage(`{"type":"object","properties":{"city":{"type":"string"}}}`),
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Role != "assistant" || len(result.ToolCalls) != 1 ||
		result.ToolCalls[0].ID != "toolu_1" || result.ToolCalls[0].Name != "weather" ||
		string(result.ToolCalls[0].Arguments) != `{"city":"Jakarta"}` {
		t.Fatalf("result = %#v", result)
	}
	wantMetadata := modelMetadata{
		Provider: providerAnthropic, API: "anthropic-messages", Model: "claude-sonnet-response",
		ResponseID: "msg_123", StopReason: "tool-calls",
		Usage: modelUsage{Input: 12, Output: 7, CacheRead: 5, CacheWrite: 3},
	}
	if result.Metadata != wantMetadata {
		t.Fatalf("metadata = %#v, want %#v", result.Metadata, wantMetadata)
	}
	if payload["system"] != "Be concise." || payload["model"] != "claude-request" ||
		payload["max_tokens"] != float64(anthropicMaxTokens) {
		t.Fatalf("request metadata = %#v", payload)
	}
	tools := payload["tools"].([]any)
	tool := tools[0].(map[string]any)
	if tool["name"] != "weather" || tool["input_schema"].(map[string]any)["type"] != "object" {
		t.Fatalf("tools = %#v", tools)
	}
	messages := payload["messages"].([]any)
	if len(messages) != 3 {
		t.Fatalf("messages = %#v", messages)
	}
	assistantBlocks := messages[1].(map[string]any)["content"].([]any)
	if len(assistantBlocks) != 2 || assistantBlocks[0].(map[string]any)["type"] != "tool_use" {
		t.Fatalf("assistant blocks = %#v", assistantBlocks)
	}
	resultMessage := messages[2].(map[string]any)
	resultBlocks := resultMessage["content"].([]any)
	if resultMessage["role"] != "user" || len(resultBlocks) != 2 ||
		resultBlocks[0].(map[string]any)["tool_use_id"] != "old_1" ||
		resultBlocks[1].(map[string]any)["tool_use_id"] != "old_2" {
		t.Fatalf("tool-result message = %#v", resultMessage)
	}
}

func TestAnthropicMessagesTextAndStopReason(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, `{"id":"msg_text","type":"message","role":"assistant","model":"claude",
			"content":[{"type":"text","text":"Hello, "},{"type":"text","text":"world."}],
			"stop_reason":"max_tokens","usage":{"input_tokens":2,"output_tokens":3}}`)
	}))
	defer server.Close()
	result, err := (&anthropicModel{client: server.Client(), endpoint: server.URL}).Complete(
		context.Background(), modelRequest{Model: "requested", Messages: []message{{Role: "user", Content: "hello"}}},
	)
	if err != nil {
		t.Fatal(err)
	}
	if result.Content != "Hello, world." || result.Metadata.StopReason != "length" {
		t.Fatalf("result = %#v", result)
	}
}

func TestAnthropicMessagesErrorsAreSafeAndBounded(t *testing.T) {
	const secret = "upstream-secret-never-expose"
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		http.Error(w, secret, http.StatusUnauthorized)
	}))
	defer server.Close()
	backend := &anthropicModel{client: server.Client(), endpoint: server.URL, apiKey: secret}

	_, err := backend.Complete(context.Background(), modelRequest{
		Model: "claude", Messages: []message{{Role: "user", Content: "hello"}},
	})
	if err == nil || strings.Contains(err.Error(), secret) {
		t.Fatalf("unsafe error = %v", err)
	}
	before := requests.Load()
	_, err = backend.Complete(context.Background(), modelRequest{
		Model: "claude", Instructions: strings.Repeat("x", maxPayloadBytes),
	})
	if err == nil || !strings.Contains(err.Error(), "request exceeds 1 MiB") || requests.Load() != before {
		t.Fatalf("oversized request: error = %v, requests = %d", err, requests.Load())
	}

	malformed := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, `{"type":"message","role":"assistant","content":[{"type":"tool_use","input":]}`)
	}))
	defer malformed.Close()
	_, err = (&anthropicModel{client: malformed.Client(), endpoint: malformed.URL}).Complete(
		context.Background(), modelRequest{Model: "claude"},
	)
	if err == nil || !strings.Contains(err.Error(), "malformed JSON") {
		t.Fatalf("malformed response error = %v", err)
	}
}

func TestAnthropicMessagesCancellation(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		close(started)
		<-release
	}))
	defer server.Close()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		_, err := (&anthropicModel{client: server.Client(), endpoint: server.URL}).Complete(
			ctx, modelRequest{Model: "claude"},
		)
		done <- err
	}()
	select {
	case <-started:
		cancel()
	case <-time.After(2 * time.Second):
		t.Fatal("request did not start")
	}
	select {
	case err := <-done:
		close(release)
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("error = %v", err)
		}
	case <-time.After(2 * time.Second):
		close(release)
		t.Fatal("cancellation did not reach request")
	}
}
