package counter

import (
	"power-chess/internal/gameplay"
	"power-chess/internal/match/resolvers"
)

// BlockadeResolver negates an attacker-removal Counter and locks the original attacker.
type BlockadeResolver struct{}

// RequiresTarget reports that Blockade uses the pending capture chain and needs no target payload.
func (BlockadeResolver) RequiresTarget() bool { return false }

// Apply resolves Blockade against the current pending capture chain.
func (BlockadeResolver) Apply(e resolvers.ResolverEngine, owner gameplay.PlayerID, _ resolvers.EffectTarget) error {
	return e.ResolveBlockade(owner)
}
