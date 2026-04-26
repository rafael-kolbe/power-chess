package power_test

import (
	"testing"

	"power-chess/internal/chess"
	"power-chess/internal/gameplay"
	"power-chess/internal/match"
	matchresolvers "power-chess/internal/match/resolvers"
)

// newArchmageTestEngine builds a minimal engine with Archmage Arsenal in Player A's hand.
// Player A's deck contains a mix of Power and non-Power cards so eligibility filtering can be tested.
func newArchmageTestEngine(t *testing.T, deckCards []gameplay.CardInstance) *match.Engine {
	t.Helper()
	archmage := gameplay.CardInstance{
		InstanceID: "aa1",
		CardID:     match.CardArchmageArsenal,
		ManaCost:   1,
		Ignition:   0,
		Cooldown:   2,
	}
	// Build a 20-card deck: provided cards first, then archmage, then fillers.
	deck := make([]gameplay.CardInstance, 0, gameplay.DefaultDeckSize)
	deck = append(deck, deckCards...)
	deck = append(deck, archmage) // also in deck so shuffle tests can confirm it is excluded
	for len(deck) < gameplay.DefaultDeckSize {
		deck = append(deck, gameplay.CardInstance{
			InstanceID: "filler",
			CardID:     "filler",
			ManaCost:   1,
			Ignition:   0,
			Cooldown:   1,
		})
	}
	state, err := gameplay.NewMatchState(deck, testDeckWith(archmage))
	if err != nil {
		t.Fatal(err)
	}
	// Give Player A the archmage card in hand; deck has the rest.
	state.Players[gameplay.PlayerA].Hand = []gameplay.CardInstance{archmage}
	state.Players[gameplay.PlayerA].Mana = 10
	board := chess.NewEmptyGame(chess.White)
	board.SetPiece(chess.Pos{Row: 7, Col: 4}, chess.Piece{Type: chess.King, Color: chess.White})
	board.SetPiece(chess.Pos{Row: 0, Col: 4}, chess.Piece{Type: chess.King, Color: chess.Black})
	board.SetPiece(chess.Pos{Row: 1, Col: 4}, chess.Piece{Type: chess.Pawn, Color: chess.Black})
	e := match.NewEngine(state, board)
	markInPlayForTest(state)
	return e
}

// powerCard3Mana returns an eligible deck card: Power, cost 3.
func powerCard3Mana() gameplay.CardInstance {
	return gameplay.CardInstance{
		InstanceID: "kt1",
		CardID:     "knight-touch",
		ManaCost:   3,
		Ignition:   0,
		Cooldown:   2,
	}
}

// powerCard4Mana returns an ineligible deck card: Power, cost 4 (above limit).
func powerCard4Mana() gameplay.CardInstance {
	return gameplay.CardInstance{
		InstanceID: "eg1",
		CardID:     "zip-line",
		ManaCost:   4,
		Ignition:   0,
		Cooldown:   4,
	}
}

// retributionCard returns an ineligible deck card: Retribution type.
func retributionCard() gameplay.CardInstance {
	return gameplay.CardInstance{
		InstanceID: "mb1",
		CardID:     "mana-burn",
		ManaCost:   1,
		Ignition:   0,
		Cooldown:   3,
	}
}

func activateArchmageAndCloseReaction(t *testing.T, e *match.Engine) {
	t.Helper()
	if err := e.ActivateCard(gameplay.PlayerA, 0); err != nil {
		t.Fatalf("activate archmage-arsenal: %v", err)
	}
	if err := e.ResolveReactionStack(); err != nil {
		t.Fatalf("close ignite reaction: %v", err)
	}
}

