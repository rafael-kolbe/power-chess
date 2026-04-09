package match

import (
	"testing"

	"power-chess/internal/chess"
	"power-chess/internal/gameplay"
)

func testDeckWith(card gameplay.CardInstance) []gameplay.CardInstance {
	d := []gameplay.CardInstance{card}
	for i := 1; i < gameplay.DefaultDeckSize; i++ {
		d = append(d, gameplay.CardInstance{
			InstanceID: "filler",
			CardID:     "filler",
			ManaCost:   1,
			Ignition:   0,
			Cooldown:   1,
		})
	}
	return d
}

func TestKnightBuffAllowsKnightMoveForOwnedPiece(t *testing.T) {
	card := gameplay.CardInstance{InstanceID: "k1", CardID: CardKnightTouch, ManaCost: 3, Ignition: 0, Cooldown: 2}
	state, err := gameplay.NewMatchState(testDeckWith(card), testDeckWith(card))
	if err != nil {
		t.Fatalf("state create error: %v", err)
	}
	state.Players[gameplay.PlayerA].Hand = []gameplay.CardInstance{card}
	state.Players[gameplay.PlayerA].Mana = 10

	board := chess.NewEmptyGame(chess.White)
	board.SetPiece(chess.Pos{Row: 7, Col: 4}, chess.Piece{Type: chess.King, Color: chess.White})
	board.SetPiece(chess.Pos{Row: 0, Col: 4}, chess.Piece{Type: chess.King, Color: chess.Black})
	board.SetPiece(chess.Pos{Row: 6, Col: 0}, chess.Piece{Type: chess.Pawn, Color: chess.White})
	board.SetPiece(chess.Pos{Row: 3, Col: 3}, chess.Piece{Type: chess.Pawn, Color: chess.Black})

	e := NewEngine(state, board)
	if err := e.StartTurn(gameplay.PlayerA); err != nil {
		t.Fatalf("start turn error: %v", err)
	}
	if err := e.ActivateCard(gameplay.PlayerA, 0); err != nil {
		t.Fatalf("activate card error: %v", err)
	}
	_ = state.ResolveIgnition(true)
	if err := e.StartTurn(gameplay.PlayerA); err != nil {
		t.Fatalf("start turn error: %v", err)
	}
	target := chess.Pos{Row: 6, Col: 0}
	if err := e.ResolvePendingEffect(gameplay.PlayerA, EffectTarget{PiecePos: &target}); err != nil {
		t.Fatalf("resolve pending effect error: %v", err)
	}
	if err := e.SubmitMove(gameplay.PlayerA, chess.Move{From: chess.Pos{6, 0}, To: chess.Pos{4, 1}}); err != nil {
		t.Fatalf("knight buff move should be legal: %v", err)
	}
}

func TestDoubleTurnPowerMoveCanCheckmate(t *testing.T) {
	card := gameplay.CardInstance{InstanceID: "d1", CardID: CardDoubleTurn, ManaCost: 4, Ignition: 1, Cooldown: 3}
	state, err := gameplay.NewMatchState(testDeckWith(card), testDeckWith(card))
	if err != nil {
		t.Fatalf("state create error: %v", err)
	}
	state.CurrentTurn = gameplay.PlayerB

	board := chess.NewEmptyGame(chess.Black)
	board.SetPiece(chess.Pos{Row: 7, Col: 0}, chess.Piece{Type: chess.King, Color: chess.White}) // a1
	board.SetPiece(chess.Pos{Row: 5, Col: 2}, chess.Piece{Type: chess.King, Color: chess.Black}) // c3
	board.SetPiece(chess.Pos{Row: 6, Col: 1}, chess.Piece{Type: chess.Queen, Color: chess.Black}) // b2
	board.SetPiece(chess.Pos{Row: 5, Col: 0}, chess.Piece{Type: chess.Rook, Color: chess.Black}) // a3
	board.SetPiece(chess.Pos{Row: 0, Col: 7}, chess.Piece{Type: chess.Bishop, Color: chess.Black}) // h8

	e := NewEngine(state, board)
	e.extraMoveLeft[gameplay.PlayerB] = 1
	e.movesThisTurn[gameplay.PlayerB] = 1

	// Power-granted extra move that causes checkmate is allowed.
	err = e.SubmitMove(gameplay.PlayerB, chess.Move{From: chess.Pos{5, 0}, To: chess.Pos{6, 0}})
	if err != nil {
		t.Fatalf("expected power move to be allowed, got: %v", err)
	}
	if !e.Chess.IsCheckmate(chess.White) {
		t.Fatalf("expected white to be in checkmate after power move")
	}
}

