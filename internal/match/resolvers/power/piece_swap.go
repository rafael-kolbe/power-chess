package power

import (
	"power-chess/internal/gameplay"
	"power-chess/internal/match/resolvers"
)

const cardPieceSwap gameplay.CardID = "piece-swap"

// PieceSwapResolver applies the "piece-swap" effect: exchanges the board positions of the
// two pieces locked as ignition targets (own non-king + opponent non-king within 2 squares).
type PieceSwapResolver struct{}

// RequiresTarget reports whether this resolver waits for resolve_pending_effect input.
func (PieceSwapResolver) RequiresTarget() bool { return false }

// Apply swaps the two pieces locked as ignition targets for the activating player.
// Validation (distance, king exclusion, self-check) was already enforced at activation time;
// Apply performs the swap unconditionally if exactly two locked targets are present.
func (PieceSwapResolver) Apply(e resolvers.ResolverEngine, owner gameplay.PlayerID, _ resolvers.EffectTarget) error {
	targets := e.ConsumeIgnitionTargets(owner, cardPieceSwap)
	if len(targets) != 2 {
		return nil
	}
	e.SwapPieces(targets[0], targets[1])
	return nil
}
