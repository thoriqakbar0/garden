# Testing Garden

Garden separates fast hermetic checks, credential-free official Eve acceptance,
external differential targets, and live-provider smoke tests. A skipped test is
not counted as compatibility evidence.

## Commands

| Command | What it proves | Network or credentials |
| --- | --- | --- |
| `make test-hermetic` | Uncached native unit, integration, workflow, protocol, and host tests | None; official environment-gated tests skip |
| `make test-race` | The same Go suite under the race detector | None; official environment-gated tests skip |
| `make check` | `go vet` plus uncached race suite | None |
| `make test-official` | Installs the pinned fixture and runs the official Eve TypeScript + sandbox vertical slice | npm registry only; no provider credential or Docker |
| `make test-all` | `make check` followed by credential-free official acceptance | npm registry only |
| `make list-tests` | Lists every top-level Go test known to the Go tool | None |

Before a direct push to `main`, run:

```sh
make test-all
git diff --check
```

`make test-official` uses Node 24+, `npm ci`, the checked-in lockfile, local
`eve@0.27.6`, Eve's deterministic `mockModel`, and the pure-JavaScript
`just-bash` sandbox. It runs both the host integration and a compiled
`garden serve --runtime eve` subprocess. They assert authored tool discovery,
exact sandboxed Bash and TypeScript outputs, tool-result feedback into the final
model answer, `session.waiting`, and clean signal-driven shutdown.

## External differential targets

The shared black-box client can run against already-running official Eve
conversation and cancellation fixtures:

```sh
EVE_OFFICIAL_BASE_URL=http://127.0.0.1:3001 \
EVE_OFFICIAL_CANCELLATION_BASE_URL=http://127.0.0.1:3002 \
  go test -count=1 -v ./internal/contracttest
```

These tests intentionally skip when the URLs are absent. They count as
differential evidence only when both targets are set and both tests pass.

## Live provider smoke tests

The OpenAI-compatible, native Anthropic, and native Google adapters are
hermetically tested with fake servers. Real-provider smoke tests remain opt-in
because they consume account quota. Build once, then use one configuration:

```sh
make build

# OpenRouter free router
GARDEN_MODEL_BACKEND=openai \
GARDEN_OPENAI_BASE_URL=https://openrouter.ai/api/v1 \
GARDEN_OPENAI_API_KEY="$OPENROUTER_API_KEY" \
GARDEN_MODEL=openrouter/free \
  ./garden run --root examples/eve-weather --message "Weather in Jakarta?"

# Cloudflare Workers AI OpenAI-compatible endpoint
GARDEN_MODEL_BACKEND=openai \
GARDEN_OPENAI_BASE_URL="https://api.cloudflare.com/client/v4/accounts/$CLOUDFLARE_ACCOUNT_ID/ai/v1" \
GARDEN_OPENAI_API_KEY="$CLOUDFLARE_API_TOKEN" \
GARDEN_MODEL=@cf/ibm-granite/granite-4.0-h-micro \
  ./garden run --root examples/eve-weather --message "Weather in Jakarta?"

# Anthropic Messages API
GARDEN_MODEL_BACKEND=anthropic \
GARDEN_ANTHROPIC_API_KEY="$ANTHROPIC_API_KEY" \
GARDEN_MODEL=anthropic/claude-sonnet-4-6 \
  ./garden run --root examples/eve-weather --message "Weather in Jakarta?"

# Google generateContent API
GARDEN_MODEL_BACKEND=google \
GARDEN_GOOGLE_API_KEY="$GOOGLE_API_KEY" \
GARDEN_MODEL=google/gemini-2.5-flash \
  ./garden run --root examples/eve-weather --message "Weather in Jakarta?"
```

For authenticated Cloudflare AI Gateway `/compat`, also set
`GARDEN_CLOUDFLARE_GATEWAY_TOKEN`. Garden sends upstream provider auth in
`Authorization` and gateway auth in `cf-aig-authorization`.

## Current evidence gaps

- Garden does not independently exercise every feature delegated to the pinned
  Eve runtime; see [Compatibility](COMPATIBILITY.md).
- The official conversation and cancellation differential targets still require
  externally started fixtures.
- Native mode does not resume the final model step after a crash following a
  durable tool result; it settles the interrupted turn without replaying tools.
- OpenRouter, Cloudflare, Anthropic, and Google live calls are opt-in, not CI
  gates. Their request shapes, headers, tool-result correlation, provider
  metadata, usage normalization, and error safety are covered hermetically.

## Go suite boundaries

`make list-tests` is the sole authoritative inventory of top-level Go tests,
fuzz targets, and benchmarks. It reads the current package graph and source, so
new or renamed tests do not require a documentation update.

The suite remains organized around these evidence boundaries:

| Area | Source boundary | Evidence |
| --- | --- | --- |
| CLI | `cmd/eve/*_test.go` | Command behavior, authentication, runtime selection, and the official Eve subprocess |
| Example | `examples/eve-weather/*_test.go` | A representative Eve-shaped project remains discoverable and runnable |
| Agent runtime | `internal/agent/*_test.go` | Provider adapters, tool lifecycles, metadata, safety, cancellation, and Codex execution |
| Differential contracts | `internal/contracttest/*_test.go` | Garden behavior against externally started official Eve fixtures when both URLs are set |
| Discovery | `internal/discover/*_test.go` | Filesystem contract discovery and invalid project rejection |
| Official Eve host | `internal/evehost/*_test.go` | Pinned runtime validation, process lifecycle, and environment-gated official integration |
| Server | `internal/server/*_test.go` | Public Eve protocol behavior, redaction, streaming, cancellation, and scheduling |
| Workflow | `internal/workflow/*_test.go` | Persistence, replay, concurrency, recovery, migration, and lifecycle invariants |

Run `make list-tests` when an exact name is needed. Run `make test-hermetic` for
the credential-free Go evidence, or use the commands above for stronger gates.
