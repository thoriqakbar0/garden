package agent

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/thoriqakbar0/garden/internal/discover"
	"github.com/thoriqakbar0/garden/internal/workflow"
)

const (
	defaultOpenAIBase = "https://api.openai.com/v1"
	defaultCodexAPI   = "https://api.openai.com/v1/responses"
	defaultCodexChat  = "https://chatgpt.com/backend-api/codex/responses"
	defaultCodexModel = "gpt-5.6-sol"
)

// RunnerFromEnvironment configures the workflow runner used by CLI and HTTP modes.
func RunnerFromEnvironment(app discover.Application) (workflow.Runner, error) {
	return runnerFromConfig(app, os.Getenv, http.DefaultClient, time.Now())
}

// ResponderFromEnvironment preserves the one-shot responder seam for tests and
// older embedders. Runtime entrypoints should use RunnerFromEnvironment.
func ResponderFromEnvironment(app discover.Application) (workflow.Responder, error) {
	runner, err := runnerFromConfig(app, os.Getenv, http.DefaultClient, time.Now())
	if err != nil {
		return nil, err
	}
	return runner.Respond, nil
}

func responderFromConfig(app discover.Application, getenv func(string) string, client *http.Client, now time.Time) (workflow.Responder, error) {
	runner, err := runnerFromConfig(app, getenv, client, now)
	if err != nil {
		return nil, err
	}
	return runner.Respond, nil
}

func runnerFromConfig(app discover.Application, getenv func(string) string, client *http.Client, now time.Time) (*Runner, error) {
	if client == nil {
		client = http.DefaultClient
	}
	backendName := strings.ToLower(strings.TrimSpace(getenv("GARDEN_MODEL_BACKEND")))
	modelName := strings.TrimSpace(getenv("GARDEN_MODEL"))
	if modelName == "" {
		modelName = app.Model
	}
	var backend model
	switch backendName {
	case "openai":
		base := strings.TrimSpace(getenv("GARDEN_OPENAI_BASE_URL"))
		apiKey := getenv("GARDEN_OPENAI_API_KEY")
		if base == "" && apiKey != "" {
			base = defaultOpenAIBase
		}
		if base == "" {
			return nil, errors.New("openai backend requires GARDEN_OPENAI_BASE_URL or GARDEN_OPENAI_API_KEY")
		}
		endpoint, err := endpointURL(base, "chat/completions")
		if err != nil {
			return nil, fmt.Errorf("invalid GARDEN_OPENAI_BASE_URL: %w", err)
		}
		backend = &openAIModel{client: client, endpoint: endpoint, apiKey: apiKey}
	case "codex":
		configured, configuredModel, err := codexFromEnvironment(getenv, client, now)
		if err != nil {
			return nil, err
		}
		backend = configured
		override := strings.TrimSpace(getenv("GARDEN_MODEL"))
		if override == "" {
			override = configuredModel
		}
		modelName, err = normalizeCodexModel(override)
		if err != nil {
			return nil, err
		}
	default:
		return nil, errors.New("GARDEN_MODEL_BACKEND must be set to openai or codex")
	}
	return NewRunner(app, backend, modelName, NativeManifest())
}

