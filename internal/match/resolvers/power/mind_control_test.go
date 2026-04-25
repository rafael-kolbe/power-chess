package power_test

import (
	"strings"
	"testing"

	"power-chess/internal/chess"
	"power-chess/internal/gameplay"
	"power-chess/internal/match"
)

func newMindControlTestEngine(t *testing.T) (*match.Engine, chess.Pos) {
	t.Helper()
	mc := gameplay.CardInstance{InstanceID: "mc1", CardID: match.CardMindControl, ManaCost: 7, Ignition: 2, Cooldown: 10}
	state, err := gameplay.NewMatchState(testDeckWith(mc), testDeckWith(mc))
	if err != nil {
		t.Fatal(err)
	}
	board := chess.NewEmptyGame(chess.White)
	board.SetPiece(chess.Pos{Row: 7, Col: 4}, chess.Piece{Type: chess.King, Color: chess.White})
	board.SetPiece(chess.Pos{Row: 0, Col: 4}, chess.Piece{Type: chess.King, Color: chess.Black})
	board.SetPiece(chess.Pos{Row: 6, Col: 2}, chess.Piece{Type: chess.Pawn, Color: chess.White})
	board.SetPiece(chess.Pos{Row: 1, Col: 2}, chess.Piece{Type: chess.Pawn, Color: chess.Black})
	target := chess.Pos{Row: 0, Col: 0}
	board.SetPiece(target, chess.Piece{Type: chess.Rook, Color: chess.Black})
	state.Players[gameplay.PlayerA].Hand = []gameplay.CardInstance{mc}
	state.Players[gameplay.PlayerA].Mana = 10
	e := match.NewEngine(state, board)
	markInPlayForTest(state)
	return e, target
}

func burnMindControlIgnition(t *testing.T, e *match.Engine) {
	t.Helper()
	if err := e.SubmitMove(gameplay.PlayerA, chess.Move{From: chess.Pos{Row: 6, Col: 2}, To: chess.Pos{Row: 5, Col: 2}}); err != nil {
		t.Fatalf("player A move 1: %v", err)
	}
	if err := e.SubmitMove(gameplay.PlayerB, chess.Move{From: chess.Pos{Row: 1, Col: 2}, To: chess.Pos{Row: 2, Col: 2}}); err != nil {
		t.Fatalf("player B move 1: %v", err)
	}
	if err := e.SubmitMove(gameplay.PlayerA, chess.Move{From: chess.Pos{Row: 5, Col: 2}, To: chess.Pos{Row: 4, Col: 2}}); err != nil {
		t.Fatalf("player A move 2: %v", err)
	}
	if err := e.SubmitMove(gameplay.PlayerB, chess.Move{From: chess.Pos{Row: 2, Col: 2}, To: chess.Pos{Row: 3, Col: 2}}); err != nil {
		t.Fatalf("player B move 2: %v", err)
	}
}

func TestMindControlResolverTransfersControlOnResolve(t *testing.T) {
	e, target := newMindControlTestEngine(t)
	if err := e.ActivateCardWithTargets(gameplay.PlayerA, 0, []chess.Pos{target}); err != nil {
		t.Fatalf("activate with target failed: %v", err)
	}
	if err := e.ResolveReactionStack(); err != nil {
		t.Fatalf("resolve ignite reaction: %v", err)
	}
	burnMindControlIgnition(t, e)
	p := e.Chess.PieceAt(target)
	if p.Type != chess.Rook || p.Color != chess.White {
		t.Fatalf("expected controlled white rook at target, got %+v", p)
	}
	mc := e.ClonePieceControlEffects()
	if len(mc) != 1 || mc[0].RemainingTurnEnds != 3 {
		t.Fatalf("expected one active piece control effect with 3 turns, got %+v", mc)
	}
}

