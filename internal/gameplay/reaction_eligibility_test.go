package gameplay

import "testing"

func TestEligibleForCaptureReactionAUTO_falseWithRetributionOnly(t *testing.T) {
	s, err := NewMatchState(StarterDeck(), StarterDeck())
	if err != nil {
		t.Fatal(err)
	}
	s.Players[PlayerA].Hand = []CardInstance{{CardID: "mana-burn", ManaCost: 1, Ignition: 0, Cooldown: 3}}
	s.Players[PlayerA].Mana = 5
	if EligibleForCaptureReactionAUTO(s, PlayerA) {
		t.Fatal("capture_attempt opening is Counter-only; Retribution alone must not count as AUTO-eligible")
	}
}

func TestEligibleForCaptureReactionAUTO_trueWithCounterOnly(t *testing.T) {
	s, err := NewMatchState(StarterDeck(), StarterDeck())
	if err != nil {
		t.Fatal(err)
	}
	s.Players[PlayerB].Hand = []CardInstance{{CardID: "counterattack", ManaCost: 1, Ignition: 0, Cooldown: 6}}
	s.Players[PlayerB].Mana = 10
	if !EligibleForCaptureReactionAUTO(s, PlayerB) {
		t.Fatal("expected true with Counter in hand")
	}
}

func TestEligibleForDisruptionReactionAUTO_falseWhenOpponentIgnitionEmpty(t *testing.T) {
	s, err := NewMatchState(StarterDeck(), StarterDeck())
	if err != nil {
		t.Fatal(err)
	}
	s.Players[PlayerB].Hand = []CardInstance{{CardID: "extinguish", ManaCost: 2, Ignition: 0, Cooldown: 2}}
	s.Players[PlayerB].Mana = 5
	// PlayerA's ignition is empty; Disruption should not be eligible.
	if EligibleForDisruptionReactionAUTO(s, PlayerB) {
		t.Fatal("expected false when opponent ignition is empty")
	}
}

func TestEligibleForDisruptionReactionAUTO_falseWhenInsufficientMana(t *testing.T) {
	s, err := NewMatchState(StarterDeck(), StarterDeck())
	if err != nil {
		t.Fatal(err)
	}
	s.Players[PlayerA].Ignition = IgnitionSlot{
		Occupied:        true,
		ActivationOwner: PlayerA,
		Card:            CardInstance{InstanceID: "ig", CardID: "knight-touch", ManaCost: 3, Ignition: 0, Cooldown: 2},
	}
	s.Players[PlayerB].Hand = []CardInstance{{CardID: "extinguish", ManaCost: 2, Ignition: 0, Cooldown: 2}}
	s.Players[PlayerB].Mana = 1 // not enough for extinguish (cost 2)
	if EligibleForDisruptionReactionAUTO(s, PlayerB) {
		t.Fatal("expected false when player cannot afford the Disruption card")
	}
}

func TestEligibleForDisruptionReactionAUTO_falseWhenNoPowerCardInHand(t *testing.T) {
	s, err := NewMatchState(StarterDeck(), StarterDeck())
	if err != nil {
		t.Fatal(err)
	}
	s.Players[PlayerA].Ignition = IgnitionSlot{
		Occupied:        true,
		ActivationOwner: PlayerA,
		Card:            CardInstance{InstanceID: "ig", CardID: "knight-touch", ManaCost: 3, Ignition: 0, Cooldown: 2},
	}
	// Only a Disruption card in hand — no Power card available to pay the banish cost.
	s.Players[PlayerB].Hand = []CardInstance{{CardID: "extinguish", ManaCost: 2, Ignition: 0, Cooldown: 2}}
	s.Players[PlayerB].Mana = 5
	if EligibleForDisruptionReactionAUTO(s, PlayerB) {
		t.Fatal("expected false when no Power card is available in hand to pay the disruption banish cost")
	}
}

func TestEligibleForDisruptionReactionAUTO_trueWhenConditionsMet(t *testing.T) {
	s, err := NewMatchState(StarterDeck(), StarterDeck())
	if err != nil {
		t.Fatal(err)
	}
	s.Players[PlayerA].Ignition = IgnitionSlot{
		Occupied:        true,
		ActivationOwner: PlayerA,
		Card:            CardInstance{InstanceID: "ig", CardID: "double-turn", ManaCost: 6, Ignition: 2, Cooldown: 9},
	}
	s.Players[PlayerB].Hand = []CardInstance{
		{CardID: "extinguish", ManaCost: 2, Ignition: 0, Cooldown: 2},
		// Power card required to pay the disruption reaction banish cost.
		{CardID: "knight-touch", ManaCost: 3, Ignition: 0, Cooldown: 2},
	}
	s.Players[PlayerB].Mana = 5
	if !EligibleForDisruptionReactionAUTO(s, PlayerB) {
		t.Fatal("expected true when opponent has ignition card, player has enough mana, and a Power card is available to banish")
	}
}
