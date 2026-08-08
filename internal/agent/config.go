package agent

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/thoriqakbar0/garden/internal/discover"
	"github.com/thoriqakbar0/garden/internal/workflow"
)

const (
	defaultOpenAIBase    = "https://api.openai.com/v1"
	defaultAnthropicBase = "https://api.anthropic.com/v1"
	defaultGoogleBase    = "https://generativelanguage.googleapis.com/v1beta"
	defaultCodexModel    = "gpt-5.6-sol"
)

// RunnerFromEnvironment configures the workflow runner used by CLI and HTTP modes.
func RunnerFromEnvironment(app discover.Application) (workflow.Runner, error) {
	return runnerFromEnvironment(app, os.Getenv, exec.LookPath, http.DefaultClient, time.Now())
}

func runnerFromEnvironment(
	app discover.Application,
	getenv func(string) string,
	lookPath func(string) (string, error),
	client *http.Client,
	now time.Time,
) (workflow.Runner, error) {
	backendName := strings.ToLower(strings.TrimSpace(getenv("GARDEN_MODEL_BACKEND")))
	if backendName == "codex" {
		return codexExecRunnerFromConfig(app, getenv, lookPath)
	}
	if backendName == "" {
		if command, err := lookPath("codex"); err == nil {
			return codexExecRunnerFromCommand(app, getenv, command)
		}
	}
	return runnerFromConfig(app, getenv, client, now)
}

// ResponderFromEnvironment preserves the one-shot responder seam for tests and
// older embedders. Runtime entrypoints should use RunnerFromEnvironment.
func ResponderFromEnvironment(app discover.Application) (workflow.Responder, error) {
	runner, err := RunnerFromEnvironment(app)
	if err != nil {
		return nil, err
	}
	return func(ctx context.Context, current string, events []workflow.Event) (string, error) {
		return runner.Run(ctx, workflow.Turn{Message: current, History: events}, func(string, any) error {
			return nil
		})
	}, nil
}

func responderFromConfig(app discover.Application, getenv func(string) string, client *http.Client, now time.Time) (workflow.Responder, error) {
	runner, err := runnerFromConfig(app, getenv, client, now)
	if err != nil {
		return nil, err
	}
	return runner.Respond, nil
}

func runnerFromConfig(app discover.Application, getenv func(string) string, client *http.Client, _ time.Time) (*Runner, error) {
	if client == nil {
		client = http.DefaultClient
	}
	backendName := strings.ToLower(strings.TrimSpace(getenv("GARDEN_MODEL_BACKEND")))
	modelName := strings.TrimSpace(getenv("GARDEN_MODEL"))
	if modelName == "" {
		modelName = app.Model
	}
	var backend model
	var provider providerID
	switch backendName {
	case "openai":
		provider = providerCompatible
		base := strings.TrimSpace(getenv("GARDEN_OPENAI_BASE_URL"))
		apiKey := getenv("GARDEN_OPENAI_API_KEY")
		if base == "" && apiKey != "" {
			base = defaultOpenAIBase
		}
		if base == "" {
			return nil, errors.New("openai backend requires GARDEN_OPENAI_BASE_URL or GARDEN_OPENAI_API_KEY")
		}
		if sameBaseURL(base, defaultOpenAIBase) {
			provider = providerOpenAI
		}
		endpoint, err := endpointURL(base, "chat/completions")
		if err != nil {
			return nil, fmt.Errorf("invalid GARDEN_OPENAI_BASE_URL: %w", err)
		}
		headers := map[string]string{}
		if token := strings.TrimSpace(getenv("GARDEN_CLOUDFLARE_GATEWAY_TOKEN")); token != "" {
			headers["cf-aig-authorization"] = "Bearer " + token
		}
		backend = &openAIModel{
			client: client, endpoint: endpoint, apiKey: apiKey,
			headers: headers, provider: provider,
		}
	case "anthropic":
		provider = providerAnthropic
		apiKey := getenv("GARDEN_ANTHROPIC_API_KEY")
		if strings.TrimSpace(apiKey) == "" {
			return nil, errors.New("anthropic backend requires GARDEN_ANTHROPIC_API_KEY")
		}
		base := strings.TrimSpace(getenv("GARDEN_ANTHROPIC_BASE_URL"))
		if base == "" {
			base = defaultAnthropicBase
		}
		endpoint, err := endpointURL(base, "messages")
		if err != nil {
			return nil, fmt.Errorf("invalid GARDEN_ANTHROPIC_BASE_URL: %w", err)
		}
		backend = &anthropicModel{client: client, endpoint: endpoint, apiKey: apiKey}
	case "google":
		provider = providerGoogle
		apiKey := getenv("GARDEN_GOOGLE_API_KEY")
		if strings.TrimSpace(apiKey) == "" {
			return nil, errors.New("google backend requires GARDEN_GOOGLE_API_KEY")
		}
		base := strings.TrimSpace(getenv("GARDEN_GOOGLE_BASE_URL"))
		if base == "" {
			base = defaultGoogleBase
		}
		validated, err := validatedBaseURL(base)
		if err != nil {
			return nil, fmt.Errorf("invalid GARDEN_GOOGLE_BASE_URL: %w", err)
		}
		backend = newGoogleModel(client, validated, apiKey)
	default:
		return nil, errors.New("GARDEN_MODEL_BACKEND must be set to openai, anthropic, google, or codex")
	}
	modelName = normalizeModelID(provider, modelName)
	return NewRunner(app, backend, modelName, NativeManifest())
}

func sameBaseURL(left, right string) bool {
	leftURL, leftErr := validatedBaseURL(left)
	rightURL, rightErr := validatedBaseURL(right)
	return leftErr == nil && rightErr == nil && leftURL == rightURL
}

func validatedBaseURL(base string) (string, error) {
	parsed, err := url.Parse(base)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return "", errors.New("must be an absolute HTTP(S) URL")
	}
	if parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", errors.New("must not contain credentials, query, or fragment")
	}
	parsed.Path = strings.TrimRight(parsed.Path, "/")
	return parsed.String(), nil
}

func endpointURL(base, suffix string) (string, error) {
	parsed, err := url.Parse(base)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return "", errors.New("must be an absolute HTTP(S) URL")
	}
	if parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", errors.New("must not contain credentials, query, or fragment")
	}
	path := strings.TrimRight(parsed.Path, "/")
	if !strings.HasSuffix(path, "/"+suffix) {
		path += "/" + suffix
	}
	parsed.Path = path
	return parsed.String(), nil
}
