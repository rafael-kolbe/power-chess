package gameplay

// EligibleForOpeningRetributionAUTO reports whether pid could queue at least one Retribution
// card as the first response in an eligible window such as ignite_reaction (mana, hand, cooldown duplicate rule only).
func EligibleForOpeningRetributionAUTO(s *MatchState, pid PlayerID) bool {
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
		if !ok || def.Type != CardTypeRetribution {
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

// EligibleForCaptureReactionAUTO reports whether pid could queue at least one legal opening
// response in capture_attempt: Counter only (economy rules only; card text conditions are ignored in AUTO).
func EligibleForCaptureReactionAUTO(s *MatchState, pid PlayerID) bool {
	return EligibleForCaptureCounterReactionAUTO(s, pid)
}

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
