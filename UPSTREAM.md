# Upstream compatibility baseline

Source: `https://github.com/vercel/eve`

Commit: `05f348023d4268c974c225c1189a283ace20b742`

Behavioral fixtures ported into native Go tests:

- `e2e/fixtures/agent-workflow-stress/evals/sequential-workflow-turns.eval.ts`
- `e2e/fixtures/agent-workflow-stress/evals/concurrent-workflow-turns.eval.ts`
- `e2e/fixtures/agent-schedules/evals/stream-resume.eval.ts`
- `e2e/fixtures/agent-schedules/evals/schedule-dispatch.eval.ts`
- `e2e/fixtures/agent-cancellation/evals/cancel-turn.eval.ts`

The Go tests assert the observable behavior rather than copying TypeScript
implementation details.
