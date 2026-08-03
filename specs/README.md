# Garden specifications

These documents define Garden as a local, self-hosted runtime for Eve-shaped
agent projects. They are the contract used to keep the Go implementation and
its compatibility tests honest.

Garden has two explicit runtime contracts. Official Eve mode supervises the
pinned project-local Eve runtime for complete authored semantics. Native mode
is Garden's independent Go implementation of a smaller self-hosted conversation
contract. Official Eve is the source for authored-project shape and observable
wire behavior in both cases.

## Status

The specifications contain both implemented requirements and the remaining
hardening contract.

| Area | Canonical implementation | Remaining gap |
| --- | --- | --- |
| Official Eve execution | Project-local pinned `eve@0.27.6` process host | Production-mode command and deployment lifecycle are not wrapped by Garden |
| Project discovery | Static Eve-shaped discovery with an explicit native execution subset | Broader authored-module execution |
| Agent execution | OpenAI-compatible native-tool loop and sandboxed Codex CLI turns | Arbitrary TypeScript and additional tool executors |
| HTTP sessions | Eve protocol-v19 create, continue, live NDJSON, resume, and cancellation | Live differential proof against every supported upstream fixture |
| Durability | Fsync JSONL, safe IDs, tail repair, restart settlement, and single-writer ownership | Resume after a durable tool result instead of settling the interrupted turn |
| Deployment | Self-hosted Go runtime with authenticated HTTP exposure; Codex mode invokes the local CLI | Distributed multi-writer storage and horizontal coordination |

The shared Garden-side contract is required on every change. An
environment-gated official Eve test that was skipped remains unverified and
must not be reported as passing differential proof.

## Specifications

- [Product boundary](product-boundary.md) — what Garden is, what it consumes,
  and what it deliberately does not provide.
- [Eve HTTP contract](projects/eve-http-contract.md) — session routes, NDJSON
  framing, cursors, continuation, cancellation, and public event ordering.
- [Agent execution](projects/agent-execution.md) — the OpenAI-compatible native
  tool loop and sandboxed Codex CLI boundary.
- [Official Eve host](projects/official-eve-host.md) — the exact authored
  TypeScript path and its process, environment, sandbox, and network boundaries.
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
