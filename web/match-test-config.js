/**
 * Match debug: payload for WebSocket `debug_match_fixture` (requires server ADMIN_DEBUG_MATCH).
 * Decks must be exactly 20 legal constructed cards per side (same rules as deck builder).
 *
 * AUTO_APPLY runs once on the first snapshot where the second player has just joined (both connected).
 * The server applies state or ignores; no client-side timing hacks.
 * Optional AUTO_CONFIRM_MULLIGAN sends `confirm_mulligan` on the next snapshot after the fixture (this client only).
 *
 * Runtime override (this browser tab only), e.g. in DevTools console before the second player joins:
 *   __powerChessMatchTest.autoApply = true
 *   __powerChessMatchTest.autoConfirmMulligan = true
 * When a property is omitted or not a boolean, the file defaults below apply.
 */

/** Default when `__powerChessMatchTest` does not override (see {@link matchTestAutoApplyEnabled}). */
export const MATCH_TEST_AUTO_APPLY = false;

/** Default when `__powerChessMatchTest` does not override (see {@link matchTestAutoConfirmMulliganEnabled}). */
export const MATCH_TEST_AUTO_CONFIRM_MULLIGAN = false;

if (typeof globalThis !== "undefined" && globalThis.__powerChessMatchTest === undefined) {
  globalThis.__powerChessMatchTest = {};
}

/**
 * @returns {boolean} Whether to auto-send `debug_match_fixture` (console or file default).
 */
export function matchTestAutoApplyEnabled() {
  const o = globalThis.__powerChessMatchTest;
  if (o && typeof o.autoApply === "boolean") {
    return o.autoApply;
  }
  return MATCH_TEST_AUTO_APPLY;
}

/**
 * @returns {boolean} Whether to auto-send `confirm_mulligan` after the fixture (console or file default).
 */
export function matchTestAutoConfirmMulliganEnabled() {
  const o = globalThis.__powerChessMatchTest;
  if (o && typeof o.autoConfirmMulligan === "boolean") {
    return o.autoConfirmMulligan;
  }
  return MATCH_TEST_AUTO_CONFIRM_MULLIGAN;
}

/**
 * Default 20-card preset (order = deck order), mirrors server `DefaultDeckPresetCardIDs`.
 * @type {readonly string[]}
 */
export const DEFAULT_DECK_20 = Object.freeze([
  "piece-swap",
  "energy-gain",
  "energy-gain",
  "knight-touch",
  "knight-touch",
  "bishop-touch",
  "bishop-touch",
  "rook-touch",
  "rook-touch",
  "sacrifice-of-the-masses",
  "backstab",
  "double-turn",
  "extinguish",
  "extinguish",
  "clairvoyance",
  "save-it-for-later",
  "retaliate",
  "retaliate",
  "mana-burn",
  "counterattack",
]);

/**
 * Builds the full `debug_match_fixture` payload (spread into send()).
 * Edit hands / mana below for your scenario; keep decks as valid 20-card lists.
 *
 * @returns {{ test_environment: boolean, white: object, black: object }}
 */
export function buildMatchDebugFixturePayload() {
  return {
    test_environment: true,
    white: {
      deck: [...DEFAULT_DECK_20],
      hand: ["piece-swap", "double-turn", "extinguish"],
      mana: 8,
      maxMana: 10,
      energizedMana: 0,
      maxEnergized: 20,
    },
    black: {
      deck: [...DEFAULT_DECK_20],
      hand: ["knight-touch", "clairvoyance", "mana-burn"],
      mana: 8,
      maxMana: 10,
      energizedMana: 0,
      maxEnergized: 20,
    },
  };
}
