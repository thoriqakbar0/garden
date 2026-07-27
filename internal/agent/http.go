package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
)

func postJSON(ctx context.Context, client *http.Client, endpoint, token string, headers map[string]string, payload any) ([]byte, string, int, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, "", 0, errors.New("encode model request")
	}
	if len(body) > maxPayloadBytes {
		return nil, "", 0, errors.New("model request exceeds 1 MiB")
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, "", 0, errors.New("create model request")
	}
	request.Header.Set("Content-Type", "application/json")
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	for name, value := range headers {
		if value != "" {
			request.Header.Set(name, value)
		}
	}
	response, err := client.Do(request)
	if err != nil {
		if ctx.Err() != nil {
			return nil, "", 0, ctx.Err()
		}
		return nil, "", 0, errors.New("model request failed")
	}
	defer response.Body.Close()
	data, err := io.ReadAll(io.LimitReader(response.Body, maxPayloadBytes+1))
	if err != nil {
		if ctx.Err() != nil {
			return nil, "", response.StatusCode, ctx.Err()
		}
		return nil, "", response.StatusCode, errors.New("read model response")
	}
	if len(data) > maxPayloadBytes {
		return nil, "", response.StatusCode, errors.New("model response exceeds 1 MiB")
	}
	return data, response.Header.Get("Content-Type"), response.StatusCode, nil
}