func TestMindControlResolverFollowsMovedTargetBeforeResolve(t *testing.T) {
	e, target := newMindControlTestEngine(t)
	if err := e.ActivateCardWithTargets(gameplay.PlayerA, 0, []chess.Pos{target}); err != nil {
		t.Fatalf("activate with target failed: %v", err)
	}
	if err := e.ResolveReactionStack(); err != nil {
		t.Fatalf("resolve ignite reaction: %v", err)
	}
	if err := e.SubmitMove(gameplay.PlayerA, chess.Move{From: chess.Pos{Row: 6, Col: 2}, To: chess.Pos{Row: 5, Col: 2}}); err != nil {
		t.Fatalf("player A move 1: %v", err)
	}
	newTarget := chess.Pos{Row: 0, Col: 1}
	if err := e.SubmitMove(gameplay.PlayerB, chess.Move{From: target, To: newTarget}); err != nil {
		t.Fatalf("player B moves targeted piece: %v", err)
	}
	if err := e.SubmitMove(gameplay.PlayerA, chess.Move{From: chess.Pos{Row: 5, Col: 2}, To: chess.Pos{Row: 4, Col: 2}}); err != nil {
		t.Fatalf("player A move 2: %v", err)
	}
	if err := e.SubmitMove(gameplay.PlayerB, chess.Move{From: chess.Pos{Row: 1, Col: 2}, To: chess.Pos{Row: 2, Col: 2}}); err != nil {
		t.Fatalf("player B move 2: %v", err)
	}
	p := e.Chess.PieceAt(newTarget)
	if p.Type != chess.Rook || p.Color != chess.White {
		t.Fatalf("expected moved target rook to be controlled at new square, got %+v", p)
	}
}

func TestMindControlResolverFailsWhenTargetNoLongerValid(t *testing.T) {
	e, target := newMindControlTestEngine(t)
	if err := e.ActivateCardWithTargets(gameplay.PlayerA, 0, []chess.Pos{target}); err != nil {
		t.Fatalf("activate with target failed: %v", err)
	}
	if err := e.ResolveReactionStack(); err != nil {
		t.Fatalf("resolve ignite reaction: %v", err)
	}
	// Simulate the locked target being removed before ignition reaches zero.
	e.Chess.SetPiece(target, chess.Piece{})
	if err := e.SubmitMove(gameplay.PlayerA, chess.Move{From: chess.Pos{Row: 6, Col: 2}, To: chess.Pos{Row: 5, Col: 2}}); err != nil {
		t.Fatalf("player A move 1: %v", err)
	}
	if err := e.SubmitMove(gameplay.PlayerB, chess.Move{From: chess.Pos{Row: 1, Col: 2}, To: chess.Pos{Row: 2, Col: 2}}); err != nil {
		t.Fatalf("player B move 1: %v", err)
	}
	if err := e.SubmitMove(gameplay.PlayerA, chess.Move{From: chess.Pos{Row: 5, Col: 2}, To: chess.Pos{Row: 4, Col: 2}}); err != nil {
		t.Fatalf("player A move 2: %v", err)
	}
	if err := e.SubmitMove(gameplay.PlayerB, chess.Move{From: chess.Pos{Row: 2, Col: 2}, To: chess.Pos{Row: 3, Col: 2}}); err != nil {
		t.Fatalf("player B move 2: %v", err)
	}
	if got := len(e.ClonePieceControlEffects()); got != 0 {
		t.Fatalf("expected no active piece control effects on failed activation, got %d", got)
	}
	events := e.PullActivationFXEvents()
	if len(events) == 0 {
		t.Fatal("expected activation fx events")
	}
	last := events[len(events)-1]
	if last.CardID != match.CardMindControl || last.Success {
		t.Fatalf("expected final activation fx fail for mind-control, got %+v", last)
	}
}

