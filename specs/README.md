# Garden specifications

These documents define Garden as a local, self-hosted runner for Eve-shaped
agent projects. They are the integration contract for the implementation work
currently happening in separate Codex worktrees.

Garden is not an Eve replacement and does not depend on Vercel runtime,
deployment, Workflow SDK, storage, or hosted infrastructure. Official Eve is
the source for authored-project shape and observable wire behavior.

## Status

The specifications describe the target, not the current `main` branch.

| Area | Canonical `main` | Candidate task work | Required before integration |
| --- | --- | --- | --- |
| Project discovery | Static Eve-shaped discovery | Reused by all tasks | Define the supported authoring subset |
| Agent execution | Deterministic echo | OpenAI-compatible model loop and native Go tool manifest | Integrate with the protocol event path |
| HTTP sessions | Garden-specific JSON endpoints | Eve routes, protocol-v19 events, flushed NDJSON, resume, cancellation | Pass the same black-box client against Garden and pinned Eve |
| Durability | JSONL replay and process-local cancellation | Tail repair and stronger lifecycle handling are in progress | Prove restart recovery and reject unsafe session IDs |
| Vercel | Adapter, build command, and deployment docs remain | Agent-loop task removes them | Remove or isolate them outside the runner product |

No candidate work is considered available until it is integrated into
canonical `main` and the acceptance suite passes there.

## Specifications

- [Product boundary](product-boundary.md) — what Garden is, what it consumes,
  and what it deliberately does not provide.
- [Eve HTTP contract](projects/eve-http-contract.md) — session routes, NDJSON
  framing, cursors, continuation, cancellation, and public event ordering.
- [Agent execution](projects/agent-execution.md) — the OpenAI-compatible
  model/tool loop and the native tool boundary.
- [Runtime integrity](projects/runtime-integrity.md) — storage, restart,
  ownership, security, and exposure requirements.
- [Integration acceptance](projects/integration-acceptance.md) — the combined
  black-box proof required before calling Garden an Eve runner.

## Normative language

`MUST`, `MUST NOT`, `SHOULD`, and `MAY` are requirements in descending order.
An unresolved item is marked `OPEN` and cannot be used as the basis for a
compatibility claim.

## Upstream baseline

Compatibility work is pinned to the Eve revision recorded in
[`UPSTREAM.md`](../UPSTREAM.md). A baseline change requires:

1. updating the pinned commit;
2. rerunning the shared differential client;
3. recording intentional protocol changes in these specifications; and
4. keeping old behavior only when Garden declares a versioned compatibility
   promise.

