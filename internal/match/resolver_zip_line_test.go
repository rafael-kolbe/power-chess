package match

import (
	"errors"
	"testing"

	"power-chess/internal/chess"
	"power-chess/internal/gameplay"
	matchresolvers "power-chess/internal/match/resolvers"
)

func newZipLineTestEngine(t *testing.T) *Engine {
	t.Helper()
	zl := gameplay.CardInstance{InstanceID: "zl1", CardID: CardZipLine, ManaCost: 4, Ignition: 0, Cooldown: 4}
	state, err := gameplay.NewMatchState(testDeckWith(zl), testDeckWith(zl))
	if err != nil {
		t.Fatal(err)
	}
	board := chess.NewEmptyGame(chess.White)
	board.SetPiece(chess.Pos{Row: 7, Col: 4}, chess.Piece{Type: chess.King, Color: chess.White})
	board.SetPiece(chess.Pos{Row: 0, Col: 4}, chess.Piece{Type: chess.King, Color: chess.Black})
	// Bishop on b2 (rank 2 = row 6, file b = col 1).
	board.SetPiece(chess.Pos{Row: 6, Col: 1}, chess.Piece{Type: chess.Bishop, Color: chess.White})
	// Pawns so Black has a legal reply later if needed.
	board.SetPiece(chess.Pos{Row: 1, Col: 4}, chess.Piece{Type: chess.Pawn, Color: chess.Black})
	state.Players[gameplay.PlayerA].Hand = []gameplay.CardInstance{zl}
	state.Players[gameplay.PlayerA].Mana = 10
	e := NewEngine(state, board)
	markInPlayForTest(state)
	return e
}

func TestZipLineQueuesPendingAndTeleportsEndsTurn(t *testing.T) {
	e := newZipLineTestEngine(t)
	from := chess.Pos{Row: 6, Col: 1}
	to := chess.Pos{Row: 6, Col: 6}

	if err := e.ActivateCardWithTargets(gameplay.PlayerA, 0, []chess.Pos{from}); err != nil {
		t.Fatalf("activate zip line: %v", err)
	}
	if err := e.ResolveReactionStack(); err != nil {
		t.Fatalf("close ignite reaction: %v", err)
	}
	if len(e.pendingEffects[gameplay.PlayerA]) != 1 {
		t.Fatalf("expected one pending effect, got %d", len(e.pendingEffects[gameplay.PlayerA]))
	}
	pe := e.pendingEffects[gameplay.PlayerA][0]
	if pe.CardID != CardZipLine || pe.ZipLineFrom == nil || *pe.ZipLineFrom != from {
		t.Fatalf("unexpected pending effect: %+v", pe)
	}

	if err := e.ResolvePendingEffect(gameplay.PlayerA, EffectTarget{TargetPos: &to}); err != nil {
		t.Fatalf("resolve pending zip line: %v", err)
	}
	if len(e.pendingEffects[gameplay.PlayerA]) != 0 {
		t.Fatalf("expected pending queue cleared")
	}
	got := e.Chess.PieceAt(to)
	if got.Type != chess.Bishop || got.Color != chess.White {
		t.Fatalf("expected white bishop at destination, got %+v", got)
	}
	if !e.Chess.PieceAt(from).IsEmpty() {
		t.Fatal("expected source square empty after zip")
	}
	if e.State.CurrentTurn != gameplay.PlayerB {
		t.Fatalf("expected turn to pass to B, got %s", e.State.CurrentTurn)
	}
	if e.Chess.Turn != chess.Black {
		t.Fatalf("expected chess Black to move, got %v", e.Chess.Turn)
	}
}

func TestZipLineIllegalDestinationKeepsPending(t *testing.T) {
	e := newZipLineTestEngine(t)
	from := chess.Pos{Row: 6, Col: 1}
	blocked := chess.Pos{Row: 6, Col: 4}
	e.Chess.SetPiece(blocked, chess.Piece{Type: chess.Pawn, Color: chess.Black})

	if err := e.ActivateCardWithTargets(gameplay.PlayerA, 0, []chess.Pos{from}); err != nil {
		t.Fatalf("activate zip line: %v", err)
	}
	if err := e.ResolveReactionStack(); err != nil {
		t.Fatalf("close ignite reaction: %v", err)
	}
	err := e.ResolvePendingEffect(gameplay.PlayerA, EffectTarget{TargetPos: &blocked})
	if err == nil {
		t.Fatal("expected illegal zip destination to fail")
	}
	if !errors.Is(err, matchresolvers.ErrEffectFailed) {
		t.Fatalf("expected ErrEffectFailed, got %v", err)
	}
	if len(e.pendingEffects[gameplay.PlayerA]) != 1 {
		t.Fatalf("expected pending to remain after failed resolve, got len=%d", len(e.pendingEffects[gameplay.PlayerA]))
	}
}

func TestZipLineSubmitMoveBlockedWhilePending(t *testing.T) {
	e := newZipLineTestEngine(t)
	from := chess.Pos{Row: 6, Col: 1}
	if err := e.ActivateCardWithTargets(gameplay.PlayerA, 0, []chess.Pos{from}); err != nil {
		t.Fatalf("activate: %v", err)
	}
	if err := e.ResolveReactionStack(); err != nil {
		t.Fatalf("resolve reaction: %v", err)
	}
	if err := e.SubmitMove(gameplay.PlayerA, chess.Move{From: chess.Pos{Row: 7, Col: 4}, To: chess.Pos{Row: 7, Col: 3}}); err == nil {
		t.Fatal("expected submit_move blocked while zip line pending")
	}
}
