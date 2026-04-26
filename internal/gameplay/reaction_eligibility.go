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
		if c.CardID == "retaliate" && !HasValidRetaliateCooldownTarget(s, pid) {
			continue
		}
		return true
	}
	return false
}

// HasValidRetaliateCooldownTarget reports whether pid can choose at least one opponent
// cooldown Power card whose full cost can be burned from the opponent's regular mana.
func HasValidRetaliateCooldownTarget(s *MatchState, pid PlayerID) bool {
	if s == nil {
		return false
	}
	opp := OppositePlayer(pid)
	oppState := s.Players[opp]
	if oppState == nil {
		return false
	}
	for _, cd := range oppState.Cooldowns {
		def, ok := CardDefinitionByID(cd.Card.CardID)
		if !ok || def.Type != CardTypePower {
			continue
		}
		if oppState.Mana >= cd.Card.ManaCost {
			return true
		}
	}
	return false
}

// EligibleForDisruptionReactionAUTO reports whether pid could queue at least one Disruption card
// as a response in an ignite_reaction window. All of the following must hold:
//   - The opponent has a card in their ignition slot.
//   - pid has a Disruption card in hand with sufficient mana (cooldown duplicate rule applies).
//   - pid has at least one Power card in hand to pay the mandatory banish cost (Disruption type rule).
func EligibleForDisruptionReactionAUTO(s *MatchState, pid PlayerID) bool {
	if s == nil {
		return false
	}
	p := s.Players[pid]
	if p == nil {
		return false
	}
	opp := OppositePlayer(pid)
	oppSlot := s.Players[opp].Ignition
	if !oppSlot.Occupied {
		return false
	}
	// Disruption reaction cost: player must have a Power card to banish.
	hasPowerCard := false
	for _, c := range p.Hand {
		def, ok := CardDefinitionByID(c.CardID)
		if ok && def.Type == CardTypePower {
			hasPowerCard = true
			break
		}
	}
	if !hasPowerCard {
		return false
	}
	onCooldown := make(map[string]struct{}, len(p.Cooldowns))
	for _, cd := range p.Cooldowns {
		onCooldown[string(cd.Card.CardID)] = struct{}{}
	}
	for _, c := range p.Hand {
		def, ok := CardDefinitionByID(c.CardID)
		if !ok || def.Type != CardTypeDisruption {
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
