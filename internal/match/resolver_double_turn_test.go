package match

import (
	"testing"

	"power-chess/internal/chess"
	"power-chess/internal/gameplay"
)

// activateDoubleTurnAndResolve sets up a minimal match where PlayerA activates a Double Turn
// card (Ignition=2) and advances turns until the resolver fires, leaving the engine ready for
// PlayerA's first extra move.
//
// Board setup: kings only so move legality is unambiguous. Returns the engine and state.
func activateDoubleTurnAndResolve(t *testing.T) (*Engine, *gameplay.MatchState, *chess.Game) {
	t.Helper()
	dt := gameplay.CardInstance{InstanceID: "dt1", CardID: CardDoubleTurn, ManaCost: 0, Ignition: 2, Cooldown: 9}
	state, err := gameplay.NewMatchState(testDeckWith(dt), testDeckWith(dt))
	if err != nil {
		t.Fatal(err)
	}

	board := chess.NewEmptyGame(chess.White)
	// White king + pawn; black king — minimal, no checks.
	board.SetPiece(chess.Pos{Row: 7, Col: 4}, chess.Piece{Type: chess.King, Color: chess.White})
	board.SetPiece(chess.Pos{Row: 0, Col: 4}, chess.Piece{Type: chess.King, Color: chess.Black})
	board.SetPiece(chess.Pos{Row: 6, Col: 0}, chess.Piece{Type: chess.Pawn, Color: chess.White})
	board.SetPiece(chess.Pos{Row: 5, Col: 2}, chess.Piece{Type: chess.Pawn, Color: chess.White})

	state.Players[gameplay.PlayerA].Hand = []gameplay.CardInstance{dt}
	state.Players[gameplay.PlayerA].Mana = 10
	e := NewEngine(state, board)
	markInPlayForTest(state)

	// Activate double-turn (Ignition=2 means it will resolve 2 turns from now for PlayerA).
	if err := e.ActivateCard(gameplay.PlayerA, 0); err != nil {
		t.Fatalf("activate double-turn: %v", err)
	}
	// Resolve empty ignite-reaction window.
	if err := e.ResolveReactionStack(); err != nil {
		t.Fatalf("resolve ignition reaction: %v", err)
	}

	// Ignition=2 means the resolver fires on PlayerA's 2nd start-of-turn after activation.
	// Each StartTurn(A) decrements A's ignition counter; StartTurn(B) does not.
	// Round 1: PlayerA moves → StartTurn(B); PlayerB moves → StartTurn(A) [ignition 2→1].
	if err := e.SubmitMove(gameplay.PlayerA, chess.Move{From: chess.Pos{Row: 6, Col: 0}, To: chess.Pos{Row: 5, Col: 0}}); err != nil {
		t.Fatalf("PlayerA move 1: %v", err)
	}
	if err := e.SubmitMove(gameplay.PlayerB, chess.Move{From: chess.Pos{Row: 0, Col: 4}, To: chess.Pos{Row: 0, Col: 3}}); err != nil {
		t.Fatalf("PlayerB move 1: %v", err)
	}
	// Round 2: PlayerA moves → StartTurn(B); PlayerB moves → StartTurn(A) [ignition 1→0 → resolves].
	if err := e.SubmitMove(gameplay.PlayerA, chess.Move{From: chess.Pos{Row: 5, Col: 0}, To: chess.Pos{Row: 4, Col: 0}}); err != nil {
		t.Fatalf("PlayerA move 2: %v", err)
	}
	if err := e.SubmitMove(gameplay.PlayerB, chess.Move{From: chess.Pos{Row: 0, Col: 3}, To: chess.Pos{Row: 0, Col: 2}}); err != nil {
		t.Fatalf("PlayerB move 2: %v", err)
	}
	// After StartTurn(A) the resolver has fired and granted the extra move.
	if e.extraMovesRemaining[gameplay.PlayerA] != 1 {
		t.Fatalf("expected 1 extra move after ignition resolves, got %d", e.extraMovesRemaining[gameplay.PlayerA])
	}
	return e, state, board
}

