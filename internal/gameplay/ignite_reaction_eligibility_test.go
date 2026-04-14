package gameplay

import "testing"

func TestEligibleForIgniteReactionAUTO_falseWithoutRetributionOrPower(t *testing.T) {
	s, err := NewMatchState(StarterDeck(), StarterDeck())
	if err != nil {
		t.Fatal(err)
	}
	s.Players[PlayerA].Hand = []CardInstance{{CardID: "counterattack", ManaCost: 1, Ignition: 0, Cooldown: 6}}
	s.Players[PlayerA].Mana = 10
	if EligibleForIgniteReactionAUTO(s, PlayerA) {
		t.Fatal("expected false without Retribution or Power opening card")
	}
}

func TestEligibleForIgniteReactionAUTO_trueWithRetribution(t *testing.T) {
	s, err := NewMatchState(StarterDeck(), StarterDeck())
	if err != nil {
		t.Fatal(err)
	}
	s.Players[PlayerB].Hand = []CardInstance{{CardID: "mana-burn", ManaCost: 1, Ignition: 0, Cooldown: 3}}
	s.Players[PlayerB].Mana = 5
	if !EligibleForIgniteReactionAUTO(s, PlayerB) {
		t.Fatal("expected true with Mana Burn")
	}
}

func TestEligibleForIgniteReactionAUTO_trueWithAnyPower(t *testing.T) {
	s, err := NewMatchState(StarterDeck(), StarterDeck())
	if err != nil {
		t.Fatal(err)
	}
	s.Players[PlayerB].Hand = []CardInstance{{CardID: "knight-touch", ManaCost: 3, Ignition: 0, Cooldown: 2}}
	s.Players[PlayerB].Mana = 5
	if !EligibleForIgniteReactionAUTO(s, PlayerB) {
		t.Fatal("expected true with Power card for opening response")
	}
}

func TestEligibleForIgniteReactionAUTO_trueWithExtinguish(t *testing.T) {
	s, err := NewMatchState(StarterDeck(), StarterDeck())
	if err != nil {
		t.Fatal(err)
	}
	s.Players[PlayerB].Hand = []CardInstance{{CardID: "extinguish", ManaCost: 2, Ignition: 0, Cooldown: 2}}
	s.Players[PlayerB].Mana = 5
	if !EligibleForIgniteReactionAUTO(s, PlayerB) {
		t.Fatal("expected true with Extinguish")
	}
}
