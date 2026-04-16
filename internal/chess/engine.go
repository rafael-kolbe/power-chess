package chess

import (
	"errors"
	"fmt"
)

var ErrKingCannotBeCaptured = errors.New("king cannot be captured")

// Color identifies player side in chess.
type Color int

const (
	White Color = iota
	Black
)

func (c Color) Opponent() Color {
	if c == White {
		return Black
	}
	return White
}

// PieceType identifies chess piece role.
type PieceType int

const (
	NoPiece PieceType = iota
	Pawn
	Knight
	Bishop
	Rook
	Queen
	King
)

type Piece struct {
	Type  PieceType
	Color Color
}

// IsEmpty reports whether this board square has no piece.
func (p Piece) IsEmpty() bool { return p.Type == NoPiece }

// Pos is a board coordinate (0-based).
type Pos struct {
	Row int
	Col int
}

// InBounds reports whether a position is inside the board limits.
func (p Pos) InBounds() bool { return p.Row >= 0 && p.Row < 8 && p.Col >= 0 && p.Col < 8 }

// Move describes a source and destination coordinate plus optional promotion.
type Move struct {
	From      Pos
	To        Pos
	Promotion PieceType
}

// CastlingRights stores available castling options for both colors.
type CastlingRights struct {
	WhiteKingSide  bool
	WhiteQueenSide bool
	BlackKingSide  bool
	BlackQueenSide bool
}

// EnPassantState tracks current en-passant capture availability.
type EnPassantState struct {
	Target  Pos
	PawnPos Pos
	Valid   bool
}

// Game contains full board state and rule flags for a chess match.
type Game struct {
	Board          [8][8]Piece
	Turn           Color
	CastlingRights CastlingRights
	EnPassant      EnPassantState
}

// NewGame creates a standard chess initial position.
func NewGame() *Game {
	g := &Game{
		Turn: White,
		CastlingRights: CastlingRights{
			WhiteKingSide:  true,
			WhiteQueenSide: true,
			BlackKingSide:  true,
			BlackQueenSide: true,
		},
	}
	back := []PieceType{Rook, Knight, Bishop, Queen, King, Bishop, Knight, Rook}
	for c := 0; c < 8; c++ {
		g.Board[0][c] = Piece{Type: back[c], Color: Black}
		g.Board[1][c] = Piece{Type: Pawn, Color: Black}
		g.Board[6][c] = Piece{Type: Pawn, Color: White}
		g.Board[7][c] = Piece{Type: back[c], Color: White}
	}
	return g
}

// NewEmptyGame creates an empty board state for tests and custom setups.
func NewEmptyGame(turn Color) *Game {
	return &Game{Turn: turn}
}

func (g *Game) clone() *Game {
	cp := *g
	return &cp
}

// Clone returns a shallow copy of the game state.
func (g *Game) Clone() *Game {
	return g.clone()
}

// PieceAt returns the piece at a board position.
func (g *Game) PieceAt(p Pos) Piece { return g.Board[p.Row][p.Col] }

// SetPiece places a piece at a board position.
func (g *Game) SetPiece(p Pos, piece Piece) {
	g.Board[p.Row][p.Col] = piece
}

// ApplyMove validates and applies a regular chess move.
func (g *Game) ApplyMove(m Move) error {
	if !m.From.InBounds() || !m.To.InBounds() {
		return fmt.Errorf("move out of bounds")
	}
	p := g.PieceAt(m.From)
	if p.IsEmpty() {
		return fmt.Errorf("no piece on source")
	}
	if p.Color != g.Turn {
		return fmt.Errorf("wrong turn")
	}
	if t := g.PieceAt(m.To); !t.IsEmpty() && t.Type == King && t.Color != p.Color {
		return ErrKingCannotBeCaptured
	}

	legal := false
	for _, cand := range g.LegalMovesFrom(m.From) {
		if cand.To == m.To {
			m.Promotion = cand.Promotion
			legal = true
			break
		}
	}
	if !legal {
		return fmt.Errorf("illegal move")
	}

	g.applyUnchecked(m)
	g.Turn = g.Turn.Opponent()
	return nil
}