func TestMindControlEffectExpiresAndRestoresOriginalColor(t *testing.T) {
	e, target := newMindControlTestEngine(t)
	if err := e.ActivateCardWithTargets(gameplay.PlayerA, 0, []chess.Pos{target}); err != nil {
		t.Fatalf("activate with target failed: %v", err)
	}
	if err := e.ResolveReactionStack(); err != nil {
		t.Fatalf("resolve ignite reaction: %v", err)
	}
	burnMindControlIgnition(t, e)
	// Tick 1 (owner A): 3 -> 2
	if err := e.SubmitMove(gameplay.PlayerA, chess.Move{From: chess.Pos{Row: 7, Col: 4}, To: chess.Pos{Row: 7, Col: 3}}); err != nil {
		t.Fatalf("owner tick 1: %v", err)
	}
	// Opponent B turn: does not decrement owner-A effect duration.
	if err := e.SubmitMove(gameplay.PlayerB, chess.Move{From: chess.Pos{Row: 0, Col: 4}, To: chess.Pos{Row: 1, Col: 4}}); err != nil {
		t.Fatalf("opponent non-tick: %v", err)
	}
	// Tick 2 (owner A): 2 -> 1
	if err := e.SubmitMove(gameplay.PlayerA, chess.Move{From: chess.Pos{Row: 7, Col: 3}, To: chess.Pos{Row: 7, Col: 4}}); err != nil {
		t.Fatalf("owner tick 2: %v", err)
	}
	if err := e.SubmitMove(gameplay.PlayerB, chess.Move{From: chess.Pos{Row: 1, Col: 4}, To: chess.Pos{Row: 2, Col: 4}}); err != nil {
		t.Fatalf("opponent non-tick 2: %v", err)
	}
	// Tick 3 (owner A): 1 -> 0, expires now.
	if err := e.SubmitMove(gameplay.PlayerA, chess.Move{From: chess.Pos{Row: 7, Col: 4}, To: chess.Pos{Row: 7, Col: 3}}); err != nil {
		t.Fatalf("owner tick 3: %v", err)
	}
	p := e.Chess.PieceAt(target)
	if p.Type != chess.Rook || p.Color != chess.Black {
		t.Fatalf("expected piece to return to original black color, got %+v", p)
	}
	if got := len(e.ClonePieceControlEffects()); got != 0 {
		t.Fatalf("expected no remaining piece control effects after expiration, got %d", got)
	}
}

func TestMindControlEffectClearsImmediatelyWhenControlledPieceIsCaptured(t *testing.T) {
	e, target := newMindControlTestEngine(t)
	// Put black king near the target so it can capture right after control starts.
	e.Chess.SetPiece(chess.Pos{Row: 0, Col: 4}, chess.Piece{})
	e.Chess.SetPiece(chess.Pos{Row: 1, Col: 1}, chess.Piece{Type: chess.King, Color: chess.Black})
	if err := e.ActivateCardWithTargets(gameplay.PlayerA, 0, []chess.Pos{target}); err != nil {
		t.Fatalf("activate with target failed: %v", err)
	}
	if err := e.ResolveReactionStack(); err != nil {
		t.Fatalf("resolve ignite reaction: %v", err)
	}
	burnMindControlIgnition(t, e)
	if got := len(e.ClonePieceControlEffects()); got != 1 {
		t.Fatalf("expected one active effect before capture, got %d", got)
	}
	// PlayerA ends turn with a king move.
	if err := e.SubmitMove(gameplay.PlayerA, chess.Move{From: chess.Pos{Row: 7, Col: 4}, To: chess.Pos{Row: 7, Col: 3}}); err != nil {
		t.Fatalf("player A move before capture: %v", err)
	}
	// PlayerB captures the controlled rook on a1 with king from b2.
	if err := e.SubmitMove(gameplay.PlayerB, chess.Move{From: chess.Pos{Row: 1, Col: 1}, To: target}); err != nil {
		t.Fatalf("player B king capture controlled piece: %v", err)
	}
	if err := e.ResolveReactionStack(); err != nil {
		t.Fatalf("resolve capture reaction: %v", err)
	}
	if got := len(e.ClonePieceControlEffects()); got != 0 {
		t.Fatalf("expected effect removed immediately after capture, got %d", got)
	}
	p := e.Chess.PieceAt(target)
	if p.Type != chess.King || p.Color != chess.Black {
		t.Fatalf("expected black king on captured square, got %+v", p)
	}
}

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
