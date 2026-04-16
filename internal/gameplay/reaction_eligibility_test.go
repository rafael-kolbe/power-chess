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
