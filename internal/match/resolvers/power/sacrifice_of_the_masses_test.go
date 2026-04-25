package power_test

import (
	"testing"

	"power-chess/internal/chess"
	"power-chess/internal/gameplay"
	"power-chess/internal/match"
)

func newSacrificeOfTheMassesTestEngine(t *testing.T) (*match.Engine, *gameplay.MatchState, chess.Pos) {
	t.Helper()

	card := gameplay.CardInstance{InstanceID: "som1", CardID: "sacrifice-of-the-masses", ManaCost: 0, Ignition: 0, Cooldown: 10}
	state, err := gameplay.NewMatchState(testDeckWith(card), testDeckWith(card))
	if err != nil {
		t.Fatal(err)
	}

	board := chess.NewEmptyGame(chess.White)
	board.SetPiece(chess.Pos{Row: 7, Col: 4}, chess.Piece{Type: chess.King, Color: chess.White})
	board.SetPiece(chess.Pos{Row: 0, Col: 4}, chess.Piece{Type: chess.King, Color: chess.Black})
	pawnPos := chess.Pos{Row: 6, Col: 0}
	board.SetPiece(pawnPos, chess.Piece{Type: chess.Pawn, Color: chess.White})

	state.Players[gameplay.PlayerA].Hand = []gameplay.CardInstance{card}
	state.Players[gameplay.PlayerA].Mana = 0
	state.Players[gameplay.PlayerB].Hand = nil
	markInPlayForTest(state)

	return match.NewEngine(state, board), state, pawnPos
}

func TestSacrificeOfTheMassesSacrificesPawnGainsManaAndDraws(t *testing.T) {
	e, state, pawnPos := newSacrificeOfTheMassesTestEngine(t)
	initialDeck := len(state.Players[gameplay.PlayerA].Deck)

	if err := e.ActivateCardWithTargets(gameplay.PlayerA, 0, []chess.Pos{pawnPos}); err != nil {
		t.Fatalf("activate sacrifice of the masses: %v", err)
	}
	if err := e.ResolveReactionStack(); err != nil {
		t.Fatalf("resolve reaction stack: %v", err)
	}

	if piece := e.Chess.PieceAt(pawnPos); !piece.IsEmpty() {
		t.Fatalf("expected sacrificed pawn square to be empty, got %+v", piece)
	}
	if got := state.Players[gameplay.PlayerA].Mana; got != 6 {
		t.Fatalf("expected PlayerA to gain 6 mana, got %d", got)
	}
	if got := len(state.Players[gameplay.PlayerA].Hand); got != 2 {
		t.Fatalf("expected PlayerA to draw 2 cards, got hand size %d", got)
	}
	if got := len(state.Players[gameplay.PlayerA].Deck); got != initialDeck-2 {
		t.Fatalf("expected PlayerA deck to lose 2 cards, got %d from initial %d", got, initialDeck)
	}
	graveyard := state.Players[gameplay.PlayerB].Graveyard
	if len(graveyard) != 1 || graveyard[0] != (gameplay.PieceRef{Color: "w", Type: "P"}) {
		t.Fatalf("expected sacrificed white pawn in opponent capture zone, got %+v", graveyard)
	}
}

func TestSacrificeOfTheMassesRejectsNonPawnTarget(t *testing.T) {
	e, _, pawnPos := newSacrificeOfTheMassesTestEngine(t)
	e.Chess.SetPiece(pawnPos, chess.Piece{Type: chess.Bishop, Color: chess.White})

	if err := e.ActivateCardWithTargets(gameplay.PlayerA, 0, []chess.Pos{pawnPos}); err == nil {
		t.Fatal("expected non-pawn target to be rejected")
	}
}

func TestSacrificeOfTheMassesRejectsFullHandBeforeIgnition(t *testing.T) {
	e, state, pawnPos := newSacrificeOfTheMassesTestEngine(t)
	filler := gameplay.CardInstance{InstanceID: "filler-hand", CardID: "energy-gain", ManaCost: 0, Ignition: 1, Cooldown: 2}
	state.Players[gameplay.PlayerA].Hand = []gameplay.CardInstance{
		state.Players[gameplay.PlayerA].Hand[0],
		filler,
		filler,
		filler,
		filler,
	}

	if err := e.ActivateCardWithTargets(gameplay.PlayerA, 0, []chess.Pos{pawnPos}); err == nil {
		t.Fatal("expected full hand to reject sacrifice before ignition")
	}
	if state.Players[gameplay.PlayerA].Ignition.Occupied {
		t.Fatal("expected card to remain out of ignition after rejected activation")
	}
	if got := len(state.Players[gameplay.PlayerA].Hand); got != gameplay.DefaultMaxHandSize {
		t.Fatalf("expected full hand to remain unchanged, got %d cards", got)
	}
}