func TestRookTouchBuffAllowsRookMovementPattern(t *testing.T) {
	card := gameplay.CardInstance{InstanceID: "r1", CardID: CardRookTouch, ManaCost: 3, Ignition: 0, Cooldown: 2}
	state, _ := gameplay.NewMatchState(testDeckWith(card), testDeckWith(card))
	state.Players[gameplay.PlayerA].Hand = []gameplay.CardInstance{card}
	state.Players[gameplay.PlayerA].Mana = 10

	board := chess.NewEmptyGame(chess.White)
	board.SetPiece(chess.Pos{Row: 7, Col: 4}, chess.Piece{Type: chess.King, Color: chess.White})
	board.SetPiece(chess.Pos{Row: 0, Col: 4}, chess.Piece{Type: chess.King, Color: chess.Black})
	board.SetPiece(chess.Pos{Row: 6, Col: 1}, chess.Piece{Type: chess.Knight, Color: chess.White})

	e := NewEngine(state, board)
	_ = e.StartTurn(gameplay.PlayerA)
	_ = e.ActivateCard(gameplay.PlayerA, 0)
	_ = state.ResolveIgnition(true)
	_ = e.StartTurn(gameplay.PlayerA)
	target := chess.Pos{Row: 6, Col: 1}
	if err := e.ResolvePendingEffect(gameplay.PlayerA, EffectTarget{PiecePos: &target}); err != nil {
		t.Fatalf("resolve pending effect error: %v", err)
	}
	if err := e.SubmitMove(gameplay.PlayerA, chess.Move{From: chess.Pos{6, 1}, To: chess.Pos{3, 1}}); err != nil {
		t.Fatalf("rook-touch movement should be legal: %v", err)
	}
}

func TestSubmitMoveAdvancesTurnToOpponent(t *testing.T) {
	state, _ := gameplay.NewMatchState(
		testDeckWith(gameplay.CardInstance{InstanceID: "a", CardID: CardDoubleTurn, ManaCost: 1, Ignition: 1, Cooldown: 1}),
		testDeckWith(gameplay.CardInstance{InstanceID: "b", CardID: CardDoubleTurn, ManaCost: 1, Ignition: 1, Cooldown: 1}),
	)
	board := chess.NewGame()
	e := NewEngine(state, board)

	if err := e.SubmitMove(gameplay.PlayerA, chess.Move{
		From: chess.Pos{Row: 6, Col: 4},
		To:   chess.Pos{Row: 4, Col: 4},
	}); err != nil {
		t.Fatalf("submit move failed: %v", err)
	}
	if e.State.CurrentTurn != gameplay.PlayerB {
		t.Fatalf("expected turn to advance to player B")
	}
}

func TestSubmitMoveReconcilesPersistedTurnMismatch(t *testing.T) {
	state, _ := gameplay.NewMatchState(
		testDeckWith(gameplay.CardInstance{InstanceID: "a", CardID: CardDoubleTurn, ManaCost: 1, Ignition: 1, Cooldown: 1}),
		testDeckWith(gameplay.CardInstance{InstanceID: "b", CardID: CardDoubleTurn, ManaCost: 1, Ignition: 1, Cooldown: 1}),
	)
	board := chess.NewGame()
	e := NewEngine(state, board)

	// Simulate stale persisted mismatch: gameplay says A, chess turn says Black.
	e.State.CurrentTurn = gameplay.PlayerA
	e.Chess.Turn = chess.Black
	if err := e.SubmitMove(gameplay.PlayerB, chess.Move{
		From: chess.Pos{Row: 1, Col: 4},
		To:   chess.Pos{Row: 3, Col: 4},
	}); err != nil {
		t.Fatalf("submit move should auto-reconcile mismatch: %v", err)
	}
}

