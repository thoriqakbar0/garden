# Eve compatibility

Garden pins [Eve `0.27.6`](UPSTREAM.md) and provides two deliberately different
execution modes. This table separates **delegation** from **proof**: official
mode runs the pinned Eve process and therefore owns authored Eve semantics, but
Garden does not claim that every upstream feature has an independent Garden
acceptance test.

Status terms:

- **Delegated** — executed by the pinned official Eve runtime.
- **Supported** — implemented by Garden native mode and covered by Garden tests.
- **Partial** — a narrower behavior is implemented and documented.
- **Discovery only** — Garden reports the declaration but does not execute it.
- **Unsupported** — unavailable in native mode.
- **Outside command surface** — available from Eve's own CLI or packages, but
  not exposed by `garden serve --runtime eve`.

## Reverse-engineered baseline

The matrix was derived from the exact pinned source at
`vercel/eve@05f348023d4268c974c225c1189a283ace20b742`, not from the latest docs.
The audit covered:

- the compiler/discovery grammar in `packages/eve/src/discover/` and
  `packages/eve/src/compiler/`;
- execution, checkpointing, cancellation, and runtime actions in
  `packages/eve/src/execution/`, `packages/eve/src/harness/`, and
  `packages/eve/src/runtime/`;
- every public package subpath in `packages/eve/package.json` (channels,
  connections, tools, hooks, state/context, sandboxes, skills, schedules,
  clients, framework adapters, extensions, instrumentation, and evals);
- the canonical slot table in `docs/reference/project-layout.md` and public API
  table in `docs/reference/typescript-api.md`; and
- all 23 upstream E2E fixture domains and 92 `.eval.ts` cases under
  `e2e/fixtures/`.

Upstream source is the behavioral authority. Garden's checked-in matrix records
which surface is delegated, independently proven, partially reimplemented,
merely discovered, or absent.

## Runtime feature matrix

