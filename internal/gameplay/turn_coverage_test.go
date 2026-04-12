package gameplay

import "testing"

// --- GrantCaptureBonusMana ---

func TestGrantCaptureBonusManaRespectsTurnCap(t *testing.T) {
	s, _ := NewMatchState(StarterDeck(), StarterDeck())
	p := s.Players[PlayerA]
	p.Mana = 0
	p.ExtraManaGainedTurn = 0
	p.ExtraManaTurnCap = 1

	s.GrantCaptureBonusMana(PlayerA)
	if p.Mana != 1 {
		t.Fatalf("expected mana 1 after first bonus, got %d", p.Mana)
	}
	if p.ExtraManaGainedTurn != 1 {
		t.Fatalf("expected ExtraManaGainedTurn 1, got %d", p.ExtraManaGainedTurn)
	}

	// Second call should be ignored because cap is already reached.
	s.GrantCaptureBonusMana(PlayerA)
	if p.Mana != 1 {
		t.Fatalf("expected mana still 1 after cap reached, got %d", p.Mana)
	}
}

func TestGrantCaptureBonusManaRespectsMaxMana(t *testing.T) {
	s, _ := NewMatchState(StarterDeck(), StarterDeck())
	p := s.Players[PlayerA]
	p.Mana = p.MaxMana // already full
	p.ExtraManaGainedTurn = 0
	p.ExtraManaTurnCap = 5

	s.GrantCaptureBonusMana(PlayerA)
	if p.Mana != p.MaxMana {
		t.Fatalf("mana should be capped at MaxMana %d, got %d", p.MaxMana, p.Mana)
	}
}

// --- GrantManaForChessCapture ---

func TestGrantManaForChessCaptureAddsOneMana(t *testing.T) {
	s, _ := NewMatchState(StarterDeck(), StarterDeck())
	p := s.Players[PlayerA]
	p.Mana = 0

	s.GrantManaForChessCapture(PlayerA)
	if p.Mana != 1 {
		t.Fatalf("expected mana 1 after chess capture, got %d", p.Mana)
	}
}

func TestGrantManaForChessCaptureDoesNotExceedMax(t *testing.T) {
	s, _ := NewMatchState(StarterDeck(), StarterDeck())
	p := s.Players[PlayerA]
	p.Mana = p.MaxMana

	s.GrantManaForChessCapture(PlayerA)
	if p.Mana != p.MaxMana {
		t.Fatalf("mana should be capped at MaxMana, got %d", p.Mana)
	}
}

// --- ConsumeCardFromHand ---

