package match

import (
	"testing"

	"power-chess/internal/chess"
	"power-chess/internal/gameplay"
)

// newDisruptionTestEngine builds a minimal engine for Disruption/Extinguish tests.
// PlayerA holds a Power card (energy-gain) in hand to activate into ignition.
// PlayerB holds an Extinguish card in hand.
func newDisruptionTestEngine(t *testing.T) (*Engine, *gameplay.MatchState) {
	t.Helper()

	egCard := gameplay.CardInstance{InstanceID: "eg1", CardID: CardEnergyGain, ManaCost: 0, Ignition: 1, Cooldown: 2}
	exCard := gameplay.CardInstance{InstanceID: "ex1", CardID: CardExtinguish, ManaCost: 2, Ignition: 0, Cooldown: 2}

	state, err := gameplay.NewMatchState(testDeckWith(egCard), testDeckWith(exCard))
	if err != nil {
		t.Fatal(err)
	}

	board := chess.NewEmptyGame(chess.White)
	board.SetPiece(chess.Pos{Row: 7, Col: 4}, chess.Piece{Type: chess.King, Color: chess.White})
	board.SetPiece(chess.Pos{Row: 0, Col: 4}, chess.Piece{Type: chess.King, Color: chess.Black})

	state.Players[gameplay.PlayerA].Hand = []gameplay.CardInstance{egCard}
	state.Players[gameplay.PlayerA].Mana = 10

	state.Players[gameplay.PlayerB].Hand = []gameplay.CardInstance{exCard}
	state.Players[gameplay.PlayerB].Mana = 10

	markInPlayForTest(state)
	return NewEngine(state, board), state
}

// TestDisruptionOwnTurnActivationSucceeds verifies that Extinguish can be activated on PlayerB's
// own turn when PlayerA has a card in their ignition slot.
func TestDisruptionOwnTurnActivationSucceeds(t *testing.T) {
	e, state := newDisruptionTestEngine(t)

	// PlayerA activates energy-gain, opening an ignite_reaction window.
	if err := e.ActivateCard(gameplay.PlayerA, 0); err != nil {
		t.Fatalf("PlayerA activate energy-gain: %v", err)
	}
	if e.State.Players[gameplay.PlayerA].Ignition.Occupied == false {
		t.Fatal("expected PlayerA's ignition to be occupied after activation")
	}

	// PlayerB resolves the window without queuing anything.
	if err := e.ResolveReactionStack(); err != nil {
		t.Fatalf("resolve reaction: %v", err)
	}

	// PlayerA moves; now it is PlayerB's turn.
	if err := e.SubmitMove(gameplay.PlayerA, chess.Move{
		From: chess.Pos{Row: 7, Col: 4},
		To:   chess.Pos{Row: 7, Col: 3},
	}); err != nil {
		t.Fatalf("PlayerA move: %v", err)
	}

	if state.CurrentTurn != gameplay.PlayerB {
		t.Fatal("expected PlayerB's turn")
	}

	// PlayerA's ignition still occupied (Ignition=1, resolves on PlayerA's next start-of-turn).
	if !state.Players[gameplay.PlayerA].Ignition.Occupied {
		t.Fatal("expected PlayerA ignition to still be occupied")
	}

	// PlayerB activates Extinguish — card enters ignition; same-turn finish resolves to cooldown.
	prevMana := state.Players[gameplay.PlayerB].Mana
	if err := e.ActivateCard(gameplay.PlayerB, 0); err != nil {
		t.Fatalf("PlayerB activate extinguish: %v", err)
	}

	// Mana must have been deducted (Extinguish costs 2).
	exDef, _ := gameplay.CardDefinitionByID(CardExtinguish)
	if state.Players[gameplay.PlayerB].Mana != prevMana-exDef.Cost {
		t.Fatalf("expected mana to drop by %d, was %d, now %d", exDef.Cost, prevMana, state.Players[gameplay.PlayerB].Mana)
	}

	if !e.HasPendingDisruptionSameTurnResolve() {
		t.Fatal("expected pending same-turn disruption resolve after placing in ignition")
	}
	if !state.Players[gameplay.PlayerB].Ignition.Occupied {
		t.Fatal("expected PlayerB's ignition to hold Extinguish before finish")
	}
	if state.Players[gameplay.PlayerB].Ignition.Card.CardID != CardExtinguish {
		t.Fatalf("expected ignition card %s, got %s", CardExtinguish, state.Players[gameplay.PlayerB].Ignition.Card.CardID)
	}

	if err := e.FinishDisruptionSameTurnResolveIfPending(); err != nil {
		t.Fatalf("FinishDisruptionSameTurnResolveIfPending: %v", err)
	}
	if e.HasPendingDisruptionSameTurnResolve() {
		t.Fatal("expected pending flag cleared after finish")
	}
	if state.Players[gameplay.PlayerB].Ignition.Occupied {
		t.Fatal("expected PlayerB ignition cleared after resolution")
	}
	if !state.Players[gameplay.PlayerA].Ignition.Occupied {
		t.Fatal("expected PlayerA ignition still occupied (Energy Gain)")
	}
	if !state.Players[gameplay.PlayerA].Ignition.EffectNegated {
		t.Fatal("expected PlayerA ignition marked negated by Extinguish")
	}
}

