package agent

import (
	"errors"
	"strings"
)

type providerID string

const (
	providerOpenAI     providerID = "openai"
	providerCompatible providerID = "openai-compatible"
	providerAnthropic  providerID = "anthropic"
	providerGoogle     providerID = "google"
)

type providerCapabilities struct {
	Streaming        bool
	NativeTools      bool
	Vision           bool
	StructuredOutput bool
	Reasoning        bool
	Usage            bool
	CacheUsage       bool
}

type providerProfile struct {
	ID           providerID
	API          string
	Capabilities providerCapabilities
}

var providerProfiles = map[providerID]providerProfile{
	providerOpenAI: {
		ID:  providerOpenAI,
		API: "openai-chat-completions",
		Capabilities: providerCapabilities{
			NativeTools: true,
			Usage:       true,
			CacheUsage:  true,
		},
	},
	providerCompatible: {
		ID:  providerCompatible,
		API: "openai-chat-completions",
		Capabilities: providerCapabilities{
			NativeTools: true,
			Usage:       true,
			CacheUsage:  true,
		},
	},
	providerAnthropic: {
		ID:  providerAnthropic,
		API: "anthropic-messages",
		Capabilities: providerCapabilities{
			NativeTools: true,
			Usage:       true,
			CacheUsage:  true,
		},
	},
	providerGoogle: {
		ID:  providerGoogle,
		API: "google-generate-content",
		Capabilities: providerCapabilities{
			NativeTools: true,
			Usage:       true,
			CacheUsage:  true,
		},
	},
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
	profile := providerProfiles[provider]
	return modelMetadata{Provider: profile.ID, API: profile.API, Model: model}
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

func addModelMetadata(data map[string]any, metadata modelMetadata) {
	if metadata.Provider == "" {
		return
	}
	data["providerMetadata"] = map[string]any{
		"api": metadata.API, "model": metadata.Model,
		"provider": string(metadata.Provider), "responseId": metadata.ResponseID,
	}
	totalInput := metadata.Usage.Input + metadata.Usage.CacheRead + metadata.Usage.CacheWrite
	data["usage"] = map[string]int{
		"inputTokens":      totalInput,
		"outputTokens":     metadata.Usage.Output,
		"cacheReadTokens":  metadata.Usage.CacheRead,
		"cacheWriteTokens": metadata.Usage.CacheWrite,
		"totalTokens":      totalInput + metadata.Usage.Output,
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
