package agent

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
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
			fmt.Fprint(w, `{"choices":[{"message":{"role":"assistant","content":null,"tool_calls":[{"id":"weather-1","type":"function","function":{"name":"get_weather","arguments":"{\"city\":\"Jakarta\"}"}}]}}]}`)
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
	if assistant["role"] != "assistant" || toolResult["role"] != "tool" ||
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

func TestRunnerEmitsEveToolLifecycle(t *testing.T) {
	backend := &sequenceModel{results: []message{
		{Role: "assistant", ToolCalls: []toolCall{{
			ID: "weather-1", Name: "get_weather", Arguments: json.RawMessage(`{"city":"Jakarta"}`),
		}}},
		{Role: "assistant", Content: "Sunny."},
	}}
	runner, err := NewRunner(discover.Application{
		Instructions: "test", Model: "model", Tools: []string{"get_weather"},
	}, backend, "model", NativeManifest())
	if err != nil {
		t.Fatal(err)
	}
	var eventTypes []string
	result, err := runner.Run(context.Background(), workflow.Turn{
		SessionID: "ses_test", TurnID: "turn_test", Message: "Weather?", Sequence: 2,
	}, func(eventType string, _ any) error {
		eventTypes = append(eventTypes, eventType)
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
	app := discover.Application{Instructions: "test", Model: "model", Tools: []string{"missing"}}
	if _, err := NewResponder(app, staticModel{}, "model", NativeManifest()); err == nil ||
		!strings.Contains(err.Error(), `declared tool "missing"`) {
		t.Fatalf("unimplemented tool error = %v", err)
	}

	app.Tools = nil
	responder, err := NewResponder(app, staticModel{result: message{
		Role: "assistant",
		ToolCalls: []toolCall{{
			ID: "call-1", Name: "get_weather", Arguments: json.RawMessage(`{"city":"secret-city"}`),
		}},
	}}, "model", NativeManifest())
	if err != nil {
		t.Fatal(err)
	}
	_, err = send(t, responder)
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
			name:   "ambiguous response",
			result: message{Role: "assistant", Content: "text", ToolCalls: []toolCall{{ID: "call", Name: "get_weather", Arguments: json.RawMessage(`{"city":"A"}`)}}},
			match:  "either text or tool calls",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			app := discover.Application{Instructions: "test", Model: "model", Tools: []string{"get_weather"}}
			responder, err := NewResponder(app, staticModel{result: test.result}, "model", NativeManifest())
			if err != nil {
				t.Fatal(err)
			}
			_, err = send(t, responder)
			if err == nil || !strings.Contains(err.Error(), test.match) {
				t.Fatalf("error = %v, want %q", err, test.match)
			}
		})
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
		app := discover.Application{Instructions: "test", Model: "model"}
		responder, err := responderFromConfig(app, env(map[string]string{
			"GARDEN_MODEL_BACKEND": "openai", "GARDEN_OPENAI_BASE_URL": server.URL,
		}), server.Client(), time.Now())
		if err != nil {
			t.Fatal(err)
		}
		assertCancelled(t, responder, started)
	})

	t.Run("tool", func(t *testing.T) {
		started := make(chan struct{})
		tool := &blockingTool{started: started}
		app := discover.Application{Instructions: "test", Model: "model", Tools: []string{"blocking"}}
		responder, err := NewResponder(app, staticModel{result: message{
			Role: "assistant", ToolCalls: []toolCall{{ID: "call", Name: "blocking", Arguments: json.RawMessage(`{}`)}},
		}}, "model", []Tool{tool})
		if err != nil {
			t.Fatal(err)
		}
		assertCancelled(t, responder, started)
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
	app := discover.Application{Instructions: "test", Model: "model"}
	responder, err := responderFromConfig(app, env(map[string]string{
		"GARDEN_MODEL_BACKEND": "openai", "GARDEN_OPENAI_BASE_URL": server.URL,
		"GARDEN_OPENAI_API_KEY": secret,
	}), server.Client(), time.Now())
	if err != nil {
		t.Fatal(err)
	}
	_, err = send(t, responder)
	if err == nil || strings.Contains(err.Error(), secret) {
		t.Fatalf("unsafe error = %v", err)
	}
}

func TestToolErrorsAndPayloadLimitsAreSafe(t *testing.T) {
	const secret = "tool-secret-never-expose"
	app := discover.Application{Instructions: "test", Model: "model", Tools: []string{"failing"}}
	responder, err := NewResponder(app, staticModel{result: message{
		Role: "assistant", ToolCalls: []toolCall{{ID: "call", Name: "failing", Arguments: json.RawMessage(`{}`)}},
	}}, "model", []Tool{failingTool{secret: secret}})
	if err != nil {
		t.Fatal(err)
	}
	_, err = send(t, responder)
	if err == nil || strings.Contains(err.Error(), secret) {
		t.Fatalf("unsafe tool error = %v", err)
	}

	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		fmt.Fprint(w, strings.Repeat("x", maxPayloadBytes+1))
	}))
	defer server.Close()
	largeApp := discover.Application{Instructions: strings.Repeat("x", maxPayloadBytes), Model: "model"}
	responder, err = responderFromConfig(largeApp, env(map[string]string{
		"GARDEN_MODEL_BACKEND": "openai", "GARDEN_OPENAI_BASE_URL": server.URL,
	}), server.Client(), time.Now())
	if err != nil {
		t.Fatal(err)
	}
	_, err = send(t, responder)
	if err == nil || !strings.Contains(err.Error(), "request exceeds 1 MiB") || requests.Load() != 0 {
		t.Fatalf("oversized request error = %v, requests = %d", err, requests.Load())
	}
}

