package evehost

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"
)

func TestOfficialEveAuthoredTypeScriptAndSandboxTerminal(t *testing.T) {
	root := os.Getenv("GARDEN_EVE_PARITY_FIXTURE_ROOT")
	if root == "" {
		t.Skip("set GARDEN_EVE_PARITY_FIXTURE_ROOT to the deterministic official Eve parity fixture")
	}
	address := availableLoopbackAddress(t)
	host, err := Open(root, address)
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	var output safeBuffer
	done := make(chan error, 1)
	go func() { done <- host.Run(ctx, &output, &output) }()
	t.Cleanup(func() {
		cancel()
		select {
		case err := <-done:
			if err != nil && !errors.Is(err, context.Canceled) {
				t.Errorf("stop official Eve: %v\n%s", err, output.String())
			}
		case <-time.After(10 * time.Second):
			t.Error("official Eve did not stop")
		}
	})

	baseURL := "http://" + address
	waitForOfficialEve(t, baseURL, done, &output)
	assertOfficialToolDiscovered(t, baseURL)
	sessionID := createOfficialSession(t, baseURL)
	assertOfficialParityStream(t, baseURL, sessionID)
}

func availableLoopbackAddress(t *testing.T) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	address := listener.Addr().String()
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}
	return address
}

func waitForOfficialEve(t *testing.T, baseURL string, done chan error, output *safeBuffer) {
	t.Helper()
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		select {
		case err := <-done:
			done <- err
			t.Fatalf("official Eve exited during startup: %v\n%s", err, output.String())
		default:
		}
		response, err := (&http.Client{Timeout: 500 * time.Millisecond}).Get(baseURL + "/eve/v1/health")
		if err == nil {
			_ = response.Body.Close()
			if response.StatusCode == http.StatusOK {
				return
			}
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatalf("official Eve did not become ready\n%s", output.String())
}

func assertOfficialToolDiscovered(t *testing.T, baseURL string) {
	t.Helper()
	response, err := http.Get(baseURL + "/eve/v1/info")
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	var payload struct {
		Tools struct {
			Authored []struct {
				LogicalPath string `json:"logicalPath"`
				Name        string `json:"name"`
			} `json:"authored"`
		} `json:"tools"`
	}
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		t.Fatal(err)
	}
	for _, tool := range payload.Tools.Authored {
		if tool.Name == "typescript_echo" && tool.LogicalPath == "tools/typescript_echo.ts" {
			return
		}
	}
	t.Fatalf("official Eve authored tools = %+v", payload.Tools.Authored)
}

func createOfficialSession(t *testing.T, baseURL string) string {
	t.Helper()
	response, err := http.Post(
		baseURL+"/eve/v1/session",
		"application/json",
		bytes.NewBufferString(`{"message":"prove TypeScript plus terminal parity"}`),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusAccepted {
		t.Fatalf("create session status = %d", response.StatusCode)
	}
	var payload struct {
		SessionID string `json:"sessionId"`
	}
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		t.Fatal(err)
	}
	if payload.SessionID == "" {
		t.Fatal("official Eve returned an empty session id")
	}
	return payload.SessionID
}

func assertOfficialParityStream(t *testing.T, baseURL, sessionID string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	request, err := http.NewRequestWithContext(
		ctx,
		http.MethodGet,
		fmt.Sprintf("%s/eve/v1/session/%s/stream?startIndex=0", baseURL, sessionID),
		nil,
	)
	if err != nil {
		t.Fatal(err)
	}
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()

	foundBash := false
	foundTypeScript := false
	foundFinal := false
	scanner := bufio.NewScanner(response.Body)
	scanner.Buffer(make([]byte, 64*1024), 1<<20)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}
		var event struct {
			Data json.RawMessage `json:"data"`
			Type string          `json:"type"`
		}
		if err := json.Unmarshal(line, &event); err != nil {
			t.Fatal(err)
		}
		switch event.Type {
		case "action.result":
			var data struct {
				Result struct {
					Output   json.RawMessage `json:"output"`
					ToolName string          `json:"toolName"`
				} `json:"result"`
			}
			if err := json.Unmarshal(event.Data, &data); err != nil {
				t.Fatal(err)
			}
			output := string(data.Result.Output)
			foundBash = foundBash || data.Result.ToolName == "bash" && strings.Contains(output, "sandbox-terminal-1to1")
			foundTypeScript = foundTypeScript || data.Result.ToolName == "typescript_echo" && strings.Contains(output, "authored-typescript") && strings.Contains(output, "GARDEN-1TO1")
		case "message.completed":
			var data struct {
				Message string `json:"message"`
			}
			if err := json.Unmarshal(event.Data, &data); err != nil {
				t.Fatal(err)
			}
			foundFinal = strings.Contains(data.Message, "sandbox-terminal-1to1") && strings.Contains(data.Message, "authored-typescript")
		case "session.waiting":
			if !foundBash || !foundTypeScript || !foundFinal {
				t.Fatalf("parity evidence: bash=%t TypeScript=%t final=%t", foundBash, foundTypeScript, foundFinal)
			}
			return
		}
	}
	if err := scanner.Err(); err != nil && !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	t.Fatalf("official Eve stream ended before session.waiting: bash=%t TypeScript=%t final=%t", foundBash, foundTypeScript, foundFinal)
}
