package match

import (
	"testing"

	"power-chess/internal/chess"
	"power-chess/internal/gameplay"
)

// newManaBurnTestEngine builds a minimal engine for Mana Burn tests.
// PlayerA holds an Energy Gain card with ManaCost=3 to activate into ignition.
// Using Energy Gain (Ignition: 1, Targets: 0) ensures the ignite_reaction window
// opens immediately after ActivateCard, so PlayerB can queue Mana Burn as retribution.
// ManaCost is set to 3 on the instance so Mana Burn burns 3.
func newManaBurnTestEngine(t *testing.T) (*Engine, *gameplay.MatchState) {
	t.Helper()

	// Energy Gain with ManaCost=3: no targets, ignition=1, reaction window opens immediately.
	egCard := gameplay.CardInstance{InstanceID: "eg1", CardID: CardEnergyGain, ManaCost: 3, Ignition: 1, Cooldown: 2}
	mbCard := gameplay.CardInstance{InstanceID: "mb1", CardID: CardManaBurn, ManaCost: 1, Ignition: 0, Cooldown: 3}

	state, err := gameplay.NewMatchState(testDeckWith(egCard), testDeckWith(mbCard))
	if err != nil {
		t.Fatal(err)
	}

	board := chess.NewEmptyGame(chess.White)
	board.SetPiece(chess.Pos{Row: 7, Col: 4}, chess.Piece{Type: chess.King, Color: chess.White})
	board.SetPiece(chess.Pos{Row: 0, Col: 4}, chess.Piece{Type: chess.King, Color: chess.Black})

	state.Players[gameplay.PlayerA].Hand = []gameplay.CardInstance{egCard}
	state.Players[gameplay.PlayerA].Mana = 10

	state.Players[gameplay.PlayerB].Hand = []gameplay.CardInstance{mbCard}
	state.Players[gameplay.PlayerB].Mana = 10

	markInPlayForTest(state)
	return NewEngine(state, board), state
}

// TestManaBurnBurnsFromRegularManaOnly verifies that Mana Burn drains only the regular mana pool
// when the opponent has enough mana to cover the full ignition card cost.
func TestManaBurnBurnsFromRegularManaOnly(t *testing.T) {
	e, state := newManaBurnTestEngine(t)

	// PlayerA activates with 8 mana → pays 3 → 5 remain; 3 go to energized pool.
	state.Players[gameplay.PlayerA].Mana = 8

	if err := e.ActivateCard(gameplay.PlayerA, 0); err != nil {
		t.Fatalf("PlayerA activate: %v", err)
	}
	// After activation: Mana = 5, EnergizedMana = 3.

	if err := e.QueueReactionCard(gameplay.PlayerB, 0, -1, EffectTarget{}); err != nil {
		t.Fatalf("PlayerB queue mana-burn: %v", err)
	}
	if err := e.ResolveReactionStack(); err != nil {
		t.Fatalf("resolve reactions: %v", err)
	}
	// Compatibility no-op (burns are applied immediately now).
	e.FlushPendingManaBurns()

	// Mana Burn burns 3 (ignition card ManaCost). All from regular pool: 5 → 2.
	if got := state.Players[gameplay.PlayerA].Mana; got != 2 {
		t.Errorf("PlayerA Mana: want 2, got %d", got)
	}
	if got := state.Players[gameplay.PlayerA].EnergizedMana; got != 3 {
		t.Errorf("PlayerA EnergizedMana: want 3 (untouched), got %d", got)
	}
}

