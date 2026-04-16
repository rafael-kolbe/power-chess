---
name: power-chess-tcg-copy-review
description: >-
  Acts as the dedicated expert for Power Chess player-facing text: English and
  pt-BR grammar, TCG rules phrasing, and semantic clarity (effects, menus,
  skills, i18n). Syncs canonical English from Go to docs and web, updates
  Portuguese with consistent terminology. Use when the user wants a copy
  review, grammar pass, semantic/rules-text audit, terminology alignment, or
  invokes this skill by name.
---

# Power Chess — expert copy, grammar, and TCG semantics

## Role

When this skill applies, treat **all** work in this area as **specialist responsibility**:

- **Grammar and style** (EN + pt-BR): idiomatic, precise, no accidental tone drift.
- **TCG semantics**: triggers, targets, timing words (**When** / **While** / **until**), and **parallel wording** across cards/skills so players infer rules the same way everywhere.
- **Terminology lock**: one English term and one pt-BR equivalent per concept across cards, skills, menus, and examples (see [reference.md](reference.md)).
- **Cross-surface consistency**: server strings, markdown, `card-metadata.gen.js` (generated), `cards-catalog.js` (PT), `app.js` i18n, and any other visible copy stay aligned after edits.

Do not defer to “generic editing”: **own** clarity of rules-as-text end to end.

## Goal

Keep **effect text**, **menus**, and **examples** consistent, **TCG-literate**, **semantically unambiguous**, and **grammatically sound** across:

- **English (canonical for rules)**: Go structs, markdown docs, UI strings that define rules.
- **Portuguese (pt-BR)**: frontend only; must reuse the same **canonical TCG terms** as English (alvo, ignição, slot de recarga, banir, retribuição, mana, oponente, etc.).

## Source-of-truth order

1. **`internal/gameplay/cards.go`** — `InitialCardCatalog()`: `Name`, `Description`, `Example` for cards.
2. **`internal/gameplay/player_skills.go`** — `InitialPlayerSkills()`: `Name`, `Description`, `Example` for player skills.
3. **`Cards.md`** / **`PlayerSkills.md`** — must mirror the Go English for those fields (project convention).
4. **`web/cards-catalog.js`** — only **`PT`** (Portuguese). English name/description/example come from **`web/card-metadata.gen.js`**, generated from Go (`go run ./cmd/export-card-metadata`).
5. **`web/app.js`** — `i18n["en-US"]` and `i18n["pt-BR"]` for menus, banners, buttons, labels (not duplicated rules text from cards unless intentionally surfaced in UI).

If English rules text changes in Go, regenerate **`web/card-metadata.gen.js`**, update **markdown**, and adjust **`PT`** in `cards-catalog.js` when the meaning shifts.

## TCG copy norms (English) + semantics

- Prefer rules language: **Target**, **Choose**, **Negate**, **Banish**, **While …**, **When …**, **where X is …** when it matches the engine.
- Use one term per concept: e.g. **ignition slot**, **cooldown slot**, **retribution** (lowercase in prose when not a card type name), **Power** / **Counter** / **Continuous** when referring to card types with capital as in rules.
- **Semantics**: if two effects work the same way, use the **same sentence pattern**; if they differ, wording must **expose** the difference (costs, timing, who chooses, what can be targeted).
- Avoid comma splices; fix **that/which** agreement; **a/an** before vowel sounds.
- Card names in examples: quoted as in data (`"Knight Touch"`).

See [reference.md](reference.md) for file paths and a **term alignment** checklist (EN ↔ pt-BR).

## Workflow (agent checklist)

1. **Inventory**
   - Cards: `cards.go` ↔ `Cards.md` ↔ `web/card-metadata.gen.js` (EN, generated) ↔ `web/cards-catalog.js` (PT only).
   - Player skills: `player_skills.go` ↔ `PlayerSkills.md` ↔ any UI that shows skill text (search `PlayerSkill`, `player_skills`, skill IDs).
   - Site chrome: `web/app.js` `i18n` blocks; `web/index.html` visible copy; other `web/*.js` with user-facing strings.

2. **English pass**
   - Fix grammar, **ambiguous phrasing**, and unclear TCG structure in **Go first** (authoritative).
   - Mirror English edits into **markdown**. After any `cards.go` catalog change (text, costs, type, order), run **`go run ./cmd/export-card-metadata`** and commit **`web/card-metadata.gen.js`**.
   - For menus-only strings, edit **`app.js` en-US** (and mirror pt-BR).

3. **Portuguese pass**
   - Update **`PT`** in `cards-catalog.js` so each card matches the **meaning and nuance** of the English; **reuse** the same Portuguese TCG vocabulary across cards and skills (not word-for-word translation when it breaks TCG idiom).
   - Update **`pt-BR`** in `app.js` for any changed menu strings; keep **register** consistent (e.g. formal UI tone) unless existing copy establishes otherwise.

4. **Preserve canonical user text**
   - If the user supplied exact card or UI copy, **do not** paraphrase unless they asked for an edit; otherwise follow `.cursor/rules/power-chess-standards.mdc`.

5. **Verify**
   - `go test ./...`
   - `go test ./...`

## When the user invokes this skill

Assume they want the **full pipeline**: diff-based consistency check, grammar fixes, terminology alignment, **all** affected files, and tests green—unless they narrow scope (e.g. “only player skills” or “only lobby strings”).

## Out of scope

- Changing game logic or costs unless the user explicitly ties copy to a rule change.
- Rewriting `PROJECT.md` unless gameplay meaning changes (per project rules).
