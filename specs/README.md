# Garden specifications

These documents define Garden as a local, self-hosted runtime for Eve-shaped
agent projects. They are the contract used to keep the Go implementation and
its compatibility tests honest.

Garden is not an Eve replacement and does not depend on Vercel runtime,
deployment, Workflow SDK, storage, or hosted infrastructure. Official Eve is
the source for authored-project shape and observable wire behavior.

## Status

The specifications contain both implemented requirements and the remaining
hardening contract.

| Area | Canonical implementation | Remaining gap |
| --- | --- | --- |
| Project discovery | Static Eve-shaped discovery with an explicit native execution subset | Broader authored-module execution |
| Agent execution | OpenAI-compatible and Codex model loop with native Go tools | Arbitrary TypeScript and additional tool executors |
| HTTP sessions | Eve protocol-v19 create, continue, live NDJSON, resume, and cancellation | Live differential proof against every supported upstream fixture |
| Durability | Fsync JSONL, safe IDs, tail repair, restart settlement, and single-writer ownership | Resume after a durable tool result instead of settling the interrupted turn |
| Deployment | Single self-hosted Go binary with authenticated HTTP exposure | Distributed multi-writer storage and horizontal coordination |

The shared Garden-side contract is required on every change. An
environment-gated official Eve test that was skipped remains unverified and
must not be reported as passing differential proof.

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
