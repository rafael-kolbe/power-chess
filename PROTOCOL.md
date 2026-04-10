# Power Chess WebSocket Protocol (v2)

This document defines the current client/server websocket contract.

## Transport

- Endpoint: `ws://<host>:8080/ws`
- Metrics endpoint: `http://<host>:8080/metrics`
- Lobby list: `GET http://<host>:8080/api/rooms` returns `{ "rooms": [ { "roomId", "roomName", "roomPrivate", "connectedA", "connectedB", "gameStarted", "occupiedByColor?" } ] }` for matches that have not ended (`matchEnded` is false). When occupancy is `1/2`, `occupiedByColor` is `White` or `Black`.
- JSON envelope format:

```json
{
  "id": "optional-correlation-id",
  "type": "message_type",
  "payload": {}
}
```

## Server -> Client Messages

### `hello`

Sent when websocket connection is established.

```json
{
  "type": "hello"
}
```

### `ack`

Acknowledges a request that was accepted and processed.

```json
{
  "id": "req-123",
  "type": "ack",
  "payload": {
    "requestId": "req-123",
    "requestType": "submit_move",
    "status": "ok|queued|duplicate",
    "code": "",
    "message": ""
  }
}
```

Duplicate requests (`same room + player + type + requestId`) return `status: "duplicate"` and are not applied again.

### `error`

Reports request failure.

```json
{
  "type": "error",
  "payload": {
    "code": "join_required",
    "message": "join_match is required before submit_move"
  }
}
```

Error codes currently used:
- `bad_request`
- `unknown_message_type`
- `join_required`
- `action_failed`
- `invalid_payload`
- `protocol_violation`

### `state_snapshot`

Broadcast room state update.

```json
{
  "type": "state_snapshot",
  "payload": {
    "roomId": "12",
    "roomName": "Let's Play!",
    "roomPrivate": false,
    "roomPassword": "",
    "connectedA": 1,
    "connectedB": 1,
    "gameStarted": true,
    "turnPlayer": "A",
    "turnSeconds": 30,
    "turnNumber": 3,
    "ignitionOn": true,
    "ignitionCard": "double-turn",
    "board": [
      ["bR","bN","bB","bQ","bK","bB","bN","bR"],
      ["bP","bP","bP","bP","bP","bP","bP","bP"],
      ["","","","","","","",""],
      ["","","","","","","",""],
      ["","","","","","","",""],
      ["","","","","","","",""],
      ["wP","wP","wP","wP","wP","wP","wP","wP"],
      ["wR","wN","wB","wQ","wK","wB","wN","wR"]
    ],
    "enPassant": { "valid": false },
    "castlingRights": {
      "whiteKingSide": true,
      "whiteQueenSide": true,
      "blackKingSide": true,
      "blackQueenSide": true
    },
    "players": [
      {
        "playerId": "A",
        "mana": 3,
        "maxMana": 10,
        "energizedMana": 2,
        "maxEnergized": 20,
        "handCount": 3,
        "cooldownCount": 0,
        "graveyardCount": 0,
        "strikes": 0
      },
      {
        "playerId": "B",
        "mana": 2,
        "maxMana": 10,
        "energizedMana": 0,
        "maxEnergized": 20,
        "handCount": 3,
        "cooldownCount": 0,
        "graveyardCount": 0,
        "strikes": 0
      }
    ],
    "pendingEffects": [
      {
        "owner": "A",
        "cardId": "knight-touch"
      }
    ],
    "pendingCapture": {
      "active": true,
      "fromRow": 6,
      "fromCol": 4,
      "toRow": 5,
      "toCol": 5,
      "actor": "A"
    },
    "reactionWindow": {
      "open": true,
      "trigger": "on-ignite",
      "actor": "A",
      "eligibleTypes": ["Retribution", "Power"],
      "stackSize": 1
    },
    "matchEnded": false,
    "winner": "",
    "endReason": "",
    "rematchA": false,
    "rematchB": false,
    "postMatchMsLeft": 30000
  }
}
```

Field `enPassant` mirrors the engine state after the last completed move: when `valid` is true, a pawn may capture on `(targetRow,targetCol)` and remove the pawn at `(pawnRow,pawnCol)`. Clients may use it for move highlighting; the server remains authoritative for legality.
Field `castlingRights` mirrors server-side castling rights and should be used by clients to avoid suggesting castling after king/rook movement has already revoked that side.
Field `turnSeconds` is the authoritative per-turn timer duration used by the server timeout loop.
When a match ends, `rematchA` / `rematchB` show rematch votes and `postMatchMsLeft` exposes remaining post-match action window before room auto-close.