// TestArchmageArsenalQueuesPendingWithEligibleChoices verifies that when Archmage Arsenal resolves,
// a pending deck-search effect is created with only the Power cards costing <= 3 (except Archmage itself).
func TestArchmageArsenalQueuesPendingWithEligibleChoices(t *testing.T) {
	deckCards := []gameplay.CardInstance{powerCard3Mana(), powerCard4Mana(), retributionCard()}
	e := newArchmageTestEngine(t, deckCards)
	activateArchmageAndCloseReaction(t, e)

	pending := e.PendingEffects()
	if len(pending) != 1 {
		t.Fatalf("expected 1 pending effect, got %d", len(pending))
	}
	pe := pending[0]
	if pe.CardID != match.CardArchmageArsenal {
		t.Fatalf("expected archmage-arsenal pending, got %s", pe.CardID)
	}
	// Only knight-touch (Power, cost 3) should be in choices; zip-line (cost 4) and mana-burn (Retribution) excluded.
	// archmage-arsenal itself must also be excluded.
	choices := pe.DeckSearchChoices
	if len(choices) != 1 {
		t.Fatalf("expected 1 eligible choice, got %d: %v", len(choices), choices)
	}
	if choices[0].CardID != "knight-touch" {
		t.Fatalf("expected knight-touch in choices, got %s", choices[0].CardID)
	}
}

// TestArchmageArsenalMovesDeckCardToHand verifies that resolving the pending effect with a valid card ID
// moves that card from deck to hand.
func TestArchmageArsenalMovesDeckCardToHand(t *testing.T) {
	deckCards := []gameplay.CardInstance{powerCard3Mana()}
	e := newArchmageTestEngine(t, deckCards)
	deckSizeBefore := len(e.State.Players[gameplay.PlayerA].Deck)
	handSizeBefore := len(e.State.Players[gameplay.PlayerA].Hand)

	activateArchmageAndCloseReaction(t, e)
	// Hand had 1 archmage which went to ignition: hand should be 0 now.
	// After pending resolved, hand should gain the chosen card.
	cardID := gameplay.CardID("knight-touch")
	if err := e.ResolvePendingEffect(gameplay.PlayerA, match.EffectTarget{TargetCard: &cardID}); err != nil {
		t.Fatalf("resolve pending: %v", err)
	}
	if len(e.PendingEffects()) != 0 {
		t.Fatal("expected pending queue cleared")
	}
	hand := e.State.Players[gameplay.PlayerA].Hand
	if len(hand) != handSizeBefore { // hand was 1 (archmage) → 0 after ignition → 1 after search
		t.Fatalf("expected hand size %d, got %d", handSizeBefore, len(hand))
	}
	if hand[0].CardID != "knight-touch" {
		t.Fatalf("expected knight-touch in hand, got %s", hand[0].CardID)
	}
	if len(e.State.Players[gameplay.PlayerA].Deck) != deckSizeBefore-1 {
		t.Fatalf("expected deck to shrink by 1")
	}
}

// TestArchmageArsenalEmptyListResolvesAsSuccess verifies that when no legal targets exist in the deck,
// resolving with a nil TargetCard (empty-modal confirmation) succeeds without moving any cards.
func TestArchmageArsenalEmptyListResolvesAsSuccess(t *testing.T) {
	// Deck has only ineligible cards (cost > 3 or wrong type).
	deckCards := []gameplay.CardInstance{powerCard4Mana(), retributionCard()}
	e := newArchmageTestEngine(t, deckCards)
	activateArchmageAndCloseReaction(t, e)

	pending := e.PendingEffects()
	if len(pending) != 1 {
		t.Fatalf("expected 1 pending effect, got %d", len(pending))
	}
	if len(pending[0].DeckSearchChoices) != 0 {
		t.Fatalf("expected no eligible choices, got %d", len(pending[0].DeckSearchChoices))
	}

	deckSizeBefore := len(e.State.Players[gameplay.PlayerA].Deck)
	// Resolve with nil TargetCard → no-op success.
	if err := e.ResolvePendingEffect(gameplay.PlayerA, match.EffectTarget{}); err != nil {
		t.Fatalf("resolve pending with no target: %v", err)
	}
	if len(e.PendingEffects()) != 0 {
		t.Fatal("expected pending cleared")
	}
	// Deck must not have changed.
	if len(e.State.Players[gameplay.PlayerA].Deck) != deckSizeBefore {
		t.Fatal("expected deck unchanged when no target chosen")
	}
}