func codexFromEnvironment(getenv func(string) string, client *http.Client, now time.Time) (model, string, error) {
	home := strings.TrimSpace(getenv("CODEX_HOME"))
	if home == "" {
		userHome, err := os.UserHomeDir()
		if err != nil {
			return nil, "", errors.New("resolve home directory for CODEX_HOME")
		}
		home = filepath.Join(userHome, ".codex")
	}
	path := filepath.Join(home, "auth.json")
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, "", fmt.Errorf("Codex auth not found at %s; run `codex login`", path)
		}
		return nil, "", fmt.Errorf("read Codex auth at %s", path)
	}
	if len(data) > maxPayloadBytes {
		return nil, "", errors.New("Codex auth.json exceeds 1 MiB")
	}
	var auth codexAuthFile
	if err := json.Unmarshal(data, &auth); err != nil {
		return nil, "", fmt.Errorf("Codex auth at %s is malformed", path)
	}
	override := strings.TrimSpace(getenv("GARDEN_CODEX_BASE_URL"))
	credentials, selectionErr := selectCodexCredentials(auth, now)
	if selectionErr != nil {
		return nil, "", selectionErr
	}
	if credentials.kind == "api-key" {
		endpoint := defaultCodexAPI
		if override != "" {
			endpoint, err = endpointURL(override, "responses")
			if err != nil {
				return nil, "", fmt.Errorf("invalid GARDEN_CODEX_BASE_URL: %w", err)
			}
		}
		return &codexModel{client: client, endpoint: endpoint, token: credentials.token}, defaultCodexModel, nil
	}
	endpoint := defaultCodexChat
	if override != "" {
		endpoint, err = endpointURL(override, "responses")
		if err != nil {
			return nil, "", fmt.Errorf("invalid GARDEN_CODEX_BASE_URL: %w", err)
		}
	}
	return &codexModel{
		client: client, endpoint: endpoint, token: credentials.token, accountID: credentials.accountID, chatGPT: true,
	}, defaultCodexModel, nil
}

type codexAuthFile struct {
	APIKey   string      `json:"OPENAI_API_KEY"`
	AuthMode string      `json:"auth_mode"`
	Tokens   codexTokens `json:"tokens"`
}

type codexTokens struct {
	AccessToken string `json:"access_token"`
	AccountID   string `json:"account_id"`
	IDToken     string `json:"id_token"`
}

type codexCredentials struct {
	kind      string
	token     string
	accountID string
}

func selectCodexCredentials(auth codexAuthFile, now time.Time) (codexCredentials, error) {
	apiKey := strings.TrimSpace(auth.APIKey)
	accessToken := strings.TrimSpace(auth.Tokens.AccessToken)
	chatGPTUsable := accessToken != "" && !tokenExpired(accessToken, now)
	mode := strings.TrimSpace(auth.AuthMode)

	if mode == "api-key" && apiKey != "" {
		return codexCredentials{kind: "api-key", token: apiKey}, nil
	}
	if mode == "chatgpt" && chatGPTUsable {
		return chatGPTCredentials(auth.Tokens, accessToken), nil
	}
	if chatGPTUsable {
		return chatGPTCredentials(auth.Tokens, accessToken), nil
	}
	if apiKey != "" {
		return codexCredentials{kind: "api-key", token: apiKey}, nil
	}
	if accessToken != "" && tokenExpired(accessToken, now) {
		return codexCredentials{}, errors.New("Codex ChatGPT access token is expired; run `codex login`")
	}
	return codexCredentials{}, errors.New("Codex auth has neither usable API-key nor ChatGPT token credentials; run `codex login`")
}

func chatGPTCredentials(tokens codexTokens, accessToken string) codexCredentials {
	accountID := strings.TrimSpace(tokens.AccountID)
	if accountID == "" {
		accountID = accountIDFromToken(strings.TrimSpace(tokens.IDToken))
	}
	if accountID == "" {
		accountID = accountIDFromToken(accessToken)
	}
	return codexCredentials{kind: "chatgpt", token: accessToken, accountID: accountID}
}

func normalizeCodexModel(model string) (string, error) {
	trimmed := strings.TrimSpace(model)
	const openAIPrefix = "openai/"
	if strings.HasPrefix(trimmed, openAIPrefix) {
		trimmed = strings.TrimPrefix(trimmed, openAIPrefix)
	}
	if trimmed == "" {
		return "", errors.New("GARDEN_MODEL must name an OpenAI model for the codex backend")
	}
	if strings.Contains(trimmed, "/") {
		return "", fmt.Errorf("codex backend supports only bare or openai/-prefixed model IDs; received %q", model)
	}
	return trimmed, nil
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
