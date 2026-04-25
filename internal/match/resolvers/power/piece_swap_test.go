package power_test

import (
	"testing"

	"power-chess/internal/chess"
	"power-chess/internal/gameplay"
	"power-chess/internal/match"
)

// newPieceSwapTestEngine builds a minimal engine for Piece Swap tests.
// PlayerA has a Piece Swap card (Ignition:0, Targets:2) and enough mana to activate it.
// The board has kings on their standard squares plus a pawn (A) on c5 and a knight (B) on e5.
func newPieceSwapTestEngine(t *testing.T) (*match.Engine, *gameplay.MatchState) {
	t.Helper()

	psCard := gameplay.CardInstance{InstanceID: "ps1", CardID: match.CardPieceSwap, ManaCost: 6, Ignition: 0, Cooldown: 6}

	state, err := gameplay.NewMatchState(testDeckWith(psCard), testDeckWith(psCard))
	if err != nil {
		t.Fatal(err)
	}

	board := chess.NewEmptyGame(chess.White)
	// Kings
	board.SetPiece(chess.Pos{Row: 7, Col: 4}, chess.Piece{Type: chess.King, Color: chess.White})
	board.SetPiece(chess.Pos{Row: 0, Col: 4}, chess.Piece{Type: chess.King, Color: chess.Black})
	// Pawn (A/White) on c5 = row 3, col 2
	board.SetPiece(chess.Pos{Row: 3, Col: 2}, chess.Piece{Type: chess.Pawn, Color: chess.White})
	// Knight (B/Black) on e5 = row 3, col 4
	board.SetPiece(chess.Pos{Row: 3, Col: 4}, chess.Piece{Type: chess.Knight, Color: chess.Black})

	state.Players[gameplay.PlayerA].Hand = []gameplay.CardInstance{psCard}
	state.Players[gameplay.PlayerA].Mana = 10

	state.Players[gameplay.PlayerB].Hand = []gameplay.CardInstance{psCard}
	state.Players[gameplay.PlayerB].Mana = 10

	markInPlayForTest(state)
	return match.NewEngine(state, board), state
}

// TestPieceSwap_SuccessSwapsPositions verifies that a valid Piece Swap swaps the two pieces.
func TestPieceSwap_SuccessSwapsPositions(t *testing.T) {
	e, state := newPieceSwapTestEngine(t)

	pawnPos := chess.Pos{Row: 3, Col: 2}   // White pawn at c5
	knightPos := chess.Pos{Row: 3, Col: 4} // Black knight at e5

	targets := []chess.Pos{pawnPos, knightPos}
	if err := e.ActivateCardWithTargets(gameplay.PlayerA, 0, targets); err != nil {
		t.Fatalf("ActivateCardWithTargets: %v", err)
	}

	// Resolve reaction window (PlayerB has no eligible cards since piece-swap with empty ignition opens normal window).
	if err := e.ResolveReactionStack(); err != nil {
		t.Fatalf("resolve: %v", err)
	}

	// After swap: White pawn should be at e5 (knightPos), Black knight at c5 (pawnPos).
	_ = state
	p1 := e.Chess.PieceAt(pawnPos)
	p2 := e.Chess.PieceAt(knightPos)
	if p1.Type != chess.Knight || p1.Color != chess.Black {
		t.Errorf("expected Black knight at c5 after swap, got %+v", p1)
	}
	if p2.Type != chess.Pawn || p2.Color != chess.White {
		t.Errorf("expected White pawn at e5 after swap, got %+v", p2)
	}
}

// TestPieceSwap_RejectsKingAsFirstTarget verifies that selecting the own king as first target is rejected.
func TestPieceSwap_RejectsKingAsFirstTarget(t *testing.T) {
	e, _ := newPieceSwapTestEngine(t)

	kingPos := chess.Pos{Row: 7, Col: 4}   // White king
	knightPos := chess.Pos{Row: 3, Col: 4} // Black knight

	targets := []chess.Pos{kingPos, knightPos}
	if err := e.ActivateCardWithTargets(gameplay.PlayerA, 0, targets); err == nil {
		t.Fatal("expected error when selecting own king as first target, got nil")
	}
}

// TestPieceSwap_RejectsKingAsSecondTarget verifies that selecting the opponent's king as second target is rejected.
func TestPieceSwap_RejectsKingAsSecondTarget(t *testing.T) {
	e, _ := newPieceSwapTestEngine(t)

	pawnPos := chess.Pos{Row: 3, Col: 2} // White pawn
	oppKing := chess.Pos{Row: 0, Col: 4} // Black king

	targets := []chess.Pos{pawnPos, oppKing}
	if err := e.ActivateCardWithTargets(gameplay.PlayerA, 0, targets); err == nil {
		t.Fatal("expected error when targeting opponent king, got nil")
	}
}

