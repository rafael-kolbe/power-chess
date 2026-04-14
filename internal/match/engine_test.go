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

// markInPlayForTest sets match flags so engine actions apply without running opening mulligan (tests only).
func markInPlayForTest(s *gameplay.MatchState) {
	s.MulliganPhaseActive = false
	s.Started = true
}

func TestDeliverCheckmateWithSingleLegalMove(t *testing.T) {
	state, err := gameplay.NewMatchState(
		testDeckWith(gameplay.CardInstance{InstanceID: "a", CardID: CardDoubleTurn, ManaCost: 1, Ignition: 1, Cooldown: 1}),
		testDeckWith(gameplay.CardInstance{InstanceID: "b", CardID: CardDoubleTurn, ManaCost: 1, Ignition: 1, Cooldown: 1}),
	)
	if err != nil {
		t.Fatal(err)
	}
	state.CurrentTurn = gameplay.PlayerB

	board := chess.NewEmptyGame(chess.Black)
	board.SetPiece(chess.Pos{Row: 7, Col: 0}, chess.Piece{Type: chess.King, Color: chess.White})
	board.SetPiece(chess.Pos{Row: 5, Col: 2}, chess.Piece{Type: chess.King, Color: chess.Black})
	board.SetPiece(chess.Pos{Row: 6, Col: 1}, chess.Piece{Type: chess.Queen, Color: chess.Black})
	board.SetPiece(chess.Pos{Row: 5, Col: 0}, chess.Piece{Type: chess.Rook, Color: chess.Black})
	board.SetPiece(chess.Pos{Row: 0, Col: 7}, chess.Piece{Type: chess.Bishop, Color: chess.Black})

	e := NewEngine(state, board)
	markInPlayForTest(state)
	if err := e.SubmitMove(gameplay.PlayerB, chess.Move{From: chess.Pos{Row: 5, Col: 0}, To: chess.Pos{Row: 6, Col: 0}}); err != nil {
		t.Fatalf("mate move: %v", err)
	}
	if !e.Chess.IsCheckmate(chess.White) {
		t.Fatal("expected checkmate on white")
	}
}

