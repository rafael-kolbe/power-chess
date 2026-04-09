package server

import (
	"testing"

	"power-chess/internal/chess"
	"power-chess/internal/gameplay"
)

// TestSnapshotIncludesEnPassantFields maps engine en-passant state into the HUD payload.
func TestSnapshotIncludesEnPassantFields(t *testing.T) {
	room, err := NewRoomSession("room-ep-snap")
	if err != nil {
		t.Fatalf(newRoomFailedFmt, err)
	}
	room.RegisterPlayerConnection(gameplay.PlayerA)
	room.RegisterPlayerConnection(gameplay.PlayerB)

	room.Engine.Chess.EnPassant = chess.EnPassantState{
		Valid:   true,
		Target:  chess.Pos{Row: 2, Col: 5},
		PawnPos: chess.Pos{Row: 2, Col: 4},
	}

	s := room.SnapshotSafe()
	if !s.EnPassant.Valid {
		t.Fatalf("expected enPassant.valid in snapshot")
	}
	if s.EnPassant.TargetRow != 2 || s.EnPassant.TargetCol != 5 {
		t.Fatalf("target mismatch: %+v", s.EnPassant)
	}
	if s.EnPassant.PawnRow != 2 || s.EnPassant.PawnCol != 4 {
		t.Fatalf("pawn pos mismatch: %+v", s.EnPassant)
	}
}

// TestSnapshotIncludesCastlingRights maps chess castling flags into snapshot payload.
func TestSnapshotIncludesCastlingRights(t *testing.T) {
	room, err := NewRoomSession("room-castling-rights-snap")
	if err != nil {
		t.Fatalf(newRoomFailedFmt, err)
	}
	room.RegisterPlayerConnection(gameplay.PlayerA)
	room.RegisterPlayerConnection(gameplay.PlayerB)
	room.Engine.Chess.CastlingRights = chess.CastlingRights{
		WhiteKingSide:  true,
		WhiteQueenSide: false,
		BlackKingSide:  false,
		BlackQueenSide: true,
	}

	s := room.SnapshotSafe()
	if !s.CastlingRights.WhiteKingSide || s.CastlingRights.WhiteQueenSide {
		t.Fatalf("unexpected white castling rights: %+v", s.CastlingRights)
	}
	if s.CastlingRights.BlackKingSide || !s.CastlingRights.BlackQueenSide {
		t.Fatalf("unexpected black castling rights: %+v", s.CastlingRights)
	}
}
