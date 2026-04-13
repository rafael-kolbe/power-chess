package gameplay

// EligibleForCaptureCounterReactionAUTO reports whether pid could queue at least one Counter
// card from hand in a capture_attempt reaction window: card type Counter, sufficient mana,
// and no copy of that card id already on cooldown (same duplicate rule as ignition).
//
// Counter card text conditions (e.g. "if buffed attacker") are intentionally ignored until
// feature/gameplay-next; see internal/match/reactions.go TODO.
func EligibleForCaptureCounterReactionAUTO(s *MatchState, pid PlayerID) bool {
	if s == nil {
		return false
	}
	p := s.Players[pid]
	if p == nil {
		return false
	}
	onCooldown := make(map[string]struct{}, len(p.Cooldowns))
	for _, cd := range p.Cooldowns {
		onCooldown[string(cd.Card.CardID)] = struct{}{}
	}
	for _, c := range p.Hand {
		def, ok := CardDefinitionByID(c.CardID)
		if !ok || def.Type != CardTypeCounter {
			continue
		}
		if _, dup := onCooldown[string(c.CardID)]; dup {
			continue
		}
		if p.Mana < def.Cost {
			continue
		}
		return true
	}
	return false
}