// ApplyPseudoLegalMove applies a power-modified movement after caller-side pattern validation.
// It still enforces turn ownership, no own-piece capture, no king capture, and self-check constraints.
func (g *Game) ApplyPseudoLegalMove(m Move) error {
	if !m.From.InBounds() || !m.To.InBounds() {
		return fmt.Errorf("move out of bounds")
	}
	p := g.PieceAt(m.From)
	if p.IsEmpty() {
		return fmt.Errorf("no piece on source")
	}
	if p.Color != g.Turn {
		return fmt.Errorf("wrong turn")
	}
	t := g.PieceAt(m.To)
	if t.Type == King {
		return ErrKingCannotBeCaptured
	}
	if !t.IsEmpty() && t.Color == p.Color {
		return fmt.Errorf("cannot capture own piece")
	}
	cp := g.clone()
	cp.applyUnchecked(m)
	if cp.IsCheck(p.Color) {
		return fmt.Errorf("move leaves king in check")
	}
	g.applyUnchecked(m)
	g.Turn = g.Turn.Opponent()
	return nil
}

func (g *Game) applyUnchecked(m Move) {
	prevEP := g.EnPassant
	g.EnPassant = EnPassantState{}
	p := g.PieceAt(m.From)
	target := g.PieceAt(m.To)

	// En passant capture.
	if p.Type == Pawn && target.IsEmpty() && m.From.Col != m.To.Col && prevEP.Valid && m.To == prevEP.Target {
		g.SetPiece(prevEP.PawnPos, Piece{})
	}

	// Castling rook movement.
	if p.Type == King && abs(m.To.Col-m.From.Col) == 2 {
		if m.To.Col > m.From.Col {
			rf, rt := Pos{Row: m.From.Row, Col: 7}, Pos{Row: m.From.Row, Col: 5}
			r := g.PieceAt(rf)
			g.SetPiece(rf, Piece{})
			g.SetPiece(rt, r)
		} else {
			rf, rt := Pos{Row: m.From.Row, Col: 0}, Pos{Row: m.From.Row, Col: 3}
			r := g.PieceAt(rf)
			g.SetPiece(rf, Piece{})
			g.SetPiece(rt, r)
		}
	}

	// Pawn double step creates en passant target.
	if p.Type == Pawn && abs(m.To.Row-m.From.Row) == 2 {
		g.EnPassant = EnPassantState{
			Target:  Pos{Row: (m.To.Row + m.From.Row) / 2, Col: m.From.Col},
			PawnPos: m.To,
			Valid:   true,
		}
	}

	g.SetPiece(m.From, Piece{})
	if p.Type == Pawn && (m.To.Row == 0 || m.To.Row == 7) {
		if m.Promotion == NoPiece {
			m.Promotion = Queen
		}
		p.Type = m.Promotion
	}
	g.SetPiece(m.To, p)

	g.updateCastlingRights(m, p)
}

func (g *Game) updateCastlingRights(m Move, moved Piece) {
	if moved.Type == King {
		if moved.Color == White {
			g.CastlingRights.WhiteKingSide, g.CastlingRights.WhiteQueenSide = false, false
		} else {
			g.CastlingRights.BlackKingSide, g.CastlingRights.BlackQueenSide = false, false
		}
	}
	if moved.Type == Rook {
		switch m.From {
		case (Pos{Row: 7, Col: 0}):
			g.CastlingRights.WhiteQueenSide = false
		case (Pos{Row: 7, Col: 7}):
			g.CastlingRights.WhiteKingSide = false
		case (Pos{Row: 0, Col: 0}):
			g.CastlingRights.BlackQueenSide = false
		case (Pos{Row: 0, Col: 7}):
			g.CastlingRights.BlackKingSide = false
		}
	}
}

// LegalMovesFrom returns all legal moves from a position for the current turn.
func (g *Game) LegalMovesFrom(from Pos) []Move {
	p := g.PieceAt(from)
	if p.IsEmpty() || p.Color != g.Turn {
		return nil
	}
	cands := g.pseudoMovesFrom(from)
	out := make([]Move, 0, len(cands))
	for _, m := range cands {
		target := g.PieceAt(m.To)
		if target.Type == King {
			continue
		}
		cp := g.clone()
		cp.applyUnchecked(m)
		if !cp.IsCheck(p.Color) {
			out = append(out, m)
		}
	}
	return out
}