func TestCodexAuthAndResponsesTransport(t *testing.T) {
	tests := []struct {
		name      string
		auth      string
		wantToken string
		wantAcct  string
		override  string
		wantModel string
	}{
		{
			name: "api key", auth: `{"OPENAI_API_KEY":"api-secret"}`,
			wantToken: "api-secret", override: "openai/gpt-test", wantModel: "gpt-test",
		},
		{
			name: "ChatGPT", auth: `{"tokens":{"access_token":"chat-secret","account_id":"acct-1"}}`,
			wantToken: "chat-secret", wantAcct: "acct-1", wantModel: defaultCodexModel,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			home := t.TempDir()
			if err := os.WriteFile(filepath.Join(home, "auth.json"), []byte(test.auth), 0o600); err != nil {
				t.Fatal(err)
			}
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Header.Get("Authorization") != "Bearer "+test.wantToken {
					t.Errorf("authorization header mismatch")
				}
				if r.Header.Get("originator") != "eve" {
					t.Errorf("originator = %q", r.Header.Get("originator"))
				}
				if r.Header.Get("ChatGPT-Account-Id") != test.wantAcct {
					t.Errorf("account header = %q", r.Header.Get("ChatGPT-Account-Id"))
				}
				var payload map[string]any
				if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
					t.Error(err)
				}
				if payload["model"] != test.wantModel || payload["store"] != false {
					t.Errorf("payload = %#v", payload)
				}
				w.Header().Set("Content-Type", "text/event-stream")
				fmt.Fprintln(w, `data: {"type":"response.completed","response":{"output":[{"type":"message","role":"assistant","content":[{"type":"output_text","text":"codex result"}]}]}}`)
				fmt.Fprintln(w, "data: [DONE]")
			}))
			defer server.Close()
			app := discover.Application{Instructions: "test", Model: "authored-model"}
			responder, err := responderFromConfig(app, env(map[string]string{
				"GARDEN_MODEL_BACKEND": "codex", "CODEX_HOME": home,
				"GARDEN_CODEX_BASE_URL": server.URL,
				"GARDEN_MODEL":          test.override,
			}), server.Client(), time.Now())
			if err != nil {
				t.Fatal(err)
			}
			result, err := send(t, responder)
			if err != nil {
				t.Fatal(err)
			}
			if result.Message != "codex result" {
				t.Fatalf("message = %q", result.Message)
			}
		})
	}
}

func TestCodexCredentialSelectionMatchesEvePrecedence(t *testing.T) {
	now := time.Unix(1_000, 0)
	freshToken := jwt(t, map[string]any{"exp": 2_000})
	expiredToken := jwt(t, map[string]any{"exp": 999})
	tests := []struct {
		name     string
		auth     codexAuthFile
		wantKind string
		want     string
	}{
		{
			name: "explicit api key wins",
			auth: codexAuthFile{
				AuthMode: "api-key", APIKey: " key ",
				Tokens: codexTokens{AccessToken: freshToken},
			},
			wantKind: "api-key", want: "key",
		},
		{
			name: "explicit ChatGPT wins",
			auth: codexAuthFile{
				AuthMode: "chatgpt", APIKey: "key",
				Tokens: codexTokens{AccessToken: freshToken},
			},
			wantKind: "chatgpt", want: freshToken,
		},
		{
			name: "ChatGPT fallback precedes key",
			auth: codexAuthFile{
				APIKey: "key", Tokens: codexTokens{AccessToken: freshToken},
			},
			wantKind: "chatgpt", want: freshToken,
		},
		{
			name: "empty tokens do not outrank key",
			auth: codexAuthFile{
				AuthMode: "chatgpt", APIKey: "key",
				Tokens: codexTokens{AccessToken: " "},
			},
			wantKind: "api-key", want: "key",
		},
		{
			name: "expired tokens do not outrank key",
			auth: codexAuthFile{
				AuthMode: "chatgpt", APIKey: "key",
				Tokens: codexTokens{AccessToken: expiredToken},
			},
			wantKind: "api-key", want: "key",
		},
		{
			name: "missing selected key falls back to ChatGPT",
			auth: codexAuthFile{
				AuthMode: "api-key", Tokens: codexTokens{AccessToken: freshToken},
			},
			wantKind: "chatgpt", want: freshToken,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := selectCodexCredentials(test.auth, now)
			if err != nil {
				t.Fatal(err)
			}
			if got.kind != test.wantKind || got.token != test.want {
				t.Fatalf("credentials = %#v, want kind %q and selected token", got, test.wantKind)
			}
		})
	}
}

