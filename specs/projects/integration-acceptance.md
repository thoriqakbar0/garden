# Project: Integration acceptance

## Purpose

Combine the HTTP-contract and agent-execution worktrees without choosing one
large overlapping implementation wholesale. Integration should preserve the
Eve public protocol, plug the real runner behind it, and enforce the runtime
integrity invariants.

## Merge order

1. **Protocol types and black-box client.** Establish one public Eve protocol
   package and run the Garden-side contract with the deterministic responder.
2. **Safe workflow store.** Add ID validation, live subscriptions, cursor
   replay, cancellation ownership, tail repair, and restart settlement.
3. **Agent execution.** Plug the OpenAI-compatible runner and native manifest
   into the responder seam.
4. **Event projection.** Translate internal model/tool lifecycle into the
   exact pinned Eve wire events. Do not expose candidate-only event names.
5. **Product cleanup.** Remove Vercel adapter/build/deployment surfaces and make
   offline echo an explicit diagnostic mode.
6. **Differential proof.** Run the same client against pinned Eve and Garden,
   then run restart and security cases specific to Garden's local storage.

Overlapping rewrites of `internal/workflow/store.go`,
`internal/server/server.go`, and their tests MUST be reconciled by invariant,
not by selecting the largest diff.

## Black-box acceptance scenario

Garden may be called a real Eve runner only when all steps pass:

1. Start Garden against `examples/eve-weather` and a deterministic fake
   OpenAI-compatible server.
2. Create a session through `POST /eve/v1/session`.
3. The fake model requests `get_weather({"city":"Jakarta"})`.
4. Garden invokes the native tool exactly once and persists enough state to
   recover it.
5. Garden sends the correlated tool result in the second model request.
6. The model returns a final answer and Garden persists it.
7. A client receives the exact Eve NDJSON sequence incrementally, including a
   terminal waiting boundary and usable continuation token.
8. Disconnect and resume from both an absolute and negative cursor without
   missing or duplicating events.
9. Continue the session after restarting Garden and prove prior conversation
   is present in the next model request.
10. Block a model or tool, cancel the exact active turn, and observe one
    `turn.cancelled` followed by `session.waiting`, with no later completion.
11. Kill Garden after the tool result is durable but before the final model
    response; restart and finish without repeating the tool.
12. Reject path traversal, encoded separators, stale turn IDs, concurrent
    turns, malformed bodies, unknown fields, oversized payloads, and corrupt
    logs with stable client errors.

## Differential evidence

The shared test client MUST run unchanged against:

- Garden's local server; and
- official Eve fixtures pinned by [`UPSTREAM.md`](../../UPSTREAM.md).

Normalization is limited to documented nondeterministic values such as opaque
IDs and timestamps. Status codes, headers, event types, event order, payload
keys, cursor behavior, continuation semantics, and cancellation semantics are
not normalized.

An environment-gated official test that was skipped does not count as passing.

## Required checks

Run from repository root:

```sh
gofmt -w <changed-go-files>
go vet ./...
go test -count=1 ./...
go test -race -count=1 ./...
git diff --check
```

Also run:

- the differential suite against pinned official Eve;
- a built-binary CLI model/tool smoke test using the fake endpoint;
- a built-binary HTTP stream/resume/cancel smoke test;
- restart-boundary tests; and
- the session-ID traversal regression suite.

## Completion statement

Completion requires all of the following to be true on canonical `main`:

- no required implementation remains only in a detached worktree;
- normal execution performs the real model/tool loop;
- the shared Eve client passes against both targets;
- restart and security acceptance cases pass;
- Vercel is absent from the core runner surface;
- README claims match the tests; and
- the work remains labeled WIP until those checks pass.