// IsCheck reports whether the specified color is currently in check.
func (g *Game) IsCheck(color Color) bool {
	king, ok := g.findKing(color)
	if !ok {
		return false
	}
	return g.IsSquareAttacked(king, color.Opponent())
}

// IsCheckmate reports whether the specified color is checkmated.
func (g *Game) IsCheckmate(color Color) bool {
	if !g.IsCheck(color) {
		return false
	}
	return !g.hasAnyLegalMove(color)
}

// IsStalemate reports whether the specified color is stalemated.
func (g *Game) IsStalemate(color Color) bool {
	if g.IsCheck(color) {
		return false
	}
	return !g.hasAnyLegalMove(color)
}

func (g *Game) hasAnyLegalMove(color Color) bool {
	orig := g.Turn
	g.Turn = color
	defer func() { g.Turn = orig }()

	for r := 0; r < 8; r++ {
		for c := 0; c < 8; c++ {
			pos := Pos{Row: r, Col: c}
			p := g.PieceAt(pos)
			if !p.IsEmpty() && p.Color == color && len(g.LegalMovesFrom(pos)) > 0 {
				return true
			}
		}
	}
	return false
}

func (g *Game) findKing(color Color) (Pos, bool) {
	for r := 0; r < 8; r++ {
		for c := 0; c < 8; c++ {
			p := g.Board[r][c]
			if p.Type == King && p.Color == color {
				return Pos{Row: r, Col: c}, true
			}
		}
	}
	return Pos{}, false
}

// IsSquareAttacked reports whether a square is attacked by the given color.
func (g *Game) IsSquareAttacked(square Pos, by Color) bool {
	for r := 0; r < 8; r++ {
		for c := 0; c < 8; c++ {
			from := Pos{Row: r, Col: c}
			p := g.PieceAt(from)
			if p.IsEmpty() || p.Color != by {
				continue
			}
			for _, m := range g.pseudoMovesForAttack(from) {
				if m.To == square {
					return true
				}
			}
		}
	}
	return false
}

func (g *Game) pseudoMovesFrom(from Pos) []Move {
	p := g.PieceAt(from)
	switch p.Type {
	case Pawn:
		return g.pawnMoves(from, p.Color, false)
	case Knight:
		return g.knightMoves(from, p.Color)
	case Bishop:
		return g.rayMoves(from, p.Color, []Pos{{1, 1}, {1, -1}, {-1, 1}, {-1, -1}})
	case Rook:
		return g.rayMoves(from, p.Color, []Pos{{1, 0}, {-1, 0}, {0, 1}, {0, -1}})
	case Queen:
		return g.rayMoves(from, p.Color, []Pos{{1, 1}, {1, -1}, {-1, 1}, {-1, -1}, {1, 0}, {-1, 0}, {0, 1}, {0, -1}})
	case King:
		return g.kingMoves(from, p.Color)
	default:
		return nil
	}
}

func (g *Game) pseudoMovesForAttack(from Pos) []Move {
	p := g.PieceAt(from)
	switch p.Type {
	case Pawn:
		return g.pawnMoves(from, p.Color, true)
	case King:
		return g.kingAttackMoves(from, p.Color)
	default:
		return g.pseudoMovesFrom(from)
	}
}

