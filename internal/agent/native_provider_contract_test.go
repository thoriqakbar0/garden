package agent

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestNativeProviderToolContinuationContract(t *testing.T) {
	t.Run("Anthropic", func(t *testing.T) {
		var requests []map[string]any
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var request map[string]any
			if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
				t.Errorf("decode request: %v", err)
				return
			}
			requests = append(requests, request)
			w.Header().Set("Content-Type", "application/json")
			if len(requests) == 1 {
				fmt.Fprint(w, `{"id":"msg-tool","type":"message","role":"assistant","model":"claude-test","content":[{"type":"text","text":"Checking."},{"type":"tool_use","id":"weather-1","name":"get_weather","input":{"city":"Jakarta"}}],"stop_reason":"tool_use"}`)
				return
			}
			fmt.Fprint(w, `{"id":"msg-final","type":"message","role":"assistant","model":"claude-test","content":[{"type":"text","text":"It is sunny."}],"stop_reason":"end_turn"}`)
		}))
		defer server.Close()

		runner, err := runnerFromConfig(weatherApplication(t), env(map[string]string{
			"GARDEN_MODEL_BACKEND":      "anthropic",
			"GARDEN_ANTHROPIC_API_KEY":  "test-key",
			"GARDEN_ANTHROPIC_BASE_URL": server.URL,
			"GARDEN_MODEL":              "anthropic/claude-test",
		}), server.Client(), time.Now())
		if err != nil {
			t.Fatal(err)
		}
		result, err := send(t, runner)
		if err != nil {
			t.Fatal(err)
		}
		if result.Message != "It is sunny." || len(requests) != 2 {
			t.Fatalf("result = %q, requests = %d", result.Message, len(requests))
		}
		messages := requests[1]["messages"].([]any)
		assistant := messages[len(messages)-2].(map[string]any)
		assistantBlocks := assistant["content"].([]any)
		toolResult := messages[len(messages)-1].(map[string]any)
		resultBlocks := toolResult["content"].([]any)
		if assistant["role"] != "assistant" || len(assistantBlocks) != 2 ||
			assistantBlocks[0].(map[string]any)["text"] != "Checking." ||
			assistantBlocks[1].(map[string]any)["id"] != "weather-1" ||
			toolResult["role"] != "user" || resultBlocks[0].(map[string]any)["tool_use_id"] != "weather-1" {
			t.Fatalf("continuation messages = %#v", messages)
		}
	})

	t.Run("Google", func(t *testing.T) {
		var requests []map[string]any
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var request map[string]any
			if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
				t.Errorf("decode request: %v", err)
				return
			}
			requests = append(requests, request)
			w.Header().Set("Content-Type", "application/json")
			if len(requests) == 1 {
				fmt.Fprint(w, `{"responseId":"google-tool","modelVersion":"gemini-test","candidates":[{"content":{"role":"model","parts":[{"text":"Checking."},{"thoughtSignature":"signed-thought","functionCall":{"id":"weather-1","name":"get_weather","args":{"city":"Jakarta"}}}]},"finishReason":"STOP"}]}`)
				return
			}
			fmt.Fprint(w, `{"responseId":"google-final","modelVersion":"gemini-test","candidates":[{"content":{"role":"model","parts":[{"text":"It is sunny."}]},"finishReason":"STOP"}]}`)
		}))
		defer server.Close()

		runner, err := runnerFromConfig(weatherApplication(t), env(map[string]string{
			"GARDEN_MODEL_BACKEND":   "google",
			"GARDEN_GOOGLE_API_KEY":  "test-key",
			"GARDEN_GOOGLE_BASE_URL": server.URL,
			"GARDEN_MODEL":           "google/gemini-test",
		}), server.Client(), time.Now())
		if err != nil {
			t.Fatal(err)
		}
		result, err := send(t, runner)
		if err != nil {
			t.Fatal(err)
		}
		if result.Message != "It is sunny." || len(requests) != 2 {
			t.Fatalf("result = %q, requests = %d", result.Message, len(requests))
		}
		contents := requests[1]["contents"].([]any)
		assistant := contents[len(contents)-2].(map[string]any)
		assistantParts := assistant["parts"].([]any)
		callPart := assistantParts[1].(map[string]any)
		toolResult := contents[len(contents)-1].(map[string]any)
		response := toolResult["parts"].([]any)[0].(map[string]any)["functionResponse"].(map[string]any)
		if assistant["role"] != "model" || assistantParts[0].(map[string]any)["text"] != "Checking." ||
			callPart["thoughtSignature"] != "signed-thought" ||
			callPart["functionCall"].(map[string]any)["id"] != "weather-1" ||
			toolResult["role"] != "user" || response["id"] != "weather-1" || response["name"] != "get_weather" {
			t.Fatalf("continuation contents = %#v", contents)
		}
	})
}
