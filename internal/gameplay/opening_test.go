package gameplay

import "testing"

func TestConfirmMulliganRedrawsSameCount(t *testing.T) {
	s, err := NewMatchState(StarterDeck(), StarterDeck())
	if err != nil {
		t.Fatal(err)
	}
	if err := BeginOpeningPhase(s); err != nil {
		t.Fatal(err)
	}
	aHand := append([]CardInstance(nil), s.Players[PlayerA].Hand...)
	if len(aHand) < 2 {
		t.Fatal("need at least 2 cards for mulligan test")
	}
	if _, err := s.ConfirmMulligan(PlayerA, []int{0, 1}); err != nil {
		t.Fatal(err)
	}
	if len(s.Players[PlayerA].Hand) != 3 {
		t.Fatalf("hand size after mulligan: want 3, got %d", len(s.Players[PlayerA].Hand))
	}
	if s.MulliganReturnedCount[PlayerA] != 2 {
		t.Fatalf("returned count: want 2, got %d", s.MulliganReturnedCount[PlayerA])
	}
}
