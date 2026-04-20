package match

import (
	"testing"

	"power-chess/internal/chess"
	"power-chess/internal/gameplay"
)

// newDisruptionTestEngine builds a minimal engine for Disruption/Extinguish tests.
// PlayerA holds a Power card (energy-gain) to activate into ignition.
// PlayerB holds an Extinguish card (index 0) and an extra Power card (index 1) so
// it can pay the mandatory banish cost when responding during ignite_reaction.
func newDisruptionTestEngine(t *testing.T) (*Engine, *gameplay.MatchState) {
	t.Helper()

	egCard := gameplay.CardInstance{InstanceID: "eg1", CardID: CardEnergyGain, ManaCost: 0, Ignition: 1, Cooldown: 2}
	exCard := gameplay.CardInstance{InstanceID: "ex1", CardID: CardExtinguish, ManaCost: 2, Ignition: 0, Cooldown: 2}
	// Extra Power card for PlayerB to banish as Disruption reaction cost.
	ktCard := gameplay.CardInstance{InstanceID: "kt1", CardID: CardKnightTouch, ManaCost: 3, Ignition: 0, Cooldown: 2}

	state, err := gameplay.NewMatchState(testDeckWith(egCard), testDeckWith(exCard))
	if err != nil {
		t.Fatal(err)
	}

	board := chess.NewEmptyGame(chess.White)
	board.SetPiece(chess.Pos{Row: 7, Col: 4}, chess.Piece{Type: chess.King, Color: chess.White})
	board.SetPiece(chess.Pos{Row: 0, Col: 4}, chess.Piece{Type: chess.King, Color: chess.Black})

	state.Players[gameplay.PlayerA].Hand = []gameplay.CardInstance{egCard}
	state.Players[gameplay.PlayerA].Mana = 10

	// PlayerB: [Extinguish(0), KnightTouch(1)]
	state.Players[gameplay.PlayerB].Hand = []gameplay.CardInstance{exCard, ktCard}
	state.Players[gameplay.PlayerB].Mana = 10

	markInPlayForTest(state)
	return NewEngine(state, board), state
}

// TestDisruptionOwnTurnActivationSucceeds verifies that Extinguish can be activated on PlayerB's
// own turn when PlayerA has a card in their ignition slot. No banish cost applies on own turn.
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

	// PlayerB activates Extinguish on own turn — no banish cost required.
	prevMana := state.Players[gameplay.PlayerB].Mana
	if err := e.ActivateCard(gameplay.PlayerB, 0); err != nil {
		t.Fatalf("PlayerB activate extinguish: %v", err)
	}

	// Mana must have been deducted (Extinguish costs 2); no extra banish cost on own turn.
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
// the first response in an ignite_reaction window when the mandatory banish cost is paid.
// PlayerB has [Extinguish(0), KnightTouch(1)]; banishHandIndex=1 banishes KnightTouch.
func TestDisruptionQueuedInIgniteReactionSucceeds(t *testing.T) {
	e, state := newDisruptionTestEngine(t)

	// PlayerA activates energy-gain, which opens an ignite_reaction window.
	if err := e.ActivateCard(gameplay.PlayerA, 0); err != nil {
		t.Fatalf("PlayerA activate: %v", err)
	}

	if e.ReactionWindow == nil || !e.ReactionWindow.Open {
		t.Fatal("expected ignite_reaction window to be open")
	}

	handBefore := len(state.Players[gameplay.PlayerB].Hand)
	banishedBefore := len(state.Players[gameplay.PlayerB].Banished)

	// PlayerB queues Extinguish (Disruption, index 0) and banishes KnightTouch (Power, index 1).
	if err := e.QueueReactionCard(gameplay.PlayerB, 0, 1, EffectTarget{}); err != nil {
		t.Fatalf("PlayerB queue Extinguish with banish: %v", err)
	}

	if len(e.ReactionStackEntries()) != 1 {
		t.Fatalf("expected 1 card on reaction stack, got %d", len(e.ReactionStackEntries()))
	}

	// The Power card (KnightTouch) must be in the Banished zone.
	banishedAfter := state.Players[gameplay.PlayerB].Banished
	if len(banishedAfter) != banishedBefore+1 {
		t.Fatalf("expected banished count to increase by 1, was %d now %d", banishedBefore, len(banishedAfter))
	}
	if banishedAfter[len(banishedAfter)-1].CardID != CardKnightTouch {
		t.Fatalf("expected KnightTouch banished, got %s", banishedAfter[len(banishedAfter)-1].CardID)
	}

	// PlayerB's hand must have shrunk by 2 (Extinguish consumed + KnightTouch banished).
	if len(state.Players[gameplay.PlayerB].Hand) != handBefore-2 {
		t.Fatalf("expected hand size %d, got %d", handBefore-2, len(state.Players[gameplay.PlayerB].Hand))
	}
}

