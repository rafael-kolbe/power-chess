# Reference — files and terminology

## Expert scope (semantics)

Same mechanical effect → same phrasing pattern where possible. Different effects → wording must **not** blur distinctions (who chooses, “may” vs “must”, windows like “this turn” vs “until end of turn”). In pt-BR, preserve those distinctions with the **same** fixed terms as in English (alvo, ignição, negar, etc.), not loose synonyms per card.

## Files to touch by area

| Area | Canonical EN (server) | Docs | Web EN | Web pt-BR |
|------|----------------------|------|--------|-----------|
| Cards | `internal/gameplay/cards.go` | `Cards.md` | `web/cards-catalog.js` → `EN` | `web/cards-catalog.js` → `PT` |
| Player skills | `internal/gameplay/player_skills.go` | `PlayerSkills.md` | Mirror in docs; add UI module if skills get a catalog later | Same pattern as cards when exposed |
| Menus / UI chrome | — | — | `web/app.js` → `i18n["en-US"]` | `web/app.js` → `i18n["pt-BR"]` |
| Protocol strings | `PROTOCOL.md` | — | — | — |

Search helpers:

```bash
rg -n "Description:|Example:" internal/gameplay/
rg -n "i18n" web/app.js
```

## pt-BR TCG term alignment (reuse consistently)

| English (rules) | pt-BR (preferred) |
|-----------------|-------------------|
| Target (verb) | Alvo / Escolha como alvo (pick one pattern per card type and stick to it) |
| Opponent | Oponente |
| Mana | Mana |
| Ignition | Ignição |
| Ignition slot | Slot de ignição |
| Cooldown (slot) | Recarga / slot de recarga |
| Retribution | Retribuição |
| Banish | Banir |
| Hand / deck / graveyard | Mão / deck / cemitério |
| Power / Counter / Continuous (card type) | Poder / Contra / Contínua (capitalize like EN when naming types) |
| Negate | Negar |
| Reveal | Revelar |

Adjust only if `PROJECT.md` or existing copy establishes a different fixed term.

## Example sync check (cards)

For each card ID in `CARD_ROWS` inside `web/cards-catalog.js`:

- `EN[id].description ===` Go `Description` and `EN[id].example ===` Go `Example` (after normalizing escapes in JS).

If you add a script later, it can diff `InitialCardCatalog()` output against `EN`; for now, manual grep or spot-check is enough.
