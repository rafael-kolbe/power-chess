const { test, expect } = require("@playwright/test");

function emptyBoard() {
  return Array.from({ length: 8 }, () => Array.from({ length: 8 }, () => ""));
}

function baseSnapshot(overrides = {}) {
  return {
    roomId: "e2e-room",
    roomName: "E2E Room",
    roomPrivate: false,
    connectedA: 1,
    connectedB: 1,
    gameStarted: true,
    turnPlayer: "A",
    turnSeconds: 30,
    turnNumber: 1,
    ignitionOn: false,
    board: emptyBoard(),
    enPassant: { valid: false },
    castlingRights: {
      whiteKingSide: true,
      whiteQueenSide: true,
      blackKingSide: true,
      blackQueenSide: true
    },
    players: [
      { playerId: "A", mana: 0, maxMana: 10, energizedMana: 0, maxEnergized: 20, handCount: 0, cooldownCount: 0, graveyardCount: 0, strikes: 0 },
      { playerId: "B", mana: 0, maxMana: 10, energizedMana: 0, maxEnergized: 20, handCount: 0, cooldownCount: 0, graveyardCount: 0, strikes: 0 }
    ],
    pendingEffects: [],
    reactionWindow: { open: false, stackSize: 0 },
    pendingCapture: { active: false },
    matchEnded: false,
    rematchA: false,
    rematchB: false,
    postMatchMsLeft: 30000,
    ...overrides
  };
}

async function installMockSocket(page) {
  await page.addInitScript(() => {
    class MockWebSocket {
      static OPEN = 1;
      static CLOSED = 3;

      constructor(url) {
        this.url = url;
        this.readyState = MockWebSocket.OPEN;
        this.sent = [];
        this.onopen = null;
        this.onmessage = null;
        this.onclose = null;
        this.onerror = null;
        globalThis.__wsMock.instances.push(this);
        setTimeout(() => {
          if (typeof this.onopen === "function") this.onopen();
        }, 0);
      }

      send(raw) {
        const env = JSON.parse(raw);
        this.sent.push(env);
        globalThis.__wsMock.lastSent = env;
        if (env.type === "join_match") {
          const joinedPlayer = env.payload?.playerId || "A";
          const ack = {
            id: env.id,
            type: "ack",
            payload: { requestId: env.id, requestType: "join_match", status: "ok" }
          };
          this.serverSend(ack);
          this.serverSend({
            type: "state_snapshot",
            payload: {
              roomId: "e2e-room",
              roomName: "E2E Room",
              roomPrivate: false,
              connectedA: 1,
              connectedB: 1,
              gameStarted: true,
              turnPlayer: joinedPlayer,
              turnSeconds: 30,
              turnNumber: 1,
              ignitionOn: false,
              board: Array.from({ length: 8 }, () => Array.from({ length: 8 }, () => "")),
              enPassant: { valid: false },
              castlingRights: {
                whiteKingSide: true,
                whiteQueenSide: true,
                blackKingSide: true,
                blackQueenSide: true
              },
              players: [
                { playerId: "A", mana: 0, maxMana: 10, energizedMana: 0, maxEnergized: 20, handCount: 0, cooldownCount: 0, graveyardCount: 0, strikes: 0 },
                { playerId: "B", mana: 0, maxMana: 10, energizedMana: 0, maxEnergized: 20, handCount: 0, cooldownCount: 0, graveyardCount: 0, strikes: 0 }
              ],
              pendingEffects: [],
              reactionWindow: { open: false, stackSize: 0 },
              pendingCapture: { active: false },
              matchEnded: false,
              rematchA: false,
              rematchB: false
            }
          });
        } else if (env.type === "request_rematch") {
          this.serverSend({
            id: env.id,
            type: "ack",
            payload: { requestId: env.id, requestType: "request_rematch", status: "ok" }
          });
        }
      }

      serverSend(msg) {
        if (typeof this.onmessage === "function") {
          this.onmessage({ data: JSON.stringify(msg) });
        }
      }

      close() {
        this.readyState = MockWebSocket.CLOSED;
        if (typeof this.onclose === "function") this.onclose();
      }
    }

    globalThis.__wsMock = {
      instances: [],
      lastSent: null,
      sendToLast(msg) {
        const ws = this.instances[this.instances.length - 1];
        if (!ws) throw new Error("No websocket instance available.");
        ws.serverSend(msg);
      }
    };

    globalThis.WebSocket = MockWebSocket;
  });
}

