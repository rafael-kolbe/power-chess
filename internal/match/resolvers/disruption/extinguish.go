// Package disruption contains effect resolvers for Disruption-type cards.
package disruption

import (
	"power-chess/internal/gameplay"
	"power-chess/internal/match/resolvers"
)

// ExtinguishResolver is the resolver for the "extinguish" Disruption card.
type ExtinguishResolver struct{}

// RequiresTarget reports whether this resolver waits for resolve_pending_effect input.
func (ExtinguishResolver) RequiresTarget() bool { return false }

// Apply marks the opponent's current ignition card as negated; it remains in their ignition zone
// until its normal burn completes, but activation attempts resolve as failure and skip the effect.
func (ExtinguishResolver) Apply(e resolvers.ResolverEngine, owner gameplay.PlayerID, _ resolvers.EffectTarget) error {
	opp := gameplay.OppositePlayer(owner)
	return e.MarkOpponentCardEffectNegated(opp)
}