// TestDisruptionOwnTurnRejectedWhenOpponentIgnitionEmpty verifies that Extinguish cannot be
// activated when the opponent has no card in their ignition slot.
func TestDisruptionOwnTurnRejectedWhenOpponentIgnitionEmpty(t *testing.T) {
	e, state := newDisruptionTestEngine(t)

	// PlayerA has not activated any card; their ignition is empty.
	// It's PlayerA's turn — skip to PlayerB's turn first.
	if err := e.SubmitMove(gameplay.PlayerA, chess.Move{
		From: chess.Pos{Row: 7, Col: 4},
		To:   chess.Pos{Row: 7, Col: 3},
	}); err != nil {
		t.Fatalf("PlayerA move: %v", err)
	}

	if state.CurrentTurn != gameplay.PlayerB {
		t.Fatal("expected PlayerB's turn")
	}

	// Attempt to activate Extinguish when PlayerA's ignition is empty.
	if err := e.ActivateCard(gameplay.PlayerB, 0); err == nil {
		t.Fatal("expected error when activating Disruption with opponent ignition empty")
	}
}

// TestDisruptionNegateOpponentIgnitionWorks verifies NegateOpponentIgnition clears the opponent slot
// (e.g. some Retribution effects). Extinguish uses MarkOpponentCardEffectNegated instead.
func TestDisruptionNegateOpponentIgnitionWorks(t *testing.T) {
	e, state := newDisruptionTestEngine(t)

	// PlayerA activates energy-gain to put a card in ignition.
	if err := e.ActivateCard(gameplay.PlayerA, 0); err != nil {
		t.Fatalf("PlayerA activate: %v", err)
	}
	if err := e.ResolveReactionStack(); err != nil {
		t.Fatalf("resolve stack: %v", err)
	}

	// Verify NegateOpponentIgnition clears the slot as expected.
	if err := e.NegateOpponentIgnition(gameplay.PlayerA); err != nil {
		t.Fatalf("NegateOpponentIgnition: %v", err)
	}

	if state.Players[gameplay.PlayerA].Ignition.Occupied {
		t.Fatal("expected PlayerA ignition to be cleared after NegateOpponentIgnition")
	}
}

func TestMarkOpponentCardEffectNegatedKeepsSlot(t *testing.T) {
	e, state := newDisruptionTestEngine(t)
	if err := e.ActivateCard(gameplay.PlayerA, 0); err != nil {
		t.Fatalf("PlayerA activate: %v", err)
	}
	if err := e.ResolveReactionStack(); err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if err := e.MarkOpponentCardEffectNegated(gameplay.PlayerA); err != nil {
		t.Fatalf("Mark: %v", err)
	}
	if !state.Players[gameplay.PlayerA].Ignition.Occupied {
		t.Fatal("ignition should remain occupied")
	}
	if !state.Players[gameplay.PlayerA].Ignition.EffectNegated {
		t.Fatal("EffectNegated should be true")
	}
}

