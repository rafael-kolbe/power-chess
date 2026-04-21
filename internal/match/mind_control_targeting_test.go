package match

import (
	"strings"
	"testing"

	"power-chess/internal/chess"
	"power-chess/internal/gameplay"
)

func TestMindControlRejectsOwnOrRoyalTargets(t *testing.T) {
	e, _ := newMindControlTestEngine(t)
	if err := e.ActivateCardWithTargets(gameplay.PlayerA, 0, []chess.Pos{{Row: 6, Col: 2}}); err == nil {
		t.Fatal("expected own-piece target to be rejected")
	}
	e, _ = newMindControlTestEngine(t)
	e.Chess.SetPiece(chess.Pos{Row: 0, Col: 1}, chess.Piece{Type: chess.Queen, Color: chess.Black})
	if err := e.ActivateCardWithTargets(gameplay.PlayerA, 0, []chess.Pos{{Row: 0, Col: 1}}); err == nil {
		t.Fatal("expected queen target to be rejected")
	}
}

func TestMindControlPreIgnitionFailsWithoutValidTargets(t *testing.T) {
	e, _ := newMindControlTestEngine(t)
	for row := 0; row < 8; row++ {
		for col := 0; col < 8; col++ {
			p := e.Chess.PieceAt(chess.Pos{Row: row, Col: col})
			if p.IsEmpty() || p.Color != chess.Black || p.Type == chess.King {
				continue
			}
			e.Chess.SetPiece(chess.Pos{Row: row, Col: col}, chess.Piece{})
		}
	}
	err := e.ActivateCard(gameplay.PlayerA, 0)
	if err == nil {
		t.Fatal("expected activation to fail without valid mind control targets")
	}
	if !strings.Contains(err.Error(), "no valid mind control targets") {
		t.Fatalf("unexpected error: %v", err)
	}
}
