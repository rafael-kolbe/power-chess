package match

import (
	"power-chess/internal/chess"
	"power-chess/internal/gameplay"
)

// rookTouchResolver applies the "rook-touch" movement enchantment using locked ignite targets.
type rookTouchResolver struct{}

// RequiresTarget reports whether this resolver waits for resolve_pending_effect input.
func (rookTouchResolver) RequiresTarget() bool { return false }

// Apply grants rook-pattern movement to one valid allied non-king/non-rook piece for EffectDuration owner turns.
func (rookTouchResolver) Apply(e *Engine, owner gameplay.PlayerID, _ EffectTarget) error {
	targets := e.consumeIgnitionTargets(owner, CardRookTouch)
	if len(targets) != 1 {
		return nil
	}
	target := targets[0]
	if !isRookTouchEligiblePiece(e.Chess, owner, target) {
		return nil
	}
	def, ok := gameplay.CardDefinitionByID(CardRookTouch)
	turns := 1
	if ok && def.EffectDuration > 0 {
		turns = def.EffectDuration
	}
	e.addMovementGrant(MovementGrant{
		Owner:               owner,
		SourceCardID:        CardRookTouch,
		Target:              target,
		Kind:                MovementGrantRookPattern,
		RemainingOwnerTurns: turns,
	})
	return nil
}

// isRookTouchEligiblePiece validates the target piece ownership and piece-type restriction.
func isRookTouchEligiblePiece(board *chess.Game, owner gameplay.PlayerID, pos chess.Pos) bool {
	p := board.PieceAt(pos)
	if p.IsEmpty() || p.Color != toColor(owner) {
		return false
	}
	return p.Type != chess.King && p.Type != chess.Rook
}
