/**
 * UI-only flags for local layout / playmat testing. Safe to edit; does not talk to the server.
 * Loaded as an ES module by `app.js` (see `index.html` type="module").
 */

/** @type {boolean} When true, the match playmat renders fake zone data for CSS tuning. */
export const PLAYMAT_UI_TEST_OVERLAY = false;
