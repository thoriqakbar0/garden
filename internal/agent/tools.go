package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

type weatherTool struct{}

func (weatherTool) Definition() ToolDefinition {
	return ToolDefinition{
		Name:        "get_weather",
		Description: "Get deterministic example weather for a city.",
		Parameters: json.RawMessage(`{
			"type":"object",
			"properties":{"city":{"type":"string","description":"City name"}},
			"required":["city"],
			"additionalProperties":false
		}`),
	}
}

func (weatherTool) Execute(ctx context.Context, arguments json.RawMessage) (json.RawMessage, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	var input struct {
		City string `json:"city"`
	}
	decoder := json.NewDecoder(bytes.NewReader(arguments))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		return nil, errors.New("get_weather arguments must contain only a string city")
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return nil, errors.New("get_weather arguments must contain one JSON object")
	}
	if input.City == "" {
		return nil, errors.New("get_weather city must not be empty")
	}
	output, err := json.Marshal(map[string]any{
		"city":         input.City,
		"temperatureF": 72,
		"condition":    "Sunny",
		"summary":      fmt.Sprintf("Sunny in %s with a light breeze.", input.City),
	})
	if err != nil {
		return nil, errors.New("encode get_weather result")
	}
	return output, nil
}

// NativeManifest returns the capabilities compiled into Garden.
func NativeManifest() []Tool {
	return []Tool{weatherTool{}}
}
