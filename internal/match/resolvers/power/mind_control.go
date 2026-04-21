package power

import (
	"power-chess/internal/chess"
	"power-chess/internal/gameplay"
	"power-chess/internal/match/resolvers"
)

const cardMindControl gameplay.CardID = "mind-control"

// MindControlResolver applies the "mind-control" effect using locked ignition targets.
type MindControlResolver struct{}

// RequiresTarget reports whether this resolver waits for resolve_pending_effect input.
func (MindControlResolver) RequiresTarget() bool { return false }

// Apply transfers control of one valid opponent non-king/non-queen piece for EffectDuration turns.
// If the locked target no longer exists or is invalid when ignition resolves, activation fails.
func (MindControlResolver) Apply(e resolvers.ResolverEngine, owner gameplay.PlayerID, _ resolvers.EffectTarget) error {
	targets := e.ConsumeIgnitionTargets(owner, cardMindControl)
	if len(targets) != 1 {
		return resolvers.ErrEffectFailed
	}
	target := targets[0]
	p := e.PieceAt(target)
	if p.IsEmpty() || p.Color == e.OwnerColor(owner) || p.Type == chess.King || p.Type == chess.Queen {
		return resolvers.ErrEffectFailed
	}
	def, ok := gameplay.CardDefinitionByID(cardMindControl)
	turns := 3
	if ok && def.EffectDuration > 0 {
		turns = def.EffectDuration
	}
	return e.AddMindControlEffect(owner, cardMindControl, target, turns)
}
