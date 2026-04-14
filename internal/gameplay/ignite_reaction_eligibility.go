package gameplay

// EligibleForIgniteReactionAUTO reports whether pid could queue at least one opening response
// in an ignite_reaction window: a Retribution card or a Power card, with sufficient mana and
// no duplicate of that card id on cooldown (same rule as ignition activation).
//
// Per-card text conditions are not evaluated; only type and economy rules apply.
func EligibleForIgniteReactionAUTO(s *MatchState, pid PlayerID) bool {
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
		if !ok {
			continue
		}
		if def.Type != CardTypeRetribution && def.Type != CardTypePower {
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
