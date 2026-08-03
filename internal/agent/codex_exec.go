package agent

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/thoriqakbar0/garden/internal/discover"
	"github.com/thoriqakbar0/garden/internal/workflow"
)

const (
	defaultCodexSandbox = "workspace-write"
	codexExecTimeout    = 10 * time.Minute
)

var errCodexTurnIncomplete = errors.New("Codex CLI omitted the completed turn boundary")

type codexExecRunner struct {
	command      string
	instructions string
	model        string
	root         string
	sandbox      string
}

type codexExecEvent struct {
	Type string        `json:"type"`
	Item codexExecItem `json:"item"`
}

type codexExecItem struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Text     string `json:"text"`
	Status   string `json:"status"`
	ExitCode *int   `json:"exit_code"`
}

type codexPrompt struct {
	Instructions string               `json:"instructions"`
	History      []codexPromptMessage `json:"history"`
	Message      string               `json:"message"`
}

type codexPromptMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

func codexExecRunnerFromConfig(
	app discover.Application,
	getenv func(string) string,
	lookPath func(string) (string, error),
) (*codexExecRunner, error) {
	command, err := lookPath("codex")
	if err != nil {
		return nil, errors.New("codex backend requires the Codex CLI on PATH; install it and run `codex login`")
	}
	root, err := filepath.Abs(app.Root)
	if err != nil {
		return nil, errors.New("resolve agent project root")
	}
	model := strings.TrimSpace(getenv("GARDEN_MODEL"))
	if model == "" {
		model = defaultCodexModel
	}
	sandbox := strings.TrimSpace(getenv("GARDEN_CODEX_SANDBOX"))
	if sandbox == "" {
		sandbox = defaultCodexSandbox
	}
	if sandbox != "read-only" && sandbox != "workspace-write" {
		return nil, errors.New("GARDEN_CODEX_SANDBOX must be read-only or workspace-write")
	}
	return &codexExecRunner{
		command: command, instructions: app.Instructions, model: model, root: root, sandbox: sandbox,
	}, nil
}

func (r *codexExecRunner) Run(ctx context.Context, turn workflow.Turn, emit workflow.Emit) (string, error) {
	if emit == nil {
		return "", errors.New("workflow event emitter is required")
	}
	prompt, err := r.prompt(turn)
	if err != nil {
		return "", err
	}
	turnContext, cancel := context.WithTimeout(ctx, codexExecTimeout)
	defer cancel()
	command := exec.CommandContext(turnContext, r.command, r.arguments()...)
	command.Dir = r.root
	command.Stdin = strings.NewReader(prompt)
	command.Stderr = io.Discard
	stdout, err := command.StdoutPipe()
	if err != nil {
		return "", errors.New("open Codex event stream")
	}
	if err := command.Start(); err != nil {
		return "", errors.New("start Codex CLI; run `codex login` and try again")
	}
	result, consumeErr := consumeCodexEvents(stdout, turn, emit)
	if consumeErr != nil {
		_ = command.Process.Kill()
	}
	waitErr := command.Wait()
	if turnContext.Err() != nil {
		return "", turnContext.Err()
	}
	if consumeErr != nil {
		if errors.Is(consumeErr, errCodexTurnIncomplete) && waitErr != nil {
			return "", codexProcessError(waitErr)
		}
		return "", consumeErr
	}
	if waitErr != nil {
		return "", codexProcessError(waitErr)
	}
	return result, nil
}

func codexProcessError(err error) error {
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return fmt.Errorf("Codex CLI exited with status %d", exitErr.ExitCode())
	}
	return errors.New("Codex CLI failed")
}

func (r *codexExecRunner) arguments() []string {
	return []string{
		"exec",
		"--json",
		"--ephemeral",
		"--color", "never",
		"--sandbox", r.sandbox,
		"--cd", r.root,
		"--model", r.model,
		"--skip-git-repo-check",
		"--ignore-user-config",
		"--strict-config",
		"-c", `approval_policy="never"`,
		"-c", `shell_environment_policy.inherit="core"`,
		"-c", `allow_login_shell=false`,
		"-",
	}
}

func (r *codexExecRunner) prompt(turn workflow.Turn) (string, error) {
	history, err := conversation(turn.History)
	if err != nil {
		return "", err
	}
	messages := make([]codexPromptMessage, 0, len(history))
	for _, item := range history {
		if item.Content == "" || (item.Role != "user" && item.Role != "assistant") {
			continue
		}
		messages = append(messages, codexPromptMessage{Role: item.Role, Content: item.Content})
	}
	payload, err := json.Marshal(codexPrompt{
		Instructions: r.instructions,
		History:      messages,
		Message:      turn.Message,
	})
	if err != nil {
		return "", errors.New("encode Codex prompt")
	}
	if len(payload) > maxPayloadBytes {
		return "", errors.New("Codex prompt exceeds 1 MiB")
	}
	return "Run the Eve-shaped agent described by this JSON. Follow its instructions and conversation history. " +
		"You may use terminal tools inside the configured sandbox. Complete the user's request, then return the final answer.\n\n" +
		string(payload), nil
}

