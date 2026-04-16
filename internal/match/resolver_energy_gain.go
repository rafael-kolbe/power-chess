package match

import "power-chess/internal/gameplay"

// energyGainResolver applies the "energy-gain" card: +4 mana when the effect activation succeeds.
//
// While the card sits in the ignition slot (including the ignite_reaction window), no mana is
// granted. Mana is added only after the ignition burn reaches zero and the resolution is recorded
// as successful (see MatchState.ResolveIgnitionFor and processResolvedIgnitions).
type energyGainResolver struct{}

// RequiresTarget reports whether this resolver waits for resolve_pending_effect input.
func (energyGainResolver) RequiresTarget() bool { return false }

// Apply grants 4 mana to the owner on successful effect resolution (Mana is capped by MatchState).
func (energyGainResolver) Apply(e *Engine, owner gameplay.PlayerID, _ EffectTarget) error {
	e.State.GrantManaFromCardEffect(owner, 4)
	return nil
}
