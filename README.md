# Garden

Garden runs [Eve](https://github.com/vercel/eve) agents locally in two explicit
modes. `--runtime eve` supervises the pinned project-local official Eve runtime,
so an unmodified Eve agent keeps its authored TypeScript, tools, hooks, channels,
connections, subagents, schedules, workflow semantics, and sandboxed terminal.
The default native mode is a standalone Go runtime for a smaller Eve-shaped
conversation contract, with either the local Codex CLI or an OpenAI-compatible
model endpoint.

Neither mode requires a hosted Garden service. Official Eve mode requires Node
24 and the pinned `eve` npm dependency in the agent project. Native mode remains
a single Go process with local workflow storage and no JavaScript runtime.

> [!WARNING]
> Garden is still work in progress. Full authored Eve behavior is available
> only through explicit official Eve mode. Native mode implements sessions,
> streaming, model and native-tool turns, the Codex terminal, cancellation, and
> local recovery; it does not execute arbitrary authored TypeScript.

## Compatibility

See the complete [Eve feature matrix](COMPATIBILITY.md) and [test inventory](TESTING.md).
Compatibility is pinned to the Eve revision in [`UPSTREAM.md`](UPSTREAM.md).
Official Eve mode is 1:1 by process ownership: the pinned official runtime
compiles and executes the project itself. Native mode has the narrower contract
described below and does not claim complete Eve parity.

| Capability | Status | Garden behavior |
| --- | --- | --- |
| Official Eve runtime | Available | `serve --runtime eve` runs project-local `eve@0.27.6`; official Eve owns authored semantics and wire behavior. |
| Authored TypeScript | Official Eve mode | Tools, hooks, channels, connections, subagents, schedules, sandboxes, and other supported Eve modules execute unchanged. |
| Sandboxed terminal | Official Eve mode | Eve's built-in terminal tools use the agent's authored or default Eve sandbox backend and session-scoped `/workspace`. |
| Eve project discovery | Available | Discovers instructions, model, tools, skills, channels, connections, subagents, schedules, and evals. |
| Model execution | Available | Sandboxed Codex CLI execution and OpenAI Chat Completions-compatible endpoints. |
| Native tool loop | Available | Model -> tool -> model with bounded requests, deadlines, cancellation, and durable Eve events. |
| Codex terminal | Available | Codex may run terminal commands inside the project sandbox; command text and output are not copied into Garden's durable log. |
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

For native mode you need:

- Go 1.25 or newer;
- macOS or Linux; and
- either a Codex login or an OpenAI-compatible model endpoint.

Official Eve mode instead needs Node 24 or newer and project-local
`eve@0.27.6`.

The local runtime allows one Garden writer per workflow store and fails closed
on platforms without its process-lock implementation.

```sh
git clone https://github.com/thoriqakbar0/garden.git
cd garden
make build
```

This command creates `./garden`. Install `garden` in your user `PATH` when you
want to run it from any directory:

```sh
make install
command -v garden
garden version
```

If `command -v` returns no path, add the default directory to your shell
`PATH`:

```sh
export PATH="$HOME/.local/bin:$PATH"
```

`make install` writes `$HOME/.local/bin/garden`. Set `BINDIR` to select another
directory:

```sh
make install BINDIR="$HOME/bin"
```

Inspect the included Eve weather agent:

```sh
./garden info --root examples/eve-weather
```

Run it with credentials created by the Codex CLI:

```sh
codex login
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

When `GARDEN_MODEL_BACKEND` is empty, Garden selects Codex if `codex` is on
`PATH`. Normal `run` and `serve` commands fail if Garden cannot detect or use a
model backend. They do not silently return a diagnostic echo.

Next, choose the relevant path:

- [run an unmodified Eve agent](#official-eve-mode);
- [configure another model backend](#model-configuration);
- [serve the Eve-compatible HTTP API](#http-runtime); or
- [adapt an Eve-shaped project](#supported-project-shape).

## Official Eve mode

Use this mode when the goal is to run an Eve agent 1:1 rather than port its
behavior into Garden's native Go subset. In the Eve project, pin and install the
baseline package:

```json
{
  "dependencies": {
    "eve": "0.27.6"
  }
}
```

Then start it through Garden:

```sh
npm install
/path/to/garden serve \
  --runtime eve \
  --root /path/to/eve-agent \
  --addr 127.0.0.1:3000
```

Garden validates the exact package version and project-local executable, then
runs `eve dev --no-ui` and owns its signal and shutdown lifecycle. Garden does
not translate the agent or intercept its protocol: the official runtime owns
compilation, model calls, durable sessions, routes, authorization, and sandbox
selection. The process receives the Eve project's environment because authored
tools and connections may require it.

To prove this path locally without provider credentials or Docker, run
`make test-official`; it uses the checked-in
[`examples/eve-parity`](examples/eve-parity) project and the pinned lockfile.

Eve's built-in `bash`, `read_file`, `write_file`, `glob`, and `grep` tools run
inside Eve's per-session sandbox, not in the Garden process. Choose and secure
the sandbox backend in the Eve project; the framework default may select Docker,
microsandbox, or the pure-JavaScript `just-bash` fallback depending on the host.

`GARDEN_MODEL_BACKEND=codex` belongs to native mode and is not injected into the
official Eve runtime. Likewise, Garden's native bearer wrapper does not wrap
official Eve routes. Keep the official host on loopback unless the Eve project
or a trusted reverse proxy provides the intended external authorization.

## Model configuration

| Variable | Meaning |
| --- | --- |
| `GARDEN_MODEL_BACKEND` | Optional explicit selection: `openai` or `codex`. When empty, Garden selects Codex if `codex` is on `PATH`. |
| `GARDEN_MODEL` | Optional model override. When unset, `openai` uses the literal model discovered in `agent/agent.ts`; `codex` defaults to `gpt-5.6-sol`. |
| `GARDEN_CODEX_SANDBOX` | Optional Codex terminal policy: `workspace-write` (default) or `read-only`. Garden rejects `danger-full-access`. |
| `GARDEN_OPENAI_BASE_URL` | OpenAI-compatible API base. Defaults to `https://api.openai.com/v1` when an API key is set. |
| `GARDEN_OPENAI_API_KEY` | Optional upstream bearer credential; local endpoints and Cloudflare Gateway BYOK may omit it. |
| `GARDEN_CLOUDFLARE_GATEWAY_TOKEN` | Optional Cloudflare AI Gateway credential, sent only as `cf-aig-authorization`. |
| `CODEX_HOME` | Codex state directory, default `~/.codex`. |
| `GARDEN_AUTH_TOKEN` | Required bearer token when `serve` binds beyond loopback. |

The `codex` backend delegates each Eve turn to `codex exec --json`. Garden uses
this backend automatically when no backend is set and the Codex CLI is on
`PATH`. Codex can inspect the project and use its terminal tools. Garden sets a
ten-minute turn deadline, disables interactive approvals, strips the parent
shell environment down to Codex's core command environment, blocks login-shell
rehydration, and confines writes to the project root. A command that needs
network or access outside the sandbox fails instead of waiting for approval.

Use read-only terminal access when the agent should inspect but never edit:

```sh
export GARDEN_CODEX_SANDBOX=read-only
```

OpenRouter's free-model router works through the same adapter:

```sh
export GARDEN_MODEL_BACKEND=openai
export GARDEN_OPENAI_BASE_URL=https://openrouter.ai/api/v1
export GARDEN_OPENAI_API_KEY="$OPENROUTER_API_KEY"
export GARDEN_MODEL=openrouter/free
./garden run --root examples/eve-weather --message "Weather in Jakarta?"
```

Cloudflare Workers AI also exposes an OpenAI-compatible endpoint. Accounts with
free Workers AI allocation can run the example without a separate model-provider
account:

```sh
export CLOUDFLARE_ACCOUNT_ID=your-account-id
export GARDEN_MODEL_BACKEND=openai
export GARDEN_OPENAI_BASE_URL="https://api.cloudflare.com/client/v4/accounts/$CLOUDFLARE_ACCOUNT_ID/ai/v1"
export GARDEN_OPENAI_API_KEY="$CLOUDFLARE_API_TOKEN"
export GARDEN_MODEL=@cf/ibm-granite/granite-4.0-h-micro
./garden run --root examples/eve-weather --message "Weather in Jakarta?"
```

For Cloudflare AI Gateway's OpenAI-compatible `/compat` endpoint, set that URL
as `GARDEN_OPENAI_BASE_URL`. If the gateway requires authentication, set
`GARDEN_CLOUDFLARE_GATEWAY_TOKEN`; Garden sends it in the dedicated
`cf-aig-authorization` header while keeping `GARDEN_OPENAI_API_KEY` available
for provider authentication.

The adapter accepts both standard JSON-object tool arguments and the
string-wrapped arguments returned by some compatible providers. Assistant tool
messages include explicit empty content for providers that require it.

OpenAI-compatible requests, responses, and native tool payloads are bounded to
1 MiB. Each OpenAI model or tool step has a 60-second deadline and one turn may
use at most eight model rounds. Codex prompts and individual JSON event records
are also bounded to 1 MiB. Credentials and upstream response bodies are not
included in public errors.

For the `openai` backend, the compiled manifest currently binds only
`get_weather`. It returns fixture data to prove the execution boundary. A
declared tool without a native binding causes startup to fail. The `codex`
backend instead uses Codex's sandboxed terminal runtime.

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
and TypeScript files are declarations, not automatically executable modules.
The OpenAI backend selects native tool bindings by discovered identifier; the
Codex backend can inspect and operate on the project through its terminal.

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

OpenAI native-tool arguments and results remain durable internal events. Codex
terminal commands record lifecycle and exit status but deliberately omit the
command text and output from Garden's durable log. The HTTP stream exposes only
event types proven by the pinned Eve-v19 contract, and its absolute and negative
cursors count that public projection.

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
make test-hermetic
make check
make test-official
make test-all
make list-tests
```

`make test-official` installs the pinned, credential-free
[`examples/eve-parity`](examples/eve-parity) fixture and proves authored
TypeScript plus sandbox execution through the official runtime. See
[`TESTING.md`](TESTING.md) for every test, validation tier, live-provider smoke
command, and remaining evidence gap.

The hermetic suite covers discovery, the OpenAI-compatible provider, sandboxed
Codex execution, the native tool loop, protocol-v19 create/continue/stream/cancel,
live flushing, cursor resume, continuation ownership, cancellation races,
session traversal rejection, concurrent sessions, and restart repair.

The same black-box contract can target pinned official Eve fixtures with
`EVE_OFFICIAL_BASE_URL` and `EVE_OFFICIAL_CANCELLATION_BASE_URL`; see
[`official_test.go`](internal/contracttest/official_test.go). A skipped official
target is not counted as differential proof.

## Scope

Garden is an independent Apache-2.0 implementation. It is not affiliated with
or endorsed by Vercel or the Eve project. It was built out of love for Eve’s
open-source SDKs.

Official Eve defines the authored project shape and observable HTTP behavior;
Garden owns the local execution, persistence, and serving implementation. See
[`THIRD_PARTY_NOTICES.md`](THIRD_PARTY_NOTICES.md) for Eve attribution and
license details.
