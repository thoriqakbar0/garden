# Product boundary

## Definition

Garden exposes two explicit execution contracts. Official Eve mode supervises
the pinned project-local official Eve CLI and therefore executes an unmodified
Eve project with Eve's own TypeScript compiler and runtime. Native mode is a Go
runtime for a documented subset of an Eve-shaped project. Native
OpenAI-compatible mode is self-contained; native Codex mode invokes a locally
installed Codex CLI authenticated with `codex login`.

The filesystem is the authoring input. In official Eve mode, Eve owns execution,
persistence, HTTP behavior, authorization, and sandboxing while Garden owns the
child-process lifecycle. In native mode, Garden owns execution, persistence,
and HTTP serving. A model endpoint may be remote, but Garden itself MUST remain
usable without a hosted Garden service.

## User-visible outcome

Given an Eve-shaped project with instructions, a model declaration, and tools
supported by the Garden binary, a user can:

1. inspect the project;
2. start or continue a durable session;
3. receive a live Eve-compatible event stream;
4. run a real model → tool → model turn;
5. cancel an active turn; and
6. restart Garden without losing the conversation or repeating a completed
   tool side effect.

## Official Eve mode

`serve --runtime eve` MUST execute the exact project-local Eve version recorded
in [`UPSTREAM.md`](../UPSTREAM.md). Garden MUST NOT fall back to a global CLI,
translate authored files, replace Eve's storage or routes, or claim parity after
starting a different package version. The official runtime owns every authored
capability it supports, including TypeScript, sandboxes, terminal tools, hooks,
channels, connections, subagents, schedules, and workflow semantics.

Garden MUST forward termination to the full Eve process group and MUST report a
non-zero child exit without leaking an invented successful state. The project is
trusted application code: the official process inherits the project environment
and authored tools execute with Eve's documented Node.js privileges. Sandboxed
terminal commands remain owned by Eve's sandbox contract.

## Native supported authoring subset

Native Garden MUST document and validate the subset it executes.

| Eve-shaped path | Garden meaning |
| --- | --- |
| `agent/instructions.md` | Required model instructions |
| `agent/agent.ts` | Static source for a literal authored model identifier |
| `agent/tools/*.{ts,js,mjs}` | Declared tool identifiers only |
| `agent/skills/**/*.md` | Discoverable prompt material; execution support must be stated separately |
| `agent/schedules/*.{ts,js,mjs}` | Discoverable schedule metadata only until schedule execution is specified |
| `agent/channels/**`, `agent/connections/**`, `agent/subagents/**`, `evals/**` | Discovery-only until a dedicated execution contract exists |

JavaScript and TypeScript files are not executable merely because Garden
discovers them. Startup MUST fail with a clear capability error when a required
tool has no supported executor. Discovery MUST NOT be presented as execution.

## Native execution boundary

The first supported tool mechanism is an explicit build-time manifest of native
Go tools. A manifest entry binds a discovered Eve tool identifier to:

- a description;
- a JSON object input schema;
- a cancellable Go implementation; and
- JSON output.

Garden MAY add HTTP, WASM, or subprocess executors later, but each requires a
separate trust and lifecycle contract. Garden MUST NOT imply arbitrary
TypeScript execution.

The Codex executor is an explicit subprocess boundary. It MUST use Codex JSONL
events, a bounded turn deadline, non-interactive approval policy, a scrubbed
shell environment, and either `read-only` or project-scoped `workspace-write`
sandboxing. Garden MUST reject unsandboxed Codex execution. Terminal command
text and output MUST NOT be copied into Garden's durable workflow log.

## Explicit non-goals

Native mode does not:

- reproduce every Eve capability;
- deploy to Vercel;
- emit `vercel.json`;
- depend on Vercel runtime or Workflow SDK;
- execute arbitrary JavaScript or TypeScript;
- provide distributed multi-writer workflow storage; or
- claim channels, connections, schedules, evals, skills, or subagents execute
  when they are only discovered.

Vercel-specific code, commands, configuration, and documentation MUST be
removed from the core runner. A future optional adapter must live behind a
separate package and binary boundary and must not change the runner contract.

## Honest modes

Normal `run` and `serve` modes MUST execute the configured model path or return
a configuration error. They MUST NOT silently fall back to the deterministic
echo responder.

An offline diagnostic responder MAY remain behind an explicit flag or test-only
entry point. Its output and documentation must identify it as a workflow
diagnostic, not an agent result.
