package retribution

import (
	"power-chess/internal/gameplay"
	"power-chess/internal/match/resolvers"
)

// RetaliateResolver applies the "retaliate" retribution card.
type RetaliateResolver struct{}

// RequiresTarget reports whether this resolver waits for resolve_pending_effect input.
func (RetaliateResolver) RequiresTarget() bool { return false }

// Apply burns exact regular mana from the opponent, then activates the selected cooldown
// Power effect for the Retaliate owner.
func (RetaliateResolver) Apply(e resolvers.ResolverEngine, owner gameplay.PlayerID, target resolvers.EffectTarget) error {
	if target.TargetCard == nil {
		return resolvers.ErrEffectFailed
	}
	return e.ResolveCooldownPowerEffect(owner, *target.TargetCard)
}