// TestPieceSwap_RejectsPieceTooFarAway verifies that pieces more than 2 Chebyshev squares apart are rejected.
func TestPieceSwap_RejectsPieceTooFarAway(t *testing.T) {
	e, _ := newPieceSwapTestEngine(t)

	// Pawn at row 3, col 2 — knight at row 0, col 4: Chebyshev = max(3,2) = 3 > 2.
	pawnPos := chess.Pos{Row: 3, Col: 2}
	farPiece := chess.Pos{Row: 0, Col: 4} // Black king (but also 3 rows away → too far)

	// Place a rook far away to test distance without hitting king rule.
	e.Chess.SetPiece(chess.Pos{Row: 0, Col: 2}, chess.Piece{Type: chess.Rook, Color: chess.Black})
	farRook := chess.Pos{Row: 0, Col: 2} // col diff=0, row diff=3 → too far
	_ = farPiece

	targets := []chess.Pos{pawnPos, farRook}
	if err := e.ActivateCardWithTargets(gameplay.PlayerA, 0, targets); err == nil {
		t.Fatal("expected error for target more than 2 squares away, got nil")
	}
}

// TestPieceSwap_RejectsSwapThatPutsOwnKingInCheck verifies that a swap that exposes the
// activating player's king to check is rejected.
//
// Setup: White king at e1 (row 7, col 4). White pawn at d2 (row 6, col 3).
// Black pawn at d3 (row 5, col 3) — 1 square directly above the White pawn.
//
// After the swap the Black pawn lands on d2 (row 6, col 3). A Black pawn on row 6, col 3
// attacks diagonally to row 7, col 4 — the White king — so the king ends up in check.
func TestPieceSwap_RejectsSwapThatPutsOwnKingInCheck(t *testing.T) {
	e, _ := newPieceSwapTestEngine(t)

	// Remove pieces placed by the default setup so only kings remain.
	e.Chess.SetPiece(chess.Pos{Row: 3, Col: 2}, chess.Piece{}) // remove White pawn
	e.Chess.SetPiece(chess.Pos{Row: 3, Col: 4}, chess.Piece{}) // remove Black knight

	// White pawn that PlayerA will try to swap.
	ownPawn := chess.Pos{Row: 6, Col: 3} // d2
	e.Chess.SetPiece(ownPawn, chess.Piece{Type: chess.Pawn, Color: chess.White})

	// Black pawn that will land on d2 after the swap and check the White king diagonally.
	oppPawn := chess.Pos{Row: 5, Col: 3} // d3 — 1 square above ownPawn (Chebyshev 1)
	e.Chess.SetPiece(oppPawn, chess.Piece{Type: chess.Pawn, Color: chess.Black})

	targets := []chess.Pos{ownPawn, oppPawn}
	if err := e.ActivateCardWithTargets(gameplay.PlayerA, 0, targets); err == nil {
		t.Fatal("expected error for swap that puts own king in check, got nil")
	}
}

// TestPieceSwap_ConfirmationOnlyWindowWhenOpponentIgnitionOccupied verifies that when the opponent
// already has a card in their ignition slot, the reaction window opens with no eligible types.
func TestPieceSwap_ConfirmationOnlyWindowWhenOpponentIgnitionOccupied(t *testing.T) {
	e, state := newPieceSwapTestEngine(t)

	// Place a card in PlayerB's ignition slot directly.
	energyCard := gameplay.CardInstance{InstanceID: "eg1", CardID: match.CardEnergyGain, ManaCost: 0, Ignition: 1, Cooldown: 2}
	state.Players[gameplay.PlayerB].Ignition = gameplay.IgnitionSlot{
		Occupied:        true,
		Card:            energyCard,
		TurnsRemaining:  1,
		ActivationOwner: gameplay.PlayerB,
	}

	pawnPos := chess.Pos{Row: 3, Col: 2}
	knightPos := chess.Pos{Row: 3, Col: 4}

	targets := []chess.Pos{pawnPos, knightPos}
	if err := e.ActivateCardWithTargets(gameplay.PlayerA, 0, targets); err != nil {
		t.Fatalf("ActivateCardWithTargets: %v", err)
	}

	rw, _, ok := e.ReactionWindowSnapshot()
	if !ok || !rw.Open {
		t.Fatal("expected reaction window to be open")
	}
	if len(rw.EligibleTypes) != 0 {
		t.Errorf("expected no eligible types (confirmation-only), got %v", rw.EligibleTypes)
	}
}
