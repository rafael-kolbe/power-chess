const { test, expect } = require("@playwright/test");

function uniqueRoomName(prefix) {
  return `${prefix}-${Date.now()}-${Math.floor(Math.random() * 10000)}`;
}

test("creates and enters a public room", async ({ page }) => {
  const roomName = uniqueRoomName("public-room");
  await page.goto("/");
  await page.fill("#roomName", roomName);
  await page.selectOption("#pieceType", "white");
  await page.click("#connectBtn");

  await expect(page.locator("#gameShell")).toBeVisible();
  await expect(page.locator("#inRoomLabel")).toContainText(roomName);
  await expect(page.locator("#waitingBanner")).toBeVisible();
});

test("requires password when creating a private room", async ({ page }) => {
  await page.goto("/");
  await page.selectOption("#localeSelect", "pt-BR");
  await page.check("#privateRoom");
  await page.fill("#roomPassword", "");
  await page.click("#connectBtn");

  await expect(page.locator("#lobbyPrivatePasswordError")).toBeVisible();
  await expect(page.locator("#lobbyPrivatePasswordError")).toContainText("senha");
  await expect(page.locator("#lobbyScreen")).toBeVisible();
});

test("shows existing room in lobby list for another player", async ({ browser }) => {
  const roomName = uniqueRoomName("join-room");
  const pageA = await browser.newPage();
  await pageA.goto("/");
  await pageA.fill("#roomName", roomName);
  await pageA.selectOption("#pieceType", "white");
  await pageA.click("#connectBtn");
  await expect(pageA.locator("#gameShell")).toBeVisible();

  const pageB = await browser.newPage();
  await pageB.goto("/");
  const roomEntry = pageB.locator("#roomList .room-list-item", { hasText: roomName }).first();
  await expect(roomEntry).toBeVisible({ timeout: 15000 });
  await expect(roomEntry).toContainText("1/2");

  await pageA.close();
  await pageB.close();
});