// TestDoubleTurnResolverGrantsExtraMove verifies that after Double Turn resolves, the engine
// records exactly one extra move for PlayerA.
func TestDoubleTurnResolverGrantsExtraMove(t *testing.T) {
	e, _, _ := activateDoubleTurnAndResolve(t)
	if got := e.extraMovesRemaining[gameplay.PlayerA]; got != 1 {
		t.Fatalf("want 1 extra move, got %d", got)
	}
	if e.DoubleTurnActiveFor() != gameplay.PlayerA {
		t.Fatalf("want DoubleTurnActiveFor=A, got %q", e.DoubleTurnActiveFor())
	}
}

// TestDoubleTurnFirstMoveDoesNotEndTurn verifies that after the first move on a Double Turn
// turn, the turn stays with PlayerA (extra move consumed, Chess.Turn restored).
// After the burn phase, the pawn is at (4,0) and the second pawn at (5,2).
func TestDoubleTurnFirstMoveDoesNotEndTurn(t *testing.T) {
	e, state, _ := activateDoubleTurnAndResolve(t)

	// First move (pawn at row 4 after burn rounds) — should NOT advance the turn.
	if err := e.SubmitMove(gameplay.PlayerA, chess.Move{From: chess.Pos{Row: 4, Col: 0}, To: chess.Pos{Row: 3, Col: 0}}); err != nil {
		t.Fatalf("PlayerA first extra move: %v", err)
	}
	if state.CurrentTurn != gameplay.PlayerA {
		t.Fatalf("turn should stay with PlayerA after first extra move, got %s", state.CurrentTurn)
	}
	if e.Chess.Turn != chess.White {
		t.Fatalf("Chess.Turn should be White after first extra move, got %v", e.Chess.Turn)
	}
	if e.extraMovesRemaining[gameplay.PlayerA] != 0 {
		t.Fatalf("extra move counter should be 0 after consumption, got %d", e.extraMovesRemaining[gameplay.PlayerA])
	}
}

// TestDoubleTurnSecondMoveEndsTurn verifies that the second move on a Double Turn turn
// advances the turn to PlayerB normally.
// After the burn phase, the burn pawn is at (4,0) and the second pawn is at (5,2).
func TestDoubleTurnSecondMoveEndsTurn(t *testing.T) {
	e, state, _ := activateDoubleTurnAndResolve(t)

	// First move (extra) — burn pawn at row 4.
	if err := e.SubmitMove(gameplay.PlayerA, chess.Move{From: chess.Pos{Row: 4, Col: 0}, To: chess.Pos{Row: 3, Col: 0}}); err != nil {
		t.Fatalf("PlayerA first extra move: %v", err)
	}
	// Second move (normal end-of-turn) — second pawn at (5,2).
	if err := e.SubmitMove(gameplay.PlayerA, chess.Move{From: chess.Pos{Row: 5, Col: 2}, To: chess.Pos{Row: 4, Col: 2}}); err != nil {
		t.Fatalf("PlayerA second move: %v", err)
	}
	if state.CurrentTurn != gameplay.PlayerB {
		t.Fatalf("turn should advance to PlayerB after second move, got %s", state.CurrentTurn)
	}
	if e.Chess.Turn != chess.Black {
		t.Fatalf("Chess.Turn should be Black after second move, got %v", e.Chess.Turn)
	}
}

// TestDoubleTurnIllegalFirstMoveRejected verifies that an illegal first move during a Double
// Turn turn is rejected and does not consume the extra move.
func TestDoubleTurnIllegalFirstMoveRejected(t *testing.T) {
	e, _, _ := activateDoubleTurnAndResolve(t)

	// Attempt an illegal move (pawn moving backwards from row 4).
	err := e.SubmitMove(gameplay.PlayerA, chess.Move{From: chess.Pos{Row: 4, Col: 0}, To: chess.Pos{Row: 5, Col: 0}})
	if err == nil {
		t.Fatal("expected illegal backward pawn move to be rejected")
	}
	// Extra move must not have been consumed.
	if e.extraMovesRemaining[gameplay.PlayerA] != 1 {
		t.Fatalf("extra move should not be consumed after rejection, got %d", e.extraMovesRemaining[gameplay.PlayerA])
	}
	if e.State.CurrentTurn != gameplay.PlayerA {
		t.Fatalf("turn should still be PlayerA after rejected move, got %s", e.State.CurrentTurn)
	}
}