// TestManaBurnBurnsFromBothPools verifies the overflow path: when the opponent's regular mana
// is less than the ignition card cost, the remainder drains from the energized pool.
func TestManaBurnBurnsFromBothPools(t *testing.T) {
	e, state := newManaBurnTestEngine(t)

	// PlayerA activates with exactly 3 mana → pays 3 → 0 remain; 3 go to energized pool.
	state.Players[gameplay.PlayerA].Mana = 3

	if err := e.ActivateCard(gameplay.PlayerA, 0); err != nil {
		t.Fatalf("PlayerA activate: %v", err)
	}
	// After activation: Mana = 0, EnergizedMana = 3.

	if err := e.QueueReactionCard(gameplay.PlayerB, 0, -1, EffectTarget{}); err != nil {
		t.Fatalf("PlayerB queue mana-burn: %v", err)
	}
	if err := e.ResolveReactionStack(); err != nil {
		t.Fatalf("resolve reactions: %v", err)
	}
	// Compatibility no-op (burns are applied immediately now).
	e.FlushPendingManaBurns()

	// Mana Burn burns 3. Regular pool is 0 → all 3 come from energized (3 → 0).
	if got := state.Players[gameplay.PlayerA].Mana; got != 0 {
		t.Errorf("PlayerA Mana: want 0, got %d", got)
	}
	if got := state.Players[gameplay.PlayerA].EnergizedMana; got != 0 {
		t.Errorf("PlayerA EnergizedMana: want 0, got %d", got)
	}
}

// TestManaBurnOverflowPartial verifies overflow when the opponent has 1 regular mana and burn=3:
// 1 from regular, 2 from energized.
func TestManaBurnOverflowPartial(t *testing.T) {
	e, state := newManaBurnTestEngine(t)

	// PlayerA: 4 mana → pays 3 → 1 regular, 3 energized.
	state.Players[gameplay.PlayerA].Mana = 4

	if err := e.ActivateCard(gameplay.PlayerA, 0); err != nil {
		t.Fatalf("PlayerA activate: %v", err)
	}

	if err := e.QueueReactionCard(gameplay.PlayerB, 0, -1, EffectTarget{}); err != nil {
		t.Fatalf("PlayerB queue mana-burn: %v", err)
	}
	if err := e.ResolveReactionStack(); err != nil {
		t.Fatalf("resolve reactions: %v", err)
	}
	// Compatibility no-op (burns are applied immediately now).
	e.FlushPendingManaBurns()

	// Burn 3: 1 from regular (→0), 2 from energized (3→1).
	if got := state.Players[gameplay.PlayerA].Mana; got != 0 {
		t.Errorf("PlayerA Mana: want 0, got %d", got)
	}
	if got := state.Players[gameplay.PlayerA].EnergizedMana; got != 1 {
		t.Errorf("PlayerA EnergizedMana: want 1, got %d", got)
	}
}

// TestManaBurnDoesNotAffectOwner verifies that Mana Burn burns only the opponent's mana,
// not the Mana Burn owner's own pools (beyond the activation cost already paid).
func TestManaBurnDoesNotAffectOwner(t *testing.T) {
	e, state := newManaBurnTestEngine(t)

	state.Players[gameplay.PlayerA].Mana = 10
	state.Players[gameplay.PlayerB].Mana = 6

	if err := e.ActivateCard(gameplay.PlayerA, 0); err != nil {
		t.Fatalf("PlayerA activate: %v", err)
	}
	// PlayerB pays 1 mana to queue Mana Burn.
	if err := e.QueueReactionCard(gameplay.PlayerB, 0, -1, EffectTarget{}); err != nil {
		t.Fatalf("PlayerB queue mana-burn: %v", err)
	}
	if err := e.ResolveReactionStack(); err != nil {
		t.Fatalf("resolve reactions: %v", err)
	}
	// Compatibility no-op (burns are applied immediately now).
	e.FlushPendingManaBurns()

	// PlayerB's mana should be 5 (paid 1 activation cost, no further drain).
	if got := state.Players[gameplay.PlayerB].Mana; got != 5 {
		t.Errorf("PlayerB Mana: want 5, got %d", got)
	}
	if got := state.Players[gameplay.PlayerB].EnergizedMana; got != 1 {
		t.Errorf("PlayerB EnergizedMana after burn: want 1 (from mana-burn cost), got %d", got)
	}
}