func (g *Game) pawnMoves(from Pos, color Color, attackOnly bool) []Move {
	dir, startRow, promoRow := -1, 6, 0
	if color == Black {
		dir, startRow, promoRow = 1, 1, 7
	}
	moves := []Move{}

	// Captures (or attack-only).
	for _, dc := range []int{-1, 1} {
		to := Pos{Row: from.Row + dir, Col: from.Col + dc}
		if !to.InBounds() {
			continue
		}
		target := g.PieceAt(to)
		if attackOnly {
			moves = append(moves, Move{From: from, To: to})
			continue
		}
		if !target.IsEmpty() && target.Color != color {
			mv := Move{From: from, To: to}
			if to.Row == promoRow {
				mv.Promotion = Queen
			}
			moves = append(moves, mv)
		}
		if g.EnPassant.Valid && to == g.EnPassant.Target {
			moves = append(moves, Move{From: from, To: to})
		}
	}
	if attackOnly {
		return moves
	}

	one := Pos{Row: from.Row + dir, Col: from.Col}
	if one.InBounds() && g.PieceAt(one).IsEmpty() {
		mv := Move{From: from, To: one}
		if one.Row == promoRow {
			mv.Promotion = Queen
		}
		moves = append(moves, mv)
		two := Pos{Row: from.Row + 2*dir, Col: from.Col}
		if from.Row == startRow && g.PieceAt(two).IsEmpty() {
			moves = append(moves, Move{From: from, To: two})
		}
	}
	return moves
}

func (g *Game) knightMoves(from Pos, color Color) []Move {
	deltas := []Pos{{2, 1}, {2, -1}, {-2, 1}, {-2, -1}, {1, 2}, {1, -2}, {-1, 2}, {-1, -2}}
	out := []Move{}
	for _, d := range deltas {
		to := Pos{Row: from.Row + d.Row, Col: from.Col + d.Col}
		if !to.InBounds() {
			continue
		}
		t := g.PieceAt(to)
		if t.IsEmpty() || t.Color != color {
			out = append(out, Move{From: from, To: to})
		}
	}
	return out
}

func (g *Game) rayMoves(from Pos, color Color, dirs []Pos) []Move {
	out := []Move{}
	for _, d := range dirs {
		r, c := from.Row+d.Row, from.Col+d.Col
		for (Pos{Row: r, Col: c}).InBounds() {
			to := Pos{Row: r, Col: c}
			t := g.PieceAt(to)
			if t.IsEmpty() {
				out = append(out, Move{From: from, To: to})
			} else {
				if t.Color != color {
					out = append(out, Move{From: from, To: to})
				}
				break
			}
			r += d.Row
			c += d.Col
		}
	}
	return out
}

func (g *Game) kingAttackMoves(from Pos, color Color) []Move {
	out := []Move{}
	for dr := -1; dr <= 1; dr++ {
		for dc := -1; dc <= 1; dc++ {
			if dr == 0 && dc == 0 {
				continue
			}
			to := Pos{Row: from.Row + dr, Col: from.Col + dc}
			if !to.InBounds() {
				continue
			}
			t := g.PieceAt(to)
			if t.IsEmpty() || t.Color != color {
				out = append(out, Move{From: from, To: to})
			}
		}
	}
	return out
}

func (g *Game) kingMoves(from Pos, color Color) []Move {
	out := g.kingAttackMoves(from, color)
	row := 7
	if color == Black {
		row = 0
	}
	if from != (Pos{Row: row, Col: 4}) || g.IsCheck(color) {
		return out
	}

	// King side.
	if (color == White && g.CastlingRights.WhiteKingSide) || (color == Black && g.CastlingRights.BlackKingSide) {
		if g.PieceAt(Pos{Row: row, Col: 5}).IsEmpty() && g.PieceAt(Pos{Row: row, Col: 6}).IsEmpty() &&
			!g.IsSquareAttacked(Pos{Row: row, Col: 5}, color.Opponent()) &&
			!g.IsSquareAttacked(Pos{Row: row, Col: 6}, color.Opponent()) {
			out = append(out, Move{From: from, To: Pos{Row: row, Col: 6}})
		}
	}
	// Queen side.
	if (color == White && g.CastlingRights.WhiteQueenSide) || (color == Black && g.CastlingRights.BlackQueenSide) {
		if g.PieceAt(Pos{Row: row, Col: 1}).IsEmpty() && g.PieceAt(Pos{Row: row, Col: 2}).IsEmpty() && g.PieceAt(Pos{Row: row, Col: 3}).IsEmpty() &&
			!g.IsSquareAttacked(Pos{Row: row, Col: 3}, color.Opponent()) &&
			!g.IsSquareAttacked(Pos{Row: row, Col: 2}, color.Opponent()) {
			out = append(out, Move{From: from, To: Pos{Row: row, Col: 2}})
		}
	}
	return out
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}
