---
name: power-chess-testing
description: Manages, runs, extends, and troubleshoots the Power Chess test suite (Go unit tests, WebSocket integration tests, and Playwright E2E tests). Use when adding tests, fixing failing tests, checking coverage, or asked about the testing strategy, test structure, or test commands.
---

# Power Chess Testing

## Running tests

```bash
# All Go tests (unit + integration)
go test ./...

# Playwright E2E (server auto-started by playwright.config.js)
npm run test:e2e

# Coverage profile
go test ./... -coverprofile=coverage.out
go tool cover -html=coverage.out
```

Before any commit: run `go test ./...` first. Only commit when all tests pass.

## Go test locations

| Package | File | What it covers |
|---------|------|----------------|
| `internal/chess` | `engine_coverage_test.go` | `IsStalemate`, `ApplyPseudoLegalMove`, `IsSquareAttacked` edge cases |
| `internal/gameplay` | `turn_coverage_test.go` | `GrantCaptureBonusMana`, `ConsumeCardFromHand`, `EndTurn`, `StartTurn`, `SelectPlayerSkill`, `tickCooldowns`, `EnterMulliganPhaseWithoutShuffle` |
| `internal/match` | `engine_coverage_test.go` | `EndTurn`, `ActivatePlayerSkill`, `PendingEffects`, `ReactionWindowSnapshot`, `EffectResolver` implementations |
| `internal/server` | `ws_integration_test.go` | Original WebSocket integration tests |
| `internal/server` | `ws_handlers_test.go` | New handler tests: confirm_mulligan, submit_move, activate_card, draw_card, leave_match, debug_match_fixture |

## Adding Go integration tests (WebSocket)

The key helpers in `ws_handlers_test.go`:

```go
// Start a test server and dial two players
srv := newTestServer(t)
cA, _ := dialAndHello(t, srv)   // clears read deadline after hello
cB, _ := dialAndHello(t, srv)

// Join both players to the same room
joinRoom(t, cA, "room-1", "")
joinRoom(t, cB, "room-1", "")

// Progress through mulligan
confirmMulliganBoth(t, cA, cB)

// Drain messages until a specific type (up to maxMsg messages)
env, found := drainUntilType(t, cA, MessageStateSnapshot, 20)
```

### gorilla/websocket gotcha

`gorilla/websocket` v1.5.3 **permanently marks a connection as broken** once a read deadline fires — even on a clean timeout with no partial frame. Rules:
- `dialAndHello` clears the read deadline after the hello (`c.SetReadDeadline(time.Time{})`)
- Never use short speculative read deadlines in tests
- Use `drainUntilType(t, conn, targetType, 20)` with a large `maxMsg` to skip buffered snapshots
- Do NOT call `drainRemainingSnapshots` (removed) or any helper that sets short deadlines then moves on

### Debug fixture

When using `debug_match_fixture` in tests, card IDs must be from `DefaultDeckPresetCardIDs()`:
- Valid: `"energy-gain"`, `"knight-touch"`, `"bishop-touch"`, `"rook-touch"`, `"backstab"`, `"extinguish"`, `"clairvoyance"`, `"save-it-for-later"`, `"retaliate"`, `"counterattack"`, `"sacrifice-of-the-masses"`, `"mana-burn"`
- Invalid (not in default deck): `"double-turn"`, `"stop-right-there"`, etc.

The server must be started with `ADMIN_DEBUG_MATCH=1` to accept `debug_match_fixture` messages.

### Request deduplication — per-connection scoping

The `join_match` idempotency key includes the client's `connID` (random per connection):
```
roomID|join_match|connID|envID
```
All other handlers use `c.requestKey(env)` which includes `c.playerID`. This prevents clients that share the same envelope counter from colliding.

## Playwright E2E tests

Configuration: `playwright.config.js`
- Server: `DATABASE_URL= ADMIN_DEBUG_MATCH=1 go run ./cmd/server`
- Server health URL: `http://127.0.0.1:8080/healthz`
- `reuseExistingServer: false` — always fresh server per run
- Test timeout: 60 s per test

### Test files

| File | What it covers |
|------|----------------|
| `tests/e2e/ui-basics.spec.js` | Page load, room creation, search filter |
| `tests/e2e/lobby-flow.spec.js` | Room list polling, joining via lobby |
| `tests/e2e/playmat-zones.spec.js` | Playmat zones visible, draw button state, pile view modal |
| `tests/e2e/rematch-modal.spec.js` | End-of-match overlay, rematch flow (uses MockWebSocket) |

### joinViaRoomList helper

The room list polls every 4 s (`setInterval(refreshRoomList, 4000)`). Use the helper defined in `playmat-zones.spec.js` for joining via the UI:

```javascript
async function joinViaRoomList(pageB, roomName) {
  const roomEntry = pageB.locator("#roomList .room-list-item", { hasText: roomName }).first();
  await expect(roomEntry).toBeVisible({ timeout: 15000 });
  await roomEntry.click();
  await expect(pageB.locator("#gameShell")).toBeVisible({ timeout: 10000 });
}
```

### MockWebSocket pattern (rematch-modal.spec.js)

Tests using `installMockSocket` must wait for the initial handshake before injecting snapshots:

```javascript
test.beforeEach(async ({ page }) => {
  await installMockSocket(page);
  // ... route setup, goto, locale, piece type, click connect ...
  // REQUIRED: wait for mock WS handshake to complete
  await expect(page.locator("#gameShell")).toBeVisible({ timeout: 5000 });
});
```

Without this wait, a race condition causes the test snapshot to arrive before the initial one, and the subsequent `matchEnded: false` snapshot hides the overlay.

### Playwright stateless mode

The Playwright webServer runs with `DATABASE_URL=` (empty), which disables PostgreSQL. The server starts in stateless/in-memory mode. Auth endpoints return 503; the frontend sets `authBackendAvailable = false`. No tokens, no deck checks.

## Coverage targets

Run coverage after adding new tests:

```bash
go test ./... -coverprofile=coverage.out && go tool cover -func=coverage.out | tail -5
```

Focus new tests on handlers and resolvers in:
- `internal/server/ws.go` — handler functions
- `internal/match/resolvers.go` — `Apply` and `RequiresTarget` implementations
- `internal/gameplay/state.go` — turn/mana helpers

## Troubleshooting

| Symptom | Likely cause | Fix |
|---------|-------------|-----|
| `i/o timeout` in WS integration test | Read deadline not cleared after `dialAndHello` | `c.SetReadDeadline(time.Time{})` after hello |
| `duplicate_request` ack for second player join | Old code missing `connID` in dedup key | Fixed in `handleJoinMatch` (connID scoping) |
| `#gameShell` stays hidden in E2E | Server returned `duplicate` for join_match | See dedup fix above |
| E2E mock test overlay not visible | `beforeEach` didn't await `#gameShell` | Add `await expect(page.locator("#gameShell")).toBeVisible()` |
| Server fails to start for Playwright | `.env` sets `DATABASE_URL` | Playwright config overrides with `DATABASE_URL=` |
| `deck_lookup_failed` in debug fixture | Card ID not in default preset | Use only cards from `DefaultDeckPresetCardIDs()` |
