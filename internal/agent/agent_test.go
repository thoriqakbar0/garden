package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/thoriqakbar0/garden/internal/discover"
	"github.com/thoriqakbar0/garden/internal/workflow"
)

func TestOpenAIWeatherToolRoundTrip(t *testing.T) {
	var mu sync.Mutex
	var requests []map[string]any
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Error(err)
		}
		mu.Lock()
		requests = append(requests, payload)
		number := len(requests)
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		if number == 1 {
			fmt.Fprint(w, `{"choices":[{"message":{"role":"assistant","content":"Checking.","tool_calls":[{"id":"weather-1","type":"function","function":{"name":"get_weather","arguments":"{\"city\":\"Jakarta\"}"}}]},"finish_reason":"tool_calls"}]}`)
			return
		}
		fmt.Fprint(w, `{"choices":[{"message":{"role":"assistant","content":"It is sunny in Jakarta."}}]}`)
	}))
	defer upstream.Close()

	app := weatherApplication(t)
	runner, err := runnerFromConfig(app, env(map[string]string{
		"GARDEN_MODEL_BACKEND":   "openai",
		"GARDEN_OPENAI_BASE_URL": upstream.URL,
		"GARDEN_MODEL":           "fake-model",
	}), upstream.Client(), time.Now())
	if err != nil {
		t.Fatal(err)
	}
	store, err := workflow.OpenRunner(t.TempDir(), runner)
	if err != nil {
		t.Fatal(err)
	}
	session, err := store.CreateSession()
	if err != nil {
		t.Fatal(err)
	}
	result, err := store.Send(context.Background(), session, "Weather?")
	if err != nil {
		t.Fatal(err)
	}
	if result.Message != "It is sunny in Jakarta." {
		t.Fatalf("message = %q", result.Message)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(requests) != 2 {
		t.Fatalf("request count = %d, want 2", len(requests))
	}
	firstMessages := requests[0]["messages"].([]any)
	if firstMessages[0].(map[string]any)["content"] != app.Instructions {
		t.Fatal("first request omitted instructions")
	}
	tools := requests[0]["tools"].([]any)
	if tools[0].(map[string]any)["function"].(map[string]any)["name"] != "get_weather" {
		t.Fatal("first request omitted get_weather")
	}
	secondMessages := requests[1]["messages"].([]any)
	assistant := secondMessages[len(secondMessages)-2].(map[string]any)
	toolResult := secondMessages[len(secondMessages)-1].(map[string]any)
	if assistant["role"] != "assistant" || assistant["content"] != "Checking." || toolResult["role"] != "tool" ||
		toolResult["tool_call_id"] != "weather-1" ||
		!strings.Contains(toolResult["content"].(string), `"city":"Jakarta"`) {
		t.Fatalf("uncorrelated second request: %#v", secondMessages)
	}
	events, err := store.Replay(session, 0)
	if err != nil {
		t.Fatal(err)
	}
	wantTail := []string{"actions.requested", "step.completed", "action.result", "step.started", "message.appended", "message.completed", "step.completed", "turn.completed", "session.waiting"}
	if len(events) < len(wantTail) {
		t.Fatalf("event count = %d, want at least %d", len(events), len(wantTail))
	}
	for index, want := range wantTail {
		if got := events[len(events)-len(wantTail)+index].Type; got != want {
			t.Fatalf("event tail[%d] = %q, want %q", index, got, want)
		}
	}
}

