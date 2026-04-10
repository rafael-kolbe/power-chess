const { test, expect } = require("@playwright/test");

test("switches locale to pt-BR and updates core labels", async ({ page }) => {
  await page.goto("/");
  await page.selectOption("#localeSelect", "pt-BR");

  await expect(page.locator("#pieceTypeLabel")).toContainText("Tipo de peça");
  await expect(page.locator("#connectBtn")).toContainText("Criar");
  await expect(page.locator("#roomListTitle")).toContainText("Salas abertas");
});

test("shows private join modal when clicking a private room", async ({ browser }) => {
  const roomName = `private-room-${Date.now()}`;
  const roomPassword = "abc123";

  const owner = await browser.newPage();
  await owner.goto("/");
  await owner.check("#privateRoom");
  await owner.fill("#roomPassword", roomPassword);
  await owner.fill("#roomName", roomName);
  await owner.click("#connectBtn");
  await expect(owner.locator("#gameShell")).toBeVisible();

  const guest = await browser.newPage();
  await guest.goto("/");
  const roomEntry = guest.locator("#roomList .room-list-item", { hasText: roomName }).first();
  await expect(roomEntry).toBeVisible({ timeout: 15000 });
  await roomEntry.click();

  await expect(guest.locator("#privateJoinOverlay")).toBeVisible();
  await expect(guest.locator("#privateJoinTitle")).toBeVisible();
  await expect(guest.locator("#privateJoinSubmit")).toBeVisible();

  await owner.close();
  await guest.close();
});

test("filters room list by search field", async ({ browser }) => {
  const roomA = `alpha-${Date.now()}`;
  const roomB = `beta-${Date.now()}`;

  const pageA = await browser.newPage();
  await pageA.goto("/");
  await pageA.fill("#roomName", roomA);
  await pageA.click("#connectBtn");
  await expect(pageA.locator("#gameShell")).toBeVisible();

  const pageB = await browser.newPage();
  await pageB.goto("/");
  await pageB.fill("#roomName", roomB);
  await pageB.click("#connectBtn");
  await expect(pageB.locator("#gameShell")).toBeVisible();

  const viewer = await browser.newPage();
  await viewer.goto("/");
  await viewer.fill("#roomSearch", roomA);
  const roomAEntry = viewer.locator("#roomList .room-list-item", { hasText: roomA }).first();
  await expect(roomAEntry).toBeVisible({ timeout: 15000 });
  await expect(viewer.locator("#roomList .room-list-item", { hasText: roomB })).toHaveCount(0);

  await pageA.close();
  await pageB.close();
  await viewer.close();
});
