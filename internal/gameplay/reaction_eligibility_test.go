package gameplay

import "testing"

func TestEligibleForCaptureCounterReactionAUTO_falseWhenNoCounter(t *testing.T) {
	s, err := NewMatchState(StarterDeck(), StarterDeck())
	if err != nil {
		t.Fatal(err)
	}
	s.Players[PlayerA].Hand = []CardInstance{{CardID: "knight-touch", ManaCost: 3, Ignition: 0, Cooldown: 2}}
	s.Players[PlayerA].Mana = 10
	if EligibleForCaptureCounterReactionAUTO(s, PlayerA) {
		t.Fatal("expected false without Counter in hand")
	}
}

func TestEligibleForCaptureCounterReactionAUTO_trueWithCounterAndMana(t *testing.T) {
	s, err := NewMatchState(StarterDeck(), StarterDeck())
	if err != nil {
		t.Fatal(err)
	}
	s.Players[PlayerB].Hand = []CardInstance{{CardID: "counterattack", ManaCost: 1, Ignition: 0, Cooldown: 6}}
	s.Players[PlayerB].Mana = 5
	if !EligibleForCaptureCounterReactionAUTO(s, PlayerB) {
		t.Fatal("expected true with Counterattack and mana")
	}
}

func TestEligibleForCaptureCounterReactionAUTO_falseWhenDuplicateOnCooldown(t *testing.T) {
	s, err := NewMatchState(StarterDeck(), StarterDeck())
	if err != nil {
		t.Fatal(err)
	}
	s.Players[PlayerB].Hand = []CardInstance{{CardID: "counterattack", ManaCost: 1, Ignition: 0, Cooldown: 6}}
	s.Players[PlayerB].Mana = 5
	s.Players[PlayerB].Cooldowns = []CooldownEntry{
		{Card: CardInstance{CardID: "counterattack", ManaCost: 1, Ignition: 0, Cooldown: 6}, TurnsRemaining: 2},
	}
	if EligibleForCaptureCounterReactionAUTO(s, PlayerB) {
		t.Fatal("expected false when same card id is on cooldown")
	}
}

func TestEligibleForCaptureCounterReactionAUTO_falseWhenNotEnoughMana(t *testing.T) {
	s, err := NewMatchState(StarterDeck(), StarterDeck())
	if err != nil {
		t.Fatal(err)
	}
	s.Players[PlayerB].Hand = []CardInstance{{CardID: "counterattack", ManaCost: 1, Ignition: 0, Cooldown: 6}}
	s.Players[PlayerB].Mana = 0
	if EligibleForCaptureCounterReactionAUTO(s, PlayerB) {
		t.Fatal("expected false without mana for Counter cost")
	}
}
