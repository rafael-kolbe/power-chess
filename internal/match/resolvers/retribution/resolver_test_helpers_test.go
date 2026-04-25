package retribution_test

import "power-chess/internal/gameplay"

func testDeckWith(card gameplay.CardInstance) []gameplay.CardInstance {
	d := []gameplay.CardInstance{card}
	for i := 1; i < gameplay.DefaultDeckSize; i++ {
		d = append(d, gameplay.CardInstance{
			InstanceID: "filler",
			CardID:     "filler",
			ManaCost:   1,
			Ignition:   0,
			Cooldown:   1,
		})
	}
	return d
}

func markInPlayForTest(s *gameplay.MatchState) {
	s.MulliganPhaseActive = false
	s.Started = true
}
