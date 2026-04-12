package gameplay

import "testing"

func TestNewMatchStateWithPresetHands(t *testing.T) {
	deck := DefaultDeckPresetCardIDs()
	handA := []CardID{"knight-touch", "energy-gain", "bishop-touch"}
	handB := []CardID{"retaliate", "backstab", "clairvoyance"}
	s, err := NewMatchStateWithPresetHands(deck, deck, handA, handB)
	if err != nil {
		t.Fatalf("NewMatchStateWithPresetHands: %v", err)
	}
	if len(s.Players[PlayerA].Hand) != 3 || len(s.Players[PlayerB].Hand) != 3 {
		t.Fatalf("expected 3 cards in each hand, got %d and %d", len(s.Players[PlayerA].Hand), len(s.Players[PlayerB].Hand))
	}
	if len(s.Players[PlayerA].Deck)+len(s.Players[PlayerA].Hand) != DefaultDeckSize {
		t.Fatalf("player A deck+hand should be %d", DefaultDeckSize)
	}
}

func TestTakeHandFromDeckByCardIDsTooManyCopies(t *testing.T) {
	deck := DefaultDeckPresetCardIDs()
	s, err := NewMatchStateWithPresetHands(deck, deck, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	// energy-gain has max 3 in default deck — ask for 4
	bad := []CardID{"energy-gain", "energy-gain", "energy-gain", "energy-gain"}
	if err := TakeHandFromDeckByCardIDs(s, PlayerA, bad); err == nil {
		t.Fatal("expected error for impossible hand")
	}
}
