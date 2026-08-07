package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestGardenBinaryHostsOfficialEveEndToEnd(t *testing.T) {
	root := os.Getenv("GARDEN_EVE_PARITY_FIXTURE_ROOT")
	if root == "" {
		t.Skip("set GARDEN_EVE_PARITY_FIXTURE_ROOT or run make test-official")
	}
	binary := filepath.Join(t.TempDir(), "garden")
	build := exec.Command("go", "build", "-trimpath", "-o", binary, ".")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build Garden: %v\n%s", err, output)
	}

	address := availableCLIAddress(t)
	command := exec.Command(binary, "serve", "--runtime", "eve", "--root", root, "--addr", address)
	var output lockedBuffer
	command.Stdout = &output
	command.Stderr = &output
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- command.Wait() }()
	t.Cleanup(func() {
		if command.ProcessState == nil || !command.ProcessState.Exited() {
			_ = command.Process.Signal(os.Interrupt)
		}
		select {
		case err := <-done:
			if err != nil {
				t.Errorf("stop Garden official host: %v\n%s", err, output.String())
			}
		case <-time.After(15 * time.Second):
			_ = command.Process.Kill()
			t.Errorf("Garden official host did not stop\n%s", output.String())
		}
	})

	baseURL := "http://" + address
	waitForGardenOfficialCLI(t, baseURL, done, &output)
	response, err := (&http.Client{Timeout: 5 * time.Second}).Post(
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
	var created struct {
		SessionID string `json:"sessionId"`
	}
	if err := json.NewDecoder(response.Body).Decode(&created); err != nil {
		t.Fatal(err)
	}

	streamContext, cancelStream := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancelStream()
	streamRequest, err := http.NewRequestWithContext(
		streamContext,
		http.MethodGet,
		fmt.Sprintf("%s/eve/v1/session/%s/stream?startIndex=0", baseURL, created.SessionID),
		nil,
	)
	if err != nil {
		t.Fatal(err)
	}
	stream, err := http.DefaultClient.Do(streamRequest)
	if err != nil {
		t.Fatal(err)
	}
	defer stream.Body.Close()
	foundBash := false
	foundTypeScript := false
	foundFinal := false
	scanner := bufio.NewScanner(stream.Body)
	scanner.Buffer(make([]byte, 64*1024), 1<<20)
	for scanner.Scan() {
		if len(bytes.TrimSpace(scanner.Bytes())) == 0 {
			continue
		}
		var event struct {
			Data json.RawMessage `json:"data"`
			Type string          `json:"type"`
		}
		if err := json.Unmarshal(scanner.Bytes(), &event); err != nil {
			t.Fatal(err)
		}
		if event.Type == "step.failed" || event.Type == "turn.failed" || event.Type == "session.failed" {
			t.Fatalf("official Eve emitted %s: %s", event.Type, event.Data)
		}
		if event.Type == "action.result" {
			var data struct {
				Result struct {
					Output struct {
						ExitCode int    `json:"exitCode"`
						Marker   string `json:"marker"`
						Stdout   string `json:"stdout"`
						Value    string `json:"value"`
					} `json:"output"`
					ToolName string `json:"toolName"`
				} `json:"result"`
			}
			if err := json.Unmarshal(event.Data, &data); err != nil {
				t.Fatal(err)
			}
			foundBash = foundBash || data.Result.ToolName == "bash" &&
				data.Result.Output.ExitCode == 0 && data.Result.Output.Stdout == "sandbox-terminal-1to1"
			foundTypeScript = foundTypeScript || data.Result.ToolName == "typescript_echo" &&
				data.Result.Output.Marker == "authored-typescript" &&
				data.Result.Output.Value == "GARDEN-1TO1:sandbox-terminal-1to1"
		}
		if event.Type == "message.completed" {
			var data struct {
				Message string `json:"message"`
			}
			if err := json.Unmarshal(event.Data, &data); err != nil {
				t.Fatal(err)
			}
			foundFinal = data.Message == "bash=sandbox-terminal-1to1; authored=authored-typescript; value=GARDEN-1TO1:sandbox-terminal-1to1"
		}
		if event.Type == "session.waiting" {
			if !foundBash || !foundTypeScript || !foundFinal {
				t.Fatalf("official evidence: bash=%t TypeScript=%t final=%t", foundBash, foundTypeScript, foundFinal)
			}
			return
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	t.Fatalf("official stream ended before session.waiting: bash=%t TypeScript=%t final=%t", foundBash, foundTypeScript, foundFinal)
}

func availableCLIAddress(t *testing.T) string {
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

func waitForGardenOfficialCLI(t *testing.T, baseURL string, done chan error, output *lockedBuffer) {
	t.Helper()
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		select {
		case err := <-done:
			done <- err
			t.Fatalf("Garden official host exited during startup: %v\n%s", err, output.String())
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
	t.Fatalf("Garden official host did not become ready\n%s", output.String())
}

type lockedBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (b *lockedBuffer) Write(data []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.b.Write(data)
}

func (b *lockedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.b.String()
}
