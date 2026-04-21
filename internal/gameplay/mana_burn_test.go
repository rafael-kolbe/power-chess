package gameplay

import "testing"

func TestBurnMana_drainsRegularManaFirst(t *testing.T) {
	s, _ := NewMatchState(StarterDeck(), StarterDeck())
	s.Players[PlayerB].Mana = 5
	s.Players[PlayerB].EnergizedMana = 10

	s.BurnMana(PlayerB, 3)

	if got := s.Players[PlayerB].Mana; got != 2 {
		t.Errorf("Mana: want 2, got %d", got)
	}
	if got := s.Players[PlayerB].EnergizedMana; got != 10 {
		t.Errorf("EnergizedMana: want 10 (untouched), got %d", got)
	}
}

func TestBurnMana_overflowDrainsEnergizedPool(t *testing.T) {
	s, _ := NewMatchState(StarterDeck(), StarterDeck())
	s.Players[PlayerB].Mana = 2
	s.Players[PlayerB].EnergizedMana = 10

	// Burn 5: 2 from regular, 3 from energized.
	s.BurnMana(PlayerB, 5)

	if got := s.Players[PlayerB].Mana; got != 0 {
		t.Errorf("Mana: want 0, got %d", got)
	}
	if got := s.Players[PlayerB].EnergizedMana; got != 7 {
		t.Errorf("EnergizedMana: want 7, got %d", got)
	}
}

func TestBurnMana_floorsAtZeroWhenBothPoolsInsufficient(t *testing.T) {
	s, _ := NewMatchState(StarterDeck(), StarterDeck())
	s.Players[PlayerB].Mana = 1
	s.Players[PlayerB].EnergizedMana = 2

	// Burn 10: exceeds both pools combined; both floor at zero.
	s.BurnMana(PlayerB, 10)

	if got := s.Players[PlayerB].Mana; got != 0 {
		t.Errorf("Mana: want 0, got %d", got)
	}
	if got := s.Players[PlayerB].EnergizedMana; got != 0 {
		t.Errorf("EnergizedMana: want 0, got %d", got)
	}
}

func TestBurnMana_zeroAmountIsNoop(t *testing.T) {
	s, _ := NewMatchState(StarterDeck(), StarterDeck())
	s.Players[PlayerB].Mana = 5
	s.Players[PlayerB].EnergizedMana = 8

	s.BurnMana(PlayerB, 0)

	if got := s.Players[PlayerB].Mana; got != 5 {
		t.Errorf("Mana: want 5, got %d", got)
	}
	if got := s.Players[PlayerB].EnergizedMana; got != 8 {
		t.Errorf("EnergizedMana: want 8, got %d", got)
	}
}

func TestBurnMana_exactlyDrainsRegularOnly(t *testing.T) {
	s, _ := NewMatchState(StarterDeck(), StarterDeck())
	s.Players[PlayerA].Mana = 4
	s.Players[PlayerA].EnergizedMana = 6

	s.BurnMana(PlayerA, 4)

	if got := s.Players[PlayerA].Mana; got != 0 {
		t.Errorf("Mana: want 0, got %d", got)
	}
	if got := s.Players[PlayerA].EnergizedMana; got != 6 {
		t.Errorf("EnergizedMana: want 6 (untouched), got %d", got)
	}
}
