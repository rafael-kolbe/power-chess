package gameplay

import (
	"crypto/rand"
	"encoding/binary"
	"errors"
	"sort"
)

// BeginOpeningPhase shuffles each deck, draws the initial hand for both players, and enters the mulligan phase.
func BeginOpeningPhase(s *MatchState) error {
	s.MulliganPhaseActive = true
	s.MulliganConfirmed = map[PlayerID]bool{
		PlayerA: false,
		PlayerB: false,
	}
	s.MulliganReturnedCount = map[PlayerID]int{
		PlayerA: -1,
		PlayerB: -1,
	}
	for _, pid := range []PlayerID{PlayerA, PlayerB} {
		s.shuffleDeck(pid)
	}
	for _, pid := range []PlayerID{PlayerA, PlayerB} {
		for range DefaultInitialDraw {
			if err := s.drawCardNoCost(pid); err != nil {
				return err
			}
		}
	}
	return nil
}

// shuffleDeck randomizes the order of cards in the player's deck using a cryptographically strong RNG.
func (s *MatchState) shuffleDeck(pid PlayerID) {
	p := s.Players[pid]
	shuffleCardSlice(p.Deck)
}

// shuffleCardSlice applies an in-place Fisher–Yates shuffle to deck.
func shuffleCardSlice(deck []CardInstance) {
	n := len(deck)
	for i := n - 1; i > 0; i-- {
		j := randIntn(i + 1)
		deck[i], deck[j] = deck[j], deck[i]
	}
}

// randIntn returns a uniform random integer in [0, n).
func randIntn(n int) int {
	if n <= 0 {
		return 0
	}
	var buf [8]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return 0
	}
	return int(binary.BigEndian.Uint64(buf[:]) % uint64(n))
}

// ConfirmMulligan returns the selected hand cards to the deck, shuffles the deck, and redraws that many cards.
// When both players have confirmed, it ends the mulligan phase and reports done=true so the engine can start the first turn.
func (s *MatchState) ConfirmMulligan(pid PlayerID, handIndices []int) (done bool, err error) {
	if !s.MulliganPhaseActive {
		return false, errors.New("not in mulligan phase")
	}
	if s.MulliganConfirmed[pid] {
		return false, errors.New("mulligan already confirmed")
	}
	p := s.Players[pid]
	seen := map[int]struct{}{}
	var uniq []int
	for _, idx := range handIndices {
		if idx < 0 || idx >= len(p.Hand) {
			return false, errors.New("invalid hand index")
		}
		if _, dup := seen[idx]; dup {
			continue
		}
		seen[idx] = struct{}{}
		uniq = append(uniq, idx)
	}
	sort.Ints(uniq)
	toReturn := make([]CardInstance, len(uniq))
	for i, idx := range uniq {
		toReturn[i] = p.Hand[idx]
	}
	// Remove from highest index to lowest so positions stay valid.
	for i := len(uniq) - 1; i >= 0; i-- {
		idx := uniq[i]
		p.Hand = append(p.Hand[:idx], p.Hand[idx+1:]...)
	}
	p.Deck = append(p.Deck, toReturn...)
	s.shuffleDeck(pid)
	n := len(toReturn)
	for i := 0; i < n; i++ {
		if err := s.drawCardNoCost(pid); err != nil {
			return false, err
		}
	}
	if s.MulliganReturnedCount == nil {
		s.MulliganReturnedCount = map[PlayerID]int{}
	}
	s.MulliganReturnedCount[pid] = n
	if s.MulliganConfirmed == nil {
		s.MulliganConfirmed = map[PlayerID]bool{}
	}
	s.MulliganConfirmed[pid] = true

	if s.MulliganConfirmed[PlayerA] && s.MulliganConfirmed[PlayerB] {
		s.MulliganPhaseActive = false
		return true, nil
	}
	return false, nil
}
