const { expect } = require("@playwright/test");

/**
 * Enables ADMIN_DEBUG_MATCH client hooks (see web/match-test-config.js).
 * Only one browser context should use autoApply=true to avoid duplicate debug_match_fixture sends.
 * @param {import('@playwright/test').Page} page
 * @param {{ autoApply?: boolean, autoConfirmMulligan?: boolean }} opts
 */
async function installMatchTestHooks(page, opts = {}) {
  const autoApply = !!opts.autoApply;
  const autoConfirmMulligan = opts.autoConfirmMulligan !== false;
  await page.addInitScript(
    ({ autoApply: aa, autoConfirmMulligan: ac }) => {
      globalThis.__powerChessMatchTest = {
        autoApply: aa,
        autoConfirmMulligan: ac,
      };
    },
    { autoApply, autoConfirmMulligan },
  );
}

/**
 * Join pageB to an existing room via the lobby list (polls every 4s).
 * @param {import('@playwright/test').Page} pageB
 * @param {string} roomName
 */
async function joinViaRoomList(pageB, roomName) {
  const roomEntry = pageB.locator("#roomList .room-list-item", { hasText: roomName }).first();
  await expect(roomEntry).toBeVisible({ timeout: 15000 });
  await roomEntry.click();
  await expect(pageB.locator("#gameShell")).toBeVisible({ timeout: 10000 });
}

/**
 * Host (white) + guest (black), debug fixture from host only, both auto-confirm mulligan.
 * @param {import('@playwright/test').Browser} browser
 * @param {string} roomName
 */
async function openTwoPlayerRoomWithDebugFixture(browser, roomName) {
  const pageA = await browser.newPage();
  const pageB = await browser.newPage();
  await installMatchTestHooks(pageA, { autoApply: true, autoConfirmMulligan: true });
  await installMatchTestHooks(pageB, { autoApply: false, autoConfirmMulligan: true });

  await pageA.goto("/");
  await pageB.goto("/");

  await pageA.selectOption("#pieceType", "white");
  await pageB.selectOption("#pieceType", "black");

  await pageA.fill("#roomName", roomName);
  await pageA.click("#connectBtn");
  await expect(pageA.locator("#gameShell")).toBeVisible({ timeout: 10000 });

  await joinViaRoomList(pageB, roomName);
  await waitForMatchReady(pageA);
  await waitForMatchReady(pageB);
  return { pageA, pageB };
}

/**
 * Drag from hand to own ignition (pointer threshold + drop), matching web/app.js wireHandStackPointerDrag.
 * Playwright's dragTo can miss pointer-driven handlers; mouse path is more reliable.
 * @param {import('@playwright/test').Page} page
 * @param {number} handIndex
 */
async function dragHandCardToIgnition(page, handIndex) {
  const wrap = page.locator(`#handSelf .pm-hand-card-wrap[data-hand-index="${handIndex}"]`);
  const ignition = page.locator("#ignitionSelf");
  await wrap.scrollIntoViewIfNeeded();
  await ignition.scrollIntoViewIfNeeded();
  await expect(wrap).toBeVisible({ timeout: 15000 });
  // Synthetic PointerEvent sequence on the hand stack (bubbling), matching overlap hit-testing
  // in web/app.js wireHandStackPointerDrag — Playwright mouse/dragTo can miss overlapping hand cards.
  await page.evaluate((idx) => {
    const stack = document.querySelector("#handSelf .pm-hand-stack");
    const ignEl = document.getElementById("ignitionSelf");
    if (!stack || !ignEl) throw new Error("dragHandCardToIgnition: missing stack or ignition");
    const wraps = [...stack.querySelectorAll(".pm-hand-card-wrap")];
    const w = wraps[idx];
    if (!w) throw new Error(`dragHandCardToIgnition: no wrap for index ${idx}`);
    const br = w.getBoundingClientRect();
    const ir = ignEl.getBoundingClientRect();
    const sx = br.left + br.width / 2;
    const sy = br.top + br.height / 2;
    const ex = ir.left + ir.width / 2;
    const ey = ir.top + ir.height / 2;
    const pid = 42;
    const mk = (type, x, y, buttons) =>
      new PointerEvent(type, {
        bubbles: true,
        cancelable: true,
        clientX: x,
        clientY: y,
        pointerId: pid,
        pointerType: "mouse",
        isPrimary: true,
        button: 0,
        buttons: buttons ?? 0,
      });
    stack.dispatchEvent(mk("pointerdown", sx, sy, 1));
    stack.dispatchEvent(mk("pointermove", sx + 20, sy + 20, 1));
    stack.dispatchEvent(mk("pointermove", ex, ey, 1));
    stack.dispatchEvent(mk("pointerup", ex, ey, 0));
  }, handIndex);
}

/**
 * Waits until the latest state_snapshot JSON in #snapshot satisfies a predicate.
 * @param {import('@playwright/test').Page} page
 * @param {(snap: object) => boolean} pred
 * @param {number} [timeout]
 */
async function waitForSnapshotPredicate(page, pred, timeout = 20000) {
  await expect.poll(
    async () => {
      const txt = await page.locator("#snapshot").textContent();
      if (!txt) return false;
      try {
        return pred(JSON.parse(txt));
      } catch {
        return false;
      }
    },
    { timeout },
  ).toBe(true);
}

/**
 * The client blocks `isGameplayInputOpen` briefly after snapshots that trigger effect animations
 * (see web/app.js blockClocksForEffects). Hand cards may still look playable; wait before driving
 * pointer actions that call sendHandCardAction.
 * @param {import('@playwright/test').Page} page
 * @param {number} [ms]
 */
async function waitForEffectAnimationGate(page, ms = 1500) {
  await page.waitForTimeout(ms);
}

/** After debug fixture + auto mulligan, wait until main match play is active (no mulligan UI). */
async function waitForMatchReady(page) {
  await waitForSnapshotPredicate(
    page,
    (j) =>
      j.connectedA > 0 &&
      j.connectedB > 0 &&
      j.mulliganPhaseActive !== true &&
      j.matchEnded !== true,
    30000,
  );
}

/**
 * Wait until a hand card is not marked inactive (gameplay input open and card playable).
 * @param {import('@playwright/test').Page} page
 * @param {number} handIndex
 */
async function waitForHandCardPlayable(page, handIndex) {
  const wrap = page.locator(`#handSelf .pm-hand-card-wrap[data-hand-index="${handIndex}"]`);
  await expect(wrap).toBeVisible({ timeout: 20000 });
  await expect.poll(
    async () => {
      return !(await wrap.evaluate((el) => el.classList.contains("pm-hand-card-wrap--inactive")));
    },
    { timeout: 20000 },
  ).toBe(true);
}

module.exports = {
  installMatchTestHooks,
  joinViaRoomList,
  openTwoPlayerRoomWithDebugFixture,
  dragHandCardToIgnition,
  waitForHandCardPlayable,
  waitForSnapshotPredicate,
  waitForMatchReady,
  waitForEffectAnimationGate,
};
