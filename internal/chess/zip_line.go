package chess

import (
	"fmt"
)

// TeleportZipLine moves a non-king piece from from to an empty to on the same rank (row).
// It does not validate chess legality beyond same-rank empty destination, king exclusion, and
// bounds. It clears en passant state, promotes a landing pawn to queen by default, and updates
// castling rights when a rook leaves its starting corner. It does not advance g.Turn.
func (g *Game) TeleportZipLine(from, to Pos) error {
	if !from.InBounds() || !to.InBounds() {
		return fmt.Errorf("zip line teleport out of bounds")
	}
	if from.Row != to.Row {
		return fmt.Errorf("zip line must stay on the same row")
	}
	if from == to {
		return fmt.Errorf("zip line requires different squares")
	}
	p := g.PieceAt(from)
	if p.IsEmpty() {
		return fmt.Errorf("no piece on zip line source")
	}
	if p.Type == King {
		return fmt.Errorf("zip line cannot move the king")
	}
	dest := g.PieceAt(to)
	if !dest.IsEmpty() {
		return fmt.Errorf("zip line destination must be empty")
	}
	m := Move{From: from, To: to}
	g.EnPassant = EnPassantState{}
	g.SetPiece(from, Piece{})
	if p.Type == Pawn && (to.Row == 0 || to.Row == 7) {
		if m.Promotion == NoPiece {
			m.Promotion = Queen
		}
		p.Type = m.Promotion
	}
	g.SetPiece(to, p)
	g.updateCastlingRights(m, p)
	return nil
}
