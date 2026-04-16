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
	// MovementGrantBishopPattern grants additional bishop-line movement (one diagonal step for pawns).
	MovementGrantBishopPattern MovementGrantKind = "bishop_pattern"
	// MovementGrantRookPattern grants additional rook-line movement (one orthogonal step for pawns).
	MovementGrantRookPattern MovementGrantKind = "rook_pattern"
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
		if !e.movementGrantMatchesMove(grant, m) {
			continue
		}
		return true
	}
	return false
}

// movementGrantMatchesMove checks whether m's geometry matches the grant pattern for the piece on m.From.
func (e *Engine) movementGrantMatchesMove(grant MovementGrant, m chess.Move) bool {
	moving := e.Chess.PieceAt(m.From)
	switch grant.Kind {
	case MovementGrantKnightPattern:
		dr := absInt(m.From.Row - m.To.Row)
		dc := absInt(m.From.Col - m.To.Col)
		return (dr == 2 && dc == 1) || (dr == 1 && dc == 2)
	case MovementGrantBishopPattern:
		return bishopTouchMovePatternLegal(e.Chess, moving, m.From, m.To)
	case MovementGrantRookPattern:
		return rookTouchMovePatternLegal(e.Chess, moving, m.From, m.To)
	default:
		return false
	}
}

// bishopTouchMovePatternLegal reports whether from->to follows bishop lines: sliding with a clear path
// for non-pawns, or exactly one diagonal step for pawns (empty or enemy capture). Own-piece
// destinations are rejected; king capture is not validated here (ApplyPseudoLegalMove enforces it).
func bishopTouchMovePatternLegal(g *chess.Game, moving chess.Piece, from, to chess.Pos) bool {
	dr := to.Row - from.Row
	dc := to.Col - from.Col
	adr := absInt(dr)
	adc := absInt(dc)
	if adr != adc || adr == 0 {
		return false
	}
	dest := g.PieceAt(to)
	if !dest.IsEmpty() && dest.Color == moving.Color {
		return false
	}
	if moving.Type == chess.Pawn {
		return adr == 1
	}
	stepR := dr / adr
	stepC := dc / adc
	for r, c, i := from.Row+stepR, from.Col+stepC, 1; i < adr; i, r, c = i+1, r+stepR, c+stepC {
		if !g.PieceAt(chess.Pos{Row: r, Col: c}).IsEmpty() {
			return false
		}
	}
	return true
}

// rookTouchMovePatternLegal reports whether from->to follows rook lines: sliding with a clear path
// for non-pawns, or exactly one orthogonal step for pawns (empty or enemy capture).
func rookTouchMovePatternLegal(g *chess.Game, moving chess.Piece, from, to chess.Pos) bool {
	dr := to.Row - from.Row
	dc := to.Col - from.Col
	adr := absInt(dr)
	adc := absInt(dc)
	if !((adr > 0 && adc == 0) || (adc > 0 && adr == 0)) {
		return false
	}
	dest := g.PieceAt(to)
	if !dest.IsEmpty() && dest.Color == moving.Color {
		return false
	}
	if moving.Type == chess.Pawn {
		return adr+adc == 1
	}
	var stepR, stepC int
	if adr > 0 {
		stepR = dr / adr
	}
	if adc > 0 {
		stepC = dc / adc
	}
	steps := adr + adc
	for r, c, i := from.Row+stepR, from.Col+stepC, 1; i < steps; i, r, c = i+1, r+stepR, c+stepC {
		if !g.PieceAt(chess.Pos{Row: r, Col: c}).IsEmpty() {
			return false
		}
	}
	return true
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

// absInt returns absolute value for small board delta calculations.
func absInt(v int) int {
	if v < 0 {
		return -v
	}
	return v
}
