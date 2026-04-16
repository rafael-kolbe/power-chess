package chess

import "testing"

func TestPawnDoubleStepAndEnPassant(t *testing.T) {
	g := NewEmptyGame(White)
	g.SetPiece(Pos{7, 4}, Piece{King, White})
	g.SetPiece(Pos{0, 4}, Piece{King, Black})
	g.SetPiece(Pos{3, 4}, Piece{Pawn, White})
	g.SetPiece(Pos{1, 5}, Piece{Pawn, Black})

	// Black double step to enable en passant.
	g.Turn = Black
	if err := g.ApplyMove(Move{From: Pos{1, 5}, To: Pos{3, 5}}); err != nil {
		t.Fatalf("black double step failed: %v", err)
	}
	// White en passant.
	if err := g.ApplyMove(Move{From: Pos{3, 4}, To: Pos{2, 5}}); err != nil {
		t.Fatalf("en passant failed: %v", err)
	}
	if !g.PieceAt(Pos{2, 5}).Type.Equals(Pawn) {
		t.Fatalf("white pawn should be on capture square")
	}
	if !g.PieceAt(Pos{3, 5}).IsEmpty() {
		t.Fatalf("captured pawn should be removed")
	}
}

func TestCastlingKingSide(t *testing.T) {
	g := NewEmptyGame(White)
	g.SetPiece(Pos{7, 4}, Piece{King, White})
	g.SetPiece(Pos{7, 7}, Piece{Rook, White})
	g.SetPiece(Pos{0, 4}, Piece{King, Black})
	g.CastlingRights.WhiteKingSide = true

	if err := g.ApplyMove(Move{From: Pos{7, 4}, To: Pos{7, 6}}); err != nil {
		t.Fatalf("castling failed: %v", err)
	}
	if g.PieceAt(Pos{7, 6}).Type != King || g.PieceAt(Pos{7, 5}).Type != Rook {
		t.Fatalf("castling pieces not moved correctly")
	}
}

func TestCheckDetection(t *testing.T) {
	g := NewEmptyGame(White)
	g.SetPiece(Pos{7, 4}, Piece{King, White})
	g.SetPiece(Pos{0, 4}, Piece{King, Black})
	g.SetPiece(Pos{3, 4}, Piece{Rook, Black})

	if !g.IsCheck(White) {
		t.Fatalf("expected white to be in check")
	}
}

func TestCheckmateDetectionSimple(t *testing.T) {
	g := NewEmptyGame(White)
	g.SetPiece(Pos{7, 0}, Piece{King, White})  // a1
	g.SetPiece(Pos{5, 2}, Piece{King, Black})  // c3
	g.SetPiece(Pos{6, 1}, Piece{Queen, Black}) // b2
	g.SetPiece(Pos{6, 0}, Piece{Rook, Black})  // a2

	if !g.IsCheckmate(White) {
		t.Fatalf("expected checkmate")
	}
}

func TestKingCannotBeCaptured(t *testing.T) {
	g := NewEmptyGame(White)
	g.SetPiece(Pos{7, 4}, Piece{King, White})
	g.SetPiece(Pos{0, 4}, Piece{King, Black})
	g.SetPiece(Pos{1, 4}, Piece{Rook, White})

	if err := g.ApplyMove(Move{From: Pos{1, 4}, To: Pos{0, 4}}); err != ErrKingCannotBeCaptured {
		t.Fatalf("capturing king must return ErrKingCannotBeCaptured, got: %v", err)
	}

	g2 := NewEmptyGame(White)
	g2.SetPiece(Pos{7, 4}, Piece{King, White})
	g2.SetPiece(Pos{0, 4}, Piece{King, Black})
	g2.SetPiece(Pos{2, 3}, Piece{Bishop, White})
	if err := g2.ApplyPseudoLegalMove(Move{From: Pos{2, 3}, To: Pos{0, 4}}); err != ErrKingCannotBeCaptured {
		t.Fatalf("expected ErrKingCannotBeCaptured, got: %v", err)
	}
}

func (p PieceType) Equals(other PieceType) bool { return p == other }
