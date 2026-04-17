package power

import (
	"power-chess/internal/gameplay"
	"power-chess/internal/match/resolvers"
)

// EnergyGainResolver applies the "energy-gain" card: +4 mana when the effect activation succeeds.
//
// While the card sits in the ignition slot (including the ignite_reaction window), no mana is
// granted. Mana is added only after the ignition burn reaches zero and the resolution is recorded
// as successful (see MatchState.ResolveIgnitionFor and processResolvedIgnitions).
type EnergyGainResolver struct{}

// RequiresTarget reports whether this resolver waits for resolve_pending_effect input.
func (EnergyGainResolver) RequiresTarget() bool { return false }

// Apply grants 4 mana to the owner on successful effect resolution (mana is capped by MatchState).
func (EnergyGainResolver) Apply(e resolvers.ResolverEngine, owner gameplay.PlayerID, _ resolvers.EffectTarget) error {
	e.GrantManaFromCardEffect(owner, 4)
	return nil
}
