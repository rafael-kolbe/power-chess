package power

import (
	"power-chess/internal/gameplay"
	"power-chess/internal/match/resolvers"
)

const cardSacrificeOfTheMasses gameplay.CardID = "sacrifice-of-the-masses"

// SacrificeOfTheMassesResolver applies the pawn sacrifice reward effect.
type SacrificeOfTheMassesResolver struct{}

// RequiresTarget reports whether this resolver waits for resolve_pending_effect input.
func (SacrificeOfTheMassesResolver) RequiresTarget() bool { return false }

// Apply sacrifices the locked owned pawn into the opponent capture zone, grants 6 mana, and draws 2 cards.
func (SacrificeOfTheMassesResolver) Apply(e resolvers.ResolverEngine, owner gameplay.PlayerID, _ resolvers.EffectTarget) error {
	targets := e.ConsumeIgnitionTargets(owner, cardSacrificeOfTheMasses)
	if len(targets) != 1 {
		return resolvers.ErrEffectFailed
	}
	if err := e.ApplyPawnSacrificeReward(owner, targets[0], 6, 2); err != nil {
		return resolvers.ErrEffectFailed
	}
	return nil
}
