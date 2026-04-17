package power

import (
	"power-chess/internal/gameplay"
	"power-chess/internal/match/resolvers"
)

// DoubleTurnResolver applies the "double-turn" card: grants the owner one extra move
// on the turn the ignition resolves.
type DoubleTurnResolver struct{}

// RequiresTarget reports whether this resolver waits for resolve_pending_effect input.
func (DoubleTurnResolver) RequiresTarget() bool { return false }

// Apply increments the owner's extra-move counter by one, allowing a second move this turn.
func (DoubleTurnResolver) Apply(e resolvers.ResolverEngine, owner gameplay.PlayerID, _ resolvers.EffectTarget) error {
	e.IncrementExtraMoves(owner)
	return nil
}
