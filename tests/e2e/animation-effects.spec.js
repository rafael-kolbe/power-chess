const { test, expect } = require("@playwright/test");
const {
  openTwoPlayerRoomWithDebugFixture,
  dragHandCardToIgnition,
  waitForHandCardPlayable,
  waitForSnapshotPredicate,
  waitForEffectAnimationGate,
} = require("./helpers/match-room.js");

/**
 * ADMIN_DEBUG_MATCH is enabled by playwright.config.js webServer.
 * Fixture payload is web/match-test-config.js buildMatchDebugFixturePayload (unchanged here).
 */

test.describe("Playmat animation signals (E2E)", () => {
  test("Power card enters ignition: ignition zone plays brief activation class", async ({ browser }) => {
    const roomName = `anim-ignite-${Date.now()}`;
    const { pageA, pageB } = await openTwoPlayerRoomWithDebugFixture(browser, roomName);

    try {
      await waitForHandCardPlayable(pageA, 1);
      // Default white hand: ["knight-touch","energy-gain","bishop-touch"] — energy-gain is Power.
      await dragHandCardToIgnition(pageA, 1);

      await waitForSnapshotPredicate(
        pageA,
        (j) => j.ignitionOn === true && j.ignitionCard === "energy-gain",
      );

      // Brief class (~650ms) is applied inside the snapshot apply chain; poll in case the chain runs late.
      await expect
        .poll(
          async () =>
            pageA.evaluate(() =>
              document.getElementById("ignitionSelf")?.classList.contains("pm-ignition-activating"),
            ),
          { timeout: 8000 },
        )
        .toBe(true);
    } finally {
      await pageA.close();
      await pageB.close();
    }
  });

  test("Ignite reaction: retaliate response puts retaliate on B cooldown", async ({
    browser,
  }) => {
    const roomName = `anim-resolve-ret-${Date.now()}`;
    const { pageA, pageB } = await openTwoPlayerRoomWithDebugFixture(browser, roomName);

    try {
      // A: knight-touch (Power) opens ignite_reaction for B.
      await waitForHandCardPlayable(pageA, 0);
      await dragHandCardToIgnition(pageA, 0);

      await waitForSnapshotPredicate(
        pageA,
        (j) => j.ignitionOn === true && j.ignitionCard === "knight-touch",
      );

      await waitForSnapshotPredicate(
        pageB,
        (j) =>
          j.reactionWindow?.open === true &&
          j.reactionWindow?.trigger === "ignite_reaction" &&
          Number(j.reactionWindow?.stackSize || 0) === 0,
      );

      await waitForEffectAnimationGate(pageB);

      // B: retaliate (Retribution) — hand index 0.
      await waitForHandCardPlayable(pageB, 0);
      await dragHandCardToIgnition(pageB, 0);

      await waitForSnapshotPredicate(pageA, (j) =>
        (j.players || []).some(
          (p) =>
            p.playerId === "B" &&
            (p.cooldownPreview || []).some((c) => c.cardId === "retaliate"),
        ),
      );
    } finally {
      await pageA.close();
      await pageB.close();
    }
  });

  test("Ignite reaction: backstab response puts backstab on B cooldown", async ({
    browser,
  }) => {
    const roomName = `anim-resolve-pwr-${Date.now()}`;
    const { pageA, pageB } = await openTwoPlayerRoomWithDebugFixture(browser, roomName);

    try {
      await waitForHandCardPlayable(pageA, 0);
      await dragHandCardToIgnition(pageA, 0);

      await waitForSnapshotPredicate(
        pageA,
        (j) => j.ignitionOn === true && j.ignitionCard === "knight-touch",
      );

      await waitForSnapshotPredicate(
        pageB,
        (j) =>
          j.reactionWindow?.open === true &&
          j.reactionWindow?.trigger === "ignite_reaction" &&
          Number(j.reactionWindow?.stackSize || 0) === 0,
      );

      await waitForEffectAnimationGate(pageB);

      // B: backstab is Power; allowed as first ignite reaction response.
      await waitForHandCardPlayable(pageB, 1);
      await dragHandCardToIgnition(pageB, 1);

      await waitForSnapshotPredicate(pageA, (j) =>
        (j.players || []).some(
          (p) =>
            p.playerId === "B" &&
            (p.cooldownPreview || []).some((c) => c.cardId === "backstab"),
        ),
      );
    } finally {
      await pageA.close();
      await pageB.close();
    }
  });
});