// TestArchmageArsenalIneligibleCardIDFails verifies that the resolver rejects a card that
// is not Power or costs more than 3.
func TestArchmageArsenalIneligibleCardIDFails(t *testing.T) {
	deckCards := []gameplay.CardInstance{powerCard4Mana(), retributionCard()}
	e := newArchmageTestEngine(t, deckCards)
	activateArchmageAndCloseReaction(t, e)

	// Try to pick zip-line (cost 4) — should fail.
	cardID := gameplay.CardID("zip-line")
	err := e.ResolvePendingEffect(gameplay.PlayerA, match.EffectTarget{TargetCard: &cardID})
	if err == nil {
		t.Fatal("expected error for ineligible card")
	}
}

// TestArchmageArsenalHandFullNoMoveOnResolve verifies that when the hand is full at resolve time,
// calling with a TargetCard fails with an effect-failed error.
func TestArchmageArsenalHandFullOnResolve(t *testing.T) {
	deckCards := []gameplay.CardInstance{powerCard3Mana()}
	e := newArchmageTestEngine(t, deckCards)
	activateArchmageAndCloseReaction(t, e)

	// Fill the hand to max before resolving.
	for len(e.State.Players[gameplay.PlayerA].Hand) < gameplay.DefaultMaxHandSize {
		e.State.Players[gameplay.PlayerA].Hand = append(
			e.State.Players[gameplay.PlayerA].Hand,
			gameplay.CardInstance{InstanceID: "pad", CardID: "energy-gain", ManaCost: 0},
		)
	}

	cardID := gameplay.CardID("knight-touch")
	err := e.ResolvePendingEffect(gameplay.PlayerA, match.EffectTarget{TargetCard: &cardID})
	if err == nil {
		t.Fatal("expected error when hand is full")
	}
	if err != matchresolvers.ErrEffectFailed {
		t.Fatalf("expected ErrEffectFailed, got %v", err)
	}
}

// TestArchmageArsenalDeckRemainingIsShuffled verifies that after a search the remaining deck
// is reshuffled (we can't assert exact order but we check the size is correct).
func TestArchmageArsenalDeckRemainingIsShuffled(t *testing.T) {
	deckCards := []gameplay.CardInstance{powerCard3Mana(), powerCard3Mana()}
	e := newArchmageTestEngine(t, deckCards)
	activateArchmageAndCloseReaction(t, e)

	sizeBefore := len(e.State.Players[gameplay.PlayerA].Deck)
	cardID := gameplay.CardID("knight-touch")
	if err := e.ResolvePendingEffect(gameplay.PlayerA, match.EffectTarget{TargetCard: &cardID}); err != nil {
		t.Fatalf("resolve: %v", err)
	}
	sizeAfter := len(e.State.Players[gameplay.PlayerA].Deck)
	if sizeAfter != sizeBefore-1 {
		t.Fatalf("expected deck size %d, got %d", sizeBefore-1, sizeAfter)
	}
}

// TestArchmageArsenalBlockedByPendingEffect verifies that activating Archmage Arsenal while
// a pending effect is unresolved is rejected.
func TestArchmageArsenalBlockedByOtherPendingEffect(t *testing.T) {
	deckCards := []gameplay.CardInstance{powerCard3Mana()}
	e := newArchmageTestEngine(t, deckCards)
	activateArchmageAndCloseReaction(t, e)

	// Now try to activate another card while pending effect is queued.
	// First add a second card to hand.
	e.State.Players[gameplay.PlayerA].Hand = append(
		e.State.Players[gameplay.PlayerA].Hand,
		gameplay.CardInstance{InstanceID: "eg2", CardID: "energy-gain", ManaCost: 0},
	)
	err := e.ActivateCard(gameplay.PlayerA, 0)
	if err == nil {
		t.Fatal("expected error when activating while pending effect exists")
	}
}