func TestKnightTouchRejectsKingAsTarget(t *testing.T) {
	card := gameplay.CardInstance{InstanceID: "k2", CardID: CardKnightTouch, ManaCost: 3, Ignition: 0, Cooldown: 2}
	state, _ := gameplay.NewMatchState(testDeckWith(card), testDeckWith(card))
	state.Players[gameplay.PlayerA].Hand = []gameplay.CardInstance{card}
	state.Players[gameplay.PlayerA].Mana = 10

	board := chess.NewEmptyGame(chess.White)
	board.SetPiece(chess.Pos{Row: 7, Col: 4}, chess.Piece{Type: chess.King, Color: chess.White})
	board.SetPiece(chess.Pos{Row: 0, Col: 4}, chess.Piece{Type: chess.King, Color: chess.Black})
	e := NewEngine(state, board)

	_ = e.StartTurn(gameplay.PlayerA)
	_ = e.ActivateCard(gameplay.PlayerA, 0)
	_ = state.ResolveIgnition(true)
	_ = e.StartTurn(gameplay.PlayerA)
	kingPos := chess.Pos{Row: 7, Col: 4}
	if err := e.ResolvePendingEffect(gameplay.PlayerA, EffectTarget{PiecePos: &kingPos}); err == nil {
		t.Fatalf("expected king target rejection")
	}
}

func TestReactionWindowRestrictsCardTypeActivation(t *testing.T) {
	counterCard := gameplay.CardInstance{InstanceID: "ct1", CardID: "counterattack", ManaCost: 1, Ignition: 0, Cooldown: 6}
	powerCard := gameplay.CardInstance{InstanceID: "pw1", CardID: CardKnightTouch, ManaCost: 3, Ignition: 0, Cooldown: 2}

	state, _ := gameplay.NewMatchState(testDeckWith(counterCard), testDeckWith(counterCard))
	state.Players[gameplay.PlayerA].Hand = []gameplay.CardInstance{powerCard, counterCard}
	state.Players[gameplay.PlayerA].Mana = 10
	e := NewEngine(state, chess.NewEmptyGame(chess.White))

	e.OpenReactionWindow("test", gameplay.PlayerA, []gameplay.CardType{gameplay.CardTypeCounter})
	if err := e.ActivateCard(gameplay.PlayerA, 0); err == nil {
		t.Fatalf("power card should be blocked by counter-only reaction window")
	}
	if err := e.QueueReactionCard(gameplay.PlayerA, 1, EffectTarget{}); err != nil {
		t.Fatalf("counter card should be queueable in counter-only reaction window: %v", err)
	}
}

func TestExtinguishNegatesOpponentIgnition(t *testing.T) {
	extinguish := gameplay.CardInstance{InstanceID: "ex1", CardID: CardExtinguish, ManaCost: 2, Ignition: 0, Cooldown: 2}
	doubleTurn := gameplay.CardInstance{InstanceID: "dt1", CardID: CardDoubleTurn, ManaCost: 4, Ignition: 1, Cooldown: 5}

	state, _ := gameplay.NewMatchState(testDeckWith(doubleTurn), testDeckWith(extinguish))
	state.CurrentTurn = gameplay.PlayerA
	state.Players[gameplay.PlayerA].Hand = []gameplay.CardInstance{doubleTurn}
	state.Players[gameplay.PlayerB].Hand = []gameplay.CardInstance{extinguish}
	state.Players[gameplay.PlayerA].Mana = 10
	state.Players[gameplay.PlayerB].Mana = 10
	e := NewEngine(state, chess.NewEmptyGame(chess.White))

	if err := e.ActivateCard(gameplay.PlayerA, 0); err != nil {
		t.Fatalf("activate double-turn failed: %v", err)
	}
	if !state.IgnitionSlot.Occupied {
		t.Fatalf("expected ignition slot occupied")
	}

	e.OpenReactionWindow("on-ignite", gameplay.PlayerA, []gameplay.CardType{gameplay.CardTypeRetribution, gameplay.CardTypePower})
	if err := e.QueueReactionCard(gameplay.PlayerB, 0, EffectTarget{}); err != nil {
		t.Fatalf("queue extinguish failed: %v", err)
	}
	if err := e.ResolveReactionStack(); err != nil {
		t.Fatalf("resolve reaction stack failed: %v", err)
	}
	if state.IgnitionSlot.Occupied {
		t.Fatalf("ignition should be cleared after extinguish")
	}
	events := state.PopResolvedIgnitions()
	if len(events) == 0 || events[len(events)-1].Success {
		t.Fatalf("expected resolved ignition with success=false")
	}
}