// TestDisruptionReactionRejectedWithoutBanishIndex verifies that a Disruption card queued as the
// first response in ignite_reaction is rejected when no banish index is provided (banishHandIndex=-1).
func TestDisruptionReactionRejectedWithoutBanishIndex(t *testing.T) {
	e, _ := newDisruptionTestEngine(t)

	if err := e.ActivateCard(gameplay.PlayerA, 0); err != nil {
		t.Fatalf("PlayerA activate: %v", err)
	}

	// Attempt to queue Extinguish without providing a banish index.
	if err := e.QueueReactionCard(gameplay.PlayerB, 0, -1, EffectTarget{}); err == nil {
		t.Fatal("expected error: disruption reaction requires banish index")
	}
}

// TestDisruptionReactionRejectedWithNonPowerBanishCard verifies that the banish cost for a
// Disruption reaction must be a Power card; non-Power cards are rejected.
func TestDisruptionReactionRejectedWithNonPowerBanishCard(t *testing.T) {
	e, state := newDisruptionTestEngine(t)

	// Replace PlayerB's Power card with a Retribution card so the banish target is non-Power.
	stopThere := gameplay.CardInstance{InstanceID: "st1", CardID: CardStopRightThere, ManaCost: 3, Ignition: 0, Cooldown: 5}
	exCard := gameplay.CardInstance{InstanceID: "ex1", CardID: CardExtinguish, ManaCost: 2, Ignition: 0, Cooldown: 2}
	state.Players[gameplay.PlayerB].Hand = []gameplay.CardInstance{exCard, stopThere}

	if err := e.ActivateCard(gameplay.PlayerA, 0); err != nil {
		t.Fatalf("PlayerA activate: %v", err)
	}

	// Attempt to banish a Retribution card as the disruption cost.
	if err := e.QueueReactionCard(gameplay.PlayerB, 0, 1, EffectTarget{}); err == nil {
		t.Fatal("expected error: banish cost must be a Power card")
	}
}

// TestDisruptionReactionRejectedBanishSameCard verifies that a player cannot specify the Disruption
// card itself as the banish target.
func TestDisruptionReactionRejectedBanishSameCard(t *testing.T) {
	e, _ := newDisruptionTestEngine(t)

	if err := e.ActivateCard(gameplay.PlayerA, 0); err != nil {
		t.Fatalf("PlayerA activate: %v", err)
	}

	// banishHandIndex == handIndex (both 0) — must be rejected.
	if err := e.QueueReactionCard(gameplay.PlayerB, 0, 0, EffectTarget{}); err == nil {
		t.Fatal("expected error: the banished card must differ from the disruption card")
	}
}

// TestDisruptionReactionBanishIndexAdjustsWhenBanishBeforeDisruption verifies that when the Power
// card to banish sits at a lower index than the Disruption card, the engine correctly adjusts the
// Disruption card's hand index after the banish.
func TestDisruptionReactionBanishIndexAdjustsWhenBanishBeforeDisruption(t *testing.T) {
	e, state := newDisruptionTestEngine(t)

	// Rearrange PlayerB's hand so Power card (KnightTouch) is at index 0 and Extinguish at index 1.
	ktCard := gameplay.CardInstance{InstanceID: "kt1", CardID: CardKnightTouch, ManaCost: 3, Ignition: 0, Cooldown: 2}
	exCard := gameplay.CardInstance{InstanceID: "ex1", CardID: CardExtinguish, ManaCost: 2, Ignition: 0, Cooldown: 2}
	state.Players[gameplay.PlayerB].Hand = []gameplay.CardInstance{ktCard, exCard}

	if err := e.ActivateCard(gameplay.PlayerA, 0); err != nil {
		t.Fatalf("PlayerA activate: %v", err)
	}

	// Queue Extinguish at index 1 and banish KnightTouch at index 0 (banishHandIndex < handIndex).
	if err := e.QueueReactionCard(gameplay.PlayerB, 1, 0, EffectTarget{}); err != nil {
		t.Fatalf("queue disruption with banish before disruption: %v", err)
	}

	if len(e.ReactionStackEntries()) != 1 {
		t.Fatalf("expected 1 card on reaction stack, got %d", len(e.ReactionStackEntries()))
	}

	// KnightTouch must be in the Banished zone.
	if len(state.Players[gameplay.PlayerB].Banished) != 1 || state.Players[gameplay.PlayerB].Banished[0].CardID != CardKnightTouch {
		t.Fatal("expected KnightTouch in Banished zone")
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
	if err := e.QueueReactionCard(gameplay.PlayerB, 0, -1, EffectTarget{}); err == nil {
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

	// PlayerB queues Counterattack (Counter) as the first reaction — no banish needed for Counter.
	if err := e.QueueReactionCard(gameplay.PlayerB, 0, -1, EffectTarget{}); err != nil {
		t.Fatalf("PlayerB queue Counterattack: %v", err)
	}

	// PlayerA now tries to queue Extinguish (Disruption) — must be rejected because the top of
	// the stack is a Counter and only Counter cards can follow Counter.
	if err := e.QueueReactionCard(gameplay.PlayerA, 0, -1, EffectTarget{}); err == nil {
		t.Fatal("expected error: Disruption cannot respond to Counter cards")
	}
}
