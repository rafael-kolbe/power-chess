// Package power contains effect resolvers for Power-type cards.
package power

import (
	"power-chess/internal/chess"
	"power-chess/internal/gameplay"
	"power-chess/internal/match/resolvers"
)

const cardKnightTouch gameplay.CardID = "knight-touch"

// KnightTouchResolver applies the "knight-touch" movement enchantment using locked ignite targets.
type KnightTouchResolver struct{}

// RequiresTarget reports whether this resolver waits for resolve_pending_effect input.
func (KnightTouchResolver) RequiresTarget() bool { return false }

// Apply grants knight-pattern movement to one valid allied non-king/non-knight piece for one owner turn.
func (KnightTouchResolver) Apply(e resolvers.ResolverEngine, owner gameplay.PlayerID, _ resolvers.EffectTarget) error {
	targets := e.ConsumeIgnitionTargets(owner, cardKnightTouch)
	if len(targets) != 1 {
		return nil
	}
	target := targets[0]
	if !isKnightTouchEligiblePiece(e, owner, target) {
		return nil
	}
	def, ok := gameplay.CardDefinitionByID(cardKnightTouch)
	turns := 1
	if ok && def.EffectDuration > 0 {
		turns = def.EffectDuration
	}
	e.AddMovementGrant(owner, cardKnightTouch, target, resolvers.MovementGrantKnightPattern, turns)
	return nil
}

// isKnightTouchEligiblePiece validates target piece ownership and piece-type restriction.
func isKnightTouchEligiblePiece(e resolvers.ResolverEngine, owner gameplay.PlayerID, pos chess.Pos) bool {
	p := e.PieceAt(pos)
	if p.IsEmpty() || p.Color != e.OwnerColor(owner) {
		return false
	}
	return p.Type != chess.King && p.Type != chess.Knight
}