func TestIgnitionZeroAllowsMultipleActivationsSameTurnWhenSlotFree(t *testing.T) {
	k1 := gameplay.CardInstance{InstanceID: "k1", CardID: CardKnightTouch, ManaCost: 3, Ignition: 0, Cooldown: 2}
	k2 := gameplay.CardInstance{InstanceID: "k2", CardID: CardRookTouch, ManaCost: 3, Ignition: 0, Cooldown: 2}
	state, _ := gameplay.NewMatchState(testDeckWith(k1), testDeckWith(k2))
	state.Players[gameplay.PlayerA].Hand = []gameplay.CardInstance{k1, k2}
	state.Players[gameplay.PlayerA].Mana = 10
	board := chess.NewEmptyGame(chess.White)
	board.SetPiece(chess.Pos{Row: 7, Col: 4}, chess.Piece{Type: chess.King, Color: chess.White})
	board.SetPiece(chess.Pos{Row: 0, Col: 4}, chess.Piece{Type: chess.King, Color: chess.Black})
	e := NewEngine(state, board)

	if err := e.ActivateCard(gameplay.PlayerA, 0); err != nil {
		t.Fatalf("first ignition-0 activate failed: %v", err)
	}
	if state.IgnitionSlot.Occupied {
		t.Fatalf("ignition slot should be free after ignition-0 resolution")
	}
	if err := e.ActivateCard(gameplay.PlayerA, 0); err != nil {
		t.Fatalf("second ignition-0 activate failed: %v", err)
	}
}

func TestRetributionCannotActivateInNormalTurnFlow(t *testing.T) {
	r := gameplay.CardInstance{InstanceID: "r1", CardID: CardStopRightThere, ManaCost: 3, Ignition: 0, Cooldown: 5}
	state, _ := gameplay.NewMatchState(testDeckWith(r), testDeckWith(r))
	state.Players[gameplay.PlayerA].Hand = []gameplay.CardInstance{r}
	state.Players[gameplay.PlayerA].Mana = 10
	e := NewEngine(state, chess.NewEmptyGame(chess.White))

	if err := e.ActivateCard(gameplay.PlayerA, 0); err == nil {
		t.Fatalf("retribution should not be activatable in normal turn flow")
	}
}

func TestCaptureTriggerWindowOpensAutomaticallyAndDefersMove(t *testing.T) {
	state, _ := gameplay.NewMatchState(testDeckWith(gameplay.CardInstance{InstanceID: "f", CardID: "double-turn", ManaCost: 1, Ignition: 1, Cooldown: 1}), testDeckWith(gameplay.CardInstance{InstanceID: "f2", CardID: "double-turn", ManaCost: 1, Ignition: 1, Cooldown: 1}))
	board := chess.NewEmptyGame(chess.White)
	board.SetPiece(chess.Pos{Row: 7, Col: 4}, chess.Piece{Type: chess.King, Color: chess.White})
	board.SetPiece(chess.Pos{Row: 0, Col: 4}, chess.Piece{Type: chess.King, Color: chess.Black})
	board.SetPiece(chess.Pos{Row: 6, Col: 4}, chess.Piece{Type: chess.Pawn, Color: chess.White})
	board.SetPiece(chess.Pos{Row: 5, Col: 5}, chess.Piece{Type: chess.Pawn, Color: chess.Black})

	e := NewEngine(state, board)
	if err := e.SubmitMove(gameplay.PlayerA, chess.Move{From: chess.Pos{6, 4}, To: chess.Pos{5, 5}}); err != nil {
		t.Fatalf("submit capture move should open reaction window: %v", err)
	}
	if e.ReactionWindow == nil || !e.ReactionWindow.Open {
		t.Fatalf("capture trigger window should be opened")
	}
	if e.pendingMove == nil {
		t.Fatalf("pending move should be stored until reaction resolution")
	}
	// Move is not applied yet.
	if board.PieceAt(chess.Pos{6, 4}).Type != chess.Pawn {
		t.Fatalf("piece should remain on source while capture chain is pending")
	}
	if err := e.ResolveReactionStack(); err != nil {
		t.Fatalf("resolving empty capture chain should apply pending move: %v", err)
	}
	if board.PieceAt(chess.Pos{5, 5}).Color != chess.White {
		t.Fatalf("pending capture move should be applied after chain resolves")
	}
}

