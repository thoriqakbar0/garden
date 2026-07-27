# Garden

Garden is an experimental, dependency-free Go runtime for
[Vercel eve](https://github.com/vercel/eve) agent projects. It reads Eve's
filesystem-first project shape and provides a small durable workflow runtime,
CLI, HTTP API, and Vercel Go Function adapter in one binary.

> [!WARNING]
> **Work in progress.** Garden can discover Eve projects and execute model-backed
> local workflows through a small supported native-tool manifest. It does not
> execute authored TypeScript or provide production-grade distributed storage.

The target product contract and worktree-integration gates live in
[`specs/`](specs/README.md). Those specifications describe the intended
Vercel-free Eve runner and do not imply that candidate task work is already
available on `main`.

Garden is useful today as a compatibility harness and initial agent runner: it
can inspect an Eve agent, run a model → native tool → model turn, stress durable
event storage, and expose the same runner through a command or HTTP server.

## What works

| Capability | Status | Current behavior |
| --- | --- | --- |
| Eve project discovery | Available | Reads instructions, model names, tools, skills, channels, connections, subagents, schedules, and eval names from disk. |
| Durable local sessions | Available | Persists ordered JSONL events under `.eve/workflow-data/`. |
| Sequential turns | Available | Serializes turns within one session and retains its turn count. |
| Concurrent sessions | Available | Runs independent sessions concurrently within one Garden process. |
| Event replay | Available | Returns a session event suffix from a non-negative `startIndex`. |
| Cancellation | Available | Cancels a matching active turn owned by the current process. |
| Schedule discovery | Available | Discovers literal cron expressions and emits Vercel cron configuration. |
| Schedule dispatch | Partial | Creates a durable session for a known schedule; it does not execute the authored schedule body. |
| Eval discovery | Available | Lists authored `.eval.ts` and `.eval.js` files. |
| Eval execution | Not implemented | Use the native Go compatibility tests for now. |
| TypeScript tools and skills | Discovery plus native binding | Source is not evaluated. A discovered tool runs only when its ID has a compiled native implementation; unsupported declarations fail startup. |
| Model providers | Available | Supports OpenAI Chat Completions-compatible endpoints and the local Codex auth cache/Responses endpoint. |
| Vercel deployment | Experimental | Emits routing and cron configuration for the included Go handler; storage remains ephemeral. |

## Quickstart

Garden requires Go 1.25 or newer.

```sh
git clone https://github.com/thoriqakbar0/garden.git
cd garden
go build -trimpath -o garden ./cmd/eve
```

Inspect the included Eve weather agent:

```sh
./garden info --root examples/eve-weather
```

The output identifies the authored model, `get_weather` tool, and
`get-weather` skill. Run a durable local turn:

```sh
export GARDEN_MODEL_BACKEND=openai
export GARDEN_OPENAI_BASE_URL=https://api.openai.com/v1
export GARDEN_OPENAI_API_KEY=...
export GARDEN_MODEL=gpt-5.4

./garden run \
  --root examples/eve-weather \
  --message "What is the weather in Jakarta?"
```

Garden makes a real model request, executes the native deterministic
`get_weather` implementation when requested, makes the correlated follow-up
model request, and returns a JSON result:

```json
{
  "sessionId": "session_<generated-id>",
  "turnId": "turn_<generated-id>",
  "message": "It is sunny in Jakarta."
}
```

Pass the returned session ID back with `--session` to continue the same
conversation. Garden includes prior completed user and assistant messages in
the next model request.

```sh
./garden run \
  --root examples/eve-weather \
  --session "session_<generated-id>" \
  --message "And tomorrow?"
```

Normal `run` and `serve` never fall back to an echo response. Missing or invalid
model configuration fails clearly.

## Model configuration

Select a backend explicitly with `GARDEN_MODEL_BACKEND`.

| Variable | Meaning |
| --- | --- |
| `GARDEN_MODEL_BACKEND` | Required: `openai` or `codex`. |
| `GARDEN_MODEL` | Optional model override. For `openai`, the authored model is used when this is unset. For `codex`, the default is `gpt-5.6-sol`; bare `gpt-*` and `openai/gpt-*` IDs are accepted. |
| `GARDEN_OPENAI_BASE_URL` | OpenAI Chat Completions-compatible API base. If only an API key is set, defaults to `https://api.openai.com/v1`. |
| `GARDEN_OPENAI_API_KEY` | Optional bearer credential for the compatible endpoint. |
| `CODEX_HOME` | Codex state directory, default `~/.codex`. Garden reads `auth.json` created by `codex login`. |
| `GARDEN_CODEX_BASE_URL` | Optional alternate Responses API base, primarily for compatible/self-hosted endpoints. |

For a local ChatGPT subscription session:

```sh
codex login
export GARDEN_MODEL_BACKEND=codex
./garden run --root examples/eve-weather --message "What is the weather in Jakarta?"
```

The Codex backend supports both API-key and ChatGPT-token forms written by the
Codex CLI and honors its `auth_mode` preference when that credential is usable.
Garden never logs credentials or upstream response bodies. This slice does not
refresh ChatGPT tokens; an expired or rejected token returns a precise error
instructing you to run `codex login` again.

Model requests, responses, tool inputs, tool outputs, and `auth.json` are
limited to 1 MiB. Each model or tool step has a 60-second deadline, and a turn
is limited to eight model rounds.

Garden's compiled native manifest currently contains only `get_weather`. It
returns deterministic fixture data to prove the execution boundary; it is not
live meteorological data. Any other discovered tool causes `run` and `serve`
startup to fail rather than pretending the authored TypeScript is executable.

## Run the HTTP server

Serve any discovered agent with the same binary:

```sh
./garden serve \
  --root examples/eve-weather \
  --addr 127.0.0.1:38181
```

Check the process and inspect the loaded application:

```sh
curl http://127.0.0.1:38181/health
curl http://127.0.0.1:38181/eve/v1/info
```

Create a session:

```sh
curl -X POST http://127.0.0.1:38181/eve/v1/session
```

Copy the returned `sessionId`, then send a turn:

```sh
curl \
  -X POST \
  -H 'content-type: application/json' \
  -d '{"message":"hello"}' \
  http://127.0.0.1:38181/eve/v1/session/session_<generated-id>/turn
```

Stop the server with `Ctrl+C`.

## Author an agent

Garden follows Eve's directory-as-agent convention. Only
`agent/instructions.md` is required.

```text
my-agent/
├── agent/
│   ├── instructions.md
│   ├── agent.ts
│   ├── channels/
│   ├── connections/
│   ├── schedules/
│   ├── skills/
│   ├── subagents/
│   └── tools/
└── evals/
```

Create the minimum project automatically:

```sh
mkdir my-agent
./garden init my-agent
./garden info --root my-agent
./garden run --root my-agent --message "hello"
```

Garden derives identifiers from relative file paths:

| Path | Meaning |
| --- | --- |
| `agent/instructions.md` | Required agent instructions. |
| `agent/agent.ts` | Optional agent definition; Garden discovers a literal `provider/model` string. |
| `agent/tools/*.{ts,js,mjs}` | Tool identifiers. |
| `agent/skills/**/*.md` | Skill identifiers; a nested `SKILL.md` resolves to its directory name. |
| `agent/channels/*.{ts,js,mjs}` | Channel identifiers. |
| `agent/connections/*.{ts,js,mjs}` | Connection identifiers. |
| `agent/subagents/<name>/` | Immediate subagent identifiers. |
| `agent/schedules/*.{ts,js}` | Schedule identifiers and literal `cron` values. |
| `evals/**/*.eval.{ts,js}` | Eval identifiers. |

Discovery is deliberately static. Garden reads paths and selected literal
configuration; it does not import modules, resolve environment-dependent
definitions, or execute authored code.

## CLI reference

| Command | Behavior |
| --- | --- |
| `garden init [directory]` | Creates `agent/instructions.md` and `agent/agent.ts`; refuses to overwrite either file. |
| `garden info [--root directory]` | Prints the discovered application as formatted JSON. |
| `garden run --message text [--root directory] [--session id]` | Creates or resumes a durable session and runs one configured model turn. |
| `garden serve [--root directory] [--addr :3000]` | Starts the HTTP runtime. |
| `garden dev ...` | Alias of `serve`; no file watcher or TUI is implemented. |
| `garden start ...` | Alias of `serve`. |
| `garden build [--root directory]` | Discovers schedules and writes `vercel.json`; it does not compile or deploy the binary. |
| `garden eval [--root directory] --list` | Prints discovered eval IDs. |
| `garden version` | Prints the Garden version. |

Unless `--root` is supplied, commands use the current directory.

## HTTP API

All request and response bodies use JSON.

| Method | Path | Behavior |
| --- | --- | --- |
| `GET` | `/health` | Returns `{"status":"ok"}`. |
| `GET` | `/eve/v1/info` | Returns the discovered application. |
| `POST` | `/eve/v1/session` | Creates a session and persists `session.started`. |
| `POST` | `/eve/v1/session/{sessionId}/turn` | Accepts `{"message":"..."}` and runs one turn. |
| `GET` | `/eve/v1/session/{sessionId}/stream?startIndex=N` | Replays persisted events beginning at index `N`. |
| `POST` | `/eve/v1/session/{sessionId}/cancel` | Accepts optional `{"turnId":"..."}` and cancels a matching active turn. |
| `GET`, `POST` | `/eve/v1/schedules/{scheduleId}/dispatch` | Creates a session for a discovered schedule. |

The server rejects unknown JSON fields. Request decoding is bounded to avoid
unlimited body reads.

## Workflow storage and concurrency

Garden stores each session as an append-only file:

```text
.eve/workflow-data/sessions/<session-id>.jsonl
```

A successful turn appends these public completion events in order:

```text
turn.started
message.completed
turn.completed
```

Model turns may durably interleave internal `assistant.tool_calls` and
`tool.completed` records before the final completion events.

New sessions begin with `session.started`. Failed and cancelled turns append
`turn.failed` or `turn.cancelled` instead of a completion event.

The local store provides:

- an `fsync` before each event append returns;
- one in-process mutex per session, so turns cannot overlap within a session;
- independent concurrency across sessions;
- replay from any persisted event index;
- process-owned cancellation with an optional stale-turn guard.

The store does not provide cross-process locking, distributed coordination,
crash recovery for an interrupted turn, log compaction, or retention policy.
Run only one Garden writer against a local store.

## Vercel adapter

Garden includes [`api/eve.go`](api/eve.go), an HTTP handler for Vercel's native
Go Function runtime. Running:

```sh
./garden build --root .
```

writes `vercel.json` with:

- a rewrite from `/eve/v1/:path*` to `/api/eve`;
- one Vercel cron entry for each discovered schedule with a literal cron value.

This adapter is a deployment experiment, not a durable production runtime.
The handler writes workflow data under the function's temporary directory, so
events can disappear between invocations and are not shared across instances.
A production deployment needs an external workflow store before relying on
session continuity.

## Example

[`examples/eve-weather`](examples/eve-weather/README.md) is adapted from the
official Vercel Eve weather fixture. It includes:

- an authored model definition;
- concise weather-agent instructions;
- a markdown weather skill;
- a typed `get_weather` declaration bound to Garden's native implementation;
- a Go compatibility test proving Garden discovers the expected shape.

Garden does not evaluate the example's TypeScript. The discovered
`get_weather` ID selects the corresponding native Go tool.

## Development

Run the focused project checks:

```sh
make test
make check
```

`make test` runs `go test ./...`. `make check` runs `go vet ./...` followed by
the full test suite with the race detector.

The native tests cover:

- filesystem discovery and path-derived identifiers;
- 100 sequential turns in one durable session;
- 50 concurrent isolated sessions;
- replay from a non-zero event index;
- active and stale-turn cancellation;
- hermetic OpenAI-compatible model → weather tool → model execution;
- malformed/undeclared tool rejection and secret-safe failures;
- local Codex API-key and ChatGPT-token transport behavior;
- HTTP session, turn, replay, and schedule dispatch;
- Vercel route and cron generation;
- discovery of the included official-shaped example.

## Project layout

```text
api/eve.go                 Vercel Go Function adapter
cmd/eve/main.go            CLI composition root
examples/eve-weather/      Runnable Eve-shaped compatibility fixture
internal/discover/         Filesystem discovery
internal/agent/            Model adapters, native tools, and execution loop
internal/server/           HTTP adapter
internal/vercel/           vercel.json generation
internal/workflow/         Durable local event store
UPSTREAM.md                Upstream baseline and ported behavior
```

## Scope and compatibility

Garden is an independent implementation, not a Vercel product and not a
drop-in replacement for the official Eve runtime. Compatibility currently
means that Garden understands a useful subset of the Eve filesystem shape and
ports selected observable workflow behaviors into Go.

The baseline and source fixtures are recorded in
[`UPSTREAM.md`](UPSTREAM.md). Garden is licensed under Apache-2.0; adapted Eve
example material is attributed in [`NOTICE`](NOTICE).
