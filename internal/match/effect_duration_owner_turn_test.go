package match

import (
	"testing"

	"power-chess/internal/chess"
	"power-chess/internal/gameplay"
	matchresolvers "power-chess/internal/match/resolvers"
)

// newEffectDurationOwnerTurnEngine builds a minimal started match for owner-turn counter tests.
func newEffectDurationOwnerTurnEngine(t *testing.T) *Engine {
	t.Helper()
	card := gameplay.CardInstance{InstanceID: "dt1", CardID: CardDoubleTurn, ManaCost: 0, Ignition: 2, Cooldown: 9}
	state, err := gameplay.NewMatchState(testDeckWith(card), testDeckWith(card))
	if err != nil {
		t.Fatal(err)
	}
	board := chess.NewEmptyGame(chess.White)
	board.SetPiece(chess.Pos{Row: 7, Col: 4}, chess.Piece{Type: chess.King, Color: chess.White})
	board.SetPiece(chess.Pos{Row: 0, Col: 4}, chess.Piece{Type: chess.King, Color: chess.Black})
	board.SetPiece(chess.Pos{Row: 6, Col: 0}, chess.Piece{Type: chess.Pawn, Color: chess.White})
	board.SetPiece(chess.Pos{Row: 1, Col: 0}, chess.Piece{Type: chess.Pawn, Color: chess.Black})
	e := NewEngine(state, board)
	markInPlayForTest(state)
	return e
}

// TestMovementGrantDurationTicksOnlyOnOwnerTurn validates shared effect_duration behavior used by
// Knight/Bishop/Rook Touch grants: counter decreases only at the end of the owner's turns.
func TestMovementGrantDurationTicksOnlyOnOwnerTurn(t *testing.T) {
	e := newEffectDurationOwnerTurnEngine(t)
	target := chess.Pos{Row: 6, Col: 0}
	e.AddMovementGrant(gameplay.PlayerA, CardKnightTouch, target, matchresolvers.MovementGrantKnightPattern, 3)

	assertTurns := func(want int) {
		t.Helper()
		grants := e.CloneMovementGrants()
		if want == 0 {
			if len(grants) != 0 {
				t.Fatalf("expected no movement grants, got %+v", grants)
			}
			return
		}
		if len(grants) != 1 || grants[0].RemainingOwnerTurns != want {
			t.Fatalf("expected one grant with %d turns, got %+v", want, grants)
		}
	}

	assertTurns(3)
	if err := e.SubmitMove(gameplay.PlayerA, chess.Move{From: chess.Pos{Row: 7, Col: 4}, To: chess.Pos{Row: 7, Col: 3}}); err != nil {
		t.Fatalf("player A move 1: %v", err)
	}
	assertTurns(2)
	if err := e.SubmitMove(gameplay.PlayerB, chess.Move{From: chess.Pos{Row: 0, Col: 4}, To: chess.Pos{Row: 0, Col: 3}}); err != nil {
		t.Fatalf("player B move 1: %v", err)
	}
	assertTurns(2)
	if err := e.SubmitMove(gameplay.PlayerA, chess.Move{From: chess.Pos{Row: 7, Col: 3}, To: chess.Pos{Row: 7, Col: 4}}); err != nil {
		t.Fatalf("player A move 2: %v", err)
	}
	assertTurns(1)
	if err := e.SubmitMove(gameplay.PlayerB, chess.Move{From: chess.Pos{Row: 0, Col: 3}, To: chess.Pos{Row: 0, Col: 4}}); err != nil {
		t.Fatalf("player B move 2: %v", err)
	}
	assertTurns(1)
	if err := e.SubmitMove(gameplay.PlayerA, chess.Move{From: chess.Pos{Row: 7, Col: 4}, To: chess.Pos{Row: 7, Col: 3}}); err != nil {
		t.Fatalf("player A move 3: %v", err)
	}
	assertTurns(0)
}

// TestDoubleTurnVisualDurationTicksOnlyOnOwnerTurn validates the owner-turn-only behavior for
// Double Turn effect_duration counters exposed to snapshots.
func TestDoubleTurnVisualDurationTicksOnlyOnOwnerTurn(t *testing.T) {
	e := newEffectDurationOwnerTurnEngine(t)
	e.doubleTurnEffectTurnsLeft[gameplay.PlayerA] = 3

	assertTurns := func(want int) {
		t.Helper()
		if got := e.DoubleTurnTurnsRemainingFor(gameplay.PlayerA); got != want {
			t.Fatalf("expected double turn turns=%d, got %d", want, got)
		}
	}

	assertTurns(3)
	if err := e.SubmitMove(gameplay.PlayerA, chess.Move{From: chess.Pos{Row: 7, Col: 4}, To: chess.Pos{Row: 7, Col: 3}}); err != nil {
		t.Fatalf("player A move 1: %v", err)
	}
	assertTurns(2)
	if err := e.SubmitMove(gameplay.PlayerB, chess.Move{From: chess.Pos{Row: 0, Col: 4}, To: chess.Pos{Row: 0, Col: 3}}); err != nil {
		t.Fatalf("player B move 1: %v", err)
	}
	assertTurns(2)
	if err := e.SubmitMove(gameplay.PlayerA, chess.Move{From: chess.Pos{Row: 7, Col: 3}, To: chess.Pos{Row: 7, Col: 4}}); err != nil {
		t.Fatalf("player A move 2: %v", err)
	}
	assertTurns(1)
}
