# Garden

Garden is an experimental, dependency-free Go runtime for
[Vercel eve](https://github.com/vercel/eve) agent projects. It reads Eve's
filesystem-first project shape and provides a small durable workflow runtime,
CLI, HTTP API, and Vercel Go Function adapter in one binary.

> [!WARNING]
> **Work in progress.** Garden can discover Eve projects and run deterministic
> local workflows. It does not execute authored TypeScript, call language
> models, or provide production-grade distributed storage yet.

The target product contract and worktree-integration gates live in
[`specs/`](specs/README.md). Those specifications describe the intended
Vercel-free Eve runner and do not imply that candidate task work is already
available on `main`.

Garden is useful today as a compatibility harness: it can inspect an Eve agent,
exercise session lifecycle behavior without credentials, stress durable event
storage, and expose the same local runtime through a command or HTTP server.

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
| TypeScript tools and skills | Discovery only | Names and files are discovered, but JavaScript/TypeScript is not evaluated. |
| Model providers and AI Gateway | Not implemented | Turns use a deterministic local responder. |
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
./garden run \
  --root examples/eve-weather \
  --message "What is the weather in Jakarta?"
```

Garden returns a JSON result:

```json
{
  "sessionId": "session_<generated-id>",
  "turnId": "turn_<generated-id>",
  "message": "stress-ack:1:What is the weather in Jakarta?"
}
```

Pass the returned session ID back with `--session` to continue the same
conversation. The next response uses `stress-ack:2:...`, proving that the
workflow history survived the process boundary.

```sh
./garden run \
  --root examples/eve-weather \
  --session "session_<generated-id>" \
  --message "And tomorrow?"
```

The deterministic `stress-ack` response is intentional. It tests workflow
behavior without hiding a model call or pretending that the TypeScript weather
tool ran.

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
| `garden run --message text [--root directory] [--session id]` | Creates or resumes a durable session and runs one deterministic turn. |
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

A successful turn appends these event types in order:

```text
turn.started
message.completed
turn.completed
```

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
- a typed `get_weather` tool;
- a Go compatibility test proving Garden discovers the expected shape.

The example is included to test compatibility. Garden does not currently run
its TypeScript tool.

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
- HTTP session, turn, replay, and schedule dispatch;
- Vercel route and cron generation;
- discovery of the included official-shaped example.

## Project layout

```text
api/eve.go                 Vercel Go Function adapter
cmd/eve/main.go            CLI composition root
examples/eve-weather/      Runnable Eve-shaped compatibility fixture
internal/discover/         Filesystem discovery
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
