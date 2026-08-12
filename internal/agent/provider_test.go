package agent

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/thoriqakbar0/garden/internal/discover"
)

func TestNormalizeModelIDOnlyStripsMatchingNativeProvider(t *testing.T) {
	tests := []struct {
		name     string
		provider providerID
		model    string
		want     string
	}{
		{name: "anthropic native", provider: providerAnthropic, model: "anthropic/claude-sonnet-5", want: "claude-sonnet-5"},
		{name: "google native", provider: providerGoogle, model: "google/gemini-3-flash", want: "gemini-3-flash"},
		{name: "openai native", provider: providerOpenAI, model: "openai/gpt-5.4", want: "gpt-5.4"},
		{name: "router model stays opaque", provider: providerOpenAI, model: "anthropic/claude-sonnet-5", want: "anthropic/claude-sonnet-5"},
		{name: "cloudflare model stays opaque", provider: providerOpenAI, model: "@cf/meta/llama", want: "@cf/meta/llama"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := normalizeModelID(test.provider, test.model); got != test.want {
				t.Fatalf("normalizeModelID(%q, %q) = %q, want %q", test.provider, test.model, got, test.want)
			}
		})
	}
}

func TestProviderProfilesDeclareCapabilities(t *testing.T) {
	for _, id := range []providerID{
		providerOpenAI, providerCompatible, providerAnthropic, providerGoogle,
	} {
		profile, ok := providerProfiles[id]
		if !ok || profile.ID != id || profile.API == "" ||
			!profile.Capabilities.NativeTools || !profile.Capabilities.Usage {
			t.Fatalf("provider profile %q = %#v", id, profile)
		}
	}
}

func TestProviderStopReasonsAreStrict(t *testing.T) {
	tests := []struct {
		name     string
		provider providerID
		reason   string
		hasCalls bool
		want     string
		wantErr  bool
	}{
		{name: "OpenAI stop", provider: providerOpenAI, reason: "stop", want: "stop"},
		{name: "OpenAI tools", provider: providerOpenAI, reason: "tool_calls", hasCalls: true, want: "tool-calls"},
		{name: "OpenAI content filter", provider: providerOpenAI, reason: "content_filter", wantErr: true},
		{name: "Anthropic length", provider: providerAnthropic, reason: "max_tokens", want: "length"},
		{name: "Anthropic refusal", provider: providerAnthropic, reason: "refusal", wantErr: true},
		{name: "Google tools", provider: providerGoogle, reason: "STOP", hasCalls: true, want: "tool-calls"},
		{name: "Google safety", provider: providerGoogle, reason: "SAFETY", wantErr: true},
		{name: "mismatched tools", provider: providerOpenAI, reason: "stop", hasCalls: true, wantErr: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := normalizeStopReason(test.provider, test.reason, test.hasCalls)
			if (err != nil) != test.wantErr || got != test.want {
				t.Fatalf("normalizeStopReason() = %q, %v; want %q, error=%t", got, err, test.want, test.wantErr)
			}
		})
	}
}

func TestProviderConfigurationSelectsNativeAdapters(t *testing.T) {
	tests := []struct {
		name        string
		values      map[string]string
		appModel    string
		wantModel   string
		wantBackend any
	}{
		{
			name: "openai compatible keeps foreign router prefix",
			values: map[string]string{
				"GARDEN_MODEL_BACKEND":   "openai",
				"GARDEN_OPENAI_BASE_URL": "https://router.example/v1",
			},
			appModel: "anthropic/claude-sonnet", wantModel: "anthropic/claude-sonnet",
			wantBackend: &openAIModel{provider: providerCompatible},
		},
		{
			name: "native openai strips provider prefix",
			values: map[string]string{
				"GARDEN_MODEL_BACKEND":   "openai",
				"GARDEN_OPENAI_BASE_URL": defaultOpenAIBase,
			},
			appModel: "openai/gpt-test", wantModel: "gpt-test",
			wantBackend: &openAIModel{provider: providerOpenAI},
		},
		{
			name: "anthropic strips native prefix",
			values: map[string]string{
				"GARDEN_MODEL_BACKEND":     "anthropic",
				"GARDEN_ANTHROPIC_API_KEY": "secret",
			},
			appModel: "anthropic/claude-sonnet", wantModel: "claude-sonnet",
			wantBackend: &anthropicModel{},
		},
		{
			name: "google strips native prefix",
			values: map[string]string{
				"GARDEN_MODEL_BACKEND":  "google",
				"GARDEN_GOOGLE_API_KEY": "secret",
			},
			appModel: "google/gemini-flash", wantModel: "gemini-flash",
			wantBackend: &googleModel{},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			runner, err := runnerFromConfig(
				discover.NativeSpec{Instructions: "test", Model: test.appModel},
				env(test.values), http.DefaultClient, time.Now(),
			)
			if err != nil {
				t.Fatal(err)
			}
			if runner.modelName != test.wantModel {
				t.Fatalf("model = %q, want %q", runner.modelName, test.wantModel)
			}
			switch test.wantBackend.(type) {
			case *openAIModel:
				backend, ok := runner.backend.(*openAIModel)
				if !ok {
					t.Fatalf("backend = %T", runner.backend)
				}
				wantProvider := test.wantBackend.(*openAIModel).provider
				if backend.provider != wantProvider {
					t.Fatalf("provider = %q, want %q", backend.provider, wantProvider)
				}
			case *anthropicModel:
				if _, ok := runner.backend.(*anthropicModel); !ok {
					t.Fatalf("backend = %T", runner.backend)
				}
			case *googleModel:
				if _, ok := runner.backend.(*googleModel); !ok {
					t.Fatalf("backend = %T", runner.backend)
				}
			}
		})
	}
}

func TestNativeProviderConfigurationRequiresCredentials(t *testing.T) {
	for _, backend := range []string{"anthropic", "google"} {
		t.Run(backend, func(t *testing.T) {
			_, err := runnerFromConfig(
				discover.NativeSpec{Instructions: "test", Model: backend + "/model"},
				env(map[string]string{"GARDEN_MODEL_BACKEND": backend}),
				http.DefaultClient, time.Now(),
			)
			if err == nil || !strings.Contains(err.Error(), "API_KEY") {
				t.Fatalf("error = %v", err)
			}
		})
	}
}