func TestEnPassantCaptureOpensCaptureTriggerWindow(t *testing.T) {
	state, _ := gameplay.NewMatchState(testDeckWith(gameplay.CardInstance{InstanceID: "f", CardID: "double-turn", ManaCost: 1, Ignition: 1, Cooldown: 1}), testDeckWith(gameplay.CardInstance{InstanceID: "f2", CardID: "double-turn", ManaCost: 1, Ignition: 1, Cooldown: 1}))
	board := chess.NewEmptyGame(chess.Black)
	board.SetPiece(chess.Pos{Row: 7, Col: 4}, chess.Piece{Type: chess.King, Color: chess.White})
	board.SetPiece(chess.Pos{Row: 0, Col: 4}, chess.Piece{Type: chess.King, Color: chess.Black})
	board.SetPiece(chess.Pos{Row: 3, Col: 4}, chess.Piece{Type: chess.Pawn, Color: chess.White})
	board.SetPiece(chess.Pos{Row: 1, Col: 5}, chess.Piece{Type: chess.Pawn, Color: chess.Black})
	e := NewEngine(state, board)
	state.CurrentTurn = gameplay.PlayerB

	// Black sets en passant.
	if err := e.SubmitMove(gameplay.PlayerB, chess.Move{From: chess.Pos{1, 5}, To: chess.Pos{3, 5}}); err != nil {
		t.Fatalf("black setup move failed: %v", err)
	}
	state.CurrentTurn = gameplay.PlayerA
	board.Turn = chess.White

	// White en passant should open capture window.
	if err := e.SubmitMove(gameplay.PlayerA, chess.Move{From: chess.Pos{3, 4}, To: chess.Pos{2, 5}}); err != nil {
		t.Fatalf("en passant submit should open reaction window: %v", err)
	}
	if e.ReactionWindow == nil || e.ReactionWindow.Trigger != "capture_attempt" {
		t.Fatalf("expected capture_attempt window for en passant")
	}
}

func TestCounterattackCancelsPendingCaptureWhenAttackerIsBuffed(t *testing.T) {
	knightTouch := gameplay.CardInstance{InstanceID: "k1", CardID: CardKnightTouch, ManaCost: 3, Ignition: 0, Cooldown: 2}
	counterattack := gameplay.CardInstance{InstanceID: "c1", CardID: CardCounterattack, ManaCost: 1, Ignition: 0, Cooldown: 6}
	state, _ := gameplay.NewMatchState(testDeckWith(knightTouch), testDeckWith(counterattack))
	state.Players[gameplay.PlayerA].Hand = []gameplay.CardInstance{knightTouch}
	state.Players[gameplay.PlayerB].Hand = []gameplay.CardInstance{counterattack}
	state.Players[gameplay.PlayerA].Mana = 10
	state.Players[gameplay.PlayerB].Mana = 10

	board := chess.NewEmptyGame(chess.White)
	board.SetPiece(chess.Pos{Row: 7, Col: 4}, chess.Piece{Type: chess.King, Color: chess.White})
	board.SetPiece(chess.Pos{Row: 0, Col: 4}, chess.Piece{Type: chess.King, Color: chess.Black})
	board.SetPiece(chess.Pos{Row: 6, Col: 0}, chess.Piece{Type: chess.Pawn, Color: chess.White})
	board.SetPiece(chess.Pos{Row: 4, Col: 1}, chess.Piece{Type: chess.Rook, Color: chess.Black})
	e := NewEngine(state, board)

	// Apply knight-touch to white pawn.
	_ = e.ActivateCard(gameplay.PlayerA, 0)
	target := chess.Pos{Row: 6, Col: 0}
	_ = e.ResolvePendingEffect(gameplay.PlayerA, EffectTarget{PiecePos: &target})
	// Attempt capture with buffed piece -> opens capture window.
	if err := e.SubmitMove(gameplay.PlayerA, chess.Move{From: chess.Pos{6, 0}, To: chess.Pos{4, 1}}); err != nil {
		t.Fatalf("expected capture attempt to open reaction window: %v", err)
	}

	if err := e.QueueReactionCard(gameplay.PlayerB, 0, EffectTarget{}); err != nil {
		t.Fatalf("counterattack should be queueable: %v", err)
	}
	if err := e.ResolveReactionStack(); err != nil {
		t.Fatalf("counterattack resolution failed: %v", err)
	}
	// Attacker removed, target still alive.
	if !board.PieceAt(chess.Pos{6, 0}).IsEmpty() {
		t.Fatalf("attacker should be captured by counterattack")
	}
	if board.PieceAt(chess.Pos{4, 1}).Type != chess.Rook {
		t.Fatalf("defending rook should remain on board")
	}
}