func consumeCodexEvents(reader io.Reader, turn workflow.Turn, emit workflow.Emit) (string, error) {
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 64*1024), maxPayloadBytes)
	steps := make(map[string]int)
	completedActions := make(map[string]struct{})
	stepIndex := 0
	stepOpen := false
	turnCompleted := false
	finalMessage := ""

	startStep := func() error {
		if stepOpen {
			return nil
		}
		if err := emit("step.started", map[string]any{
			"sequence": turn.Sequence, "stepIndex": stepIndex, "turnId": turn.TurnID,
		}); err != nil {
			return err
		}
		stepOpen = true
		return nil
	}
	requestAction := func(item codexExecItem) error {
		if _, exists := steps[item.ID]; exists {
			return nil
		}
		if !callIDPattern.MatchString(item.ID) {
			return errors.New("Codex CLI returned a malformed command item ID")
		}
		if err := startStep(); err != nil {
			return err
		}
		if err := emit("actions.requested", map[string]any{
			"actions": []actionRequest{{
				CallID: item.ID, Input: json.RawMessage(`{}`), Kind: "tool-call", ToolName: "terminal",
			}},
			"sequence": turn.Sequence, "stepIndex": stepIndex, "turnId": turn.TurnID,
		}); err != nil {
			return err
		}
		if err := emit("step.completed", map[string]any{
			"finishReason": "tool-calls", "sequence": turn.Sequence,
			"stepIndex": stepIndex, "turnId": turn.TurnID,
		}); err != nil {
			return err
		}
		steps[item.ID] = stepIndex
		stepIndex++
		stepOpen = false
		return nil
	}

	for scanner.Scan() {
		var event codexExecEvent
		if err := json.Unmarshal(scanner.Bytes(), &event); err != nil {
			return "", errors.New("Codex CLI returned malformed JSON events")
		}
		switch event.Type {
		case "item.started":
			if event.Item.Type == "command_execution" {
				if err := requestAction(event.Item); err != nil {
					return "", err
				}
			}
		case "item.completed":
			switch event.Item.Type {
			case "command_execution":
				if _, exists := completedActions[event.Item.ID]; exists {
					return "", errors.New("Codex CLI repeated a completed command item")
				}
				if err := requestAction(event.Item); err != nil {
					return "", err
				}
				status := event.Item.Status
				if status == "" {
					status = "completed"
				}
				output := map[string]any{"status": status}
				if event.Item.ExitCode != nil {
					output["exitCode"] = *event.Item.ExitCode
				}
				encoded, err := json.Marshal(output)
				if err != nil {
					return "", errors.New("encode Codex command result")
				}
				result := actionResult{
					CallID: event.Item.ID, Kind: "tool-result",
					Output: encoded, ToolName: "terminal",
				}
				if status != "completed" {
					result.IsError = true
				}
				if err := emit("action.result", map[string]any{
					"result":   result,
					"sequence": turn.Sequence, "status": status,
					"stepIndex": steps[event.Item.ID], "turnId": turn.TurnID,
				}); err != nil {
					return "", err
				}
				completedActions[event.Item.ID] = struct{}{}
			case "agent_message":
				if strings.TrimSpace(event.Item.Text) != "" {
					finalMessage = event.Item.Text
				}
			}
		case "turn.completed":
			turnCompleted = true
		case "turn.failed", "error":
			return "", errors.New("Codex CLI turn failed")
		}
	}
	if err := scanner.Err(); err != nil {
		return "", errors.New("Codex CLI event exceeded 1 MiB")
	}
	if !turnCompleted {
		return "", errCodexTurnIncomplete
	}
	if strings.TrimSpace(finalMessage) == "" {
		return "", errors.New("Codex CLI omitted a final answer")
	}
	if err := startStep(); err != nil {
		return "", err
	}
	if err := emit("message.appended", map[string]any{
		"messageDelta": finalMessage, "messageSoFar": finalMessage,
		"sequence": turn.Sequence, "stepIndex": stepIndex, "turnId": turn.TurnID,
	}); err != nil {
		return "", err
	}
	if err := emit("message.completed", map[string]any{
		"finishReason": "stop", "message": finalMessage,
		"sequence": turn.Sequence, "stepIndex": stepIndex, "turnId": turn.TurnID,
	}); err != nil {
		return "", err
	}
	if err := emit("step.completed", map[string]any{
		"finishReason": "stop", "sequence": turn.Sequence,
		"stepIndex": stepIndex, "turnId": turn.TurnID,
	}); err != nil {
		return "", err
	}
	return finalMessage, nil
}