// TestDoubleTurnKingCaptureIsIllegal verifies that a move attempting to directly capture the
// opponent's king is rejected, even during a Double Turn extra move.
//
// Board: white queen on the same file as the black king with a clear path.
// Burn pawn shuttles during ignition; queen is untouched and ready on the double-turn.
func TestDoubleTurnKingCaptureIsIllegal(t *testing.T) {
	dt := gameplay.CardInstance{InstanceID: "dt1", CardID: CardDoubleTurn, ManaCost: 0, Ignition: 2, Cooldown: 9}
	state, err := gameplay.NewMatchState(testDeckWith(dt), testDeckWith(dt))
	if err != nil {
		t.Fatal(err)
	}

	board := chess.NewEmptyGame(chess.White)
	board.SetPiece(chess.Pos{Row: 7, Col: 7}, chess.Piece{Type: chess.King, Color: chess.White})
	board.SetPiece(chess.Pos{Row: 0, Col: 7}, chess.Piece{Type: chess.King, Color: chess.Black})
	// White queen on col 0, far from the kings; pawn burns on col 2.
	board.SetPiece(chess.Pos{Row: 5, Col: 0}, chess.Piece{Type: chess.Queen, Color: chess.White})
	board.SetPiece(chess.Pos{Row: 6, Col: 2}, chess.Piece{Type: chess.Pawn, Color: chess.White})

	state.Players[gameplay.PlayerA].Hand = []gameplay.CardInstance{dt}
	state.Players[gameplay.PlayerA].Mana = 10
	e := NewEngine(state, board)
	markInPlayForTest(state)

	if err := e.ActivateCard(gameplay.PlayerA, 0); err != nil {
		t.Fatalf("activate: %v", err)
	}
	_ = e.ResolveReactionStack()

	// Two rounds of ignition burn using pawn (col 2) and black king shuffle.
	_ = e.SubmitMove(gameplay.PlayerA, chess.Move{From: chess.Pos{Row: 6, Col: 2}, To: chess.Pos{Row: 5, Col: 2}})
	_ = e.SubmitMove(gameplay.PlayerB, chess.Move{From: chess.Pos{Row: 0, Col: 7}, To: chess.Pos{Row: 0, Col: 6}})
	_ = e.SubmitMove(gameplay.PlayerA, chess.Move{From: chess.Pos{Row: 5, Col: 2}, To: chess.Pos{Row: 4, Col: 2}})
	_ = e.SubmitMove(gameplay.PlayerB, chess.Move{From: chess.Pos{Row: 0, Col: 6}, To: chess.Pos{Row: 0, Col: 7}})
	// Double Turn is now active for PlayerA. Queen is at (5,0); black king is at (0,7).

	// First extra move: queen advances to row 1 on col 0.
	if err := e.SubmitMove(gameplay.PlayerA, chess.Move{From: chess.Pos{Row: 5, Col: 0}, To: chess.Pos{Row: 1, Col: 0}}); err != nil {
		t.Fatalf("queen advance: %v", err)
	}
	// Second move: attempt to capture the black king directly — must be rejected.
	if err := e.SubmitMove(gameplay.PlayerA, chess.Move{From: chess.Pos{Row: 1, Col: 0}, To: chess.Pos{Row: 0, Col: 7}}); err == nil {
		t.Fatal("expected king capture to be rejected")
	}
}

// TestDoubleTurnDoubleTurnActiveForClearsAfterSecondMove ensures DoubleTurnActiveFor returns ""
// once the extra moves have been fully consumed.
// After the burn phase, burn pawn is at (4,0); second pawn at (5,2).
func TestDoubleTurnDoubleTurnActiveForClearsAfterSecondMove(t *testing.T) {
	e, _, _ := activateDoubleTurnAndResolve(t)

	if e.DoubleTurnActiveFor() == "" {
		t.Fatal("expected DoubleTurnActiveFor to be set before moves")
	}

	// First move (extra) — burn pawn at row 4.
	if err := e.SubmitMove(gameplay.PlayerA, chess.Move{From: chess.Pos{Row: 4, Col: 0}, To: chess.Pos{Row: 3, Col: 0}}); err != nil {
		t.Fatalf("first move: %v", err)
	}
	// Still PlayerA's turn but no extra moves remain.
	if e.DoubleTurnActiveFor() != "" {
		t.Fatalf("DoubleTurnActiveFor should be empty after extra move consumed, got %q", e.DoubleTurnActiveFor())
	}

	// Second move ends the turn — second pawn at (5,2).
	if err := e.SubmitMove(gameplay.PlayerA, chess.Move{From: chess.Pos{Row: 5, Col: 2}, To: chess.Pos{Row: 4, Col: 2}}); err != nil {
		t.Fatalf("second move: %v", err)
	}
	if e.DoubleTurnActiveFor() != "" {
		t.Fatalf("DoubleTurnActiveFor should remain empty after full turn, got %q", e.DoubleTurnActiveFor())
	}
}

