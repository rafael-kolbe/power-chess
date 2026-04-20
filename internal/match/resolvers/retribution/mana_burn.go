package retribution

import (
	"power-chess/internal/gameplay"
	"power-chess/internal/match/resolvers"
)

const cardManaBurn gameplay.CardID = "mana-burn"

// ManaBurnResolver applies the "mana-burn" retribution card.
// On successful resolution it burns mana from the opponent equal to the ManaCost of the card
// currently in the opponent's ignition slot. Regular mana is drained first; any remainder
// drains from the energized mana pool.
type ManaBurnResolver struct{}

// RequiresTarget reports whether this resolver waits for resolve_pending_effect input.
func (ManaBurnResolver) RequiresTarget() bool { return false }

// Apply burns the opponent's mana equal to the cost of their igniting card.
func (ManaBurnResolver) Apply(e resolvers.ResolverEngine, owner gameplay.PlayerID, _ resolvers.EffectTarget) error {
	opp := gameplay.OppositePlayer(owner)
	cost := e.IgnitionCardCost(opp)
	if cost <= 0 {
		return nil
	}
	e.BurnManaFromOpponent(opp, cost)
	return nil
}
