package match

import (
	"power-chess/internal/chess"
	"power-chess/internal/gameplay"
)

// bishopTouchResolver applies the "bishop-touch" movement enchantment using locked ignite targets.
type bishopTouchResolver struct{}

// RequiresTarget reports whether this resolver waits for resolve_pending_effect input.
func (bishopTouchResolver) RequiresTarget() bool { return false }

// Apply grants bishop-pattern movement to one valid allied non-king/non-bishop piece for EffectDuration owner turns.
func (bishopTouchResolver) Apply(e *Engine, owner gameplay.PlayerID, _ EffectTarget) error {
	targets := e.consumeIgnitionTargets(owner, CardBishopTouch)
	if len(targets) != 1 {
		return nil
	}
	target := targets[0]
	if !isBishopTouchEligiblePiece(e.Chess, owner, target) {
		return nil
	}
	def, ok := gameplay.CardDefinitionByID(CardBishopTouch)
	turns := 1
	if ok && def.EffectDuration > 0 {
		turns = def.EffectDuration
	}
	e.addMovementGrant(MovementGrant{
		Owner:               owner,
		SourceCardID:        CardBishopTouch,
		Target:              target,
		Kind:                MovementGrantBishopPattern,
		RemainingOwnerTurns: turns,
	})
	return nil
}

// isBishopTouchEligiblePiece validates the target piece ownership and piece-type restriction.
func isBishopTouchEligiblePiece(board *chess.Game, owner gameplay.PlayerID, pos chess.Pos) bool {
	p := board.PieceAt(pos)
	if p.IsEmpty() || p.Color != toColor(owner) {
		return false
	}
	return p.Type != chess.King && p.Type != chess.Bishop
}