// TestDisruptionQueuedInIgniteReactionSucceeds verifies that Extinguish can be queued as
// the first response in an ignite_reaction window.
func TestDisruptionQueuedInIgniteReactionSucceeds(t *testing.T) {
	e, _ := newDisruptionTestEngine(t)

	// PlayerA activates energy-gain, which opens an ignite_reaction window.
	if err := e.ActivateCard(gameplay.PlayerA, 0); err != nil {
		t.Fatalf("PlayerA activate: %v", err)
	}

	if e.ReactionWindow == nil || !e.ReactionWindow.Open {
		t.Fatal("expected ignite_reaction window to be open")
	}

	// PlayerB queues Extinguish (Disruption) as the first reaction.
	if err := e.QueueReactionCard(gameplay.PlayerB, 0, EffectTarget{}); err != nil {
		t.Fatalf("PlayerB queue Extinguish: %v", err)
	}

	if len(e.ReactionStackEntries()) != 1 {
		t.Fatalf("expected 1 card on reaction stack, got %d", len(e.ReactionStackEntries()))
	}
}

// TestDisruptionRejectedInCaptureAttemptWindow verifies that Extinguish cannot be played
// in a capture_attempt reaction window (Disruption is not an eligible type there).
func TestDisruptionRejectedInCaptureAttemptWindow(t *testing.T) {
	e, state := newDisruptionTestEngine(t)

	// Set up a capture_attempt window manually.
	e.OpenReactionWindow("capture_attempt", gameplay.PlayerA, []gameplay.CardType{gameplay.CardTypeCounter})

	// PlayerB tries to queue Extinguish in a capture_attempt window.
	state.Players[gameplay.PlayerB].Mana = 10
	if err := e.QueueReactionCard(gameplay.PlayerB, 0, EffectTarget{}); err == nil {
		t.Fatal("expected error when queuing Disruption in capture_attempt window")
	}
}

// TestDisruptionCannotFollowCounter verifies that Extinguish (Disruption) cannot be queued
// as a response to a Counter card in the reaction stack.
func TestDisruptionCannotFollowCounter(t *testing.T) {
	// Re-use newDisruptionTestEngine; manually override hands after setup.
	e, state := newDisruptionTestEngine(t)

	caCard := gameplay.CardInstance{InstanceID: "ca1", CardID: "counterattack", ManaCost: 1, Ignition: 0, Cooldown: 6}
	exCard := gameplay.CardInstance{InstanceID: "ex1", CardID: CardExtinguish, ManaCost: 2, Ignition: 0, Cooldown: 2}
	egCard := gameplay.CardInstance{InstanceID: "eg1", CardID: CardEnergyGain, ManaCost: 0, Ignition: 1, Cooldown: 2}

	state.Players[gameplay.PlayerA].Hand = []gameplay.CardInstance{egCard}
	state.Players[gameplay.PlayerB].Hand = []gameplay.CardInstance{caCard, exCard}
	state.Players[gameplay.PlayerA].Mana = 10
	state.Players[gameplay.PlayerB].Mana = 10

	// PlayerA activates energy-gain, opening ignite_reaction.
	if err := e.ActivateCard(gameplay.PlayerA, 0); err != nil {
		t.Fatalf("PlayerA activate: %v", err)
	}

	if e.ReactionWindow == nil || !e.ReactionWindow.Open {
		t.Fatal("expected ignite_reaction window open")
	}

	// Add Counter to eligible types so PlayerB can queue it (capture flag not set, but we force it).
	e.ReactionWindow.EligibleTypes = append(e.ReactionWindow.EligibleTypes, gameplay.CardTypeCounter)

	// PlayerB queues Counterattack (Counter) as the first reaction.
	if err := e.QueueReactionCard(gameplay.PlayerB, 0, EffectTarget{}); err != nil {
		t.Fatalf("PlayerB queue Counterattack: %v", err)
	}

	// PlayerA now tries to queue Extinguish (Disruption) — must be rejected because the top of
	// the stack is a Counter and only Counter cards can follow Counter.
	if err := e.QueueReactionCard(gameplay.PlayerA, 0, EffectTarget{}); err == nil {
		t.Fatal("expected error: Disruption cannot respond to Counter cards")
	}
}
