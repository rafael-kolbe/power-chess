const { test, expect } = require("@playwright/test");

/**
 * Helper: join pageB to an existing room by clicking its entry in the room list.
 * Uses a longer timeout than the default since room list polling fires every 4 s.
 */
async function joinViaRoomList(pageB, roomName) {
  const roomEntry = pageB.locator("#roomList .room-list-item", { hasText: roomName }).first();
  await expect(roomEntry).toBeVisible({ timeout: 15000 });
  await roomEntry.click();
  await expect(pageB.locator("#gameShell")).toBeVisible({ timeout: 10000 });
}

test("playmat zones are present in the DOM when entering a room", async ({ browser }) => {
  const pageA = await browser.newPage();
  const pageB = await browser.newPage();

  await pageA.goto("/");
  await pageB.goto("/");

  const roomName = `pm-zones-${Date.now()}`;
  await pageA.fill("#roomName", roomName);
  await pageA.click("#connectBtn");
  await expect(pageA.locator("#gameShell")).toBeVisible();

  await joinViaRoomList(pageB, roomName);

  // Both players should see the playmat zone elements.
  for (const page of [pageA, pageB]) {
    await expect(page.locator("#deckSelf")).toBeVisible();
    await expect(page.locator("#deckOpp")).toBeVisible();
    await expect(page.locator("#handSelf")).toBeVisible();
    await expect(page.locator("#handOpp")).toBeVisible();
    await expect(page.locator("#ignitionSelf")).toBeVisible();
    await expect(page.locator("#ignitionOpp")).toBeVisible();
    await expect(page.locator("#cooldownSelf")).toBeVisible();
    await expect(page.locator("#cooldownOpp")).toBeVisible();
    await expect(page.locator("#graveyardSelf")).toBeVisible();
    await expect(page.locator("#graveyardOpp")).toBeVisible();
    await expect(page.locator("#banishSelf")).toBeVisible();
    await expect(page.locator("#banishOpp")).toBeVisible();
    await expect(page.locator("#drawBtn")).toBeVisible();
  }

  await pageA.close();
  await pageB.close();
});

test("DRAW button is disabled when it is not the player's turn", async ({ browser }) => {
  const pageA = await browser.newPage();
  const pageB = await browser.newPage();

  await pageA.goto("/");
  await pageB.goto("/");

  const roomName = `pm-draw-${Date.now()}`;
  await pageA.fill("#roomName", roomName);
  await pageA.click("#connectBtn");
  await expect(pageA.locator("#gameShell")).toBeVisible();

  await joinViaRoomList(pageB, roomName);

  // Page B joined as the second player. Turn starts at player A.
  // Page B should have the draw button disabled (not their turn).
  await expect(pageB.locator("#drawBtn")).toBeDisabled({ timeout: 5000 });

  await pageA.close();
  await pageB.close();
});

test("pile view modal opens when VIEW button is clicked", async ({ browser }) => {
  const pageA = await browser.newPage();
  const pageB = await browser.newPage();

  await pageA.goto("/");
  await pageB.goto("/");

  const roomName = `pm-view-${Date.now()}`;
  await pageA.fill("#roomName", roomName);
  await pageA.click("#connectBtn");
  await expect(pageA.locator("#gameShell")).toBeVisible();

  await joinViaRoomList(pageB, roomName);

  // Click the banish zone — modal should open (even when empty).
  await pageA.click("#banishSelf");
  await expect(pageA.locator("#pileViewModal")).toBeVisible();
  await pageA.click("#pileViewCloseBtn");
  await expect(pageA.locator("#pileViewModal")).toBeHidden();

  await pageA.close();
  await pageB.close();
});
