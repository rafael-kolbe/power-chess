// Package disruption contains effect resolvers for Disruption-type cards.
package disruption

import (
	"power-chess/internal/gameplay"
	"power-chess/internal/match/resolvers"
)

// ExtinguishResolver is the resolver for the "extinguish" Disruption card.
// The card effect (negating the opponent's ignition) is not yet implemented;
// the resolver is a deliberate no-op placeholder until the effect is built.
type ExtinguishResolver struct{}

// RequiresTarget reports whether this resolver waits for resolve_pending_effect input.
func (ExtinguishResolver) RequiresTarget() bool { return false }

// Apply is a placeholder; the Extinguish effect will be implemented in a future iteration.
func (ExtinguishResolver) Apply(_ resolvers.ResolverEngine, _ gameplay.PlayerID, _ resolvers.EffectTarget) error {
	return nil
}