func TestOpenAIMetadataIsNormalized(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"id":"resp-1","model":"gpt-response","choices":[{"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}],"usage":{"prompt_tokens":11,"completion_tokens":3,"prompt_tokens_details":{"cached_tokens":4}}}`)
	}))
	defer server.Close()

	result, err := (&openAIModel{
		client: server.Client(), endpoint: server.URL, apiKey: "test", provider: providerOpenAI,
	}).Complete(context.Background(), modelRequest{Model: "gpt-request", Instructions: "test"})
	if err != nil {
		t.Fatal(err)
	}
	metadata := result.Metadata
	if metadata.Provider != providerOpenAI || metadata.API != "openai-chat-completions" ||
		metadata.Model != "gpt-response" || metadata.ResponseID != "resp-1" ||
		metadata.StopReason != "stop" || metadata.Usage.Input != 7 ||
		metadata.Usage.Output != 3 || metadata.Usage.CacheRead != 4 {
		t.Fatalf("metadata = %#v", metadata)
	}
}

func TestOpenAICompatibleMetadataAndUsageRemainDurable(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"id":"router-response","model":"routed-model","choices":[{"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}],"usage":{"prompt_tokens":5,"completion_tokens":2,"prompt_tokens_details":{"cached_tokens":2}}}`)
	}))
	defer server.Close()

	runner, err := runnerFromConfig(
		discover.NativeSpec{Instructions: "test", Model: "router/model"},
		env(map[string]string{
			"GARDEN_MODEL_BACKEND":   "openai",
			"GARDEN_OPENAI_BASE_URL": server.URL,
		}), server.Client(), time.Now(),
	)
	if err != nil {
		t.Fatal(err)
	}
	var completed map[string]any
	_, err = runner.Run(context.Background(), workflow.Turn{Message: "hello"}, func(event workflow.RunnerEvent) error {
		if event.Type() == "step.completed" {
			completed = runnerEventData(t, event)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	metadata := completed["providerMetadata"].(map[string]any)
	usage := completed["usage"].(map[string]any)
	if metadata["provider"] != "openai-compatible" || metadata["api"] != "openai-chat-completions" ||
		metadata["model"] != "routed-model" || metadata["responseId"] != "router-response" {
		t.Fatalf("provider metadata = %#v", metadata)
	}
	if usage["inputTokens"] != float64(5) || usage["outputTokens"] != float64(2) ||
		usage["cacheReadTokens"] != float64(2) || usage["totalTokens"] != float64(7) {
		t.Fatalf("usage = %#v", usage)
	}
}

func TestRunnerIgnoresWhitespaceContentDuringToolLifecycle(t *testing.T) {
	backend := &sequenceModel{results: []message{
		{Role: "assistant", Content: " \n ", ToolCalls: []toolCall{{
			ID: "weather-1", Name: "get_weather", Arguments: json.RawMessage(`{"city":"Jakarta"}`),
		}}},
		{Role: "assistant", Content: "Sunny."},
	}}
	runner, err := NewRunner(discover.NativeSpec{
		Instructions: "test", Model: "model", Tools: []string{"get_weather"},
	}, backend, "model", NativeManifest())
	if err != nil {
		t.Fatal(err)
	}
	var eventTypes []string
	result, err := runner.Run(context.Background(), workflow.Turn{
		SessionID: "ses_test", TurnID: "turn_test", Message: "Weather?", Sequence: 2,
	}, func(event workflow.RunnerEvent) error {
		eventTypes = append(eventTypes, event.Type())
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if result != "Sunny." {
		t.Fatalf("result = %q", result)
	}
	want := []string{
		"step.started", "actions.requested", "step.completed", "action.result",
		"step.started", "message.appended", "message.completed", "step.completed",
	}
	if fmt.Sprint(eventTypes) != fmt.Sprint(want) {
		t.Fatalf("event types = %v, want %v", eventTypes, want)
	}
}

func TestRunnerEmitsNormalizedProviderMetadata(t *testing.T) {
	backend := staticModel{result: message{
		Role: "assistant", Content: "ok",
		Metadata: modelMetadata{
			Provider: providerAnthropic, API: "anthropic-messages", Model: "claude-test",
			ResponseID: "msg-1", StopReason: "stop",
			Usage: modelUsage{Input: 7, Output: 3, CacheRead: 2, CacheWrite: 1},
		},
	}}
	runner, err := NewRunner(
		discover.NativeSpec{Instructions: "test", Model: "claude-test"},
		backend, "claude-test", NativeManifest(),
	)
	if err != nil {
		t.Fatal(err)
	}
	var completed map[string]any
	_, err = runner.Run(context.Background(), workflow.Turn{Message: "hello"}, func(event workflow.RunnerEvent) error {
		if event.Type() == "step.completed" {
			completed = runnerEventData(t, event)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	providerMetadata := completed["providerMetadata"].(map[string]any)
	usage := completed["usage"].(map[string]any)
	if providerMetadata["provider"] != "anthropic" || providerMetadata["api"] != "anthropic-messages" ||
		providerMetadata["model"] != "claude-test" || providerMetadata["responseId"] != "msg-1" {
		t.Fatalf("provider metadata = %#v", providerMetadata)
	}
	if usage["inputTokens"] != float64(10) || usage["outputTokens"] != float64(3) ||
		usage["cacheReadTokens"] != float64(2) || usage["cacheWriteTokens"] != float64(1) ||
		usage["totalTokens"] != float64(13) {
		t.Fatalf("usage = %#v", usage)
	}
}

func TestConversationExcludesInterruptedTurns(t *testing.T) {
	events := []workflow.Event{
		{Index: 0, Type: "message.received", TurnID: "turn_failed", Data: json.RawMessage(`{"message":"do not replay"}`)},
		{Index: 1, Type: "actions.requested", TurnID: "turn_failed", Data: json.RawMessage(`{"actions":[{"callId":"call-1","input":{},"kind":"tool-call","toolName":"get_weather"}]}`)},
		{Index: 2, Type: "turn.failed", TurnID: "turn_failed", Data: json.RawMessage(`{}`)},
		{Index: 3, Type: "message.received", TurnID: "turn_ok", Data: json.RawMessage(`{"message":"keep"}`)},
		{Index: 4, Type: "message.completed", TurnID: "turn_ok", Data: json.RawMessage(`{"message":"kept"}`)},
		{Index: 5, Type: "turn.completed", TurnID: "turn_ok", Data: json.RawMessage(`{}`)},
	}
	messages, err := conversation(events)
	if err != nil {
		t.Fatal(err)
	}
	if len(messages) != 2 || messages[0].Content != "keep" || messages[1].Content != "kept" {
		t.Fatalf("messages = %#v", messages)
	}
}

func TestRejectsUnimplementedAndUndeclaredTools(t *testing.T) {
	app := discover.NativeSpec{Instructions: "test", Model: "model", Tools: []string{"missing"}}
	if _, err := NewRunner(app, staticModel{}, "model", NativeManifest()); err == nil ||
		!strings.Contains(err.Error(), `declared tool "missing"`) {
		t.Fatalf("unimplemented tool error = %v", err)
	}

	app.Tools = nil
	runner, err := NewRunner(app, staticModel{result: message{
		Role: "assistant",
		ToolCalls: []toolCall{{
			ID: "call-1", Name: "get_weather", Arguments: json.RawMessage(`{"city":"secret-city"}`),
		}},
	}}, "model", NativeManifest())
	if err != nil {
		t.Fatal(err)
	}
	_, err = send(t, runner)
	if err == nil || !strings.Contains(err.Error(), `undeclared tool "get_weather"`) {
		t.Fatalf("undeclared tool error = %v", err)
	}
}

func TestRejectsMalformedAndDuplicateToolCalls(t *testing.T) {
	tests := []struct {
		name   string
		result message
		match  string
	}{
		{
			name: "duplicate IDs",
			result: message{Role: "assistant", ToolCalls: []toolCall{
				{ID: "same", Name: "get_weather", Arguments: json.RawMessage(`{"city":"A"}`)},
				{ID: "same", Name: "get_weather", Arguments: json.RawMessage(`{"city":"B"}`)},
			}},
			match: "duplicate",
		},
		{
			name: "malformed arguments",
			result: message{Role: "assistant", ToolCalls: []toolCall{
				{ID: "call", Name: "get_weather", Arguments: json.RawMessage(`[]`)},
			}},
			match: "non-object",
		},
		{
			name:   "empty response",
			result: message{Role: "assistant"},
			match:  "text or tool calls",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			app := discover.NativeSpec{Instructions: "test", Model: "model", Tools: []string{"get_weather"}}
			runner, err := NewRunner(app, staticModel{result: test.result}, "model", NativeManifest())
			if err != nil {
				t.Fatal(err)
			}
			_, err = send(t, runner)
			if err == nil || !strings.Contains(err.Error(), test.match) {
				t.Fatalf("error = %v, want %q", err, test.match)
			}
		})
	}
}

func TestOpenAIToolArgumentsAcceptProviderDoubleEncoding(t *testing.T) {
	arguments, err := normalizeToolArguments(`"{\"city\":\"Jakarta\"}"`)
	if err != nil {
		t.Fatal(err)
	}
	if string(arguments) != `{"city":"Jakarta"}` {
		t.Fatalf("arguments = %s", arguments)
	}
}

func TestCloudflareGatewayTokenHeader(t *testing.T) {
	const gatewayToken = "gateway-token"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("cf-aig-authorization"); got != "Bearer "+gatewayToken {
			t.Errorf("cf-aig-authorization = %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"choices":[{"message":{"role":"assistant","content":"ok"}}]}`)
	}))
	defer server.Close()

	app := discover.NativeSpec{Instructions: "test", Model: "model"}
	runner, err := runnerFromConfig(app, env(map[string]string{
		"GARDEN_MODEL_BACKEND":            "openai",
		"GARDEN_OPENAI_BASE_URL":          server.URL,
		"GARDEN_CLOUDFLARE_GATEWAY_TOKEN": gatewayToken,
	}), server.Client(), time.Now())
	if err != nil {
		t.Fatal(err)
	}
	result, err := send(t, runner)
	if err != nil {
		t.Fatal(err)
	}
	if result.Message != "ok" {
		t.Fatalf("result = %q", result.Message)
	}
}

func TestCancellationReachesModelAndTool(t *testing.T) {
	t.Run("model", func(t *testing.T) {
		started := make(chan struct{})
		var once sync.Once
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var payload map[string]any
			_ = json.NewDecoder(r.Body).Decode(&payload)
			once.Do(func() { close(started) })
			<-r.Context().Done()
		}))
		defer server.Close()
		app := discover.NativeSpec{Instructions: "test", Model: "model"}
		runner, err := runnerFromConfig(app, env(map[string]string{
			"GARDEN_MODEL_BACKEND": "openai", "GARDEN_OPENAI_BASE_URL": server.URL,
		}), server.Client(), time.Now())
		if err != nil {
			t.Fatal(err)
		}
		assertCancelled(t, runner, started)
	})

	t.Run("tool", func(t *testing.T) {
		started := make(chan struct{})
		tool := &blockingTool{started: started}
		app := discover.NativeSpec{Instructions: "test", Model: "model", Tools: []string{"blocking"}}
		runner, err := NewRunner(app, staticModel{result: message{
			Role: "assistant", ToolCalls: []toolCall{{ID: "call", Name: "blocking", Arguments: json.RawMessage(`{}`)}},
		}}, "model", []Tool{tool})
		if err != nil {
			t.Fatal(err)
		}
		assertCancelled(t, runner, started)
		if !tool.cancelled.Load() {
			t.Fatal("tool did not observe cancellation")
		}
	})
}

func TestUpstreamErrorsDoNotLeakSecrets(t *testing.T) {
	const secret = "credential-never-expose"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, secret, http.StatusUnauthorized)
	}))
	defer server.Close()
	app := discover.NativeSpec{Instructions: "test", Model: "model"}
	runner, err := runnerFromConfig(app, env(map[string]string{
		"GARDEN_MODEL_BACKEND": "openai", "GARDEN_OPENAI_BASE_URL": server.URL,
		"GARDEN_OPENAI_API_KEY": secret,
	}), server.Client(), time.Now())
	if err != nil {
		t.Fatal(err)
	}
	_, err = send(t, runner)
	if err == nil || strings.Contains(err.Error(), secret) {
		t.Fatalf("unsafe error = %v", err)
	}
}

