package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/thoriqakbar0/garden/internal/discover"
	"github.com/thoriqakbar0/garden/internal/workflow"
)

func TestCodexExecRunsTerminalInsideSandbox(t *testing.T) {
	root := t.TempDir()
	command := filepath.Join(t.TempDir(), "codex")
	script := `#!/bin/sh
cat >/dev/null
printf '%s\n' \
  '{"type":"thread.started","thread_id":"thread-1"}' \
  '{"type":"turn.started"}' \
  '{"type":"item.started","item":{"id":"item_1","type":"command_execution","command":"echo credential-never-persist"}}' \
  '{"type":"item.completed","item":{"id":"item_1","type":"command_execution","command":"echo credential-never-persist","aggregated_output":"credential-never-persist","exit_code":0,"status":"completed"}}' \
  '{"type":"item.completed","item":{"id":"item_2","type":"agent_message","text":"Terminal task completed."}}' \
  '{"type":"turn.completed","usage":{"input_tokens":1,"output_tokens":1}}'
`
	if err := os.WriteFile(command, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	runner := &codexExecRunner{
		command: command, instructions: "Use the terminal when useful.",
		model: defaultCodexModel, root: root, sandbox: defaultCodexSandbox,
	}
	var eventTypes []string
	var durable strings.Builder
	result, err := runner.Run(context.Background(), workflow.Turn{
		SessionID: "ses_test", TurnID: "turn_test", Message: "Inspect the project.", Sequence: 1,
	}, func(event workflow.RunnerEvent) error {
		eventTypes = append(eventTypes, event.Type())
		payload, payloadErr := event.Payload()
		if payloadErr != nil {
			return payloadErr
		}
		durable.Write(payload)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if result != "Terminal task completed." {
		t.Fatalf("result = %q", result)
	}
	want := []string{
		"step.started", "actions.requested", "step.completed", "action.result",
		"step.started", "message.appended", "message.completed", "step.completed",
	}
	if fmt.Sprint(eventTypes) != fmt.Sprint(want) {
		t.Fatalf("event types = %v, want %v", eventTypes, want)
	}
	if strings.Contains(durable.String(), "credential-never-persist") {
		t.Fatal("durable events exposed terminal command or output")
	}
}

func TestCodexExecConfigurationIsSandboxed(t *testing.T) {
	root := t.TempDir()
	app := discover.NativeSpec{Root: root, Instructions: "test"}
	runner, err := codexExecRunnerFromConfig(app, env(nil), func(name string) (string, error) {
		if name != "codex" {
			t.Fatalf("looked up %q", name)
		}
		return "/usr/local/bin/codex", nil
	})
	if err != nil {
		t.Fatal(err)
	}
	args := strings.Join(runner.arguments(), " ")
	for _, required := range []string{
		"--sandbox workspace-write",
		"--ephemeral",
		"--ignore-user-config",
		`approval_policy="never"`,
		`shell_environment_policy.inherit="core"`,
		`allow_login_shell=false`,
	} {
		if !strings.Contains(args, required) {
			t.Fatalf("arguments %q omit %q", args, required)
		}
	}

	_, err = codexExecRunnerFromConfig(app, env(map[string]string{
		"GARDEN_CODEX_SANDBOX": "danger-full-access",
	}), func(string) (string, error) { return "/usr/local/bin/codex", nil })
	if err == nil || !strings.Contains(err.Error(), "read-only or workspace-write") {
		t.Fatalf("unsafe sandbox error = %v", err)
	}
}

func TestCodexExecReportsEarlyProcessExit(t *testing.T) {
	command := filepath.Join(t.TempDir(), "codex")
	if err := os.WriteFile(command, []byte("#!/bin/sh\nexit 42\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	runner := &codexExecRunner{
		command: command, instructions: "test", model: defaultCodexModel,
		root: t.TempDir(), sandbox: defaultCodexSandbox,
	}
	_, err := runner.Run(context.Background(), workflow.Turn{}, func(workflow.RunnerEvent) error { return nil })
	if err == nil || err.Error() != "Codex CLI exited with status 42" {
		t.Fatalf("early exit error = %v", err)
	}
}

func TestRuntimeSelectsCodexExecBackend(t *testing.T) {
	app := discover.NativeSpec{Root: t.TempDir(), Instructions: "test"}
	runner, err := runnerFromEnvironment(
		app,
		env(map[string]string{"GARDEN_MODEL_BACKEND": "codex"}),
		func(string) (string, error) { return "/usr/local/bin/codex", nil },
		nil,
		time.Time{},
	)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := runner.(*codexExecRunner); !ok {
		t.Fatalf("runner = %T, want *codexExecRunner", runner)
	}
}

func TestRuntimeAutoDetectsCodexExecBackend(t *testing.T) {
	app := discover.NativeSpec{Root: t.TempDir(), Instructions: "test"}
	lookups := 0
	runner, err := runnerFromEnvironment(
		app,
		env(nil),
		func(name string) (string, error) {
			lookups++
			if name != "codex" {
				t.Fatalf("looked up %q", name)
			}
			return "/usr/local/bin/codex", nil
		},
		nil,
		time.Time{},
	)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := runner.(*codexExecRunner); !ok {
		t.Fatalf("runner = %T, want *codexExecRunner", runner)
	}
	if lookups != 1 {
		t.Fatalf("Codex CLI lookups = %d, want 1", lookups)
	}
}

func TestRuntimeRequiresBackendWhenCodexIsMissing(t *testing.T) {
	app := discover.NativeSpec{Root: t.TempDir(), Instructions: "test", Model: "model"}
	_, err := runnerFromEnvironment(
		app,
		env(nil),
		func(string) (string, error) { return "", errors.New("not found") },
		nil,
		time.Time{},
	)
	if err == nil || !strings.Contains(err.Error(), "GARDEN_MODEL_BACKEND") {
		t.Fatalf("error = %v", err)
	}
}

func TestRuntimeDoesNotOverrideExplicitBackend(t *testing.T) {
	app := discover.NativeSpec{Root: t.TempDir(), Instructions: "test", Model: "model"}
	_, err := runnerFromEnvironment(
		app,
		env(map[string]string{"GARDEN_MODEL_BACKEND": "unsupported"}),
		func(string) (string, error) {
			t.Fatal("explicit backend caused a Codex lookup")
			return "", nil
		},
		nil,
		time.Time{},
	)
	if err == nil || !strings.Contains(err.Error(), "GARDEN_MODEL_BACKEND") {
		t.Fatalf("error = %v", err)
	}
}

func TestCodexExecPromptCarriesCompletedConversation(t *testing.T) {
	runner := &codexExecRunner{instructions: "Be concise.", root: t.TempDir()}
	prompt, err := runner.prompt(workflow.Turn{
		Message: "second question",
		History: []workflow.Event{
			{Index: 0, Type: "message.received", TurnID: "turn_1", Data: json.RawMessage(`{"message":"first question"}`)},
			{Index: 1, Type: "message.completed", TurnID: "turn_1", Data: json.RawMessage(`{"message":"first answer"}`)},
			{Index: 2, Type: "turn.completed", TurnID: "turn_1", Data: json.RawMessage(`{}`)},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"Be concise.", "first question", "first answer", "second question"} {
		if !strings.Contains(prompt, expected) {
			t.Fatalf("prompt omits %q", expected)
		}
	}
}

func TestCodexExecPropagatesCancellation(t *testing.T) {
	command := filepath.Join(t.TempDir(), "codex")
	script := `#!/bin/sh
cat >/dev/null
printf '%s\n' '{"type":"item.started","item":{"id":"item_1","type":"command_execution"}}'
exec sleep 10
`
	if err := os.WriteFile(command, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	runner := &codexExecRunner{
		command: command, instructions: "test", model: defaultCodexModel,
		root: t.TempDir(), sandbox: defaultCodexSandbox,
	}
	started := time.Now()
	_, err := runner.Run(ctx, workflow.Turn{TurnID: "turn_test"}, func(event workflow.RunnerEvent) error {
		if event.Type() == "actions.requested" {
			cancel()
		}
		return nil
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation error = %v", err)
	}
	if elapsed := time.Since(started); elapsed > 2*time.Second {
		t.Fatalf("Codex process took %s to stop after cancellation", elapsed)
	}
}
