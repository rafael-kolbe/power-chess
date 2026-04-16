package match

import (
	"power-chess/internal/chess"
	"power-chess/internal/gameplay"
)

// MovementGrantKind describes a movement pattern extension granted by a resolved effect.
type MovementGrantKind string

const (
	// MovementGrantKnightPattern grants additional knight movement while preserving native moves.
	MovementGrantKnightPattern MovementGrantKind = "knight_pattern"
)

// MovementGrant stores one active piece movement modifier owned by a player.
type MovementGrant struct {
	Owner               gameplay.PlayerID
	SourceCardID        gameplay.CardID
	Target              chess.Pos
	Kind                MovementGrantKind
	RemainingOwnerTurns int
}

// addMovementGrant appends or replaces an active movement grant for the same owner/source/target.
func (e *Engine) addMovementGrant(grant MovementGrant) {
	next := make([]MovementGrant, 0, len(e.movementGrants)+1)
	for _, g := range e.movementGrants {
		if g.Owner == grant.Owner && g.SourceCardID == grant.SourceCardID && g.Target == grant.Target {
			continue
		}
		next = append(next, g)
	}
	next = append(next, grant)
	e.movementGrants = next
}

// canUseAugmentedMovement reports whether an active movement grant allows m for pid.
func (e *Engine) canUseAugmentedMovement(pid gameplay.PlayerID, m chess.Move) bool {
	piece := e.Chess.PieceAt(m.From)
	if piece.IsEmpty() || piece.Color != toColor(pid) {
		return false
	}
	for _, grant := range e.movementGrants {
		if grant.Owner != pid || grant.Target != m.From || grant.RemainingOwnerTurns <= 0 {
			continue
		}
		if !movementGrantMatches(grant.Kind, m.From, m.To) {
			continue
		}
		return true
	}
	return false
}

// advanceMovementGrantPosition moves grants that are attached to the moved piece.
func (e *Engine) advanceMovementGrantPosition(pid gameplay.PlayerID, from, to chess.Pos) {
	for i := range e.movementGrants {
		g := &e.movementGrants[i]
		if g.Owner == pid && g.Target == from {
			g.Target = to
		}
	}
}

// expireMovementGrantsAfterOwnerTurn decrements durations for grants owned by pid.
func (e *Engine) expireMovementGrantsAfterOwnerTurn(pid gameplay.PlayerID) {
	next := make([]MovementGrant, 0, len(e.movementGrants))
	for _, grant := range e.movementGrants {
		if grant.Owner == pid {
			grant.RemainingOwnerTurns--
		}
		if grant.RemainingOwnerTurns > 0 {
			next = append(next, grant)
		}
	}
	e.movementGrants = next
}

// pruneStaleMovementGrants removes grants whose target square no longer holds an owner piece.
func (e *Engine) pruneStaleMovementGrants() {
	next := make([]MovementGrant, 0, len(e.movementGrants))
	for _, grant := range e.movementGrants {
		p := e.Chess.PieceAt(grant.Target)
		if p.IsEmpty() || p.Color != toColor(grant.Owner) {
			continue
		}
		next = append(next, grant)
	}
	e.movementGrants = next
}

// movementGrantMatches checks whether from->to satisfies the grant movement pattern.
func movementGrantMatches(kind MovementGrantKind, from, to chess.Pos) bool {
	if kind != MovementGrantKnightPattern {
		return false
	}
	dr := absInt(from.Row - to.Row)
	dc := absInt(from.Col - to.Col)
	return (dr == 2 && dc == 1) || (dr == 1 && dc == 2)
}

// absInt returns absolute value for small board delta calculations.
func absInt(v int) int {
	if v < 0 {
		return -v
	}
	return v
}
