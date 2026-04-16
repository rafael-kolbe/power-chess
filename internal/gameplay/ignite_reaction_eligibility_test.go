package gameplay

import "testing"

func TestEligibleForIgniteReactionAUTO_falseWithoutRetribution(t *testing.T) {
	s, err := NewMatchState(StarterDeck(), StarterDeck())
	if err != nil {
		t.Fatal(err)
	}
	s.Players[PlayerA].Hand = []CardInstance{{CardID: "counterattack", ManaCost: 1, Ignition: 0, Cooldown: 6}}
	s.Players[PlayerA].Mana = 10
	if EligibleForIgniteReactionAUTO(s, PlayerA) {
		t.Fatal("expected false without Retribution opening card")
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

func TestEligibleForIgniteReactionAUTO_falseWithPowerOnly(t *testing.T) {
	s, err := NewMatchState(StarterDeck(), StarterDeck())
	if err != nil {
		t.Fatal(err)
	}
	s.Players[PlayerB].Hand = []CardInstance{{CardID: "knight-touch", ManaCost: 3, Ignition: 0, Cooldown: 2}}
	s.Players[PlayerB].Mana = 5
	if EligibleForIgniteReactionAUTO(s, PlayerB) {
		t.Fatal("expected false when hand has only Power (illegal opening ignite response)")
	}
}

func TestEligibleForIgniteReactionAUTO_falseWithExtinguishPower(t *testing.T) {
	s, err := NewMatchState(StarterDeck(), StarterDeck())
	if err != nil {
		t.Fatal(err)
	}
	s.Players[PlayerB].Hand = []CardInstance{{CardID: "extinguish", ManaCost: 2, Ignition: 0, Cooldown: 2}}
	s.Players[PlayerB].Mana = 5
	if EligibleForIgniteReactionAUTO(s, PlayerB) {
		t.Fatal("expected false when hand has only Extinguish (Power cannot open ignite response)")
	}
}

func TestEligibleForIgniteReactionAUTO_falseWithCounterWhenMaybeCaptureAttemptUnset(t *testing.T) {
	s, err := NewMatchState(StarterDeck(), StarterDeck())
	if err != nil {
		t.Fatal(err)
	}
	s.Players[PlayerA].Ignition = IgnitionSlot{
		Occupied:        true,
		ActivationOwner: PlayerA,
		Card:            CardInstance{InstanceID: "ig", CardID: "knight-touch", ManaCost: 3, Ignition: 0, Cooldown: 2},
	}
	s.Players[PlayerB].Hand = []CardInstance{{CardID: "counterattack", ManaCost: 1, Ignition: 0, Cooldown: 6}}
	s.Players[PlayerB].Mana = 10
	if EligibleForIgniteReactionAUTO(s, PlayerB) {
		t.Fatal("expected false: Counter opening requires MaybeCaptureAttemptOnIgnition on ignited card")
	}
}

func TestEligibleForIgniteReactionAUTO_falseWithCounterWhenNonCaptureIgnited(t *testing.T) {
	s, err := NewMatchState(StarterDeck(), StarterDeck())
	if err != nil {
		t.Fatal(err)
	}
	s.Players[PlayerA].Ignition = IgnitionSlot{
		Occupied:        true,
		ActivationOwner: PlayerA,
		Card:            CardInstance{InstanceID: "ig", CardID: "energy-gain", ManaCost: 0, Ignition: 1, Cooldown: 2},
	}
	s.Players[PlayerB].Hand = []CardInstance{{CardID: "counterattack", ManaCost: 1, Ignition: 0, Cooldown: 6}}
	s.Players[PlayerB].Mana = 10
	if EligibleForIgniteReactionAUTO(s, PlayerB) {
		t.Fatal("expected false when only Counter is available and ignited card has MaybeCaptureAttemptOnIgnition false")
	}
}