func TestCodexExtractsAccountIDFromNestedTokenClaims(t *testing.T) {
	accessToken := jwt(t, map[string]any{
		"exp": 2_000,
		"https://api.openai.com/auth": map[string]any{
			"chatgpt_account_id": "account-from-access",
		},
	})
	idToken := jwt(t, map[string]any{
		"chatgpt_account_id": "account-from-id",
	})
	credentials, err := selectCodexCredentials(codexAuthFile{
		Tokens: codexTokens{AccessToken: accessToken, IDToken: idToken},
	}, time.Unix(1_000, 0))
	if err != nil {
		t.Fatal(err)
	}
	if credentials.accountID != "account-from-id" {
		t.Fatalf("account ID = %q", credentials.accountID)
	}

	credentials, err = selectCodexCredentials(codexAuthFile{
		Tokens: codexTokens{AccessToken: accessToken},
	}, time.Unix(1_000, 0))
	if err != nil {
		t.Fatal(err)
	}
	if credentials.accountID != "account-from-access" {
		t.Fatalf("fallback account ID = %q", credentials.accountID)
	}
}

func TestCodexModelNormalization(t *testing.T) {
	tests := []struct {
		input string
		want  string
		ok    bool
	}{
		{input: "gpt-5.6-sol", want: "gpt-5.6-sol", ok: true},
		{input: " openai/gpt-5.6-sol ", want: "gpt-5.6-sol", ok: true},
		{input: "anthropic/claude-sonnet", ok: false},
		{input: "openai/", ok: false},
	}
	for _, test := range tests {
		got, err := normalizeCodexModel(test.input)
		if test.ok && (err != nil || got != test.want) {
			t.Fatalf("normalize %q = %q, %v", test.input, got, err)
		}
		if !test.ok && err == nil {
			t.Fatalf("normalize %q unexpectedly succeeded as %q", test.input, got)
		}
	}
}

func TestCodexExpiredTokenError(t *testing.T) {
	home := t.TempDir()
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"none"}`))
	payload := base64.RawURLEncoding.EncodeToString([]byte(`{"exp":1}`))
	auth := fmt.Sprintf(`{"tokens":{"access_token":"%s.%s.signature"}}`, header, payload)
	if err := os.WriteFile(filepath.Join(home, "auth.json"), []byte(auth), 0o600); err != nil {
		t.Fatal(err)
	}
	_, _, err := codexFromEnvironment(env(map[string]string{"CODEX_HOME": home}), http.DefaultClient, time.Now())
	if err == nil || err.Error() != "Codex ChatGPT access token is expired; run `codex login`" {
		t.Fatalf("error = %v", err)
	}
	if strings.Contains(err.Error(), payload) {
		t.Fatal("expired-token error leaked token")
	}
}

func TestConfigurationIsExplicit(t *testing.T) {
	app := discover.Application{Instructions: "test", Model: "model"}
	_, err := responderFromConfig(app, env(nil), http.DefaultClient, time.Now())
	if err == nil || !strings.Contains(err.Error(), "GARDEN_MODEL_BACKEND") {
		t.Fatalf("error = %v", err)
	}
}

func TestCompletedHistoryIsSentInSessionOrder(t *testing.T) {
	backend := &sequenceModel{results: []message{
		{Role: "assistant", Content: "first answer"},
		{Role: "assistant", Content: "second answer"},
	}}
	app := discover.Application{Instructions: "instructions", Model: "model"}
	responder, err := NewResponder(app, backend, "model", NativeManifest())
	if err != nil {
		t.Fatal(err)
	}
	store, err := workflow.Open(t.TempDir(), responder)
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
	app := discover.Application{Instructions: "test", Model: "model", Tools: []string{"get_weather"}}
	responder, err := NewResponder(app, backend, "model", NativeManifest())
	if err != nil {
		t.Fatal(err)
	}
	_, err = send(t, responder)
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

func weatherApplication(t *testing.T) discover.Application {
	t.Helper()
	app, err := discover.ApplicationAt(filepath.Join("..", "..", "examples", "eve-weather"))
	if err != nil {
		t.Fatal(err)
	}
	return app
}

func env(values map[string]string) func(string) string {
	return func(name string) string { return values[name] }
}

func jwt(t *testing.T, claims map[string]any) string {
	t.Helper()
	payload, err := json.Marshal(claims)
	if err != nil {
		t.Fatal(err)
	}
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"none"}`))
	return header + "." + base64.RawURLEncoding.EncodeToString(payload) + ".signature"
}

func send(t *testing.T, responder workflow.Responder) (workflow.TurnResult, error) {
	t.Helper()
	store, err := workflow.Open(t.TempDir(), responder)
	if err != nil {
		t.Fatal(err)
	}
	session, err := store.CreateSession()
	if err != nil {
		t.Fatal(err)
	}
	return store.Send(context.Background(), session, "hello")
}

func assertCancelled(t *testing.T, responder workflow.Responder, started <-chan struct{}) {
	t.Helper()
	store, err := workflow.Open(t.TempDir(), responder)
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
