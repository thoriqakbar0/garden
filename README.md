# Garden

Garden is a self-hosted runtime for [Eve](https://github.com/vercel/eve)-shaped
agents. It is a dependency-free Go binary that discovers an agent from the
filesystem, runs real model and native-tool turns, exposes Eve protocol-v19
HTTP streams, and keeps durable local workflow state.

Garden targets the core conversational runner contract, not the complete Eve
authoring framework. It does not require Vercel runtime, Workflow SDK, hosted
storage, or a JavaScript process.

> [!WARNING]
> Garden is still work in progress. The session, streaming, model, native-tool,
> cancellation, and local recovery paths are implemented. Arbitrary authored
> TypeScript, sandboxes, channels, connections, hooks, subagents, and eval
> execution remain outside the supported runtime subset.

## Compatibility

Compatibility is pinned to the Eve revision in [`UPSTREAM.md`](UPSTREAM.md).
The percentage is intentionally not presented as full Eve parity: Garden
implements roughly 80% of the scoped self-hosted conversational-runtime
contract, not 80% of Eve's entire framework and tooling surface.

| Capability | Status | Garden behavior |
| --- | --- | --- |
| Eve project discovery | Available | Discovers instructions, model, tools, skills, channels, connections, subagents, schedules, and evals. |
| Model execution | Available | OpenAI Chat Completions-compatible endpoints and local Codex credentials. |
| Native tool loop | Available | Model -> tool -> model with bounded requests, deadlines, cancellation, and durable Eve events. |
| Eve HTTP sessions | Available | Create, continue, stream, and cancel routes with protocol-v19 envelopes and headers. |
| Live NDJSON | Available | Immediate prelude, incremental flush, absolute replay, and tail-relative replay. |
| Continuation tokens | Available | Opaque token ownership follows Eve's channel/session boundary. |
| Local durability | Available | Fsync-backed JSONL, safe identifiers, partial-tail repair, and deterministic restart settlement. |
| Concurrency | Available | One active turn per session; independent sessions run concurrently. |
| Self-hosted exposure | Available | Loopback by default; bearer authentication is required for non-loopback binding. |
| Schedule discovery | Available | Discovers static schedule metadata. |
| Schedule execution | Partial | Dispatch creates a durable session; authored schedule code is not evaluated. |
| TypeScript tools | Native binding only | A discovered tool must have a compiled Go implementation. |
| Skills and other authored modules | Discovery only | TypeScript and markdown modules are not dynamically executed. |
| Interrupted post-tool resume | Not implemented | Restart settles an interrupted turn without repeating it; it does not resume the final model step. |
| Distributed storage | Not implemented | One Garden writer owns one local workflow store. |

## Quickstart

Garden requires Go 1.25 or newer. The local single-writer runtime currently
supports macOS and Linux and fails closed on platforms without its process-lock
implementation.

```sh
git clone https://github.com/thoriqakbar0/garden.git
cd garden
CGO_ENABLED=0 go build -trimpath -o garden ./cmd/eve
```

Inspect the included Eve weather agent:

```sh
./garden info --root examples/eve-weather
```

Run it with the credential created by the Codex CLI:

```sh
codex login
export GARDEN_MODEL_BACKEND=codex
./garden run \
  --root examples/eve-weather \
  --message "What is the weather in Jakarta?"
```

The result identifies the durable session and turn:

```json
{"sessionId":"ses_<generated>","turnId":"turn_<generated>","message":"It is sunny in Jakarta."}
```

Continue the same local conversation with `--session`:

```sh
./garden run \
  --root examples/eve-weather \
  --session "ses_<generated>" \
  --message "And tomorrow?"
```

Normal `run` and `serve` commands require a configured model backend. They do
not silently return a diagnostic echo.

## Model configuration

| Variable | Meaning |
| --- | --- |
| `GARDEN_MODEL_BACKEND` | Required: `openai` or `codex`. |
| `GARDEN_MODEL` | Optional model override. Otherwise Garden uses the literal model discovered in `agent/agent.ts`. |
| `GARDEN_OPENAI_BASE_URL` | OpenAI-compatible API base. Defaults to `https://api.openai.com/v1` when an API key is set. |
| `GARDEN_OPENAI_API_KEY` | Optional bearer credential; local endpoints may omit it. |
| `CODEX_HOME` | Codex state directory, default `~/.codex`. |
| `GARDEN_CODEX_BASE_URL` | Optional compatible Responses API base. |
| `GARDEN_AUTH_TOKEN` | Required bearer token when `serve` binds beyond loopback. |

OpenAI-compatible example:

```sh
export GARDEN_MODEL_BACKEND=openai
export GARDEN_OPENAI_API_KEY=...
export GARDEN_MODEL=gpt-5.4-mini
./garden run --root examples/eve-weather --message "Weather in Jakarta?"
```

Requests, responses, tool inputs, tool outputs, and Codex auth input are bounded
to 1 MiB. Each model or tool step has a 60-second deadline and one turn may use
at most eight model rounds. Credentials and upstream response bodies are not
included in public errors.

The compiled manifest currently binds only `get_weather`. It returns fixture
data to prove the execution boundary. A declared tool without a native binding
causes startup to fail.

## HTTP runtime

Garden listens on loopback by default:

```sh
./garden serve --root examples/eve-weather
```

Health and redacted application information:

```sh
curl http://127.0.0.1:3000/health
curl http://127.0.0.1:3000/eve/v1/info
```

Create and start a session in one request:

```sh
curl -i \
  -X POST \
  -H 'content-type: application/json' \
  -d '{"message":"Weather in Jakarta?"}' \
  http://127.0.0.1:3000/eve/v1/session
```

The `202` response contains `sessionId` and `continuationToken`. Open its live
protocol-v19 stream:

```sh
curl -N http://127.0.0.1:3000/eve/v1/session/ses_<id>/stream
```

Continue through the token-owned route:

```sh
curl \
  -X POST \
  -H 'content-type: application/json' \
  -d '{"message":"And tomorrow?","continuationToken":"eve:<opaque>"}' \
  http://127.0.0.1:3000/eve/v1/session/ses_<id>
```

Cancel an active turn:

```sh
curl \
  -X POST \
  -H 'content-type: application/json' \
  -d '{"turnId":"turn_<id>"}' \
  http://127.0.0.1:3000/eve/v1/session/ses_<id>/cancel
```

Streams use `application/x-ndjson; charset=utf-8`, protocol version `19`, and
support `startIndex=N` for absolute or negative tail-relative replay.

### Network exposure

Non-loopback binding requires authentication:

```sh
export GARDEN_AUTH_TOKEN="$(openssl rand -hex 32)"
./garden serve \
  --root examples/eve-weather \
  --addr 0.0.0.0:3000
```

Clients then send `Authorization: Bearer $GARDEN_AUTH_TOKEN`. A configured
token is enforced on loopback too, which lets a local TLS reverse proxy retain
Garden's authentication boundary. Loopback requires no token only when the
variable is unset.

## Supported project shape

Only `agent/instructions.md` is required.

```text
my-agent/
├── agent/
│   ├── instructions.md
│   ├── agent.ts
│   ├── tools/
│   ├── skills/
│   ├── channels/
│   ├── connections/
│   ├── schedules/
│   └── subagents/
└── evals/
```

Garden statically reads paths and selected literal configuration. JavaScript
and TypeScript files are declarations, not executable code. Native tool
bindings are selected by the discovered tool identifier.

## Workflow integrity

Session events live under:

```text
.eve/workflow-data/sessions/<session-id>.jsonl
```

The local store provides durable append before publication, an in-memory mirror
of fsynced events for cursor-linear replay, contiguous event indexes, one active
turn per session, safe no-follow session paths, partial-tail repair, and
deterministic settlement of turns that were active when the process stopped.
Cancellation does not return `accepted` until its intent is durable. Settlement
then ends with exactly one `turn.cancelled` and the following
`session.waiting`; recovery finishes that boundary after a crash.

Native tool arguments and results remain durable internal events. The HTTP
stream exposes only event types proven by the pinned Eve-v19 contract, and its
absolute and negative cursors count that public projection.

The store is intentionally local and single-writer. Back up the project and
its `.eve` directory together. Pre-v19 Garden session logs are validated and
atomically migrated in place on first open; invalid or mixed-format logs stop
startup instead of being guessed through.

## CLI

| Command | Behavior |
| --- | --- |
| `garden init [directory]` | Creates the minimum Eve-shaped project without overwriting files. |
| `garden info [--root directory]` | Prints the full local discovery result. |
| `garden run --message text [--root directory] [--session id]` | Runs one model turn and waits for its durable boundary. |
| `garden serve [--root directory] [--addr address]` | Starts the self-hosted HTTP runtime. |
| `garden dev ...` | Alias of `serve`; no watcher or TUI. |
| `garden start ...` | Alias of `serve`. |
| `garden eval [--root directory] --list` | Lists discovered eval identifiers. |
| `garden version` | Prints the Garden version. |

## Development

```sh
make test
make check
```

The hermetic suite covers discovery, the OpenAI-compatible and Codex provider
boundaries, the native tool loop, protocol-v19 create/continue/stream/cancel,
live flushing, cursor resume, continuation ownership, cancellation races,
session traversal rejection, concurrent sessions, and restart repair.

The same black-box contract can target pinned official Eve fixtures with
`EVE_OFFICIAL_BASE_URL` and `EVE_OFFICIAL_CANCELLATION_BASE_URL`; see
[`official_test.go`](internal/contracttest/official_test.go). A skipped official
target is not counted as differential proof.

## Scope

Garden is an independent Apache-2.0 implementation and is not a Vercel product.
Official Eve defines the authored project shape and observable HTTP behavior;
Garden owns the local execution, persistence, and serving implementation.
