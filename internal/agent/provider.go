package agent

import (
	"errors"
	"strings"

	"github.com/thoriqakbar0/garden/internal/workflow"
)

type providerID string

const (
	providerOpenAI     providerID = "openai"
	providerCompatible providerID = "openai-compatible"
	providerAnthropic  providerID = "anthropic"
	providerGoogle     providerID = "google"
)

var providerAPIs = map[providerID]string{
	providerOpenAI:     "openai-chat-completions",
	providerCompatible: "openai-chat-completions",
	providerAnthropic:  "anthropic-messages",
	providerGoogle:     "google-generate-content",
}

type modelUsage struct {
	Input      int
	Output     int
	CacheRead  int
	CacheWrite int
}

type modelMetadata struct {
	Provider   providerID
	API        string
	Model      string
	ResponseID string
	StopReason string
	Usage      modelUsage
}

func metadataFor(provider providerID, model string) modelMetadata {
	api, ok := providerAPIs[provider]
	if !ok {
		return modelMetadata{Model: model}
	}
	return modelMetadata{Provider: provider, API: api, Model: model}
}

func normalizeStopReason(provider providerID, raw string, hasToolCalls bool) (string, error) {
	reason := strings.ToLower(strings.TrimSpace(raw))
	if reason == "" {
		if hasToolCalls {
			return "tool-calls", nil
		}
		return "stop", nil
	}

	switch provider {
	case providerOpenAI, providerCompatible:
		switch reason {
		case "tool_calls", "function_call":
			if hasToolCalls {
				return "tool-calls", nil
			}
		case "stop":
			if !hasToolCalls {
				return "stop", nil
			}
		case "length":
			if !hasToolCalls {
				return "length", nil
			}
		}
	case providerAnthropic:
		switch reason {
		case "tool_use":
			if hasToolCalls {
				return "tool-calls", nil
			}
		case "end_turn", "stop_sequence":
			if !hasToolCalls {
				return "stop", nil
			}
		case "max_tokens", "model_context_window_exceeded":
			if !hasToolCalls {
				return "length", nil
			}
		}
	case providerGoogle:
		switch reason {
		case "stop":
			if hasToolCalls {
				return "tool-calls", nil
			}
			return "stop", nil
		case "max_tokens":
			if !hasToolCalls {
				return "length", nil
			}
		}
	}
	return "", errors.New("model returned an unsupported finish reason")
}

func workflowCompletionMetadata(metadata modelMetadata) workflow.CompletionMetadata {
	if metadata.Provider == "" {
		return workflow.CompletionMetadata{}
	}
	totalInput := metadata.Usage.Input + metadata.Usage.CacheRead + metadata.Usage.CacheWrite
	return workflow.CompletionMetadata{
		Provider: &workflow.ProviderMetadata{
			API: metadata.API, Model: metadata.Model,
			Provider: string(metadata.Provider), ResponseID: metadata.ResponseID,
		},
		Usage: &workflow.TokenUsage{
			InputTokens: totalInput, OutputTokens: metadata.Usage.Output,
			CacheReadTokens: metadata.Usage.CacheRead, CacheWriteTokens: metadata.Usage.CacheWrite,
			TotalTokens: totalInput + metadata.Usage.Output,
		},
	}
}

func normalizeModelID(provider providerID, model string) string {
	trimmed := strings.TrimSpace(model)
	prefix := string(provider) + "/"
	if strings.HasPrefix(strings.ToLower(trimmed), prefix) {
		return strings.TrimSpace(trimmed[len(prefix):])
	}
	return trimmed
}

func nonnegative(value int) int {
	if value < 0 {
		return 0
	}
	return value
}
