package power

import (
	"power-chess/internal/chess"
	"power-chess/internal/gameplay"
	"power-chess/internal/match/resolvers"
)

const cardBishopTouch gameplay.CardID = "bishop-touch"

// BishopTouchResolver applies the "bishop-touch" movement enchantment using locked ignite targets.
type BishopTouchResolver struct{}

// RequiresTarget reports whether this resolver waits for resolve_pending_effect input.
func (BishopTouchResolver) RequiresTarget() bool { return false }

// Apply grants bishop-pattern movement to one valid allied non-king/non-bishop piece for EffectDuration owner turns.
func (BishopTouchResolver) Apply(e resolvers.ResolverEngine, owner gameplay.PlayerID, _ resolvers.EffectTarget) error {
	targets := e.ConsumeIgnitionTargets(owner, cardBishopTouch)
	if len(targets) != 1 {
		return nil
	}
	target := targets[0]
	if !isBishopTouchEligiblePiece(e, owner, target) {
		return nil
	}
	def, ok := gameplay.CardDefinitionByID(cardBishopTouch)
	turns := 1
	if ok && def.EffectDuration > 0 {
		turns = def.EffectDuration
	}
	e.AddMovementGrant(owner, cardBishopTouch, target, resolvers.MovementGrantBishopPattern, turns)
	return nil
}

// isBishopTouchEligiblePiece validates target piece ownership and piece-type restriction.
func isBishopTouchEligiblePiece(e resolvers.ResolverEngine, owner gameplay.PlayerID, pos chess.Pos) bool {
	p := e.PieceAt(pos)
	if p.IsEmpty() || p.Color != e.OwnerColor(owner) {
		return false
	}
	return p.Type != chess.King && p.Type != chess.Bishop
}
