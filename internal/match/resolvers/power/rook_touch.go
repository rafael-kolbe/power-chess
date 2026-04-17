package power

import (
	"power-chess/internal/chess"
	"power-chess/internal/gameplay"
	"power-chess/internal/match/resolvers"
)

const cardRookTouch gameplay.CardID = "rook-touch"

// RookTouchResolver applies the "rook-touch" movement enchantment using locked ignite targets.
type RookTouchResolver struct{}

// RequiresTarget reports whether this resolver waits for resolve_pending_effect input.
func (RookTouchResolver) RequiresTarget() bool { return false }

// Apply grants rook-pattern movement to one valid allied non-king/non-rook piece for EffectDuration owner turns.
func (RookTouchResolver) Apply(e resolvers.ResolverEngine, owner gameplay.PlayerID, _ resolvers.EffectTarget) error {
	targets := e.ConsumeIgnitionTargets(owner, cardRookTouch)
	if len(targets) != 1 {
		return nil
	}
	target := targets[0]
	if !isRookTouchEligiblePiece(e, owner, target) {
		return nil
	}
	def, ok := gameplay.CardDefinitionByID(cardRookTouch)
	turns := 1
	if ok && def.EffectDuration > 0 {
		turns = def.EffectDuration
	}
	e.AddMovementGrant(owner, cardRookTouch, target, resolvers.MovementGrantRookPattern, turns)
	return nil
}

// isRookTouchEligiblePiece validates target piece ownership and piece-type restriction.
func isRookTouchEligiblePiece(e resolvers.ResolverEngine, owner gameplay.PlayerID, pos chess.Pos) bool {
	p := e.PieceAt(pos)
	if p.IsEmpty() || p.Color != e.OwnerColor(owner) {
		return false
	}
	return p.Type != chess.King && p.Type != chess.Rook
}
