# Project: Agent execution

## Goal

This document specifies native mode. Exact authored Eve execution is a separate
contract in [Official Eve host](official-eve-host.md).

Run one real agent turn through an OpenAI-compatible model and an explicitly
supported native tool:

```text
user message
  → model request with instructions and tool schemas
  → assistant tool call
  → native Go tool execution
  → tool result in conversation
  → second model request
  → final assistant response
```

The implementation is complete only when removing either the tool execution or
the second model request makes the integration test fail.

## Model boundary

The initial model adapter uses the OpenAI Chat Completions-compatible HTTP
shape. Configuration is explicit:

| Variable | Meaning |
| --- | --- |
| `GARDEN_OPENAI_BASE_URL` | Compatible API base URL |
| `GARDEN_OPENAI_API_KEY` | Optional bearer credential |
| `GARDEN_MODEL` | Model override for endpoints that do not use the authored identifier |

If only the API key is set, Garden MAY default the base URL to the OpenAI API.
If no usable endpoint/model configuration exists, normal agent execution MUST
fail clearly; it MUST NOT silently return an echo response.

The adapter MUST:

- bound request and response bodies;
- reject invalid or ambiguous model messages;
- reject duplicate or malformed tool-call IDs;
- reject empty final responses;
- propagate cancellation and deadlines;
- omit credentials and upstream response bodies from errors and logs; and
- cap the number of model/tool rounds.

Initial safety limits are a 1 MiB JSON payload, 60 seconds per model or tool
step, and eight model rounds. Changing these values requires tests for timeout,
cancellation, and payload rejection.

## Native tool manifest

The binary owns an explicit manifest:

```go
type Tool interface {
    Definition() ToolDefinition
    Execute(context.Context, json.RawMessage) (json.RawMessage, error)
}
```

Each definition MUST have a unique safe name, non-empty description, and valid
JSON Schema whose root type is `object`. Tool input is untrusted and MUST be
strictly decoded. Tool output MUST be valid JSON and remain within the payload
limit.

Discovered files declare desired tool IDs; they do not provide executable code.
Startup MUST fail if a declared tool lacks a manifest entry. A model request
for an undeclared tool MUST fail the turn without invoking anything.

The first manifest contains a deterministic native `get_weather` tool solely
to prove the execution boundary. It is an example capability, not a claim of
live meteorological data.

## Codex execution boundary

With `GARDEN_MODEL_BACKEND=codex`, Garden delegates the complete turn to the
local Codex CLI instead of exposing Garden's native manifest to a raw model
endpoint. Codex MAY use terminal tools within the agent project. Garden MUST:

- consume `codex exec --json` events without parsing human output;
- run non-interactively with either `read-only` or `workspace-write` sandboxing;
- reject `danger-full-access`;
- prevent shell commands from inheriting the parent process environment;
- propagate cancellation and enforce a bounded turn deadline;
- translate command and final-message boundaries into Garden workflow events;
- omit command text and terminal output from durable Garden events; and
- require a completed Codex turn and non-empty final agent message.

Garden owns Eve session history. Codex executions are ephemeral, and each new
turn receives the completed conversation reconstructed from Garden's durable
events.

## Conversation and durability

The model request MUST include:

1. the discovered instructions as the system message;
2. prior completed user and assistant messages in session order;
3. the current user message; and
4. only the tool schemas supported for the loaded project.

When the model requests tools, Garden MUST preserve the assistant tool-call
message and each correlated tool result for the next model round. Tool-call
state needed for crash recovery MUST be durable before a side effect begins.

The public Eve event projection is governed by
[the HTTP contract](eve-http-contract.md). Internal execution events may be
richer, but the HTTP layer must not leak Garden-only protocol types.

## Required tests

Hermetic tests MUST use a fake OpenAI-compatible server and the real
`examples/eve-weather` project. They must prove:

- exactly two model requests for the one-tool scenario;
- the first request contains instructions and `get_weather`;
- the model-requested arguments reach the native tool;
- the second request contains the original assistant tool call and correlated
  JSON tool result;
- the final assistant response is persisted and returned;
- undeclared and unimplemented tools are rejected;
- malformed model and tool payloads are rejected;
- cancellation reaches a blocking model and a blocking tool;
- model and tool errors do not leak secrets; and
- the CLI and HTTP server select the same runner.

Hermetic Codex-exec tests MUST use a fake executable and prove sandbox flags,
conversation projection, terminal lifecycle translation, final answer capture,
and omission of command text and output from durable events. They MUST NOT
require live credentials.

A credentialed smoke test is useful but optional. Hermetic proof is required
and must never depend on a developer secret.
