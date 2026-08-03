# Project: Official Eve host

## Decision

Garden achieves 1:1 authored Eve behavior by supervising the pinned official
Eve runtime, not by translating TypeScript into Go. This is an explicit mode:

```sh
garden serve --runtime eve --root /path/to/agent
```

The version pair is recorded in [`UPSTREAM.md`](../../UPSTREAM.md). A different
installed Eve version is an error, not a compatibility fallback.

## Ownership boundary

Garden MUST:

- resolve the project root and require it to be a directory;
- read `node_modules/eve/package.json` with a bounded parser;
- require the exact pinned package name and version;
- resolve `node_modules/.bin/eve` and reject a target outside the project's
  `node_modules` tree;
- invoke `dev --no-ui --host <host> --port <port>` without a shell;
- attach the complete child process tree to one owned lifecycle;
- forward standard output and error for direct operator diagnosis;
- terminate the process group on cancellation, then force-kill after a bounded
  grace period; and
- return a safe error for startup or non-zero exit.

Official Eve MUST own:

- filesystem discovery and TypeScript compilation;
- model and tool execution;
- hooks, channels, connections, subagents, schedules, and eval semantics;
- workflow persistence and protocol events;
- route authentication and continuation ownership; and
- sandbox backend selection, terminal execution, workspace persistence, and
  sandbox network policy.

Garden MUST NOT wrap or reinterpret those surfaces in parity mode. Every extra
translation layer would create a second semantics source and invalidate the 1:1
claim.

## Trust and exposure

An Eve project is trusted application code. The child inherits the project
environment so authored model providers, tools, and connections can resolve
their configured credentials. Authored tools run in Eve's Node.js app runtime
and can access that environment. Model-driven terminal commands use Eve's
separate per-session sandbox and do not receive `process.env` under Eve's
sandbox contract.

Garden's native `GARDEN_AUTH_TOKEN` handler and native Codex backend do not apply
to official Eve mode. External exposure requires authorization implemented by
the Eve agent or a trusted reverse proxy. Loopback is the safe default.

## Acceptance

The mode is accepted only when current evidence proves all of the following:

1. a fake project-local CLI receives the exact arguments and is stopped on
   cancellation;
2. missing, mismatched, escaped, or non-executable installations fail closed;
3. a fresh project with the pinned npm package starts only through Garden;
4. `/eve/v1/info` reports an authored `.ts` tool from that project;
5. one real session emits a sandboxed `bash` action result;
6. the same turn emits an authored TypeScript tool result and feeds it into the
   next model step; and
7. the official event stream reaches `session.waiting` with the authored final
   message.

The live fixture MUST use a deterministic authored model so the proof requires
no developer credential. A skipped official target is not passing evidence.