test.beforeEach(async ({ page }) => {
  await installMockSocket(page);
  await page.route("**/api/rooms", async (route) => {
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({ rooms: [] })
    });
  });
  await page.goto("/");
  await page.selectOption("#localeSelect", "pt-BR");
  await page.selectOption("#pieceType", "white");
  await page.click("#connectBtn");
});

test("shows proposed rematch notification for opponent", async ({ page }) => {
  await page.evaluate((snapshot) => {
    globalThis.__wsMock.sendToLast({ type: "state_snapshot", payload: snapshot });
  }, baseSnapshot({ matchEnded: true, winner: "A", endReason: "left_room", rematchA: false, rematchB: true }));

  await expect(page.locator("#matchEndOverlay")).toBeVisible();
  await expect(page.locator("#matchEndBody")).toContainText("Novo jogo proposto, clique em 'Jogar novamente' para aceitar.");
});

test("sends request_rematch and disables button until resolution", async ({ page }) => {
  await page.evaluate((snapshot) => {
    globalThis.__wsMock.sendToLast({ type: "state_snapshot", payload: snapshot });
  }, baseSnapshot({ matchEnded: true, winner: "A", endReason: "left_room" }));

  await page.evaluate(() => {
    const btn = document.getElementById("matchEndRematch");
    if (!btn) throw new Error("matchEndRematch button missing");
    btn.click();
  });
  const requestType = await page.evaluate(() => globalThis.__wsMock.lastSent?.type);
  expect(requestType).toBe("request_rematch");
});

test("if opponent leaves after proposal, keeps player gets proper message and actions", async ({ page }) => {
  await page.evaluate((snapshot) => {
    globalThis.__wsMock.sendToLast({ type: "state_snapshot", payload: snapshot });
  }, baseSnapshot({ matchEnded: true, winner: "A", endReason: "left_room", rematchA: true, rematchB: false, connectedA: 1, connectedB: 0 }));

  await expect(page.locator("#matchEndBody")).toContainText("O outro jogador saiu da sala.");
  const actionState = await page.evaluate(() => ({
    stayHidden: document.getElementById("matchEndStay")?.classList.contains("hidden"),
    rematchHidden: document.getElementById("matchEndRematch")?.classList.contains("hidden")
  }));
  expect(actionState.stayHidden).toBe(false);
  expect(actionState.rematchHidden).toBe(true);
});

test("shows waiting message after local rematch vote", async ({ page }) => {
  await page.evaluate((snapshot) => {
    globalThis.__wsMock.sendToLast({ type: "state_snapshot", payload: snapshot });
  }, baseSnapshot({ matchEnded: true, winner: "A", endReason: "left_room", rematchA: true, rematchB: false, connectedA: 1, connectedB: 1 }));

  await expect(page.locator("#matchEndBody")).toContainText("Aguardando o adversário aceitar o novo jogo.");
});

test("hides end-match overlay when a new match starts", async ({ page }) => {
  await page.evaluate((snapshot) => {
    globalThis.__wsMock.sendToLast({ type: "state_snapshot", payload: snapshot });
  }, baseSnapshot({ matchEnded: true, winner: "A", endReason: "left_room" }));
  await expect(page.locator("#matchEndOverlay")).toBeVisible();

  await page.evaluate((snapshot) => {
    globalThis.__wsMock.sendToLast({ type: "state_snapshot", payload: snapshot });
  }, baseSnapshot({ matchEnded: false, gameStarted: true, rematchA: false, rematchB: false }));

  await expect(page.locator("#matchEndOverlay")).toBeHidden();
});
