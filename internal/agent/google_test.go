package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestGoogleGenerateContentToolRoundTrip(t *testing.T) {
	t.Parallel()
	var received map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("x-goog-api-key"); got != "super-secret" {
			t.Errorf("x-goog-api-key = %q", got)
		}
		if got := r.Header.Get("Authorization"); got != "" {
			t.Errorf("Authorization unexpectedly set: %q", got)
		}
		if strings.Contains(r.URL.RawQuery, "super-secret") {
			t.Error("API key leaked into query")
		}
		if err := json.NewDecoder(r.Body).Decode(&received); err != nil {
			t.Errorf("decode request: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{
			"responseId":"response-1",
			"modelVersion":"gemini-2.5-flash-001",
			"candidates":[{"content":{"role":"model","parts":[
				{"functionCall":{"name":"weather","args":{"city":"Oslo"}}}
			]},"finishReason":"STOP"}],
			"usageMetadata":{"promptTokenCount":100,"candidatesTokenCount":7,"cachedContentTokenCount":20}
		}`)
	}))
	defer server.Close()

	backend := &googleModel{client: server.Client(), endpoint: server.URL, apiKey: "super-secret"}
	result, err := backend.Complete(context.Background(), modelRequest{
		Instructions: "Be concise.",
		Model:        "gemini-2.5-flash",
		Tools: []ToolDefinition{{
			Name: "weather", Description: "Get weather.",
			Parameters: json.RawMessage(`{"type":"object","properties":{"city":{"type":"string"}}}`),
		}},
		Messages: []message{
			{Role: "user", Content: "Weather?"},
			{Role: "assistant", ToolCalls: []toolCall{{ID: "old-call", Name: "weather", Arguments: json.RawMessage(`{"city":"Paris"}`)}}},
			{Role: "tool", ToolCallID: "old-call", Content: `{"temperature":18}`},
			{Role: "assistant", Content: "It is 18 degrees."},
			{Role: "user", Content: "And Oslo?"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Role != "assistant" || len(result.ToolCalls) != 1 {
		t.Fatalf("unexpected result: %#v", result)
	}
	call := result.ToolCalls[0]
	if !callIDPattern.MatchString(call.ID) || call.Name != "weather" || string(call.Arguments) != `{"city":"Oslo"}` {
		t.Fatalf("unexpected call: %#v", call)
	}
	wantMetadata := modelMetadata{
		Provider: providerGoogle, API: "google-generate-content", Model: "gemini-2.5-flash-001",
		ResponseID: "response-1", StopReason: "tool-calls",
		Usage: modelUsage{Input: 80, Output: 7, CacheRead: 20},
	}
	if result.Metadata != wantMetadata {
		t.Fatalf("metadata = %#v, want %#v", result.Metadata, wantMetadata)
	}

	system := received["systemInstruction"].(map[string]any)
	if got := system["parts"].([]any)[0].(map[string]any)["text"]; got != "Be concise." {
		t.Fatalf("system instruction = %#v", got)
	}
	contents := received["contents"].([]any)
	if len(contents) != 5 {
		t.Fatalf("contents length = %d", len(contents))
	}
	modelCall := contents[1].(map[string]any)["parts"].([]any)[0].(map[string]any)["functionCall"].(map[string]any)
	if modelCall["id"] != "old-call" || modelCall["name"] != "weather" {
		t.Fatalf("historical function call = %#v", modelCall)
	}
	functionResponse := contents[2].(map[string]any)["parts"].([]any)[0].(map[string]any)["functionResponse"].(map[string]any)
	if functionResponse["id"] != "old-call" || functionResponse["name"] != "weather" {
		t.Fatalf("function response = %#v", functionResponse)
	}
	if got := functionResponse["response"].(map[string]any)["temperature"]; got != float64(18) {
		t.Fatalf("function response value = %#v", got)
	}
	declarations := received["tools"].([]any)[0].(map[string]any)["functionDeclarations"].([]any)
	if declarations[0].(map[string]any)["name"] != "weather" {
		t.Fatalf("declarations = %#v", declarations)
	}
}

func TestGoogleBuildsModelEndpointAndKeepsKeyOutOfURL(t *testing.T) {
	t.Parallel()
	var path, query string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path, query = r.URL.Path, r.URL.RawQuery
		if r.Header.Get("x-goog-api-key") != "key" {
			t.Error("missing Google API key header")
		}
		fmt.Fprint(w, `{"candidates":[{"content":{"role":"model","parts":[{"text":"hello"}]},"finishReason":"STOP"}]}`)
	}))
	defer server.Close()
	backend := newGoogleModel(server.Client(), server.URL+"/v1beta/", "key")
	result, err := backend.Complete(context.Background(), modelRequest{Model: "models/gemini-2.5-pro", Messages: []message{{Role: "user", Content: "hi"}}})
	if err != nil {
		t.Fatal(err)
	}
	if result.Content != "hello" || result.Metadata.StopReason != "stop" {
		t.Fatalf("result = %#v", result)
	}
	if path != "/v1beta/models/gemini-2.5-pro:generateContent" || query != "" {
		t.Fatalf("request URL path=%q query=%q", path, query)
	}
}

func TestGoogleToolCallIDsAreUniqueDuringConcurrentCalls(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, `{"candidates":[{"content":{"role":"model","parts":[{"functionCall":{"name":"run","args":{}}}]},"finishReason":"STOP"}]}`)
	}))
	defer server.Close()
	backend := &googleModel{client: server.Client(), endpoint: server.URL}

	const count = 40
	ids := make(chan string, count)
	errs := make(chan error, count)
	var wg sync.WaitGroup
	for range count {
		wg.Add(1)
		go func() {
			defer wg.Done()
			result, err := backend.Complete(context.Background(), modelRequest{Model: "gemini", Messages: []message{{Role: "user", Content: "go"}}})
			if err != nil {
				errs <- err
				return
			}
			ids <- result.ToolCalls[0].ID
		}()
	}
	wg.Wait()
	close(ids)
	close(errs)
	for err := range errs {
		t.Fatal(err)
	}
	seen := make(map[string]bool, count)
	for id := range ids {
		if seen[id] || !callIDPattern.MatchString(id) {
			t.Fatalf("duplicate or malformed ID %q", id)
		}
		seen[id] = true
	}
	if len(seen) != count {
		t.Fatalf("got %d IDs, want %d", len(seen), count)
	}
}

func TestGoogleErrorsAreSafeAndCancellationPropagates(t *testing.T) {
	t.Parallel()
	t.Run("HTTP body is not disclosed", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
			fmt.Fprint(w, `{"error":"private upstream detail"}`)
		}))
		defer server.Close()
		backend := &googleModel{client: server.Client(), endpoint: server.URL}
		_, err := backend.Complete(context.Background(), modelRequest{Model: "gemini"})
		if err == nil || !strings.Contains(err.Error(), "HTTP 401") || strings.Contains(err.Error(), "private") {
			t.Fatalf("unsafe error: %v", err)
		}
	})
	t.Run("cancellation", func(t *testing.T) {
		started := make(chan struct{})
		release := make(chan struct{})
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			close(started)
			<-release
		}))
		defer server.Close()
		backend := &googleModel{client: server.Client(), endpoint: server.URL}
		ctx, cancel := context.WithCancel(context.Background())
		done := make(chan error, 1)
		go func() {
			_, err := backend.Complete(ctx, modelRequest{Model: "gemini"})
			done <- err
		}()
		<-started
		cancel()
		select {
		case err := <-done:
			close(release)
			if !errors.Is(err, context.Canceled) {
				t.Fatalf("error = %v", err)
			}
		case <-time.After(2 * time.Second):
			close(release)
			t.Fatal("Complete did not return after cancellation")
		}
	})
}

func TestGoogleRejectsInvalidHistoryAndResponsePayloads(t *testing.T) {
	t.Parallel()
	backend := &googleModel{endpoint: "http://unused.invalid"}
	_, err := backend.Complete(context.Background(), modelRequest{
		Model: "gemini", Messages: []message{{Role: "tool", ToolCallID: "missing", Content: `{}`}},
	})
	if err == nil || !strings.Contains(err.Error(), "no matching") {
		t.Fatalf("error = %v", err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, `{"candidates":[{"content":{"role":"model","parts":[{"functionCall":{"name":"run","args":[]}}]}}]}`)
	}))
	defer server.Close()
	backend = &googleModel{client: server.Client(), endpoint: server.URL}
	_, err = backend.Complete(context.Background(), modelRequest{Model: "gemini"})
	if err == nil || !strings.Contains(err.Error(), "invalid arguments") {
		t.Fatalf("error = %v", err)
	}
}