func TestToolErrorsAndPayloadLimitsAreSafe(t *testing.T) {
	const secret = "tool-secret-never-expose"
	app := discover.NativeSpec{Instructions: "test", Model: "model", Tools: []string{"failing"}}
	runner, err := NewRunner(app, staticModel{result: message{
		Role: "assistant", ToolCalls: []toolCall{{ID: "call", Name: "failing", Arguments: json.RawMessage(`{}`)}},
	}}, "model", []Tool{failingTool{secret: secret}})
	if err != nil {
		t.Fatal(err)
	}
	_, err = send(t, runner)
	if err == nil || strings.Contains(err.Error(), secret) {
		t.Fatalf("unsafe tool error = %v", err)
	}

	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		fmt.Fprint(w, strings.Repeat("x", maxPayloadBytes+1))
	}))
	defer server.Close()
	largeApp := discover.NativeSpec{Instructions: strings.Repeat("x", maxPayloadBytes), Model: "model"}
	runner, err = runnerFromConfig(largeApp, env(map[string]string{
		"GARDEN_MODEL_BACKEND": "openai", "GARDEN_OPENAI_BASE_URL": server.URL,
	}), server.Client(), time.Now())
	if err != nil {
		t.Fatal(err)
	}
	_, err = send(t, runner)
	if err == nil || !strings.Contains(err.Error(), "request exceeds 1 MiB") || requests.Load() != 0 {
		t.Fatalf("oversized request error = %v, requests = %d", err, requests.Load())
	}
}