func TestCounterattackRejectedWhenAttackerNotBuffed(t *testing.T) {
	counterattack := gameplay.CardInstance{InstanceID: "c2", CardID: CardCounterattack, ManaCost: 1, Ignition: 0, Cooldown: 6}
	state, _ := gameplay.NewMatchState(testDeckWith(counterattack), testDeckWith(counterattack))
	state.Players[gameplay.PlayerA].Hand = []gameplay.CardInstance{}
	state.Players[gameplay.PlayerB].Hand = []gameplay.CardInstance{counterattack}
	state.Players[gameplay.PlayerB].Mana = 10
	board := chess.NewEmptyGame(chess.White)
	board.SetPiece(chess.Pos{Row: 7, Col: 4}, chess.Piece{Type: chess.King, Color: chess.White})
	board.SetPiece(chess.Pos{Row: 0, Col: 4}, chess.Piece{Type: chess.King, Color: chess.Black})
	board.SetPiece(chess.Pos{Row: 6, Col: 4}, chess.Piece{Type: chess.Pawn, Color: chess.White})
	board.SetPiece(chess.Pos{Row: 5, Col: 5}, chess.Piece{Type: chess.Pawn, Color: chess.Black})
	e := NewEngine(state, board)

	_ = e.SubmitMove(gameplay.PlayerA, chess.Move{From: chess.Pos{6, 4}, To: chess.Pos{5, 5}})
	if err := e.QueueReactionCard(gameplay.PlayerB, 0, EffectTarget{}); err != nil {
		t.Fatalf("queue should succeed before resolver validation: %v", err)
	}
	if err := e.ResolveReactionStack(); err == nil {
		t.Fatalf("counterattack should fail when attacker is not buffed by power")
	}
}

func TestBlockadeNegatesCounterattackAndCancelsPendingCapture(t *testing.T) {
	knightTouch := gameplay.CardInstance{InstanceID: "k1", CardID: CardKnightTouch, ManaCost: 3, Ignition: 0, Cooldown: 2}
	counterattack := gameplay.CardInstance{InstanceID: "c1", CardID: CardCounterattack, ManaCost: 1, Ignition: 0, Cooldown: 6}
	blockade := gameplay.CardInstance{InstanceID: "b1", CardID: CardBlockade, ManaCost: 0, Ignition: 0, Cooldown: 3}
	state, _ := gameplay.NewMatchState(testDeckWith(knightTouch), testDeckWith(counterattack))
	state.Players[gameplay.PlayerA].Hand = []gameplay.CardInstance{knightTouch, blockade}
	state.Players[gameplay.PlayerB].Hand = []gameplay.CardInstance{counterattack}
	state.Players[gameplay.PlayerA].Mana = 10
	state.Players[gameplay.PlayerB].Mana = 10
	board := chess.NewEmptyGame(chess.White)
	board.SetPiece(chess.Pos{Row: 7, Col: 4}, chess.Piece{Type: chess.King, Color: chess.White})
	board.SetPiece(chess.Pos{Row: 0, Col: 4}, chess.Piece{Type: chess.King, Color: chess.Black})
	board.SetPiece(chess.Pos{Row: 6, Col: 0}, chess.Piece{Type: chess.Pawn, Color: chess.White})
	board.SetPiece(chess.Pos{Row: 4, Col: 1}, chess.Piece{Type: chess.Rook, Color: chess.Black})
	e := NewEngine(state, board)

	_ = e.ActivateCard(gameplay.PlayerA, 0)
	target := chess.Pos{Row: 6, Col: 0}
	_ = e.ResolvePendingEffect(gameplay.PlayerA, EffectTarget{PiecePos: &target})
	_ = e.SubmitMove(gameplay.PlayerA, chess.Move{From: chess.Pos{6, 0}, To: chess.Pos{4, 1}})
	if err := e.QueueReactionCard(gameplay.PlayerB, 0, EffectTarget{}); err != nil {
		t.Fatalf("counterattack should queue: %v", err)
	}
	if err := e.QueueReactionCard(gameplay.PlayerA, 0, EffectTarget{}); err != nil {
		t.Fatalf("blockade should queue in response to counterattack: %v", err)
	}
	if err := e.ResolveReactionStack(); err != nil {
		t.Fatalf("reaction stack should resolve with blockade negation: %v", err)
	}
	// Pending capture canceled, attacker remains on original square.
	if board.PieceAt(chess.Pos{6, 0}).Type != chess.Pawn {
		t.Fatalf("attacker should remain on original square after blockade")
	}
	if board.PieceAt(chess.Pos{4, 1}).Type != chess.Rook {
		t.Fatalf("target piece should remain after blockade canceled capture")
	}
}