| Eve capability | Official mode | Garden proof | Native mode |
| --- | --- | --- | --- |
| Project discovery and TypeScript compilation | Delegated | Pinned CLI validation; parity fixture discovers authored `.ts` tools | Partial nested-layout discovery; no compilation or flat-layout grammar |
| Markdown and TypeScript instructions, composition, dynamic instructions | Delegated | Process-level delegation only | Markdown `agent/instructions.md` only |
| Authored model configuration and AI SDK models | Delegated | Credential-free authored `mockModel` fixture | Partial literal model discovery plus Codex or OpenAI-compatible override |
| Multi-step model/tool loop | Delegated | Parity fixture proves Bash → TypeScript tool → final model response | Supported, bounded to eight model rounds |
| Compaction, token limits, structured output | Delegated | Not independently covered by Garden | Unsupported |
| Static and dynamic authored tools | Delegated | Static TypeScript tool proven end to end | Native bindings only; currently `get_weather` |
| Eve built-in filesystem, Bash, web, todo, question, skill, connection, agent, and Workflow tools | Delegated | Authored Bash wrapper proven with `just-bash` | Unsupported; Codex terminal is a separate runtime |
| Tool approvals and human-in-the-loop input | Delegated | Not independently covered by Garden | Unsupported |
| Sandboxes: Docker, microsandbox, Vercel, just-bash, workspace lifecycle | Delegated | `just-bash` command proven in the parity fixture | No Eve sandbox; Codex offers project-scoped read-only/workspace-write policy |
| Skills and runtime skill loading | Delegated | Process-level delegation only | Discovery only |
| Hooks and authored durable/session state | Delegated | Process-level delegation only | Unsupported |
| MCP and OpenAPI connections, authorization, token brokering | Delegated | Process-level delegation only | Discovery only |
| Local, nested, remote, and dynamic subagents | Delegated | Process-level delegation only | Discovery only |
| TypeScript and Markdown schedules | Delegated | Process-level delegation only | Partial metadata discovery and session dispatch; authored handler is not run |
| Default Eve HTTP channel | Delegated | Shared contract targets exist; live parity fixture proves create/stream/wait | Supported text-message subset |
| Custom HTTP/WebSocket and Slack, Discord, Teams, Telegram, Twilio, GitHub, Linear, Chat SDK channels | Delegated when authored | Not independently covered by Garden | Discovery only |
| Create and continue session | Delegated | Shared black-box contract plus parity fixture | Supported |
| Live protocol-v19 NDJSON and incremental flush | Delegated | Shared black-box contract targets | Supported |
| Absolute and negative cursor replay | Delegated | Shared black-box contract targets | Supported |
| Continuation-token ownership | Delegated | Shared black-box contract targets | Supported |
| Turn cancellation | Delegated | Separate official cancellation target exists but is environment-gated | Supported and race-tested |
| Attachments, client context, input responses, per-turn output schema | Delegated | Not independently covered by Garden | Unsupported; native request accepts text only |
| Durable workflow checkpoints and crash recovery | Delegated to Eve Workflow runtime | Upstream ownership only | Partial local fsync JSONL, repair, settlement, and restart history |
| Resume final model step after a crash following a durable tool result | Delegated | Not independently covered by Garden | Unsupported; native mode settles the interrupted turn without repeating it |
| Concurrent sessions | Delegated | Shared contract/stress fixtures available | Supported; one active turn per session |
| Distributed storage and multiple writers | Delegated to the configured Eve deployment | Not independently covered by Garden | Unsupported; one local writer owns the store |
| Evals: cases, datasets, judges, reporters, targets | Outside `garden serve`; use project-local `eve eval` | Garden only checks discovery/listing | Discovery only |
| Extensions and contributed capability mounts | Delegated at Eve runtime; author/build/install stays in Eve CLI | Not independently covered by Garden | Unsupported |
| Instrumentation and OpenTelemetry | Delegated | Not independently covered by Garden | Unsupported |
| Eve TypeScript client and React/Vue/Svelte integrations | Can target the official server | Garden uses a Go contract client, not the framework clients | Basic text-route interoperability only; advanced client payloads unsupported |
| Next/Nuxt/SvelteKit integration, `eve build`, `eve start`, deploy, setup, and dev TUI | Outside Garden command surface | Garden intentionally invokes only `eve dev --no-ui` | Garden supplies its own init/info/run/serve CLI |
| Authored route/channel authentication | Delegated | Garden does not wrap official routes | Native bearer protection only |

## Protocol-v19 event surface

Official mode exposes the full Eve event union from the pinned runtime. Native
mode currently emits this tested public subset:

`session.started`, `turn.started`, `message.received`, `step.started`,
`message.appended`, `message.completed`, `step.completed`, `turn.completed`,
`turn.failed`, `turn.cancelled`, and `session.waiting`.

Native tool-call/action details are durable internally but intentionally omitted
from its public stream. Native mode does not currently emit Eve's HITL,
subagent, reasoning, structured-result, compaction, authorization,
`session.completed`, or `session.failed` event families. Applications requiring
those events should use official mode.

## What “official parity” means

`garden serve --runtime eve` validates and supervises the exact project-local
`eve@0.27.6` binary. Eve owns compilation, runtime behavior, persistence,
sandboxes, tools, channels, hooks, connections, schedules, and subagents. Garden
does not translate those features, because translation would create a second
semantics source.

The checked-in [`examples/eve-parity`](examples/eve-parity) acceptance fixture
proves a credential-free vertical slice: Garden launches official Eve, Eve
compiles authored TypeScript, a deterministic authored model requests a
sandboxed Bash command and an authored TypeScript tool, both results return to
the model, and the stream reaches `session.waiting` with the final answer.
Run it with `make test-official`.

Features marked “process-level delegation only” remain capabilities of the
pinned Eve process, not independently reimplemented or exhaustively retested by
Garden. See [Testing](TESTING.md) for the exact evidence boundary.