func TestConfigurationIsExplicit(t *testing.T) {
	app := discover.NativeSpec{Instructions: "test", Model: "model"}
	_, err := runnerFromConfig(app, env(nil), http.DefaultClient, time.Now())
	if err == nil || !strings.Contains(err.Error(), "GARDEN_MODEL_BACKEND") {
		t.Fatalf("error = %v", err)
	}
}

func TestCompletedHistoryIsSentInSessionOrder(t *testing.T) {
	backend := &sequenceModel{results: []message{
		{Role: "assistant", Content: "first answer"},
		{Role: "assistant", Content: "second answer"},
	}}
	app := discover.NativeSpec{Instructions: "instructions", Model: "model"}
	runner, err := NewRunner(app, backend, "model", NativeManifest())
	if err != nil {
		t.Fatal(err)
	}
	store, err := workflow.OpenRunner(t.TempDir(), runner)
	if err != nil {
		t.Fatal(err)
	}
	session, err := store.CreateSession()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Send(context.Background(), session, "first question"); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Send(context.Background(), session, "second question"); err != nil {
		t.Fatal(err)
	}
	if len(backend.requests) != 2 {
		t.Fatalf("requests = %d", len(backend.requests))
	}
	messages := backend.requests[1].Messages
	if len(messages) != 3 ||
		messages[0].Role != "user" || messages[0].Content != "first question" ||
		messages[1].Role != "assistant" || messages[1].Content != "first answer" ||
		messages[2].Role != "user" || messages[2].Content != "second question" {
		t.Fatalf("history = %#v", messages)
	}
}