func TestConsumeCardFromHandSucceeds(t *testing.T) {
	s, _ := NewMatchState(StarterDeck(), StarterDeck())
	p := s.Players[PlayerA]
	card := CardInstance{InstanceID: "c1", CardID: "double-turn", ManaCost: 4, Ignition: 1, Cooldown: 5}
	p.Hand = []CardInstance{card}
	p.Mana = 10

	consumed, err := s.ConsumeCardFromHand(PlayerA, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if consumed.CardID != "double-turn" {
		t.Fatalf("expected consumed card double-turn, got %s", consumed.CardID)
	}
	if len(p.Hand) != 0 {
		t.Fatalf("hand should be empty after consuming only card")
	}
	if p.Mana != 6 {
		t.Fatalf("expected mana 6 after consuming 4-cost card, got %d", p.Mana)
	}
	if p.EnergizedMana != 4 {
		t.Fatalf("expected energized mana 4, got %d", p.EnergizedMana)
	}
}

func TestConsumeCardFromHandErrorsOnBadIndex(t *testing.T) {
	s, _ := NewMatchState(StarterDeck(), StarterDeck())
	s.Players[PlayerA].Hand = []CardInstance{}

	if _, err := s.ConsumeCardFromHand(PlayerA, 0); err == nil {
		t.Fatal("expected error for empty hand index")
	}
	if _, err := s.ConsumeCardFromHand(PlayerA, -1); err == nil {
		t.Fatal("expected error for negative index")
	}
}

func TestConsumeCardFromHandErrorsOnInsufficientMana(t *testing.T) {
	s, _ := NewMatchState(StarterDeck(), StarterDeck())
	p := s.Players[PlayerA]
	p.Hand = []CardInstance{{InstanceID: "c1", CardID: "double-turn", ManaCost: 4, Ignition: 1, Cooldown: 5}}
	p.Mana = 0

	if _, err := s.ConsumeCardFromHand(PlayerA, 0); err == nil {
		t.Fatal("expected error for insufficient mana")
	}
}

// --- SendCardToCooldown ---

func TestSendCardToCooldownAppendsToCooldowns(t *testing.T) {
	s, _ := NewMatchState(StarterDeck(), StarterDeck())
	card := CardInstance{InstanceID: "c1", CardID: "double-turn", ManaCost: 4, Ignition: 1, Cooldown: 5}

	s.SendCardToCooldown(PlayerA, card)
	p := s.Players[PlayerA]
	if len(p.Cooldowns) != 1 {
		t.Fatalf("expected 1 cooldown entry, got %d", len(p.Cooldowns))
	}
	if p.Cooldowns[0].TurnsRemaining != 5 {
		t.Fatalf("expected cooldown turns 5, got %d", p.Cooldowns[0].TurnsRemaining)
	}
	if p.Cooldowns[0].Card.CardID != "double-turn" {
		t.Fatalf("wrong card in cooldown")
	}
}

// --- ResolveIgnition ---

func TestResolveIgnitionClearsSlotAndQueuesEvent(t *testing.T) {
	s, _ := NewMatchState(StarterDeck(), StarterDeck())
	card := CardInstance{InstanceID: "c1", CardID: "double-turn", ManaCost: 4, Ignition: 1, Cooldown: 5}
	s.IgnitionSlot = IgnitionSlot{Card: card, TurnsRemaining: 0, Occupied: true, ActivationOwner: PlayerA}

	if err := s.ResolveIgnition(true); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.IgnitionSlot.Occupied {
		t.Fatal("ignition slot should be cleared")
	}
	if len(s.ResolvedQueue) != 1 {
		t.Fatalf("expected 1 resolved event, got %d", len(s.ResolvedQueue))
	}
	ev := s.ResolvedQueue[0]
	if ev.Owner != PlayerA {
		t.Fatalf("expected owner PlayerA, got %s", ev.Owner)
	}
	if !ev.Success {
		t.Fatal("expected success=true")
	}
	if len(s.Players[PlayerA].Cooldowns) != 1 {
		t.Fatalf("expected card sent to cooldown, got %d entries", len(s.Players[PlayerA].Cooldowns))
	}
}

func TestResolveIgnitionFailureFlagPreserved(t *testing.T) {
	s, _ := NewMatchState(StarterDeck(), StarterDeck())
	card := CardInstance{InstanceID: "c1", CardID: "extinguish", ManaCost: 2, Ignition: 0, Cooldown: 2}
	s.IgnitionSlot = IgnitionSlot{Card: card, TurnsRemaining: 0, Occupied: true, ActivationOwner: PlayerB}

	_ = s.ResolveIgnition(false)
	if len(s.ResolvedQueue) != 1 || s.ResolvedQueue[0].Success {
		t.Fatal("expected success=false in resolved queue")
	}
}

func TestResolveIgnitionErrorOnEmptySlot(t *testing.T) {
	s, _ := NewMatchState(StarterDeck(), StarterDeck())
	if err := s.ResolveIgnition(true); err == nil {
		t.Fatal("expected error when ignition slot is empty")
	}
}

// --- PopResolvedIgnitions ---

func TestPopResolvedIgnitionsClearsQueue(t *testing.T) {
	s, _ := NewMatchState(StarterDeck(), StarterDeck())
	s.ResolvedQueue = []ResolvedIgnitionEvent{
		{Owner: PlayerA, Card: CardInstance{CardID: "double-turn"}, Success: true},
		{Owner: PlayerB, Card: CardInstance{CardID: "extinguish"}, Success: false},
	}

	events := s.PopResolvedIgnitions()
	if len(events) != 2 {
		t.Fatalf("expected 2 popped events, got %d", len(events))
	}
	if len(s.ResolvedQueue) != 0 {
		t.Fatal("queue should be empty after pop")
	}
}

func TestPopResolvedIgnitionsOnEmptyQueueReturnsEmpty(t *testing.T) {
	s, _ := NewMatchState(StarterDeck(), StarterDeck())
	events := s.PopResolvedIgnitions()
	if events != nil && len(events) != 0 {
		t.Fatalf("expected empty slice, got %v", events)
	}
}

// --- handleSaveItForLater (via ActivateCard) ---

func TestSaveItForLaterClearsIgnitionAndReturnsCardToHand(t *testing.T) {
	s, _ := NewMatchState(StarterDeck(), StarterDeck())
	// Put a different card in ignition.
	blocked := CardInstance{InstanceID: "b1", CardID: "double-turn", ManaCost: 4, Ignition: 1, Cooldown: 5}
	s.IgnitionSlot = IgnitionSlot{Card: blocked, TurnsRemaining: 1, Occupied: true, ActivationOwner: PlayerA}
	p := s.Players[PlayerA]
	sil := CardInstance{InstanceID: "sil1", CardID: "save-it-for-later", ManaCost: 0, Ignition: 0, Cooldown: 1}
	p.Hand = []CardInstance{sil}
	p.Mana = 10
	s.Started = true

	if err := s.ActivateCard(PlayerA, 0); err != nil {
		t.Fatalf("save-it-for-later activation failed: %v", err)
	}
	// Slot should be cleared.
	if s.IgnitionSlot.Occupied {
		t.Fatal("ignition slot should be cleared by save-it-for-later")
	}
	// Blocked card should be returned to the owner's hand.
	if len(p.Hand) != 1 || p.Hand[0].CardID != "double-turn" {
		t.Fatalf("expected blocked card returned to hand, got %v", p.Hand)
	}
	// Mana was refunded (blocked card's ManaCost 4 granted back).
	if p.Mana != 10+4 && p.Mana != p.MaxMana {
		t.Fatalf("unexpected mana after save-it-for-later: %d", p.Mana)
	}
}

func TestSaveItForLaterRequiresOccupiedSlot(t *testing.T) {
	s, _ := NewMatchState(StarterDeck(), StarterDeck())
	p := s.Players[PlayerA]
	sil := CardInstance{InstanceID: "sil1", CardID: "save-it-for-later", ManaCost: 0, Ignition: 0, Cooldown: 1}
	p.Hand = []CardInstance{sil}
	p.Mana = 10
	s.Started = true

	if err := s.ActivateCard(PlayerA, 0); err == nil {
		t.Fatal("expected error when ignition slot is empty")
	}
}

// --- SortedCardIDsForValidation ---

func TestSortedCardIDsForValidationSortsLexicographically(t *testing.T) {
	ids := []CardID{"rook-touch", "bishop-touch", "double-turn", "knight-touch"}
	sorted := SortedCardIDsForValidation(ids)
	for i := 1; i < len(sorted); i++ {
		if sorted[i-1] > sorted[i] {
			t.Fatalf("not sorted at index %d: %s > %s", i, sorted[i-1], sorted[i])
		}
	}
}

func TestSortedCardIDsForValidationDoesNotMutateOriginal(t *testing.T) {
	ids := []CardID{"z", "a", "m"}
	original := append([]CardID(nil), ids...)
	_ = SortedCardIDsForValidation(ids)
	for i, id := range ids {
		if id != original[i] {
			t.Fatalf("original slice mutated at index %d", i)
		}
	}
}

// --- EnterMulliganPhaseWithoutShuffle ---

func TestEnterMulliganPhaseWithoutShuffleSetsMulliganFlags(t *testing.T) {
	s, _ := NewMatchState(StarterDeck(), StarterDeck())
	EnterMulliganPhaseWithoutShuffle(s)

	if !s.MulliganPhaseActive {
		t.Fatal("MulliganPhaseActive should be true")
	}
	if s.MulliganConfirmed[PlayerA] || s.MulliganConfirmed[PlayerB] {
		t.Fatal("MulliganConfirmed should default to false")
	}
	if s.MulliganReturnedCount[PlayerA] != -1 || s.MulliganReturnedCount[PlayerB] != -1 {
		t.Fatalf("MulliganReturnedCount should be -1, got A=%d B=%d",
			s.MulliganReturnedCount[PlayerA], s.MulliganReturnedCount[PlayerB])
	}
}

// --- EndTurn errors ---

func TestEndTurnRejectsWrongPlayer(t *testing.T) {
	s, _ := NewMatchState(StarterDeck(), StarterDeck())
	// Current turn is A.
	if err := s.EndTurn(PlayerB); err == nil {
		t.Fatal("expected error ending B's turn when it's A's turn")
	}
}

func TestEndTurnIncrementsTurnNumberOnlyAfterB(t *testing.T) {
	s, _ := NewMatchState(StarterDeck(), StarterDeck())
	initial := s.TurnNumber
	// A ends → B's turn; turn number unchanged.
	_ = s.EndTurn(PlayerA)
	if s.TurnNumber != initial {
		t.Fatalf("turn number should not increment after A's end, got %d", s.TurnNumber)
	}
	// B ends → A's turn; turn number increments.
	_ = s.EndTurn(PlayerB)
	if s.TurnNumber != initial+1 {
		t.Fatalf("expected turn number %d after B's end, got %d", initial+1, s.TurnNumber)
	}
}

// --- HandleTurnTimeout ---

func TestHandleTurnTimeoutAddStrike(t *testing.T) {
	s, _ := NewMatchState(StarterDeck(), StarterDeck())
	lost, err := s.HandleTurnTimeout(PlayerA)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if lost {
		t.Fatal("player should not lose after first timeout")
	}
	if s.Players[PlayerA].Strikes != 1 {
		t.Fatalf("expected 1 strike, got %d", s.Players[PlayerA].Strikes)
	}
	if s.CurrentTurn != PlayerB {
		t.Fatal("turn should advance after timeout strike")
	}
}

func TestHandleTurnTimeoutThirdStrikeLoses(t *testing.T) {
	s, _ := NewMatchState(StarterDeck(), StarterDeck())
	s.Players[PlayerA].Strikes = 2

	lost, err := s.HandleTurnTimeout(PlayerA)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !lost {
		t.Fatal("player should lose after 3rd strike")
	}
}

func TestHandleTurnTimeoutRejectsWrongPlayer(t *testing.T) {
	s, _ := NewMatchState(StarterDeck(), StarterDeck())
	if _, err := s.HandleTurnTimeout(PlayerB); err == nil {
		t.Fatal("expected error for non-current player timeout")
	}
}

// --- StartTurn errors ---

func TestStartTurnRejectsWrongPlayer(t *testing.T) {
	s, _ := NewMatchState(StarterDeck(), StarterDeck())
	if err := s.StartTurn(PlayerB); err == nil {
		t.Fatal("expected error starting turn for non-current player")
	}
}

func TestStartTurnGrantsMana(t *testing.T) {
	s, _ := NewMatchState(StarterDeck(), StarterDeck())
	before := s.Players[PlayerA].Mana
	_ = s.StartTurn(PlayerA)
	if s.Players[PlayerA].Mana != before+1 && s.Players[PlayerA].Mana != s.Players[PlayerA].MaxMana {
		t.Fatalf("expected mana to increase by 1 or be capped, got %d", s.Players[PlayerA].Mana)
	}
}

// --- SelectPlayerSkill errors ---

func TestSelectPlayerSkillRejectsAfterStart(t *testing.T) {
	s, _ := NewMatchState(StarterDeck(), StarterDeck())
	s.Started = true
	if err := s.SelectPlayerSkill(PlayerA, "reinforcements"); err == nil {
		t.Fatal("expected error selecting skill after match start")
	}
}

func TestSelectPlayerSkillRejectsDuringMulligan(t *testing.T) {
	s, _ := NewMatchState(StarterDeck(), StarterDeck())
	s.MulliganPhaseActive = true
	if err := s.SelectPlayerSkill(PlayerA, "reinforcements"); err == nil {
		t.Fatal("expected error selecting skill during mulligan")
	}
}

func TestSelectPlayerSkillRejectsInvalidSkill(t *testing.T) {
	s, _ := NewMatchState(StarterDeck(), StarterDeck())
	if err := s.SelectPlayerSkill(PlayerA, "does-not-exist"); err == nil {
		t.Fatal("expected error for invalid skill ID")
	}
}

// --- tickCooldowns via StartTurn (indirect) ---

func TestTickCooldownsReturnsCardToDeckWhenExpired(t *testing.T) {
	s, _ := NewMatchState(StarterDeck(), StarterDeck())
	card := CardInstance{InstanceID: "c1", CardID: "double-turn", ManaCost: 4, Ignition: 1, Cooldown: 1}
	p := s.Players[PlayerA]
	p.Cooldowns = []CooldownEntry{{Card: card, TurnsRemaining: 1}}
	deckBefore := len(p.Deck)

	// One StartTurn tick reduces TurnsRemaining from 1 → 0 → returns to deck.
	_ = s.StartTurn(PlayerA)
	if len(p.Cooldowns) != 0 {
		t.Fatalf("expired cooldown should be removed, got %d", len(p.Cooldowns))
	}
	if len(p.Deck) != deckBefore+1 {
		t.Fatalf("expected card returned to deck, deck size: %d", len(p.Deck))
	}
}

func TestTickCooldownsKeepsCardsWithRemainingTurns(t *testing.T) {
	s, _ := NewMatchState(StarterDeck(), StarterDeck())
	card := CardInstance{InstanceID: "c1", CardID: "double-turn", ManaCost: 4, Ignition: 1, Cooldown: 5}
	p := s.Players[PlayerA]
	p.Cooldowns = []CooldownEntry{{Card: card, TurnsRemaining: 3}}

	_ = s.StartTurn(PlayerA)
	if len(p.Cooldowns) != 1 {
		t.Fatalf("cooldown should remain with turns left, got %d", len(p.Cooldowns))
	}
	if p.Cooldowns[0].TurnsRemaining != 2 {
		t.Fatalf("expected 2 turns remaining, got %d", p.Cooldowns[0].TurnsRemaining)
	}
}

// --- AddToGraveyard ---

func TestAddToGraveyardAppendsToPlayer(t *testing.T) {
	s, _ := NewMatchState(StarterDeck(), StarterDeck())
	piece := PieceRef{Color: "white", Type: "pawn"}
	s.AddToGraveyard(PlayerA, piece)

	g := s.Players[PlayerA].Graveyard
	if len(g) != 1 {
		t.Fatalf("expected 1 graveyard entry, got %d", len(g))
	}
	if g[0].Type != "pawn" {
		t.Fatalf("wrong piece in graveyard")
	}
}

// --- ResurrectFromGraveyard ---

func TestResurrectFromGraveyardRemovesAndReturnsEntry(t *testing.T) {
	s, _ := NewMatchState(StarterDeck(), StarterDeck())
	p := PieceRef{Color: "white", Type: "knight"}
	s.Players[PlayerA].Graveyard = []PieceRef{p}

	piece, err := s.ResurrectFromGraveyard(PlayerA, PlayerA, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if piece.Type != "knight" {
		t.Fatalf("expected knight, got %s", piece.Type)
	}
	if len(s.Players[PlayerA].Graveyard) != 0 {
		t.Fatal("graveyard should be empty after resurrection")
	}
}

func TestResurrectFromGraveyardErrorsOnBadIndex(t *testing.T) {
	s, _ := NewMatchState(StarterDeck(), StarterDeck())
	if _, err := s.ResurrectFromGraveyard(PlayerA, PlayerA, 0); err == nil {
		t.Fatal("expected error for out-of-range graveyard index")
	}
}

// --- NewMatchState size validation ---

func TestNewMatchStateRejectsWrongDeckSize(t *testing.T) {
	short := StarterDeck()[:5]
	if _, err := NewMatchState(short, StarterDeck()); err == nil {
		t.Fatal("expected error for deck size mismatch")
	}
}