// TestDoubleTurnActivatePlayerSkillClearsExtraMoves ensures that if a player uses a skill
// during a Double Turn turn, the extra move is discarded and the turn advances normally.
func TestDoubleTurnActivatePlayerSkillClearsExtraMoves(t *testing.T) {
	dt := gameplay.CardInstance{InstanceID: "dt1", CardID: CardDoubleTurn, ManaCost: 0, Ignition: 2, Cooldown: 9}
	state, err := gameplay.NewMatchState(testDeckWith(dt), testDeckWith(dt))
	if err != nil {
		t.Fatal(err)
	}
	// Must select skill before match is started.
	if err := state.SelectPlayerSkill(gameplay.PlayerA, "reinforcements"); err != nil {
		t.Fatalf("SelectPlayerSkill: %v", err)
	}

	board := chess.NewEmptyGame(chess.White)
	board.SetPiece(chess.Pos{Row: 7, Col: 4}, chess.Piece{Type: chess.King, Color: chess.White})
	board.SetPiece(chess.Pos{Row: 0, Col: 4}, chess.Piece{Type: chess.King, Color: chess.Black})
	board.SetPiece(chess.Pos{Row: 6, Col: 0}, chess.Piece{Type: chess.Pawn, Color: chess.White})
	board.SetPiece(chess.Pos{Row: 5, Col: 2}, chess.Piece{Type: chess.Pawn, Color: chess.White})

	state.Players[gameplay.PlayerA].Hand = []gameplay.CardInstance{dt}
	state.Players[gameplay.PlayerA].Mana = 10
	e := NewEngine(state, board)
	markInPlayForTest(state)

	if err := e.ActivateCard(gameplay.PlayerA, 0); err != nil {
		t.Fatalf("activate: %v", err)
	}
	_ = e.ResolveReactionStack()

	// Two full rounds to resolve ignition.
	_ = e.SubmitMove(gameplay.PlayerA, chess.Move{From: chess.Pos{Row: 6, Col: 0}, To: chess.Pos{Row: 5, Col: 0}})
	_ = e.SubmitMove(gameplay.PlayerB, chess.Move{From: chess.Pos{Row: 0, Col: 4}, To: chess.Pos{Row: 0, Col: 3}})
	_ = e.SubmitMove(gameplay.PlayerA, chess.Move{From: chess.Pos{Row: 5, Col: 0}, To: chess.Pos{Row: 4, Col: 0}})
	_ = e.SubmitMove(gameplay.PlayerB, chess.Move{From: chess.Pos{Row: 0, Col: 3}, To: chess.Pos{Row: 0, Col: 2}})

	if e.extraMovesRemaining[gameplay.PlayerA] != 1 {
		t.Fatalf("expected extra move, got %d", e.extraMovesRemaining[gameplay.PlayerA])
	}

	// Use the skill instead of moving — the extra move should be discarded.
	state.Players[gameplay.PlayerA].EnergizedMana = state.Players[gameplay.PlayerA].MaxEnergizedMana
	if err := e.ActivatePlayerSkill(gameplay.PlayerA); err != nil {
		t.Fatalf("activate skill: %v", err)
	}
	if state.CurrentTurn != gameplay.PlayerB {
		t.Fatalf("turn should advance to PlayerB after skill activation, got %s", state.CurrentTurn)
	}
	if e.extraMovesRemaining[gameplay.PlayerA] != 0 {
		t.Fatalf("extra moves should be cleared after skill activation, got %d", e.extraMovesRemaining[gameplay.PlayerA])
	}
}