func TestModelRoundsAreCapped(t *testing.T) {
	backend := &repeatingToolModel{}
	app := discover.NativeSpec{Instructions: "test", Model: "model", Tools: []string{"get_weather"}}
	runner, err := NewRunner(app, backend, "model", NativeManifest())
	if err != nil {
		t.Fatal(err)
	}
	_, err = send(t, runner)
	if err == nil || !strings.Contains(err.Error(), "maximum of 8 rounds") {
		t.Fatalf("error = %v", err)
	}
	if backend.calls.Load() != maxModelRounds {
		t.Fatalf("model calls = %d, want %d", backend.calls.Load(), maxModelRounds)
	}
}

type staticModel struct {
	result message
}

func (m staticModel) Complete(context.Context, modelRequest) (message, error) {
	return m.result, nil
}

type sequenceModel struct {
	requests []modelRequest
	results  []message
}

func (m *sequenceModel) Complete(_ context.Context, request modelRequest) (message, error) {
	m.requests = append(m.requests, request)
	result := m.results[0]
	m.results = m.results[1:]
	return result, nil
}

type repeatingToolModel struct {
	calls atomic.Int32
}

func (m *repeatingToolModel) Complete(context.Context, modelRequest) (message, error) {
	call := m.calls.Add(1)
	return message{
		Role: "assistant",
		ToolCalls: []toolCall{{
			ID: fmt.Sprintf("call-%d", call), Name: "get_weather",
			Arguments: json.RawMessage(`{"city":"Jakarta"}`),
		}},
	}, nil
}

