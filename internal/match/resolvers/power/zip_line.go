package power

import (
	"power-chess/internal/gameplay"
	"power-chess/internal/match/resolvers"
)

const cardZipLine gameplay.CardID = "zip-line"

// ZipLineResolver applies the "zip-line" effect: teleport the locked piece to an empty square on
// the same rank, then consume the owner's chess turn.
type ZipLineResolver struct{}

// RequiresTarget reports that the destination square is chosen via resolve_pending_effect.
func (ZipLineResolver) RequiresTarget() bool { return true }

// Apply teleports the locked source piece to target.TargetPos using the engine hook.
func (ZipLineResolver) Apply(e resolvers.ResolverEngine, owner gameplay.PlayerID, target resolvers.EffectTarget) error {
	if target.PiecePos == nil || target.TargetPos == nil {
		return resolvers.ErrEffectFailed
	}
	from := *target.PiecePos
	to := *target.TargetPos
	return e.ApplyZipLineTeleport(owner, from, to)
}