When building each snapshot, the server re-evaluates checkmate/stalemate from the live board. If the room was previously marked ended only as `both_disconnected_cancelled` but the position is decisively over, `winner` and `endReason` are updated to `checkmate` or `stalemate` before the payload is sent. The same reconciliation runs when the last client disconnects, so a terminal position is not overwritten by the double-disconnect cancel path.

## Client -> Server Messages

### `ping`

```json
{
  "id": "req-1",
  "type": "ping",
  "payload": { "timestamp": 1710000000 }
}
```

### `join_match`

```json
{
  "id": "req-2",
  "type": "join_match",
  "payload": {
    "roomId": "12",
    "roomName": "Let's Play!",
    "pieceType": "random",
    "playerId": "A",
    "isPrivate": false,
    "password": ""
  }
}
```

Behavior notes:
- On `join_match`, backend first attempts to load persisted room state from PostgreSQL (when persistence is enabled).
- `roomId` must be a positive integer string. If omitted/empty, backend creates a new room with auto-incremented integer id.
- `roomName` is the display name of the room. If omitted/empty, server uses `Let's Play!`.
- `pieceType` accepts `white`, `black`, `random`.
- `isPrivate` + `password` configure private rooms on creation and are validated on join.
- If no persisted room exists, backend creates a new in-memory room.

### `submit_move`

```json
{
  "id": "req-3",
  "type": "submit_move",
  "payload": {
    "fromRow": 6,
    "fromCol": 4,
    "toRow": 4,
    "toCol": 4
  }
}
```

### `activate_card`

```json
{
  "id": "req-4",
  "type": "activate_card",
  "payload": { "handIndex": 0 }
}
```

### `resolve_pending_effect`

```json
{
  "id": "req-5",
  "type": "resolve_pending_effect",
  "payload": {
    "pieceRow": 6,
    "pieceCol": 0
  }
}
```

### `queue_reaction`

```json
{
  "id": "req-6",
  "type": "queue_reaction",
  "payload": {
    "handIndex": 1,
    "pieceRow": 4,
    "pieceCol": 4
  }
}
```

### `resolve_reactions`

```json
{
  "id": "req-7",
  "type": "resolve_reactions",
  "payload": {}
}
```

### `leave_match`

Intentional room exit by the local player.

```json
{
  "id": "req-8",
  "type": "leave_match",
  "payload": {}
}
```

### `stay_in_room`

Used by a single remaining player after match end to reset room into waiting state.

```json
{
  "id": "req-9",
  "type": "stay_in_room",
  "payload": {}
}
```

### `request_rematch`

Used after match end while both players remain connected. Match resets when both sides vote and players automatically swap sides (`A <-> B`), so colors are inverted in the next game.

```json
{
  "id": "req-10",
  "type": "request_rematch",
  "payload": {}
}
```

## Capture Trigger Window

- When a valid capture move is submitted, backend opens a reaction window with trigger `capture_attempt`.
- En passant capture attempts also open `capture_attempt`.
- The move is kept as pending and is not applied immediately.
- The first reaction in this chain must come from the opponent and must be a `Counter` card.
- Reactions resolve in LIFO order.
- If no reaction is queued, `resolve_reactions` applies the pending capture move.
- `Counterattack` validation: only valid if the pending attacker is currently buffed by a `Power` card.
- `Blockade` validation: only valid when responding directly to `Counterattack` in the same `capture_attempt` chain.
- `Blockade` effect: negates `Counterattack`, cancels pending capture, and keeps attacker on original square.

## Integration Test Coverage

Current websocket integration tests cover:
- multi-client `join_match` flow with `state_snapshot` broadcast
- `ack` contract validation for normal and duplicate request handling
- request idempotency by room + player + type + requestId
- disconnect timeout and match cancellation rules
- persistence adapter integration for room save/load hooks
- in-memory telemetry counters and `/metrics` HTTP exposure

## Disconnect Rules

- If both players disconnect, match is canceled with no winner (`endReason: both_disconnected_cancelled`).
- If one player disconnects while the other stays connected, a 60-second grace timer starts.
- If disconnected player does not return in time, remaining player wins (`endReason: disconnect_timeout`).
- If a player explicitly sends `leave_match` while the opponent is still connected, the opponent wins immediately (`endReason: left_room`).

## Turn Timeout Rules

- Turn duration defaults to 30 seconds (`turnSeconds` in snapshot).
- If the active player times out, they receive 1 strike and turn immediately passes to the opponent.
- On 3rd strike, active player loses immediately (`endReason: strike_limit`).

