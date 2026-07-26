# Project: Runtime integrity

## Goal

Make the local runner safe to expose to a model and side-effecting tools. A
durable log is insufficient unless identities, ownership, cancellation, and
restart behavior are explicit.

## Session identity

Session IDs and turn IDs are opaque validated values. Before any filesystem
operation, Garden MUST reject:

- empty IDs;
- `.` and `..`;
- path separators and encoded separators;
- absolute paths;
- values outside the documented character and length set; and
- paths that resolve through a symlink outside the session store.

Generated IDs SHOULD use a fixed prefix and cryptographically random payload.
User-provided IDs are accepted only after the same validation.

The regression suite MUST prove that `../outside`, absolute paths, encoded
slashes, and malformed IDs cannot create, read, modify, or cancel state outside
the store.

## Storage invariants

For each session:

- event indexes are contiguous and strictly increasing;
- an append is durable before it becomes observable;
- replay never observes a partially written event;
- only one active turn owns the session;
- a turn has exactly one terminal state;
- completed tool side effects are never repeated during recovery; and
- two Garden processes cannot both become writers accidentally.

JSONL MAY remain the initial format if Garden implements:

- a process-level single-writer lock;
- per-session ownership;
- bounded append behavior that does not reread the entire history;
- safe detection and truncation of a corrupt final partial record;
- explicit handling of corruption before the final record; and
- recovery of interrupted lifecycle states.

If these invariants cannot be expressed cleanly with JSONL, use transactional
SQLite. The storage choice is subordinate to the invariants.

## Restart recovery

On startup Garden MUST inspect non-terminal turns. Recovery must deterministically
settle or resume them; it must not leave ambiguous state.

At minimum, the test suite must kill and restart Garden at these boundaries:

1. after `turn.started`;
2. after a model requests a tool but before execution;
3. after tool completion but before the next model response;
4. after `message.completed` but before `turn.completed`; and
5. during a flushed NDJSON stream.

The tool-completed boundary requires an idempotency record or durable result so
the tool is not executed twice.

## Cancellation ownership

Cancellation belongs to the active turn, not merely the session. It MUST:

- propagate through model and tool contexts;
- survive transport disconnects;
- never cancel a newer turn through a stale turn ID;
- persist one terminal cancellation event; and
- prevent later completion from being appended.

Process restart clears no durable cancellation fact, although it may invalidate
an in-memory cancellation function.

## Network and trust boundary

The server MUST default to a loopback address. Binding to a non-loopback address
requires an explicit exposure option and authentication policy.

Before real model spending or side-effecting tools are enabled:

- `/info` MUST NOT expose absolute paths, secrets, or full instructions by
  default;
- state-changing schedule dispatch MUST NOT use `GET`;
- errors MUST use stable public messages rather than filesystem or upstream
  details;
- request bodies and streams MUST have resource limits; and
- logs MUST redact authorization headers, API keys, model bodies containing
  secrets, and tool error details.

Authentication and authorization are separate from user-selected project or
session filters. A client-provided ID is not proof of access.

