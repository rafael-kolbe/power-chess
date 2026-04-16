package match

import (
	"power-chess/internal/chess"
	"power-chess/internal/gameplay"
)

// knightTouchResolver applies the "knight-touch" movement enchantment using locked ignite targets.
type knightTouchResolver struct{}

// RequiresTarget reports whether this resolver waits for resolve_pending_effect input.
func (knightTouchResolver) RequiresTarget() bool { return false }

// Apply grants knight-pattern movement to one valid allied non-king/non-knight piece for one owner turn.
func (knightTouchResolver) Apply(e *Engine, owner gameplay.PlayerID, _ EffectTarget) error {
	targets := e.consumeIgnitionTargets(owner, CardKnightTouch)
	if len(targets) != 1 {
		return nil
	}
	target := targets[0]
	if !isKnightTouchEligiblePiece(e.Chess, owner, target) {
		return nil
	}
	def, ok := gameplay.CardDefinitionByID(CardKnightTouch)
	turns := 1
	if ok && def.EffectDuration > 0 {
		turns = def.EffectDuration
	}
	e.addMovementGrant(MovementGrant{
		Owner:               owner,
		SourceCardID:        CardKnightTouch,
		Target:              target,
		Kind:                MovementGrantKnightPattern,
		RemainingOwnerTurns: turns,
	})
	return nil
}

// isKnightTouchEligiblePiece validates the target piece ownership and piece-type restriction.
func isKnightTouchEligiblePiece(board *chess.Game, owner gameplay.PlayerID, pos chess.Pos) bool {
	p := board.PieceAt(pos)
	if p.IsEmpty() || p.Color != toColor(owner) {
		return false
	}
	return p.Type != chess.King && p.Type != chess.Knight
}