func TestSubmitMoveAdvancesTurnToOpponent(t *testing.T) {
	state, _ := gameplay.NewMatchState(
		testDeckWith(gameplay.CardInstance{InstanceID: "a", CardID: CardDoubleTurn, ManaCost: 1, Ignition: 1, Cooldown: 1}),
		testDeckWith(gameplay.CardInstance{InstanceID: "b", CardID: CardDoubleTurn, ManaCost: 1, Ignition: 1, Cooldown: 1}),
	)
	board := chess.NewGame()
	e := NewEngine(state, board)
	markInPlayForTest(state)

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
	markInPlayForTest(state)

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

func TestReactionWindowRestrictsCardTypeActivation(t *testing.T) {
	counterCard := gameplay.CardInstance{InstanceID: "ct1", CardID: "counterattack", ManaCost: 1, Ignition: 0, Cooldown: 6}
	powerCard := gameplay.CardInstance{InstanceID: "pw1", CardID: CardKnightTouch, ManaCost: 3, Ignition: 0, Cooldown: 2}

	state, _ := gameplay.NewMatchState(testDeckWith(counterCard), testDeckWith(counterCard))
	state.Players[gameplay.PlayerA].Hand = []gameplay.CardInstance{powerCard, counterCard}
	state.Players[gameplay.PlayerA].Mana = 10
	e := NewEngine(state, chess.NewEmptyGame(chess.White))
	markInPlayForTest(state)

	e.OpenReactionWindow("test", gameplay.PlayerA, []gameplay.CardType{gameplay.CardTypeCounter})
	if err := e.ActivateCard(gameplay.PlayerA, 0); err == nil {
		t.Fatalf("power card should be blocked by counter-only reaction window")
	}
	if err := e.QueueReactionCard(gameplay.PlayerA, 1, EffectTarget{}); err != nil {
		t.Fatalf("counter card should be queueable in counter-only reaction window: %v", err)
	}
}

func TestIgniteReactionResolveClearsIgnitionAfterOpponentPowerResponse(t *testing.T) {
	extinguish := gameplay.CardInstance{InstanceID: "ex1", CardID: CardExtinguish, ManaCost: 2, Ignition: 0, Cooldown: 2}
	knight := gameplay.CardInstance{InstanceID: "k1", CardID: CardKnightTouch, ManaCost: 3, Ignition: 0, Cooldown: 2}

	state, _ := gameplay.NewMatchState(testDeckWith(knight), testDeckWith(extinguish))
	state.CurrentTurn = gameplay.PlayerA
	state.Players[gameplay.PlayerA].Hand = []gameplay.CardInstance{knight}
	state.Players[gameplay.PlayerB].Hand = []gameplay.CardInstance{extinguish}
	state.Players[gameplay.PlayerA].Mana = 10
	state.Players[gameplay.PlayerB].Mana = 10
	e := NewEngine(state, chess.NewEmptyGame(chess.White))
	markInPlayForTest(state)

	if err := e.ActivateCard(gameplay.PlayerA, 0); err != nil {
		t.Fatalf("activate knight-touch failed: %v", err)
	}
	if !state.IgnitionSlot.Occupied {
		t.Fatalf("expected ignition slot occupied")
	}
	rw, _, ok := e.ReactionWindowSnapshot()
	if !ok || rw.Trigger != "ignite_reaction" {
		t.Fatalf("expected ignite_reaction window open, got ok=%v rw=%+v", ok, rw)
	}

	if err := e.QueueReactionCard(gameplay.PlayerB, 0, EffectTarget{}); err != nil {
		t.Fatalf("queue opponent Power response failed: %v", err)
	}
	if err := e.ResolveReactionStack(); err != nil {
		t.Fatalf("resolve reaction stack failed: %v", err)
	}
	if state.IgnitionSlot.Occupied {
		t.Fatalf("ignition-0 card should finish after ignite chain resolves")
	}
}

func TestIgniteReactionKeepsDelayedCardInSlotAfterChain(t *testing.T) {
	eg := gameplay.CardInstance{InstanceID: "e1", CardID: "energy-gain", ManaCost: 0, Ignition: 1, Cooldown: 2}
	state, _ := gameplay.NewMatchState(testDeckWith(eg), testDeckWith(eg))
	state.Players[gameplay.PlayerA].Hand = []gameplay.CardInstance{eg}
	state.Players[gameplay.PlayerA].Mana = 10
	e := NewEngine(state, chess.NewEmptyGame(chess.White))
	markInPlayForTest(state)
	if err := e.ActivateCard(gameplay.PlayerA, 0); err != nil {
		t.Fatalf("activate: %v", err)
	}
	if err := e.ResolveReactionStack(); err != nil {
		t.Fatalf("resolve empty ignite chain: %v", err)
	}
	if !state.IgnitionSlot.Occupied {
		t.Fatal("energy-gain should stay in ignition until burn completes")
	}
	if state.IgnitionSlot.TurnsRemaining != 1 {
		t.Fatalf("expected 1 turn remaining on slot, got %d", state.IgnitionSlot.TurnsRemaining)
	}
	if len(state.Players[gameplay.PlayerA].Cooldowns) != 0 {
		t.Fatal("card must not go to cooldown before ignition resolves")
	}
}

func TestIgnitionZeroNeedsReactionResolveBeforeNextActivation(t *testing.T) {
	k1 := gameplay.CardInstance{InstanceID: "k1", CardID: CardKnightTouch, ManaCost: 3, Ignition: 0, Cooldown: 2}
	k2 := gameplay.CardInstance{InstanceID: "k2", CardID: CardRookTouch, ManaCost: 3, Ignition: 0, Cooldown: 2}
	state, _ := gameplay.NewMatchState(testDeckWith(k1), testDeckWith(k2))
	state.Players[gameplay.PlayerA].Hand = []gameplay.CardInstance{k1, k2}
	state.Players[gameplay.PlayerA].Mana = 10
	board := chess.NewEmptyGame(chess.White)
	board.SetPiece(chess.Pos{Row: 7, Col: 4}, chess.Piece{Type: chess.King, Color: chess.White})
	board.SetPiece(chess.Pos{Row: 0, Col: 4}, chess.Piece{Type: chess.King, Color: chess.Black})
	e := NewEngine(state, board)
	markInPlayForTest(state)

	if err := e.ActivateCard(gameplay.PlayerA, 0); err != nil {
		t.Fatalf("first ignition-0 activate failed: %v", err)
	}
	if !state.IgnitionSlot.Occupied {
		t.Fatalf("ignition slot should stay occupied while ignite reaction window is open")
	}
	if err := e.ActivateCard(gameplay.PlayerA, 0); err == nil {
		t.Fatalf("second activation must be blocked while first card awaits response")
	}
	if err := e.ResolveReactionStack(); err != nil {
		t.Fatalf("resolving empty ignite chain failed: %v", err)
	}
	if err := e.ActivateCard(gameplay.PlayerA, 0); err != nil {
		t.Fatalf("second ignition-0 activate failed: %v", err)
	}
}

func TestSubmitMoveBlockedDuringIgniteReaction(t *testing.T) {
	doubleTurn := gameplay.CardInstance{InstanceID: "dt1", CardID: CardDoubleTurn, ManaCost: 4, Ignition: 1, Cooldown: 5}
	state, _ := gameplay.NewMatchState(testDeckWith(doubleTurn), testDeckWith(doubleTurn))
	state.CurrentTurn = gameplay.PlayerA
	state.Players[gameplay.PlayerA].Hand = []gameplay.CardInstance{doubleTurn}
	state.Players[gameplay.PlayerA].Mana = 10
	board := chess.NewEmptyGame(chess.White)
	board.SetPiece(chess.Pos{Row: 7, Col: 4}, chess.Piece{Type: chess.King, Color: chess.White})
	board.SetPiece(chess.Pos{Row: 0, Col: 4}, chess.Piece{Type: chess.King, Color: chess.Black})
	board.SetPiece(chess.Pos{Row: 6, Col: 4}, chess.Piece{Type: chess.Pawn, Color: chess.White})
	e := NewEngine(state, board)
	markInPlayForTest(state)
	if err := e.ActivateCard(gameplay.PlayerA, 0); err != nil {
		t.Fatalf("activate: %v", err)
	}
	if err := e.SubmitMove(gameplay.PlayerA, chess.Move{From: chess.Pos{Row: 6, Col: 4}, To: chess.Pos{Row: 5, Col: 4}}); err == nil {
		t.Fatal("expected submit_move blocked during ignite_reaction")
	}
}

func TestIgniteReactionRejectsActorStartingChain(t *testing.T) {
	stopThere := gameplay.CardInstance{InstanceID: "s1", CardID: CardStopRightThere, ManaCost: 3, Ignition: 0, Cooldown: 5}
	state, _ := gameplay.NewMatchState(testDeckWith(stopThere), testDeckWith(stopThere))
	state.Players[gameplay.PlayerA].Hand = []gameplay.CardInstance{stopThere}
	state.Players[gameplay.PlayerA].Mana = 10
	e := NewEngine(state, chess.NewEmptyGame(chess.White))
	markInPlayForTest(state)
	e.OpenReactionWindow("ignite_reaction", gameplay.PlayerA, []gameplay.CardType{gameplay.CardTypeRetribution, gameplay.CardTypePower})
	if err := e.QueueReactionCard(gameplay.PlayerA, 0, EffectTarget{}); err == nil {
		t.Fatal("expected actor cannot open ignite_reaction chain")
	}
}

func TestRetributionCannotActivateInNormalTurnFlow(t *testing.T) {
	r := gameplay.CardInstance{InstanceID: "r1", CardID: CardStopRightThere, ManaCost: 3, Ignition: 0, Cooldown: 5}
	state, _ := gameplay.NewMatchState(testDeckWith(r), testDeckWith(r))
	state.Players[gameplay.PlayerA].Hand = []gameplay.CardInstance{r}
	state.Players[gameplay.PlayerA].Mana = 10
	e := NewEngine(state, chess.NewEmptyGame(chess.White))
	markInPlayForTest(state)

	if err := e.ActivateCard(gameplay.PlayerA, 0); err == nil {
		t.Fatalf("retribution should not be activatable in normal turn flow")
	}
}

// TestActivateCardDuringIgniteReactionQueuesLikeQueueReaction ensures clients that send
// activate_card while a reaction window is open still enqueue the stack (responder is off-turn).
func TestActivateCardDuringIgniteReactionQueuesLikeQueueReaction(t *testing.T) {
	stopThere := gameplay.CardInstance{InstanceID: "s1", CardID: CardStopRightThere, ManaCost: 3, Ignition: 0, Cooldown: 5}
	state, _ := gameplay.NewMatchState(testDeckWith(stopThere), testDeckWith(stopThere))
	state.CurrentTurn = gameplay.PlayerA
	state.Players[gameplay.PlayerB].Hand = []gameplay.CardInstance{stopThere}
	state.Players[gameplay.PlayerB].Mana = 10
	e := NewEngine(state, chess.NewEmptyGame(chess.White))
	markInPlayForTest(state)
	e.OpenReactionWindow("ignite_reaction", gameplay.PlayerA, []gameplay.CardType{gameplay.CardTypeRetribution, gameplay.CardTypePower})
	if err := e.ActivateCard(gameplay.PlayerB, 0); err != nil {
		t.Fatalf("activate_card during ignite_reaction should queue retribution: %v", err)
	}
	_, sz, ok := e.ReactionWindowSnapshot()
	if !ok || sz != 1 {
		t.Fatalf("expected one queued reaction, ok=%v sz=%d", ok, sz)
	}
}

func TestCanPlayerExtendIgniteChain(t *testing.T) {
	stopThere := gameplay.CardInstance{InstanceID: "s1", CardID: CardStopRightThere, ManaCost: 3, Ignition: 0, Cooldown: 5}
	ext := gameplay.CardInstance{InstanceID: "e1", CardID: CardExtinguish, ManaCost: 2, Ignition: 0, Cooldown: 2}
	state, _ := gameplay.NewMatchState(testDeckWith(stopThere), testDeckWith(ext))
	state.CurrentTurn = gameplay.PlayerA
	state.Players[gameplay.PlayerB].Hand = []gameplay.CardInstance{stopThere}
	state.Players[gameplay.PlayerB].Mana = 10
	state.Players[gameplay.PlayerA].Hand = []gameplay.CardInstance{ext}
	state.Players[gameplay.PlayerA].Mana = 10
	e := NewEngine(state, chess.NewEmptyGame(chess.White))
	markInPlayForTest(state)
	e.OpenReactionWindow("ignite_reaction", gameplay.PlayerA, []gameplay.CardType{gameplay.CardTypeRetribution, gameplay.CardTypePower})
	if err := e.QueueReactionCard(gameplay.PlayerB, 0, EffectTarget{}); err != nil {
		t.Fatalf("queue: %v", err)
	}
	if e.CanPlayerExtendIgniteChain(gameplay.PlayerA) {
		t.Fatal("expected A cannot extend after Retribution with only Power in hand")
	}
	state.Players[gameplay.PlayerA].Hand = []gameplay.CardInstance{stopThere}
	if !e.CanPlayerExtendIgniteChain(gameplay.PlayerA) {
		t.Fatal("expected A can extend with Retribution")
	}
}

func TestReactionStackEntriesBottomFirstOrder(t *testing.T) {
	power := gameplay.CardInstance{InstanceID: "p1", CardID: CardDoubleTurn, ManaCost: 1, Ignition: 1, Cooldown: 1}
	ret := gameplay.CardInstance{InstanceID: "r1", CardID: CardStopRightThere, ManaCost: 3, Ignition: 0, Cooldown: 5}
	state, _ := gameplay.NewMatchState(testDeckWith(power), testDeckWith(ret))
	state.CurrentTurn = gameplay.PlayerA
	state.Players[gameplay.PlayerB].Hand = []gameplay.CardInstance{power}
	state.Players[gameplay.PlayerB].Mana = 10
	state.Players[gameplay.PlayerA].Hand = []gameplay.CardInstance{ret}
	state.Players[gameplay.PlayerA].Mana = 10
	e := NewEngine(state, chess.NewEmptyGame(chess.White))
	markInPlayForTest(state)
	e.OpenReactionWindow("ignite_reaction", gameplay.PlayerA, []gameplay.CardType{gameplay.CardTypeRetribution, gameplay.CardTypePower})
	if err := e.QueueReactionCard(gameplay.PlayerB, 0, EffectTarget{}); err != nil {
		t.Fatalf("queue B: %v", err)
	}
	if err := e.QueueReactionCard(gameplay.PlayerA, 0, EffectTarget{}); err != nil {
		t.Fatalf("queue A: %v", err)
	}
	ents := e.ReactionStackEntries()
	if len(ents) != 2 {
		t.Fatalf("expected 2 stack entries, got %d", len(ents))
	}
	if ents[0].Owner != gameplay.PlayerB || ents[0].CardID != CardDoubleTurn {
		t.Fatalf("expected bottom Power from B, got %+v", ents[0])
	}
	if ents[1].Owner != gameplay.PlayerA || ents[1].CardID != CardStopRightThere {
		t.Fatalf("expected top Retribution from A, got %+v", ents[1])
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
	markInPlayForTest(state)
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
	markInPlayForTest(state)
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

func TestCounterChainNoOpThenCaptureStillApplies(t *testing.T) {
	counterattack := gameplay.CardInstance{InstanceID: "c1", CardID: CardCounterattack, ManaCost: 1, Ignition: 0, Cooldown: 6}
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
	markInPlayForTest(state)

	if err := e.SubmitMove(gameplay.PlayerA, chess.Move{From: chess.Pos{6, 4}, To: chess.Pos{5, 5}}); err != nil {
		t.Fatalf("capture attempt: %v", err)
	}
	if err := e.QueueReactionCard(gameplay.PlayerB, 0, EffectTarget{}); err != nil {
		t.Fatalf("counter should queue on any capture attempt: %v", err)
	}
	if err := e.ResolveReactionStack(); err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if board.PieceAt(chess.Pos{5, 5}).Type != chess.Pawn || board.PieceAt(chess.Pos{5, 5}).Color != chess.White {
		t.Fatal("capture should complete after noop Counter effects")
	}
}

func TestCounterChainTwoCountersThenCaptureCompletes(t *testing.T) {
	counterattack := gameplay.CardInstance{InstanceID: "c1", CardID: CardCounterattack, ManaCost: 1, Ignition: 0, Cooldown: 6}
	blockade := gameplay.CardInstance{InstanceID: "b1", CardID: CardBlockade, ManaCost: 0, Ignition: 0, Cooldown: 3}
	state, _ := gameplay.NewMatchState(testDeckWith(counterattack), testDeckWith(counterattack))
	state.Players[gameplay.PlayerA].Hand = []gameplay.CardInstance{blockade}
	state.Players[gameplay.PlayerB].Hand = []gameplay.CardInstance{counterattack}
	state.Players[gameplay.PlayerA].Mana = 10
	state.Players[gameplay.PlayerB].Mana = 10
	board := chess.NewEmptyGame(chess.White)
	board.SetPiece(chess.Pos{Row: 7, Col: 4}, chess.Piece{Type: chess.King, Color: chess.White})
	board.SetPiece(chess.Pos{Row: 0, Col: 4}, chess.Piece{Type: chess.King, Color: chess.Black})
	board.SetPiece(chess.Pos{Row: 6, Col: 4}, chess.Piece{Type: chess.Pawn, Color: chess.White})
	board.SetPiece(chess.Pos{Row: 5, Col: 5}, chess.Piece{Type: chess.Pawn, Color: chess.Black})
	e := NewEngine(state, board)
	markInPlayForTest(state)

	if err := e.SubmitMove(gameplay.PlayerA, chess.Move{From: chess.Pos{6, 4}, To: chess.Pos{5, 5}}); err != nil {
		t.Fatalf("capture attempt: %v", err)
	}
	if err := e.QueueReactionCard(gameplay.PlayerB, 0, EffectTarget{}); err != nil {
		t.Fatalf("defender counter: %v", err)
	}
	if err := e.QueueReactionCard(gameplay.PlayerA, 0, EffectTarget{}); err != nil {
		t.Fatalf("attacker second counter: %v", err)
	}
	if err := e.ResolveReactionStack(); err != nil {
		t.Fatalf("resolve stack: %v", err)
	}
	if board.PieceAt(chess.Pos{5, 5}).Type != chess.Pawn || board.PieceAt(chess.Pos{5, 5}).Color != chess.White {
		t.Fatal("pending capture should still apply after counter chain")
	}
}

// TestEngineDrawCard validates draw_card turn and mana constraints.
func TestEngineDrawCard(t *testing.T) {
	card := gameplay.CardInstance{InstanceID: "x1", CardID: "test", ManaCost: 1, Ignition: 0, Cooldown: 1}
	state, err := gameplay.NewMatchState(testDeckWith(card), testDeckWith(card))
	if err != nil {
		t.Fatalf("state: %v", err)
	}
	e := NewEngine(state, chess.NewGame())
	markInPlayForTest(state)

	// Drawing on wrong turn (B's turn by default? No — A starts).
	_ = e.State.EndTurn(gameplay.PlayerA)
	if err := e.DrawCard(gameplay.PlayerA); err == nil {
		t.Error("expected error: drawing on wrong turn")
	}

	// Reset to A's turn with sufficient mana.
	_ = e.State.EndTurn(gameplay.PlayerB)
	e.State.Players[gameplay.PlayerA].Mana = 10
	handBefore := len(e.State.Players[gameplay.PlayerA].Hand)
	deckBefore := len(e.State.Players[gameplay.PlayerA].Deck)

	if err := e.DrawCard(gameplay.PlayerA); err != nil {
		t.Fatalf("draw should succeed: %v", err)
	}
	if len(e.State.Players[gameplay.PlayerA].Hand) != handBefore+1 {
		t.Errorf("hand should grow by 1")
	}
	if len(e.State.Players[gameplay.PlayerA].Deck) != deckBefore-1 {
		t.Errorf("deck should shrink by 1")
	}

	// No mana.
	e.State.Players[gameplay.PlayerA].Mana = 0
	if err := e.DrawCard(gameplay.PlayerA); err == nil {
		t.Error("expected error: no mana")
	}

	// Full hand.
	e.State.Players[gameplay.PlayerA].Mana = 20
	for len(e.State.Players[gameplay.PlayerA].Hand) < gameplay.DefaultMaxHandSize {
		_ = e.State.Players[gameplay.PlayerA].Deck
		e.State.Players[gameplay.PlayerA].Hand = append(e.State.Players[gameplay.PlayerA].Hand, card)
	}
	if err := e.DrawCard(gameplay.PlayerA); err == nil {
		t.Error("expected error: hand full")
	}
}

// TestChessCaptureGrantsMana ensures capturing a piece awards +1 mana (capped by max mana).
func TestChessCaptureGrantsMana(t *testing.T) {
	state, err := gameplay.NewMatchState(
		testDeckWith(gameplay.CardInstance{InstanceID: "a", CardID: CardDoubleTurn, ManaCost: 1, Ignition: 1, Cooldown: 1}),
		testDeckWith(gameplay.CardInstance{InstanceID: "b", CardID: CardDoubleTurn, ManaCost: 1, Ignition: 1, Cooldown: 1}),
	)
	if err != nil {
		t.Fatal(err)
	}
	e := NewEngine(state, chess.NewGame())
	markInPlayForTest(state)
	if err := e.StartTurn(gameplay.PlayerA); err != nil {
		t.Fatal(err)
	}
	if err := e.SubmitMove(gameplay.PlayerA, chess.Move{From: chess.Pos{Row: 6, Col: 4}, To: chess.Pos{Row: 4, Col: 4}}); err != nil {
		t.Fatalf("e4: %v", err)
	}
	if err := e.SubmitMove(gameplay.PlayerB, chess.Move{From: chess.Pos{Row: 1, Col: 3}, To: chess.Pos{Row: 3, Col: 3}}); err != nil {
		t.Fatalf("d5: %v", err)
	}
	manaBefore := state.Players[gameplay.PlayerA].Mana
	if err := e.SubmitMove(gameplay.PlayerA, chess.Move{From: chess.Pos{Row: 4, Col: 4}, To: chess.Pos{Row: 3, Col: 3}}); err != nil {
		t.Fatalf("exd5: %v", err)
	}
	// Captures open a reaction window; the move (and capture mana) apply when the stack resolves.
	if err := e.ResolveReactionStack(); err != nil {
		t.Fatalf("resolve capture: %v", err)
	}
	if state.Players[gameplay.PlayerA].Mana != manaBefore+1 {
		t.Fatalf("capture should grant +1 mana: before %d after %d", manaBefore, state.Players[gameplay.PlayerA].Mana)
	}
}
