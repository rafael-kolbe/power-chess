package gameplay

// igniteReactionFirstCardIDs lists Power cards that may open an ignite_reaction chain as the
// opponent's first response (same as QueueReactionCard rules in internal/match/reactions.go).
var igniteReactionFirstCardIDs = map[string]struct{}{
	"extinguish": {},
}

// EligibleForIgniteReactionAUTO reports whether pid could queue at least one opening response
// in an ignite_reaction window: a Retribution card, or Extinguish, with sufficient mana and
// no duplicate of that card id on cooldown (same rule as ignition activation).
//
// Retribution card text conditions are intentionally ignored until feature/gameplay-next;
// see internal/match/reactions.go TODO for Counter parity.
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
		if def.Type == CardTypeRetribution {
			if _, dup := onCooldown[string(c.CardID)]; dup {
				continue
			}
			if p.Mana < def.Cost {
				continue
			}
			return true
		}
		if def.Type == CardTypePower {
			if _, ok := igniteReactionFirstCardIDs[string(c.CardID)]; !ok {
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
	}
	return false
}
