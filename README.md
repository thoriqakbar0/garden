# Garden

Garden is an independent Go compatibility runtime for
[Vercel eve](https://github.com/vercel/eve). It keeps the filesystem-first
authoring model while moving the durable runtime into one dependency-free Go
binary.

This canonical repository currently ports:

- `agent/` discovery for instructions, model, tools, skills, connections,
  channels, subagents, and schedules;
- `evals/` discovery and `eve eval --list`;
- append-only JSONL workflow sessions with ordered events;
- serialized turns within a session and concurrent independent sessions;
- replay from `startIndex`, active-turn cancellation, and stale-turn guards;
- schedule discovery and development dispatch;
- the `/eve/v1` HTTP surface;
- Vercel Go Function routing and cron generation.

The native tests carry over the important assertions from upstream's
`agent-workflow-stress`, `agent-schedules`, and `agent-cancellation` fixtures:
100 sequential turns, 50 concurrent sessions, durable tail replay, schedule
dispatch, and cancellation.

## Build and run

```sh
go build -o garden ./cmd/eve
./garden info
./garden serve --addr :3000
```

The local runtime is deterministic: a turn replies
`stress-ack:<turn-number>:<message>`. This makes workflow behavior testable
without model credentials. AI Gateway calls, authored TypeScript tool
execution, sandbox backends, production-durable Vercel storage, channel
adapters, approvals, and subagent execution are not ported yet.

## HTTP

```text
POST /eve/v1/session
POST /eve/v1/session/{sessionId}/turn
GET  /eve/v1/session/{sessionId}/stream?startIndex=0
POST /eve/v1/session/{sessionId}/cancel
POST /eve/v1/schedules/{scheduleId}/dispatch
GET  /eve/v1/info
```

`eve build` writes `vercel.json` for Vercel's native Go runtime and maps all
`/eve/v1/*` requests to [`api/eve.go`](api/eve.go). The function's `/tmp`
event log is suitable only for compatibility smoke tests because Vercel
function filesystems are ephemeral; production durability still needs an
external workflow store.

## Upstream baseline

The initial port was checked against `vercel/eve` main at
`05f348023d4268c974c225c1189a283ace20b742` on 2026-07-26. No upstream source
code is vendored; `NOTICE` records attribution under Apache-2.0.
