# Project: Eve HTTP contract

## Goal

Expose Garden sessions through the observable Eve protocol while keeping the
runtime local and Vercel-free. One black-box client MUST exercise Garden and a
pinned official Eve fixture without target-specific assertions.

## Routes

### Create and start a session

`POST /eve/v1/session`

Request:

```json
{"message":"Weather in Jakarta?"}
```

The server MUST:

- reject an empty message, malformed JSON, unknown fields, and oversized
  bodies;
- create the session and start its first turn;
- return `202 Accepted`;
- set `Content-Type: application/json`;
- set `Cache-Control: no-store`;
- set `x-eve-session-id` to the returned session ID; and
- return exactly `ok`, `sessionId`, and `continuationToken`.

The continuation token MUST be opaque and MUST NOT encode a filesystem path.

### Continue a session

`POST /eve/v1/session/{sessionId}`

Request:

```json
{
  "message": "And tomorrow?",
  "continuationToken": "eve:<opaque>"
}
```

The continuation token, rather than the URL session ID, selects the channel
session. A valid continuation returns `200 OK`, `Content-Type:
application/json`, and exactly `ok` and `sessionId`. The response header and
body MUST identify the actual selected session.

The pinned Eve fixture has this non-obvious behavior:

- a token owned by the URL session continues that session;
- a syntactically valid but unowned token starts a new session;
- the new session ID is returned in both the response body and
  `x-eve-session-id`, even though the URL named another existing session; and
- `session.waiting` returns the token that owns the next continuation.

Garden MUST reproduce this behavior at the wire boundary. Internally it MUST
still treat tokens as opaque capabilities, validate their syntax, and prevent a
client from deriving one from a session ID.

### Stream events

`GET /eve/v1/session/{sessionId}/stream?startIndex=N`

A successful response MUST include:

```text
Content-Type: application/x-ndjson; charset=utf-8
Cache-Control: no-store, no-transform
X-Accel-Buffering: no
x-eve-session-id: <session-id>
x-eve-stream-format: ndjson
x-eve-stream-version: 19
```

The response MUST flush one blank line immediately, then encode one complete
event per newline. It MUST remain open while the turn is active and flush each
event without buffering the complete turn.

`startIndex` is a signed integer:

- omitted: replay from index `0`, then follow live events;
- `N >= 0`: replay from absolute event index `N`;
- `N < 0`: replay from `max(eventCount + N, 0)`.

Malformed cursors return `400` with the stable JSON error:

```json
{"error":"Expected startIndex to be an integer.","ok":false}
```

Disconnecting a stream MUST NOT cancel its turn. Reconnecting from the first
unseen cursor MUST produce no missing or duplicate durable events.

### Cancel a turn

`POST /eve/v1/session/{sessionId}/cancel`

The body MAY contain a `turnId`. The server returns `202 Accepted` with exactly
`ok`, `sessionId`, and `status`.

- A matching active turn MUST receive cancellation.
- A stale guarded `turnId` MUST NOT cancel a different turn.
- A late cancellation MUST be harmless and report `no_active_turn`.
- A cancelled turn MUST end with exactly one `turn.cancelled`, followed by
  `session.waiting`.
- No completion event may appear after `turn.cancelled`.

## Event envelope

Every NDJSON event is an immutable object:

```json
{
  "type": "turn.started",
  "data": {},
  "meta": {"at":"<RFC3339Nano UTC timestamp>"}
}
```

Replays MUST preserve the original envelope byte-for-byte or as an equivalent
JSON value. Public event types and payload fields are controlled by the pinned
Eve protocol, not by Garden implementation details.

For a successful text-only turn, protocol version 19 currently expects:

```text
session.started        # first turn only
turn.started
message.received
step.started
message.appended
message.completed
step.completed
turn.completed
session.waiting
```

The differential suite is authoritative if the pinned official fixture differs.

Tool calls and tool results MAY have richer internal events. They MUST NOT add
Garden-only public event types such as `actions.requested` or `action.result`
unless the pinned Eve fixture proves those exact wire events and payloads.

## Differential suite

The shared suite MUST cover:

- create and continuation status, headers, and exact response keys;
- continuation-token ownership and stale-token behavior;
- immediate NDJSON prelude and incremental flushing;
- exact event order and allowed payload keys;
- absolute and tail-relative resume;
- disconnect and reconnect;
- live cancellation, stale guarded cancellation, and late cancellation;
- malformed JSON, unknown fields, invalid cursors, unknown sessions, and
  oversized bodies.

Official Eve runs MAY be gated by fixture URLs, but a skipped official run is
not evidence of differential compatibility. Release evidence must name the
pinned Eve revision and include a completed run against both targets.
