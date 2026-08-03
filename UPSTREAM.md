# Upstream compatibility baseline

Source: `https://github.com/vercel/eve`

Commit: `05f348023d4268c974c225c1189a283ace20b742`

Official runtime package: `eve@0.27.6`

Behavioral fixtures ported into native Go tests:

- `e2e/fixtures/agent-workflow-stress/evals/sequential-workflow-turns.eval.ts`
- `e2e/fixtures/agent-workflow-stress/evals/concurrent-workflow-turns.eval.ts`
- `e2e/fixtures/agent-schedules/evals/stream-resume.eval.ts`
- `e2e/fixtures/agent-schedules/evals/schedule-dispatch.eval.ts`
- `e2e/fixtures/agent-cancellation/evals/cancel-turn.eval.ts`

The Go tests assert the observable behavior rather than copying TypeScript
implementation details.

The shared client lives in `internal/contracttest/client.go`. Garden runs it in
`internal/server/server_test.go`; the environment-gated official targets live
in `internal/contracttest/official_test.go`:

- `EVE_OFFICIAL_BASE_URL` targets the official conversation fixture.
- `EVE_OFFICIAL_CANCELLATION_BASE_URL` targets the official cancellation fixture.

A skipped official target is not differential evidence.
