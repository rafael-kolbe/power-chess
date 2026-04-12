package gameplay

import "fmt"

// TakeHandFromDeckByCardIDs moves one instance of each listed card ID from the player's deck to hand,
// in order. Duplicate IDs in ids consume separate deck copies (multiset match).
func TakeHandFromDeckByCardIDs(s *MatchState, pid PlayerID, ids []CardID) error {
	if len(ids) > DefaultMaxHandSize {
		return fmt.Errorf("hand list exceeds max hand size %d", DefaultMaxHandSize)
	}
	p := s.Players[pid]
	for _, id := range ids {
		if _, ok := CardDefinitionByID(id); !ok {
			return fmt.Errorf("unknown card id %q", id)
		}
		found := -1
		for i, c := range p.Deck {
			if c.CardID == id {
				found = i
				break
			}
		}
		if found < 0 {
			return fmt.Errorf("hand card %q not present in deck (insufficient copies)", id)
		}
		card := p.Deck[found]
		p.Deck = append(p.Deck[:found], p.Deck[found+1:]...)
		p.Hand = append(p.Hand, card)
	}
	return nil
}

// NewMatchStateWithPresetHands builds a match from two legal 20-card decks, then moves listed cards
// into each hand without shuffling. Mulligan flags are not set; caller should apply skills, then
// EnterMulliganPhaseWithoutShuffle.
func NewMatchStateWithPresetHands(deckA, deckB []CardID, handA, handB []CardID) (*MatchState, error) {
	instA, err := DeckInstancesFromCardIDs(deckA)
	if err != nil {
		return nil, err
	}
	instB, err := DeckInstancesFromCardIDs(deckB)
	if err != nil {
		return nil, err
	}
	s, err := NewMatchState(instA, instB)
	if err != nil {
		return nil, err
	}
	if err := TakeHandFromDeckByCardIDs(s, PlayerA, handA); err != nil {
		return nil, err
	}
	if err := TakeHandFromDeckByCardIDs(s, PlayerB, handB); err != nil {
		return nil, err
	}
	return s, nil
}

// EnterMulliganPhaseWithoutShuffle activates mulligan bookkeeping when opening hands are already
// placed (e.g. debug fixtures). Does not shuffle or draw.
func EnterMulliganPhaseWithoutShuffle(s *MatchState) {
	s.MulliganPhaseActive = true
	s.MulliganConfirmed = map[PlayerID]bool{
		PlayerA: false,
		PlayerB: false,
	}
	s.MulliganReturnedCount = map[PlayerID]int{
		PlayerA: -1,
		PlayerB: -1,
	}
}