type blockingTool struct {
	started   chan struct{}
	once      sync.Once
	cancelled atomic.Bool
}

type failingTool struct {
	secret string
}

func (t failingTool) Definition() ToolDefinition {
	return ToolDefinition{Name: "failing", Description: "Always fail.", Parameters: json.RawMessage(`{"type":"object"}`)}
}

func (t failingTool) Execute(context.Context, json.RawMessage) (json.RawMessage, error) {
	return nil, errors.New(t.secret)
}

func (t *blockingTool) Definition() ToolDefinition {
	return ToolDefinition{Name: "blocking", Description: "Block until cancelled.", Parameters: json.RawMessage(`{"type":"object"}`)}
}

func (t *blockingTool) Execute(ctx context.Context, _ json.RawMessage) (json.RawMessage, error) {
	t.once.Do(func() { close(t.started) })
	<-ctx.Done()
	t.cancelled.Store(true)
	return nil, ctx.Err()
}

func weatherApplication(t *testing.T) discover.NativeSpec {
	t.Helper()
	app, err := discover.ApplicationAt(filepath.Join("..", "..", "examples", "eve-weather"))
	if err != nil {
		t.Fatal(err)
	}
	return app.Native()
}

func env(values map[string]string) func(string) string {
	return func(name string) string { return values[name] }
}

func runnerEventData(t *testing.T, event workflow.RunnerEvent) map[string]any {
	t.Helper()
	payload, err := event.Payload()
	if err != nil {
		t.Fatal(err)
	}
	var data map[string]any
	if err := json.Unmarshal(payload, &data); err != nil {
		t.Fatal(err)
	}
	return data
}

func send(t *testing.T, runner workflow.Runner) (workflow.TurnResult, error) {
	t.Helper()
	store, err := workflow.OpenRunner(t.TempDir(), runner)
	if err != nil {
		t.Fatal(err)
	}
	session, err := store.CreateSession()
	if err != nil {
		t.Fatal(err)
	}
	return store.Send(context.Background(), session, "hello")
}

func assertCancelled(t *testing.T, runner workflow.Runner, started <-chan struct{}) {
	t.Helper()
	store, err := workflow.OpenRunner(t.TempDir(), runner)
	if err != nil {
		t.Fatal(err)
	}
	session, err := store.CreateSession()
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() {
		_, sendErr := store.Send(context.Background(), session, "hello")
		done <- sendErr
	}()
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("step did not start")
	}
	if result, err := store.Cancel(context.Background(), session, ""); err != nil || result != workflow.CancelAccepted {
		t.Fatal("turn cancellation was not accepted")
	}
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("error = %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("cancellation did not propagate")
	}
}
