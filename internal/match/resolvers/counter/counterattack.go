package counter

import (
	"power-chess/internal/gameplay"
	"power-chess/internal/match/resolvers"
)

// CounterattackResolver captures a Power-buffed attacking piece instead of the defender.
type CounterattackResolver struct{}

// RequiresTarget reports that Counterattack uses the pending capture and needs no target payload.
func (CounterattackResolver) RequiresTarget() bool { return false }

// Apply resolves Counterattack against the current pending capture attempt.
func (CounterattackResolver) Apply(e resolvers.ResolverEngine, owner gameplay.PlayerID, _ resolvers.EffectTarget) error {
	return e.ResolveCounterattack(owner)
}
