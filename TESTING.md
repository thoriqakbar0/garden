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

The provider adapter is hermetically tested with fake OpenAI-compatible servers.
Real-provider smoke tests remain opt-in because they consume an account quota.
Build once, then use either configuration:

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
- OpenRouter and Cloudflare live calls are opt-in, not CI gates. Their request
  shapes, headers, double-encoded arguments, tool-result correlation, and error
  safety are covered hermetically.

## Complete Go test inventory

The authoritative live inventory is `make list-tests`. At this revision there
are 76 top-level tests; table-driven subcases add further scenarios.

### `cmd/eve/main_test.go` (10)

- `TestHelpDocumentsGardenInstallation`
- `TestUnknownCommandReturnsGardenUsage`
- `TestHelpReportsOutputFailure`
- `TestCommandHelpSucceeds`
- `TestAuthenticatedHandlerRequiresTokenForPublicBind`
- `TestServeDefaultsToLoopback`
- `TestServeSelectsOfficialEveRuntimeExplicitly`
- `TestAuthenticatedHandlerProtectsPublicBind`
- `TestAuthenticatedHandlerLeavesLoopbackLocal`
- `TestConfiguredTokenAlsoProtectsLoopback`

### `cmd/eve/official_cli_integration_test.go` (1)

- `TestGardenBinaryHostsOfficialEveEndToEnd`

### `examples/eve-weather/example_test.go` (1)

- `TestOfficialEveShapeIsRunnableByGarden`

### `internal/agent/agent_test.go` (13)

- `TestOpenAIWeatherToolRoundTrip`
- `TestRunnerEmitsEveToolLifecycle`
- `TestConversationExcludesInterruptedTurns`
- `TestRejectsUnimplementedAndUndeclaredTools`
- `TestRejectsMalformedAndDuplicateToolCalls`
- `TestOpenAIToolArgumentsAcceptProviderDoubleEncoding`
- `TestCloudflareGatewayTokenHeader`
- `TestCancellationReachesModelAndTool`
- `TestUpstreamErrorsDoNotLeakSecrets`
- `TestToolErrorsAndPayloadLimitsAreSafe`
- `TestConfigurationIsExplicit`
- `TestCompletedHistoryIsSentInSessionOrder`
- `TestModelRoundsAreCapped`

### `internal/agent/codex_exec_test.go` (9)

- `TestCodexExecRunsTerminalInsideSandbox`
- `TestCodexExecConfigurationIsSandboxed`
- `TestCodexExecReportsEarlyProcessExit`
- `TestRuntimeSelectsCodexExecBackend`
- `TestRuntimeAutoDetectsCodexExecBackend`
- `TestRuntimeRequiresBackendWhenCodexIsMissing`
- `TestRuntimeDoesNotOverrideExplicitBackend`
- `TestCodexExecPromptCarriesCompletedConversation`
- `TestCodexExecPropagatesCancellation`

### `internal/contracttest/official_test.go` (2)

- `TestOfficialEveConversationContract`
- `TestOfficialEveCancellationContract`

### `internal/discover/discover_test.go` (2)

- `TestApplicationAtDiscoversFilesystemContract`
- `TestApplicationAtRejectsDuplicateScheduleIDs`

### `internal/evehost/host_test.go` (8)

- `TestHostRunsPinnedProjectLocalEve`
- `TestHostRejectsUnpinnedEve`
- `TestHostRequiresProjectLocalEve`
- `TestHostRejectsCLIOutsideNodeModules`
- `TestHostRejectsNonExecutableCLI`
- `TestHostRejectsInvalidAddress`
- `TestHostDoesNotStartWithCancelledContext`
- `TestHostCancellationStopsEveProcess`

### `internal/evehost/official_integration_test.go` (1)

- `TestOfficialEveAuthoredTypeScriptAndSandboxTerminal`

### `internal/server/server_test.go` (7)

- `TestGardenPassesSharedEveConversationContract`
- `TestGardenPassesSharedEveCancellationContract`
- `TestToolInternalsStayOffPublicStreamAndCursor`
- `TestStreamDisconnectDoesNotCancelActiveTurn`
- `TestCancelValidationMatchesEve`
- `TestScheduleDispatchUsesDiscoveredIdentifier`
- `TestInfoRedactsProjectRootAndInstructions`

### `internal/workflow/cancel_emit_order_test.go` (1)

- `TestAcceptedCancelRejectsEmitAlreadyWaitingToAppend`

### `internal/workflow/legacy_protocol_test.go` (1)

- `TestMigratedLegacyMessageHasCanonicalPublicLifecycle`

### `internal/workflow/recovery_invariants_test.go` (1)

- `TestOpenRejectsRecoveryInvariantViolations`

### `internal/workflow/store_test.go` (19)

- `TestSequentialTurnsPersistAndReplay`
- `TestConcurrentSessionsRemainIsolated`
- `TestCancellationConsumesStaleGuardWithoutCancellingActiveTurn`
- `TestSessionIDCannotEscapeStore`
- `TestContinuationTokenSelectsOwnerAndCreatesUnownedSession`
- `TestReplaySupportsTailRelativeCursor`
- `TestStoreAllowsOnlyOneWriter`
- `TestCloseWakesEventWaiters`
- `TestOpenSessionRootCannotBeRedirectedAfterStartup`
- `TestConcurrentUnownedContinuationHasOneOwner`
- `TestAcceptedCancellationWinsAgainstLateRunnerSuccess`
- `TestOpenRejectsSymlinkedSessionLog`
- `TestOpenRejectsSymlinkedSessionsDirectory`
- `TestOpenRejectsCorruptLifecyclePayload`
- `TestOpenRejectsCompletionAfterDurableCancellationIntent`
- `TestOpenAtomicallyMigratesLegacySessionLog`
- `TestOpenRepairsPartialTailAndSettlesInterruptedTurn`
- `TestOpenRepairsMissingWaitingAfterDurableTerminal`
- `TestOpenFinishesDurableCancellationIntent`
