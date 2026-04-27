package match

import (
	"errors"
	"strings"
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

func TestSubmitMoveRejectsDirectKingCaptureWithSpecificReason(t *testing.T) {
	state, _ := gameplay.NewMatchState(
		testDeckWith(gameplay.CardInstance{InstanceID: "a", CardID: CardDoubleTurn, ManaCost: 1, Ignition: 1, Cooldown: 1}),
		testDeckWith(gameplay.CardInstance{InstanceID: "b", CardID: CardDoubleTurn, ManaCost: 1, Ignition: 1, Cooldown: 1}),
	)
	board := chess.NewEmptyGame(chess.White)
	board.SetPiece(chess.Pos{Row: 7, Col: 4}, chess.Piece{Type: chess.King, Color: chess.White})
	board.SetPiece(chess.Pos{Row: 0, Col: 4}, chess.Piece{Type: chess.King, Color: chess.Black})
	board.SetPiece(chess.Pos{Row: 1, Col: 4}, chess.Piece{Type: chess.Rook, Color: chess.White})
	e := NewEngine(state, board)
	markInPlayForTest(state)

	err := e.SubmitMove(gameplay.PlayerA, chess.Move{From: chess.Pos{Row: 1, Col: 4}, To: chess.Pos{Row: 0, Col: 4}})
	if !errors.Is(err, chess.ErrKingCannotBeCaptured) {
		t.Fatalf("expected king-capture rejection, got %v", err)
	}
}

func TestSubmitMoveRejectsPinnedPieceWithSpecificReason(t *testing.T) {
	state, _ := gameplay.NewMatchState(
		testDeckWith(gameplay.CardInstance{InstanceID: "a", CardID: CardDoubleTurn, ManaCost: 1, Ignition: 1, Cooldown: 1}),
		testDeckWith(gameplay.CardInstance{InstanceID: "b", CardID: CardDoubleTurn, ManaCost: 1, Ignition: 1, Cooldown: 1}),
	)
	board := chess.NewEmptyGame(chess.White)
	board.SetPiece(chess.Pos{Row: 7, Col: 4}, chess.Piece{Type: chess.King, Color: chess.White})
	board.SetPiece(chess.Pos{Row: 0, Col: 4}, chess.Piece{Type: chess.King, Color: chess.Black})
	board.SetPiece(chess.Pos{Row: 6, Col: 4}, chess.Piece{Type: chess.Rook, Color: chess.White})
	board.SetPiece(chess.Pos{Row: 0, Col: 4}, chess.Piece{Type: chess.King, Color: chess.Black})
	board.SetPiece(chess.Pos{Row: 3, Col: 4}, chess.Piece{Type: chess.Rook, Color: chess.Black})
	e := NewEngine(state, board)
	markInPlayForTest(state)

	err := e.SubmitMove(gameplay.PlayerA, chess.Move{From: chess.Pos{Row: 6, Col: 4}, To: chess.Pos{Row: 6, Col: 5}})
	if err == nil || !strings.Contains(err.Error(), "pinned") {
		t.Fatalf("expected pinned-piece rejection, got %v", err)
	}
}

func TestSubmitMoveRejectsMoveThatDoesNotResolveCheckWithSpecificReason(t *testing.T) {
	state, _ := gameplay.NewMatchState(
		testDeckWith(gameplay.CardInstance{InstanceID: "a", CardID: CardDoubleTurn, ManaCost: 1, Ignition: 1, Cooldown: 1}),
		testDeckWith(gameplay.CardInstance{InstanceID: "b", CardID: CardDoubleTurn, ManaCost: 1, Ignition: 1, Cooldown: 1}),
	)
	board := chess.NewEmptyGame(chess.White)
	board.SetPiece(chess.Pos{Row: 7, Col: 4}, chess.Piece{Type: chess.King, Color: chess.White})
	board.SetPiece(chess.Pos{Row: 0, Col: 0}, chess.Piece{Type: chess.King, Color: chess.Black})
	board.SetPiece(chess.Pos{Row: 0, Col: 4}, chess.Piece{Type: chess.Rook, Color: chess.Black})
	board.SetPiece(chess.Pos{Row: 6, Col: 0}, chess.Piece{Type: chess.Pawn, Color: chess.White})
	e := NewEngine(state, board)
	markInPlayForTest(state)

	err := e.SubmitMove(gameplay.PlayerA, chess.Move{From: chess.Pos{Row: 6, Col: 0}, To: chess.Pos{Row: 5, Col: 0}})
	if err == nil || !strings.Contains(err.Error(), "does not resolve check") {
		t.Fatalf("expected unresolved-check rejection, got %v", err)
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
	if err := e.QueueReactionCard(gameplay.PlayerA, 1, -1, EffectTarget{}); err != nil {
		t.Fatalf("counter card should be queueable in counter-only reaction window: %v", err)
	}
}

func TestIgniteReactionResolveClearsIgnitionAfterOpponentRetributionResponse(t *testing.T) {
	stopThere := gameplay.CardInstance{InstanceID: "s1", CardID: CardStopRightThere, ManaCost: 3, Ignition: 0, Cooldown: 5}
	knight := gameplay.CardInstance{InstanceID: "k1", CardID: CardKnightTouch, ManaCost: 3, Ignition: 0, Cooldown: 2}

	state, _ := gameplay.NewMatchState(testDeckWith(knight), testDeckWith(stopThere))
	state.CurrentTurn = gameplay.PlayerA
	state.Players[gameplay.PlayerA].Hand = []gameplay.CardInstance{knight}
	state.Players[gameplay.PlayerB].Hand = []gameplay.CardInstance{stopThere}
	state.Players[gameplay.PlayerA].Mana = 10
	state.Players[gameplay.PlayerB].Mana = 10
	board := chess.NewEmptyGame(chess.White)
	board.SetPiece(chess.Pos{Row: 7, Col: 4}, chess.Piece{Type: chess.King, Color: chess.White})
	board.SetPiece(chess.Pos{Row: 0, Col: 4}, chess.Piece{Type: chess.King, Color: chess.Black})
	board.SetPiece(chess.Pos{Row: 6, Col: 4}, chess.Piece{Type: chess.Pawn, Color: chess.White})
	e := NewEngine(state, board)
	markInPlayForTest(state)

	if err := e.ActivateCard(gameplay.PlayerA, 0); err != nil {
		t.Fatalf("activate knight-touch failed: %v", err)
	}
	if !state.Players[gameplay.PlayerA].Ignition.Occupied {
		t.Fatalf("expected ignition slot occupied")
	}
	if err := e.SubmitIgnitionTargets(gameplay.PlayerA, []chess.Pos{{Row: 6, Col: 4}}); err != nil {
		t.Fatalf("submit ignition targets: %v", err)
	}
	rw, _, ok := e.ReactionWindowSnapshot()
	if !ok || rw.Trigger != "ignite_reaction" {
		t.Fatalf("expected ignite_reaction window open, got ok=%v rw=%+v", ok, rw)
	}

	if err := e.QueueReactionCard(gameplay.PlayerB, 0, -1, EffectTarget{}); err != nil {
		t.Fatalf("queue opponent Retribution response failed: %v", err)
	}
	if err := e.ResolveReactionStack(); err != nil {
		t.Fatalf("resolve reaction stack failed: %v", err)
	}
	if state.Players[gameplay.PlayerA].Ignition.Occupied {
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
	if !state.Players[gameplay.PlayerA].Ignition.Occupied {
		t.Fatal("energy-gain should stay in ignition until burn completes")
	}
	if state.Players[gameplay.PlayerA].Ignition.TurnsRemaining != 1 {
		t.Fatalf("expected 1 turn remaining on slot, got %d", state.Players[gameplay.PlayerA].Ignition.TurnsRemaining)
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
	board.SetPiece(chess.Pos{Row: 6, Col: 4}, chess.Piece{Type: chess.Pawn, Color: chess.White})
	e := NewEngine(state, board)
	markInPlayForTest(state)

	if err := e.ActivateCard(gameplay.PlayerA, 0); err != nil {
		t.Fatalf("first ignition-0 activate failed: %v", err)
	}
	if err := e.SubmitIgnitionTargets(gameplay.PlayerA, []chess.Pos{{Row: 6, Col: 4}}); err != nil {
		t.Fatalf("submit ignition targets: %v", err)
	}
	if !state.Players[gameplay.PlayerA].Ignition.Occupied {
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
	doubleTurn := gameplay.CardInstance{InstanceID: "dt1", CardID: CardDoubleTurn, ManaCost: 6, Ignition: 2, Cooldown: 9}
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
	e.OpenReactionWindow("ignite_reaction", gameplay.PlayerA, []gameplay.CardType{gameplay.CardTypeRetribution})
	if err := e.QueueReactionCard(gameplay.PlayerA, 0, -1, EffectTarget{}); err == nil {
		t.Fatal("expected actor cannot open ignite_reaction chain")
	}
}

func TestActivateCardWithTargetsLocksKnightTouchTargetForSnapshot(t *testing.T) {
	knight := gameplay.CardInstance{InstanceID: "k1", CardID: CardKnightTouch, ManaCost: 3, Ignition: 0, Cooldown: 2}
	state, _ := gameplay.NewMatchState(testDeckWith(knight), testDeckWith(knight))
	state.Players[gameplay.PlayerA].Hand = []gameplay.CardInstance{knight}
	state.Players[gameplay.PlayerA].Mana = 10
	e := NewEngine(state, chess.NewGame())
	markInPlayForTest(state)
	target := chess.Pos{Row: 6, Col: 4}
	if err := e.ActivateCardWithTargets(gameplay.PlayerA, 0, []chess.Pos{target}); err != nil {
		t.Fatalf("activate with target failed: %v", err)
	}
	owner, cardID, pieces, ok := e.IgnitionTargetSnapshot()
	if !ok {
		t.Fatal("expected ignition target snapshot to be present")
	}
	if owner != gameplay.PlayerA || cardID != CardKnightTouch {
		t.Fatalf("unexpected snapshot metadata: owner=%s card=%s", owner, cardID)
	}
	if len(pieces) != 1 || pieces[0] != target {
		t.Fatalf("unexpected target pieces snapshot: %+v", pieces)
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
// ignite_card (hand→ignition) while a reaction window is open still enqueue the stack (responder is off-turn).
func TestActivateCardDuringIgniteReactionQueuesLikeQueueReaction(t *testing.T) {
	stopThere := gameplay.CardInstance{InstanceID: "s1", CardID: CardStopRightThere, ManaCost: 3, Ignition: 0, Cooldown: 5}
	state, _ := gameplay.NewMatchState(testDeckWith(stopThere), testDeckWith(stopThere))
	state.CurrentTurn = gameplay.PlayerA
	state.Players[gameplay.PlayerB].Hand = []gameplay.CardInstance{stopThere}
	state.Players[gameplay.PlayerB].Mana = 10
	e := NewEngine(state, chess.NewEmptyGame(chess.White))
	markInPlayForTest(state)
	e.OpenReactionWindow("ignite_reaction", gameplay.PlayerA, []gameplay.CardType{gameplay.CardTypeRetribution})
	if err := e.ActivateCard(gameplay.PlayerB, 0); err != nil {
		t.Fatalf("ignite during ignite_reaction should queue retribution: %v", err)
	}
	_, sz, ok := e.ReactionWindowSnapshot()
	if !ok || sz != 1 {
		t.Fatalf("expected one queued reaction, ok=%v sz=%d", ok, sz)
	}
}

func TestActivateCardRejectsNotEnoughManaWithCardName(t *testing.T) {
	kt := gameplay.CardInstance{InstanceID: "kt1", CardID: CardKnightTouch, ManaCost: 3, Ignition: 0, Cooldown: 2}
	state, _ := gameplay.NewMatchState(testDeckWith(kt), testDeckWith(kt))
	state.CurrentTurn = gameplay.PlayerA
	state.Players[gameplay.PlayerA].Hand = []gameplay.CardInstance{kt}
	state.Players[gameplay.PlayerA].Mana = 2
	e := NewEngine(state, chess.NewEmptyGame(chess.White))
	markInPlayForTest(state)

	err := e.ActivateCard(gameplay.PlayerA, 0)
	if err == nil || !strings.Contains(err.Error(), "not enough mana to activate Knight Touch: need 3, you have 2") {
		t.Fatalf("expected detailed mana rejection, got %v", err)
	}
}

func TestActivateCardRejectsCardOnCooldownWithCardName(t *testing.T) {
	eg := gameplay.CardInstance{InstanceID: "eg1", CardID: CardEnergyGain, ManaCost: 0, Ignition: 1, Cooldown: 2}
	state, _ := gameplay.NewMatchState(testDeckWith(eg), testDeckWith(eg))
	state.CurrentTurn = gameplay.PlayerA
	state.Players[gameplay.PlayerA].Hand = []gameplay.CardInstance{eg}
	state.Players[gameplay.PlayerA].Cooldowns = []gameplay.CooldownEntry{{Card: eg, TurnsRemaining: 1}}
	e := NewEngine(state, chess.NewEmptyGame(chess.White))
	markInPlayForTest(state)

	err := e.ActivateCard(gameplay.PlayerA, 0)
	if err == nil || !strings.Contains(err.Error(), "Energy Gain is still on cooldown") {
		t.Fatalf("expected cooldown rejection, got %v", err)
	}
}

func TestQueueReactionRejectsCardOnCooldownWithCardName(t *testing.T) {
	eg := gameplay.CardInstance{InstanceID: "eg1", CardID: CardEnergyGain, ManaCost: 0, Ignition: 1, Cooldown: 2}
	mb := gameplay.CardInstance{InstanceID: "mb1", CardID: CardManaBurn, ManaCost: 1, Ignition: 0, Cooldown: 3}
	state, _ := gameplay.NewMatchState(testDeckWith(eg), testDeckWith(mb))
	state.CurrentTurn = gameplay.PlayerA
	state.Players[gameplay.PlayerA].Hand = []gameplay.CardInstance{eg}
	state.Players[gameplay.PlayerA].Mana = 10
	state.Players[gameplay.PlayerB].Hand = []gameplay.CardInstance{mb}
	state.Players[gameplay.PlayerB].Mana = 10
	state.Players[gameplay.PlayerB].Cooldowns = []gameplay.CooldownEntry{{Card: mb, TurnsRemaining: 1}}
	e := NewEngine(state, chess.NewEmptyGame(chess.White))
	markInPlayForTest(state)

	if err := e.ActivateCard(gameplay.PlayerA, 0); err != nil {
		t.Fatalf("activate energy gain: %v", err)
	}
	err := e.QueueReactionCard(gameplay.PlayerB, 0, -1, EffectTarget{})
	if err == nil || !strings.Contains(err.Error(), "Mana Burn is still on cooldown") {
		t.Fatalf("expected reaction cooldown rejection, got %v", err)
	}
}

func TestCanPlayerExtendIgniteChain(t *testing.T) {
	manaBurn := gameplay.CardInstance{InstanceID: "mb1", CardID: "mana-burn", ManaCost: 1, Ignition: 0, Cooldown: 3}
	stopThere := gameplay.CardInstance{InstanceID: "s1", CardID: CardStopRightThere, ManaCost: 3, Ignition: 0, Cooldown: 5}
	retaliate := gameplay.CardInstance{InstanceID: "r2", CardID: "retaliate", ManaCost: 2, Ignition: 0, Cooldown: 9}
	state, _ := gameplay.NewMatchState(testDeckWith(manaBurn), testDeckWith(stopThere))
	state.CurrentTurn = gameplay.PlayerA
	state.Players[gameplay.PlayerA].Ignition = gameplay.IgnitionSlot{
		Occupied:        true,
		Card:            gameplay.CardInstance{InstanceID: "slot", CardID: "rook-touch", ManaCost: 3, Ignition: 0, Cooldown: 2},
		TurnsRemaining:  1,
		ActivationOwner: gameplay.PlayerA,
	}
	state.Players[gameplay.PlayerB].Hand = []gameplay.CardInstance{manaBurn}
	state.Players[gameplay.PlayerB].Mana = 10
	e := NewEngine(state, chess.NewEmptyGame(chess.White))
	markInPlayForTest(state)
	e.OpenReactionWindow("ignite_reaction", gameplay.PlayerA, []gameplay.CardType{gameplay.CardTypeRetribution})
	if err := e.QueueReactionCard(gameplay.PlayerB, 0, -1, EffectTarget{}); err != nil {
		t.Fatalf("queue opening retribution: %v", err)
	}
	state.Players[gameplay.PlayerA].Hand = []gameplay.CardInstance{retaliate}
	state.Players[gameplay.PlayerA].Mana = 10
	if e.CanPlayerExtendIgniteChain(gameplay.PlayerA) {
		t.Fatal("expected A cannot extend while ignition occupied without a clearing Retribution")
	}
	state.Players[gameplay.PlayerA].Hand = []gameplay.CardInstance{stopThere}
	if !e.CanPlayerExtendIgniteChain(gameplay.PlayerA) {
		t.Fatal("expected A can extend with Stop Right There (clears opponent ignition)")
	}
}

func TestCanPlayerExtendCounterChainIgnitionGate(t *testing.T) {
	ct := gameplay.CardInstance{InstanceID: "c1", CardID: CardCounterattack, ManaCost: 1, Ignition: 0, Cooldown: 6}
	bd := gameplay.CardInstance{InstanceID: "b1", CardID: CardBlockade, ManaCost: 0, Ignition: 0, Cooldown: 3}
	state, _ := gameplay.NewMatchState(testDeckWith(ct), testDeckWith(bd))
	e := NewEngine(state, chess.NewEmptyGame(chess.White))
	markInPlayForTest(state)
	e.OpenReactionWindow("capture_attempt", gameplay.PlayerA, []gameplay.CardType{gameplay.CardTypeCounter})
	resolverCT := e.resolvers[CardCounterattack]
	e.reactions.Push(ReactionAction{Owner: gameplay.PlayerB, Card: ct, Resolver: resolverCT})

	state.Players[gameplay.PlayerA].Ignition = gameplay.IgnitionSlot{
		Occupied:        true,
		Card:            gameplay.CardInstance{InstanceID: "x", CardID: "energy-gain", ManaCost: 0, Ignition: 1, Cooldown: 2},
		TurnsRemaining:  1,
		ActivationOwner: gameplay.PlayerA,
	}
	state.Players[gameplay.PlayerA].Hand = []gameplay.CardInstance{ct}
	state.Players[gameplay.PlayerA].Mana = 10
	if e.CanPlayerExtendCounterChain(gameplay.PlayerA) {
		t.Fatal("expected no extend with only Counterattack while ignition occupied")
	}
	state.Players[gameplay.PlayerA].Hand = []gameplay.CardInstance{bd}
	if !e.CanPlayerExtendCounterChain(gameplay.PlayerA) {
		t.Fatal("expected extend with Blockade while ignition occupied")
	}

	state.Players[gameplay.PlayerA].Ignition = gameplay.IgnitionSlot{}
	state.Players[gameplay.PlayerA].Hand = []gameplay.CardInstance{ct}
	if !e.CanPlayerExtendCounterChain(gameplay.PlayerA) {
		t.Fatal("expected extend with Counterattack when ignition is free")
	}
}

func TestReactionStackEntriesBottomFirstOrder(t *testing.T) {
	manaBurn := gameplay.CardInstance{InstanceID: "mb1", CardID: "mana-burn", ManaCost: 1, Ignition: 0, Cooldown: 3}
	ret := gameplay.CardInstance{InstanceID: "r1", CardID: CardStopRightThere, ManaCost: 3, Ignition: 0, Cooldown: 5}
	state, _ := gameplay.NewMatchState(testDeckWith(manaBurn), testDeckWith(ret))
	state.CurrentTurn = gameplay.PlayerA
	state.Players[gameplay.PlayerB].Hand = []gameplay.CardInstance{manaBurn}
	state.Players[gameplay.PlayerB].Mana = 10
	state.Players[gameplay.PlayerA].Hand = []gameplay.CardInstance{ret}
	state.Players[gameplay.PlayerA].Mana = 10
	e := NewEngine(state, chess.NewEmptyGame(chess.White))
	markInPlayForTest(state)
	e.OpenReactionWindow("ignite_reaction", gameplay.PlayerA, []gameplay.CardType{gameplay.CardTypeRetribution})
	if err := e.QueueReactionCard(gameplay.PlayerB, 0, -1, EffectTarget{}); err != nil {
		t.Fatalf("queue B: %v", err)
	}
	if err := e.QueueReactionCard(gameplay.PlayerA, 0, -1, EffectTarget{}); err != nil {
		t.Fatalf("queue A: %v", err)
	}
	ents := e.ReactionStackEntries()
	if len(ents) != 2 {
		t.Fatalf("expected 2 stack entries, got %d", len(ents))
	}
	if ents[0].Owner != gameplay.PlayerB || ents[0].CardID != "mana-burn" {
		t.Fatalf("expected bottom Mana Burn from B, got %+v", ents[0])
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
	if err := e.SubmitMove(gameplay.PlayerA, chess.Move{From: chess.Pos{Row: 6, Col: 4}, To: chess.Pos{Row: 5, Col: 5}}); err != nil {
		t.Fatalf("submit capture move should open reaction window: %v", err)
	}
	if e.ReactionWindow == nil || !e.ReactionWindow.Open {
		t.Fatalf("capture trigger window should be opened")
	}
	if len(e.ReactionWindow.EligibleTypes) != 1 || e.ReactionWindow.EligibleTypes[0] != gameplay.CardTypeCounter {
		t.Fatalf("capture_attempt should allow Counter only, got %v", e.ReactionWindow.EligibleTypes)
	}
	if e.pendingMove == nil {
		t.Fatalf("pending move should be stored until reaction resolution")
	}
	// Move is not applied yet.
	if board.PieceAt(chess.Pos{Row: 6, Col: 4}).Type != chess.Pawn {
		t.Fatalf("piece should remain on source while capture chain is pending")
	}
	if err := e.ResolveReactionStack(); err != nil {
		t.Fatalf("resolving empty capture chain should apply pending move: %v", err)
	}
	if board.PieceAt(chess.Pos{Row: 5, Col: 5}).Color != chess.White {
		t.Fatalf("pending capture move should be applied after chain resolves")
	}
}

// TestCaptureAttemptRejectedWhenPinnedCaptureWouldExposeKing ensures capture_attempt does not open
// when the capture is pseudo-visible but illegal (absolute pin); the player can submit another move.
func TestCaptureAttemptRejectedWhenPinnedCaptureWouldExposeKing(t *testing.T) {
	state, _ := gameplay.NewMatchState(testDeckWith(gameplay.CardInstance{InstanceID: "f", CardID: "double-turn", ManaCost: 1, Ignition: 1, Cooldown: 1}), testDeckWith(gameplay.CardInstance{InstanceID: "f2", CardID: "double-turn", ManaCost: 1, Ignition: 1, Cooldown: 1}))
	board := chess.NewEmptyGame(chess.White)
	// a-file pin: Ka1, Ra2 vs Qa8. Rook captures horizontally on rank-2 (e.g. h2) leaving the a-file open.
	board.SetPiece(chess.Pos{Row: 7, Col: 0}, chess.Piece{Type: chess.King, Color: chess.White})
	board.SetPiece(chess.Pos{Row: 6, Col: 0}, chess.Piece{Type: chess.Rook, Color: chess.White})
	board.SetPiece(chess.Pos{Row: 0, Col: 0}, chess.Piece{Type: chess.Queen, Color: chess.Black})
	board.SetPiece(chess.Pos{Row: 0, Col: 4}, chess.Piece{Type: chess.King, Color: chess.Black})
	board.SetPiece(chess.Pos{Row: 6, Col: 7}, chess.Piece{Type: chess.Knight, Color: chess.Black})

	e := NewEngine(state, board)
	markInPlayForTest(state)
	illegalCapture := chess.Move{From: chess.Pos{Row: 6, Col: 0}, To: chess.Pos{Row: 6, Col: 7}}
	if err := e.SubmitMove(gameplay.PlayerA, illegalCapture); err == nil {
		t.Fatal("expected illegal pinned capture to be rejected before capture_attempt")
	}
	if e.ReactionWindow != nil && e.ReactionWindow.Open && e.ReactionWindow.Trigger == "capture_attempt" {
		t.Fatal("capture_attempt window must not open for an illegal capture")
	}
	if e.pendingMove != nil {
		t.Fatal("pending move must not be set when capture is rejected")
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
	if err := e.SubmitMove(gameplay.PlayerB, chess.Move{From: chess.Pos{Row: 1, Col: 5}, To: chess.Pos{Row: 3, Col: 5}}); err != nil {
		t.Fatalf("black setup move failed: %v", err)
	}
	state.CurrentTurn = gameplay.PlayerA
	board.Turn = chess.White

	// White en passant should open capture window.
	if err := e.SubmitMove(gameplay.PlayerA, chess.Move{From: chess.Pos{Row: 3, Col: 4}, To: chess.Pos{Row: 2, Col: 5}}); err != nil {
		t.Fatalf("en passant submit should open reaction window: %v", err)
	}
	if e.ReactionWindow == nil || e.ReactionWindow.Trigger != "capture_attempt" {
		t.Fatalf("expected capture_attempt window for en passant")
	}
}

func TestCaptureAttemptRejectsRetributionOpening(t *testing.T) {
	stopThere := gameplay.CardInstance{InstanceID: "s1", CardID: CardStopRightThere, ManaCost: 3, Ignition: 0, Cooldown: 5}
	state, _ := gameplay.NewMatchState(testDeckWith(stopThere), testDeckWith(stopThere))
	state.Players[gameplay.PlayerB].Hand = []gameplay.CardInstance{stopThere}
	state.Players[gameplay.PlayerB].Mana = 10
	board := chess.NewEmptyGame(chess.White)
	board.SetPiece(chess.Pos{Row: 7, Col: 4}, chess.Piece{Type: chess.King, Color: chess.White})
	board.SetPiece(chess.Pos{Row: 0, Col: 4}, chess.Piece{Type: chess.King, Color: chess.Black})
	board.SetPiece(chess.Pos{Row: 6, Col: 4}, chess.Piece{Type: chess.Pawn, Color: chess.White})
	board.SetPiece(chess.Pos{Row: 5, Col: 5}, chess.Piece{Type: chess.Pawn, Color: chess.Black})
	e := NewEngine(state, board)
	markInPlayForTest(state)
	if err := e.SubmitMove(gameplay.PlayerA, chess.Move{From: chess.Pos{Row: 6, Col: 4}, To: chess.Pos{Row: 5, Col: 5}}); err != nil {
		t.Fatalf("capture attempt: %v", err)
	}
	if err := e.QueueReactionCard(gameplay.PlayerB, 0, -1, EffectTarget{}); err == nil {
		t.Fatal("expected Retribution rejected as first response on capture_attempt")
	}
}

func TestIgniteReactionEligibleRetributionOnlyUntilMaybeCaptureAttempt(t *testing.T) {
	knight := gameplay.CardInstance{InstanceID: "k1", CardID: CardKnightTouch, ManaCost: 3, Ignition: 0, Cooldown: 2}
	stopThere := gameplay.CardInstance{InstanceID: "s1", CardID: CardStopRightThere, ManaCost: 3, Ignition: 0, Cooldown: 5}
	state, _ := gameplay.NewMatchState(testDeckWith(knight), testDeckWith(stopThere))
	state.CurrentTurn = gameplay.PlayerA
	state.Players[gameplay.PlayerA].Hand = []gameplay.CardInstance{knight}
	state.Players[gameplay.PlayerB].Hand = []gameplay.CardInstance{stopThere}
	state.Players[gameplay.PlayerA].Mana = 10
	state.Players[gameplay.PlayerB].Mana = 10
	board := chess.NewEmptyGame(chess.White)
	board.SetPiece(chess.Pos{Row: 7, Col: 4}, chess.Piece{Type: chess.King, Color: chess.White})
	board.SetPiece(chess.Pos{Row: 0, Col: 4}, chess.Piece{Type: chess.King, Color: chess.Black})
	board.SetPiece(chess.Pos{Row: 6, Col: 4}, chess.Piece{Type: chess.Pawn, Color: chess.White})
	e := NewEngine(state, board)
	markInPlayForTest(state)
	if err := e.ActivateCard(gameplay.PlayerA, 0); err != nil {
		t.Fatalf("activate: %v", err)
	}
	if err := e.SubmitIgnitionTargets(gameplay.PlayerA, []chess.Pos{{Row: 6, Col: 4}}); err != nil {
		t.Fatalf("submit ignition targets: %v", err)
	}
	rw, _, ok := e.ReactionWindowSnapshot()
	if !ok || rw.Trigger != "ignite_reaction" {
		t.Fatalf("expected ignite window")
	}
	var hasRet, hasCt bool
	for _, tpe := range rw.EligibleTypes {
		if tpe == gameplay.CardTypeRetribution {
			hasRet = true
		}
		if tpe == gameplay.CardTypeCounter {
			hasCt = true
		}
	}
	if !hasRet || hasCt {
		t.Fatalf("ignite_reaction should list Retribution only until catalog sets MaybeCaptureAttemptOnIgnition, got %v", rw.EligibleTypes)
	}
	if err := e.QueueReactionCard(gameplay.PlayerB, 0, -1, EffectTarget{}); err != nil {
		t.Fatalf("queue Retribution opening on ignite: %v", err)
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

	if err := e.SubmitMove(gameplay.PlayerA, chess.Move{From: chess.Pos{Row: 6, Col: 4}, To: chess.Pos{Row: 5, Col: 5}}); err != nil {
		t.Fatalf("capture attempt: %v", err)
	}
	if err := e.QueueReactionCard(gameplay.PlayerB, 0, -1, EffectTarget{}); err != nil {
		t.Fatalf("counter should queue on any capture attempt: %v", err)
	}
	if err := e.ResolveReactionStack(); err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if board.PieceAt(chess.Pos{Row: 5, Col: 5}).Type != chess.Pawn || board.PieceAt(chess.Pos{Row: 5, Col: 5}).Color != chess.White {
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

	if err := e.SubmitMove(gameplay.PlayerA, chess.Move{From: chess.Pos{Row: 6, Col: 4}, To: chess.Pos{Row: 5, Col: 5}}); err != nil {
		t.Fatalf("capture attempt: %v", err)
	}
	if err := e.QueueReactionCard(gameplay.PlayerB, 0, -1, EffectTarget{}); err != nil {
		t.Fatalf("defender counter: %v", err)
	}
	if err := e.QueueReactionCard(gameplay.PlayerA, 0, -1, EffectTarget{}); err != nil {
		t.Fatalf("attacker second counter: %v", err)
	}
	if err := e.ResolveReactionStack(); err != nil {
		t.Fatalf("resolve stack: %v", err)
	}
	if board.PieceAt(chess.Pos{Row: 5, Col: 5}).Type != chess.Pawn || board.PieceAt(chess.Pos{Row: 5, Col: 5}).Color != chess.White {
		t.Fatal("pending capture should still apply after counter chain")
	}
}

// TestResolveReactionStackSequentialActivationFX ensures each resolved reaction card emits one
// activate_card (ActivationFXEvent) in LIFO stack order before the next card is processed.
func TestResolveReactionStackSequentialActivationFX(t *testing.T) {
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

	if err := e.SubmitMove(gameplay.PlayerA, chess.Move{From: chess.Pos{Row: 6, Col: 4}, To: chess.Pos{Row: 5, Col: 5}}); err != nil {
		t.Fatalf("capture attempt: %v", err)
	}
	if err := e.QueueReactionCard(gameplay.PlayerB, 0, -1, EffectTarget{}); err != nil {
		t.Fatalf("defender counter: %v", err)
	}
	if err := e.QueueReactionCard(gameplay.PlayerA, 0, -1, EffectTarget{}); err != nil {
		t.Fatalf("attacker second counter: %v", err)
	}
	if err := e.ResolveReactionStack(); err != nil {
		t.Fatalf("resolve stack: %v", err)
	}
	ev := e.PullActivationFXEvents()
	if len(ev) != 2 {
		t.Fatalf("expected 2 activation fx events, got %d %+v", len(ev), ev)
	}
	// LIFO: Blockade (A, queued second) resolves before Counterattack (B).
	if ev[0].Owner != gameplay.PlayerA || ev[0].CardID != CardBlockade || !ev[0].Success {
		t.Fatalf("expected first fx Blockade from A, got %+v", ev[0])
	}
	if ev[1].Owner != gameplay.PlayerB || ev[1].CardID != CardCounterattack || !ev[1].Success {
		t.Fatalf("expected second fx Counterattack from B, got %+v", ev[1])
	}
	if len(e.PullActivationFXEvents()) != 0 {
		t.Fatal("expected PullActivationFXEvents to drain")
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
	gy := state.Players[gameplay.PlayerA].Graveyard
	if len(gy) != 1 || gy[0].Color != "b" || gy[0].Type != "P" {
		t.Fatalf("capturer graveyard should record the captured black pawn, got %+v", gy)
	}
}

func TestProcessResolvedIgnitionsEmitsActivationFXEvents(t *testing.T) {
	k1 := gameplay.CardInstance{InstanceID: "k1", CardID: CardKnightTouch, ManaCost: 3, Ignition: 0, Cooldown: 2}
	state, err := gameplay.NewMatchState(testDeckWith(k1), testDeckWith(k1))
	if err != nil {
		t.Fatal(err)
	}
	state.Players[gameplay.PlayerA].Ignition = gameplay.IgnitionSlot{
		Occupied:        true,
		Card:            k1,
		TurnsRemaining:  0,
		ActivationOwner: gameplay.PlayerA,
	}
	e := NewEngine(state, chess.NewEmptyGame(chess.White))
	if err := state.ResolveIgnitionFor(gameplay.PlayerA, true); err != nil {
		t.Fatal(err)
	}
	if err := e.processResolvedIgnitions(); err != nil {
		t.Fatal(err)
	}
	ev := e.PullActivationFXEvents()
	if len(ev) != 1 || ev[0].Owner != gameplay.PlayerA || ev[0].CardID != CardKnightTouch || !ev[0].Success {
		t.Fatalf("unexpected activation fx: %+v", ev)
	}
	if len(e.PullActivationFXEvents()) != 0 {
		t.Fatal("expected PullActivationFXEvents to drain")
	}
}

func TestKnightTouchGrantsKnightPatternForOneOwnerTurn(t *testing.T) {
	kt := gameplay.CardInstance{InstanceID: "kt1", CardID: CardKnightTouch, ManaCost: 3, Ignition: 0, Cooldown: 2}
	filler := gameplay.CardInstance{InstanceID: "f1", CardID: CardDoubleTurn, ManaCost: 1, Ignition: 1, Cooldown: 1}
	state, err := gameplay.NewMatchState(testDeckWith(kt), testDeckWith(filler))
	if err != nil {
		t.Fatal(err)
	}
	state.CurrentTurn = gameplay.PlayerA
	state.Players[gameplay.PlayerA].Hand = []gameplay.CardInstance{kt}
	state.Players[gameplay.PlayerA].Mana = 10
	state.Players[gameplay.PlayerB].Hand = []gameplay.CardInstance{}
	board := chess.NewEmptyGame(chess.White)
	board.SetPiece(chess.Pos{Row: 7, Col: 4}, chess.Piece{Type: chess.King, Color: chess.White})
	board.SetPiece(chess.Pos{Row: 0, Col: 4}, chess.Piece{Type: chess.King, Color: chess.Black})
	board.SetPiece(chess.Pos{Row: 4, Col: 4}, chess.Piece{Type: chess.Pawn, Color: chess.White}) // e4
	board.SetPiece(chess.Pos{Row: 1, Col: 0}, chess.Piece{Type: chess.Pawn, Color: chess.Black}) // a7
	e := NewEngine(state, board)
	markInPlayForTest(state)

	if err := e.ActivateCardWithTargets(gameplay.PlayerA, 0, []chess.Pos{{Row: 4, Col: 4}}); err != nil {
		t.Fatalf("activate knight-touch with target failed: %v", err)
	}
	if err := e.ResolveReactionStack(); err != nil {
		t.Fatalf("resolve ignite chain failed: %v", err)
	}
	if err := e.SubmitMove(gameplay.PlayerA, chess.Move{From: chess.Pos{Row: 4, Col: 4}, To: chess.Pos{Row: 2, Col: 5}}); err != nil {
		t.Fatalf("target pawn should gain knight movement on owner turn: %v", err)
	}
	if board.PieceAt(chess.Pos{Row: 2, Col: 5}).Type != chess.Pawn || board.PieceAt(chess.Pos{Row: 2, Col: 5}).Color != chess.White {
		t.Fatalf("expected white pawn moved to f6 with knight pattern")
	}
	// B plays a7->a6 to hand turn back to A.
	if err := e.SubmitMove(gameplay.PlayerB, chess.Move{From: chess.Pos{Row: 1, Col: 0}, To: chess.Pos{Row: 2, Col: 0}}); err != nil {
		t.Fatalf("black reply move failed: %v", err)
	}
	// Grant must expire after A's previous turn, so knight-like jump is now illegal.
	if err := e.SubmitMove(gameplay.PlayerA, chess.Move{From: chess.Pos{Row: 2, Col: 5}, To: chess.Pos{Row: 0, Col: 6}}); err == nil {
		t.Fatal("knight-touch movement grant should expire on next owner turn")
	}
}

func TestKnightTouchKeepsNativePieceMovement(t *testing.T) {
	kt := gameplay.CardInstance{InstanceID: "kt1", CardID: CardKnightTouch, ManaCost: 3, Ignition: 0, Cooldown: 2}
	state, err := gameplay.NewMatchState(testDeckWith(kt), testDeckWith(kt))
	if err != nil {
		t.Fatal(err)
	}
	state.CurrentTurn = gameplay.PlayerA
	state.Players[gameplay.PlayerA].Hand = []gameplay.CardInstance{kt}
	state.Players[gameplay.PlayerA].Mana = 10
	board := chess.NewEmptyGame(chess.White)
	board.SetPiece(chess.Pos{Row: 7, Col: 4}, chess.Piece{Type: chess.King, Color: chess.White})
	board.SetPiece(chess.Pos{Row: 0, Col: 4}, chess.Piece{Type: chess.King, Color: chess.Black})
	board.SetPiece(chess.Pos{Row: 4, Col: 4}, chess.Piece{Type: chess.Pawn, Color: chess.White}) // e4
	e := NewEngine(state, board)
	markInPlayForTest(state)

	if err := e.ActivateCardWithTargets(gameplay.PlayerA, 0, []chess.Pos{{Row: 4, Col: 4}}); err != nil {
		t.Fatalf("activate knight-touch with target failed: %v", err)
	}
	if err := e.ResolveReactionStack(); err != nil {
		t.Fatalf("resolve ignite chain failed: %v", err)
	}
	// Native pawn movement must remain available (accumulated, not replaced).
	if err := e.SubmitMove(gameplay.PlayerA, chess.Move{From: chess.Pos{Row: 4, Col: 4}, To: chess.Pos{Row: 3, Col: 4}}); err != nil {
		t.Fatalf("native pawn move should remain legal under knight-touch: %v", err)
	}
}

func TestBishopTouchGrantsBishopSlideForRook(t *testing.T) {
	bt := gameplay.CardInstance{InstanceID: "bt1", CardID: CardBishopTouch, ManaCost: 3, Ignition: 0, Cooldown: 2}
	filler := gameplay.CardInstance{InstanceID: "f1", CardID: CardDoubleTurn, ManaCost: 1, Ignition: 1, Cooldown: 1}
	state, err := gameplay.NewMatchState(testDeckWith(bt), testDeckWith(filler))
	if err != nil {
		t.Fatal(err)
	}
	state.CurrentTurn = gameplay.PlayerA
	state.Players[gameplay.PlayerA].Hand = []gameplay.CardInstance{bt}
	state.Players[gameplay.PlayerA].Mana = 10
	state.Players[gameplay.PlayerB].Hand = []gameplay.CardInstance{}
	board := chess.NewEmptyGame(chess.White)
	board.SetPiece(chess.Pos{Row: 7, Col: 4}, chess.Piece{Type: chess.King, Color: chess.White})
	board.SetPiece(chess.Pos{Row: 0, Col: 4}, chess.Piece{Type: chess.King, Color: chess.Black})
	// Rook on b2 (rank 2 file b).
	board.SetPiece(chess.Pos{Row: 6, Col: 1}, chess.Piece{Type: chess.Rook, Color: chess.White})
	board.SetPiece(chess.Pos{Row: 1, Col: 0}, chess.Piece{Type: chess.Pawn, Color: chess.Black})
	e := NewEngine(state, board)
	markInPlayForTest(state)

	if err := e.ActivateCardWithTargets(gameplay.PlayerA, 0, []chess.Pos{{Row: 6, Col: 1}}); err != nil {
		t.Fatalf("activate bishop-touch with target failed: %v", err)
	}
	if err := e.ResolveReactionStack(); err != nil {
		t.Fatalf("resolve ignite chain failed: %v", err)
	}
	// Diagonal b2 -> d4 with empty c3.
	if err := e.SubmitMove(gameplay.PlayerA, chess.Move{From: chess.Pos{Row: 6, Col: 1}, To: chess.Pos{Row: 4, Col: 3}}); err != nil {
		t.Fatalf("rook should move as bishop along a clear diagonal: %v", err)
	}
	if board.PieceAt(chess.Pos{Row: 4, Col: 3}).Type != chess.Rook {
		t.Fatal("expected rook on d4")
	}
}

func TestBishopTouchPawnLimitedToOneDiagonalStep(t *testing.T) {
	bt := gameplay.CardInstance{InstanceID: "bt1", CardID: CardBishopTouch, ManaCost: 3, Ignition: 0, Cooldown: 2}
	filler := gameplay.CardInstance{InstanceID: "f1", CardID: CardDoubleTurn, ManaCost: 1, Ignition: 1, Cooldown: 1}
	state, err := gameplay.NewMatchState(testDeckWith(bt), testDeckWith(filler))
	if err != nil {
		t.Fatal(err)
	}
	state.CurrentTurn = gameplay.PlayerA
	state.Players[gameplay.PlayerA].Hand = []gameplay.CardInstance{bt}
	state.Players[gameplay.PlayerA].Mana = 10
	state.Players[gameplay.PlayerB].Hand = []gameplay.CardInstance{}
	board := chess.NewEmptyGame(chess.White)
	board.SetPiece(chess.Pos{Row: 7, Col: 4}, chess.Piece{Type: chess.King, Color: chess.White})
	board.SetPiece(chess.Pos{Row: 0, Col: 4}, chess.Piece{Type: chess.King, Color: chess.Black})
	board.SetPiece(chess.Pos{Row: 4, Col: 4}, chess.Piece{Type: chess.Pawn, Color: chess.White})
	board.SetPiece(chess.Pos{Row: 1, Col: 0}, chess.Piece{Type: chess.Pawn, Color: chess.Black})
	e := NewEngine(state, board)
	markInPlayForTest(state)

	if err := e.ActivateCardWithTargets(gameplay.PlayerA, 0, []chess.Pos{{Row: 4, Col: 4}}); err != nil {
		t.Fatalf("activate bishop-touch with target failed: %v", err)
	}
	if err := e.ResolveReactionStack(); err != nil {
		t.Fatalf("resolve ignite chain failed: %v", err)
	}
	// Two-step diagonal (e4 -> g6) must be rejected for a pawn.
	if err := e.SubmitMove(gameplay.PlayerA, chess.Move{From: chess.Pos{Row: 4, Col: 4}, To: chess.Pos{Row: 2, Col: 6}}); err == nil {
		t.Fatal("bishop-touch pawn should not slide two diagonal squares")
	}
	if err := e.SubmitMove(gameplay.PlayerA, chess.Move{From: chess.Pos{Row: 4, Col: 4}, To: chess.Pos{Row: 3, Col: 5}}); err != nil {
		t.Fatalf("bishop-touch pawn should allow one diagonal step: %v", err)
	}
}

func TestBishopTouchMovementGrantExpiresAfterOwnerTurn(t *testing.T) {
	bt := gameplay.CardInstance{InstanceID: "bt1", CardID: CardBishopTouch, ManaCost: 3, Ignition: 0, Cooldown: 2}
	filler := gameplay.CardInstance{InstanceID: "f1", CardID: CardDoubleTurn, ManaCost: 1, Ignition: 1, Cooldown: 1}
	state, err := gameplay.NewMatchState(testDeckWith(bt), testDeckWith(filler))
	if err != nil {
		t.Fatal(err)
	}
	state.CurrentTurn = gameplay.PlayerA
	state.Players[gameplay.PlayerA].Hand = []gameplay.CardInstance{bt}
	state.Players[gameplay.PlayerA].Mana = 10
	state.Players[gameplay.PlayerB].Hand = []gameplay.CardInstance{}
	board := chess.NewEmptyGame(chess.White)
	board.SetPiece(chess.Pos{Row: 7, Col: 4}, chess.Piece{Type: chess.King, Color: chess.White})
	board.SetPiece(chess.Pos{Row: 0, Col: 4}, chess.Piece{Type: chess.King, Color: chess.Black})
	board.SetPiece(chess.Pos{Row: 6, Col: 1}, chess.Piece{Type: chess.Rook, Color: chess.White})
	board.SetPiece(chess.Pos{Row: 1, Col: 0}, chess.Piece{Type: chess.Pawn, Color: chess.Black})
	e := NewEngine(state, board)
	markInPlayForTest(state)

	if err := e.ActivateCardWithTargets(gameplay.PlayerA, 0, []chess.Pos{{Row: 6, Col: 1}}); err != nil {
		t.Fatalf("activate bishop-touch with target failed: %v", err)
	}
	if err := e.ResolveReactionStack(); err != nil {
		t.Fatalf("resolve ignite chain failed: %v", err)
	}
	if err := e.SubmitMove(gameplay.PlayerA, chess.Move{From: chess.Pos{Row: 6, Col: 1}, To: chess.Pos{Row: 4, Col: 3}}); err != nil {
		t.Fatalf("first bishop-line move should succeed: %v", err)
	}
	if err := e.SubmitMove(gameplay.PlayerB, chess.Move{From: chess.Pos{Row: 1, Col: 0}, To: chess.Pos{Row: 2, Col: 0}}); err != nil {
		t.Fatalf("black reply failed: %v", err)
	}
	if err := e.SubmitMove(gameplay.PlayerA, chess.Move{From: chess.Pos{Row: 4, Col: 3}, To: chess.Pos{Row: 2, Col: 5}}); err == nil {
		t.Fatal("bishop-touch grant should expire after owner turn")
	}
}

func TestRookTouchGrantsRookSlideForKnight(t *testing.T) {
	rt := gameplay.CardInstance{InstanceID: "rt1", CardID: CardRookTouch, ManaCost: 3, Ignition: 0, Cooldown: 2}
	filler := gameplay.CardInstance{InstanceID: "f1", CardID: CardDoubleTurn, ManaCost: 1, Ignition: 1, Cooldown: 1}
	state, err := gameplay.NewMatchState(testDeckWith(rt), testDeckWith(filler))
	if err != nil {
		t.Fatal(err)
	}
	state.CurrentTurn = gameplay.PlayerA
	state.Players[gameplay.PlayerA].Hand = []gameplay.CardInstance{rt}
	state.Players[gameplay.PlayerA].Mana = 10
	state.Players[gameplay.PlayerB].Hand = []gameplay.CardInstance{}
	board := chess.NewEmptyGame(chess.White)
	board.SetPiece(chess.Pos{Row: 7, Col: 4}, chess.Piece{Type: chess.King, Color: chess.White})
	board.SetPiece(chess.Pos{Row: 0, Col: 4}, chess.Piece{Type: chess.King, Color: chess.Black})
	// Knight on b2 (rank 2 file b), clear b-file to b7.
	board.SetPiece(chess.Pos{Row: 6, Col: 1}, chess.Piece{Type: chess.Knight, Color: chess.White})
	board.SetPiece(chess.Pos{Row: 1, Col: 0}, chess.Piece{Type: chess.Pawn, Color: chess.Black})
	e := NewEngine(state, board)
	markInPlayForTest(state)

	if err := e.ActivateCardWithTargets(gameplay.PlayerA, 0, []chess.Pos{{Row: 6, Col: 1}}); err != nil {
		t.Fatalf("activate rook-touch with target failed: %v", err)
	}
	if err := e.ResolveReactionStack(); err != nil {
		t.Fatalf("resolve ignite chain failed: %v", err)
	}
	if err := e.SubmitMove(gameplay.PlayerA, chess.Move{From: chess.Pos{Row: 6, Col: 1}, To: chess.Pos{Row: 1, Col: 1}}); err != nil {
		t.Fatalf("knight should move as rook along a clear file: %v", err)
	}
	if board.PieceAt(chess.Pos{Row: 1, Col: 1}).Type != chess.Knight {
		t.Fatal("expected knight on b7")
	}
}

func TestRookTouchPawnLimitedToOneOrthogonalStep(t *testing.T) {
	rt := gameplay.CardInstance{InstanceID: "rt1", CardID: CardRookTouch, ManaCost: 3, Ignition: 0, Cooldown: 2}
	filler := gameplay.CardInstance{InstanceID: "f1", CardID: CardDoubleTurn, ManaCost: 1, Ignition: 1, Cooldown: 1}
	state, err := gameplay.NewMatchState(testDeckWith(rt), testDeckWith(filler))
	if err != nil {
		t.Fatal(err)
	}
	state.CurrentTurn = gameplay.PlayerA
	state.Players[gameplay.PlayerA].Hand = []gameplay.CardInstance{rt}
	state.Players[gameplay.PlayerA].Mana = 10
	state.Players[gameplay.PlayerB].Hand = []gameplay.CardInstance{}
	board := chess.NewEmptyGame(chess.White)
	board.SetPiece(chess.Pos{Row: 7, Col: 4}, chess.Piece{Type: chess.King, Color: chess.White})
	board.SetPiece(chess.Pos{Row: 0, Col: 4}, chess.Piece{Type: chess.King, Color: chess.Black})
	board.SetPiece(chess.Pos{Row: 4, Col: 4}, chess.Piece{Type: chess.Pawn, Color: chess.White})
	board.SetPiece(chess.Pos{Row: 1, Col: 0}, chess.Piece{Type: chess.Pawn, Color: chess.Black})
	e := NewEngine(state, board)
	markInPlayForTest(state)

	if err := e.ActivateCardWithTargets(gameplay.PlayerA, 0, []chess.Pos{{Row: 4, Col: 4}}); err != nil {
		t.Fatalf("activate rook-touch with target failed: %v", err)
	}
	if err := e.ResolveReactionStack(); err != nil {
		t.Fatalf("resolve ignite chain failed: %v", err)
	}
	// Two steps on the same rank or file must be rejected for a pawn.
	if err := e.SubmitMove(gameplay.PlayerA, chess.Move{From: chess.Pos{Row: 4, Col: 4}, To: chess.Pos{Row: 2, Col: 4}}); err == nil {
		t.Fatal("rook-touch pawn should not slide two squares on a file")
	}
	if err := e.SubmitMove(gameplay.PlayerA, chess.Move{From: chess.Pos{Row: 4, Col: 4}, To: chess.Pos{Row: 3, Col: 4}}); err != nil {
		t.Fatalf("rook-touch pawn should allow one orthogonal step: %v", err)
	}
}

func TestRookTouchMovementGrantExpiresAfterOwnerTurn(t *testing.T) {
	rt := gameplay.CardInstance{InstanceID: "rt1", CardID: CardRookTouch, ManaCost: 3, Ignition: 0, Cooldown: 2}
	filler := gameplay.CardInstance{InstanceID: "f1", CardID: CardDoubleTurn, ManaCost: 1, Ignition: 1, Cooldown: 1}
	state, err := gameplay.NewMatchState(testDeckWith(rt), testDeckWith(filler))
	if err != nil {
		t.Fatal(err)
	}
	state.CurrentTurn = gameplay.PlayerA
	state.Players[gameplay.PlayerA].Hand = []gameplay.CardInstance{rt}
	state.Players[gameplay.PlayerA].Mana = 10
	state.Players[gameplay.PlayerB].Hand = []gameplay.CardInstance{}
	board := chess.NewEmptyGame(chess.White)
	board.SetPiece(chess.Pos{Row: 7, Col: 4}, chess.Piece{Type: chess.King, Color: chess.White})
	board.SetPiece(chess.Pos{Row: 0, Col: 4}, chess.Piece{Type: chess.King, Color: chess.Black})
	board.SetPiece(chess.Pos{Row: 6, Col: 1}, chess.Piece{Type: chess.Knight, Color: chess.White})
	board.SetPiece(chess.Pos{Row: 1, Col: 0}, chess.Piece{Type: chess.Pawn, Color: chess.Black})
	e := NewEngine(state, board)
	markInPlayForTest(state)

	if err := e.ActivateCardWithTargets(gameplay.PlayerA, 0, []chess.Pos{{Row: 6, Col: 1}}); err != nil {
		t.Fatalf("activate rook-touch with target failed: %v", err)
	}
	if err := e.ResolveReactionStack(); err != nil {
		t.Fatalf("resolve ignite chain failed: %v", err)
	}
	if err := e.SubmitMove(gameplay.PlayerA, chess.Move{From: chess.Pos{Row: 6, Col: 1}, To: chess.Pos{Row: 1, Col: 1}}); err != nil {
		t.Fatalf("first rook-line move should succeed: %v", err)
	}
	if err := e.SubmitMove(gameplay.PlayerB, chess.Move{From: chess.Pos{Row: 1, Col: 0}, To: chess.Pos{Row: 2, Col: 0}}); err != nil {
		t.Fatalf("black reply failed: %v", err)
	}
	if err := e.SubmitMove(gameplay.PlayerA, chess.Move{From: chess.Pos{Row: 1, Col: 1}, To: chess.Pos{Row: 1, Col: 7}}); err == nil {
		t.Fatal("rook-touch grant should expire after owner turn")
	}
}

func TestEnergyGainResolverGrantsManaViaProcessResolvedIgnitions(t *testing.T) {
	eg := gameplay.CardInstance{InstanceID: "eg1", CardID: CardEnergyGain, ManaCost: 0, Ignition: 1, Cooldown: 2}
	state, err := gameplay.NewMatchState(testDeckWith(eg), testDeckWith(eg))
	if err != nil {
		t.Fatal(err)
	}
	state.Players[gameplay.PlayerA].Mana = 2
	state.Players[gameplay.PlayerA].MaxMana = 10
	state.Players[gameplay.PlayerA].Ignition = gameplay.IgnitionSlot{
		Occupied:        true,
		Card:            eg,
		TurnsRemaining:  0,
		ActivationOwner: gameplay.PlayerA,
	}
	e := NewEngine(state, chess.NewEmptyGame(chess.White))
	markInPlayForTest(state)

	if err := state.ResolveIgnitionFor(gameplay.PlayerA, true); err != nil {
		t.Fatalf("resolve ignition: %v", err)
	}
	if err := e.processResolvedIgnitions(); err != nil {
		t.Fatalf("process resolved ignitions: %v", err)
	}
	if state.Players[gameplay.PlayerA].Mana != 6 {
		t.Fatalf("expected mana 2+4=6, got %d", state.Players[gameplay.PlayerA].Mana)
	}
}

func TestEnergyGainDoesNotGrantManaOnFailedResolution(t *testing.T) {
	eg := gameplay.CardInstance{InstanceID: "eg1", CardID: CardEnergyGain, ManaCost: 0, Ignition: 1, Cooldown: 2}
	state, err := gameplay.NewMatchState(testDeckWith(eg), testDeckWith(eg))
	if err != nil {
		t.Fatal(err)
	}
	state.Players[gameplay.PlayerA].Mana = 3
	state.Players[gameplay.PlayerA].MaxMana = 10
	state.Players[gameplay.PlayerA].Ignition = gameplay.IgnitionSlot{
		Occupied:        true,
		Card:            eg,
		TurnsRemaining:  0,
		ActivationOwner: gameplay.PlayerA,
	}
	e := NewEngine(state, chess.NewEmptyGame(chess.White))
	markInPlayForTest(state)

	if err := state.ResolveIgnitionFor(gameplay.PlayerA, false); err != nil {
		t.Fatalf("resolve ignition: %v", err)
	}
	if err := e.processResolvedIgnitions(); err != nil {
		t.Fatalf("process resolved ignitions: %v", err)
	}
	if state.Players[gameplay.PlayerA].Mana != 3 {
		t.Fatalf("failed resolution should not add mana, got %d", state.Players[gameplay.PlayerA].Mana)
	}
}

func TestEnergyGainManaCappedAtMaxMana(t *testing.T) {
	eg := gameplay.CardInstance{InstanceID: "eg1", CardID: CardEnergyGain, ManaCost: 0, Ignition: 1, Cooldown: 2}
	state, err := gameplay.NewMatchState(testDeckWith(eg), testDeckWith(eg))
	if err != nil {
		t.Fatal(err)
	}
	state.Players[gameplay.PlayerA].Mana = 8
	state.Players[gameplay.PlayerA].MaxMana = 10
	state.Players[gameplay.PlayerA].Ignition = gameplay.IgnitionSlot{
		Occupied:        true,
		Card:            eg,
		TurnsRemaining:  0,
		ActivationOwner: gameplay.PlayerA,
	}
	e := NewEngine(state, chess.NewEmptyGame(chess.White))
	markInPlayForTest(state)

	if err := state.ResolveIgnitionFor(gameplay.PlayerA, true); err != nil {
		t.Fatalf("resolve ignition: %v", err)
	}
	if err := e.processResolvedIgnitions(); err != nil {
		t.Fatalf("process resolved ignitions: %v", err)
	}
	if state.Players[gameplay.PlayerA].Mana != 10 {
		t.Fatalf("expected mana capped at max 10, got %d", state.Players[gameplay.PlayerA].Mana)
	}
}

func TestRetaliateRejectsWhenOpponentCannotPayRegularManaForCooldownPower(t *testing.T) {
	eg := gameplay.CardInstance{InstanceID: "eg1", CardID: CardEnergyGain, ManaCost: 0, Ignition: 1, Cooldown: 2}
	ret := gameplay.CardInstance{InstanceID: "ret1", CardID: CardRetaliate, ManaCost: 2, Ignition: 0, Cooldown: 9}
	kt := gameplay.CardInstance{InstanceID: "kt-cd", CardID: CardKnightTouch, ManaCost: 3, Ignition: 0, Cooldown: 2}
	state, err := gameplay.NewMatchState(testDeckWith(eg), testDeckWith(ret))
	if err != nil {
		t.Fatal(err)
	}
	state.CurrentTurn = gameplay.PlayerA
	state.Players[gameplay.PlayerA].Hand = []gameplay.CardInstance{eg}
	state.Players[gameplay.PlayerA].Mana = 2
	state.Players[gameplay.PlayerA].EnergizedMana = 10
	state.Players[gameplay.PlayerA].Cooldowns = []gameplay.CooldownEntry{{Card: kt, TurnsRemaining: 1}}
	state.Players[gameplay.PlayerB].Hand = []gameplay.CardInstance{ret}
	state.Players[gameplay.PlayerB].Mana = 10
	e := NewEngine(state, chess.NewEmptyGame(chess.White))
	markInPlayForTest(state)

	if err := e.ActivateCard(gameplay.PlayerA, 0); err != nil {
		t.Fatalf("activate energy-gain: %v", err)
	}
	target := CardKnightTouch
	if err := e.QueueReactionCard(gameplay.PlayerB, 0, -1, EffectTarget{TargetCard: &target}); err == nil {
		t.Fatal("expected Retaliate to reject cooldown Power when opponent has insufficient regular mana")
	}
	if len(state.Players[gameplay.PlayerB].Hand) != 1 {
		t.Fatal("rejected Retaliate must remain in hand")
	}
}

func TestRetaliateBurnsRegularManaAndCopiesTargetlessPowerBeforeNextChainCard(t *testing.T) {
	eg := gameplay.CardInstance{InstanceID: "eg1", CardID: CardEnergyGain, ManaCost: 3, Ignition: 1, Cooldown: 2}
	ret := gameplay.CardInstance{InstanceID: "ret1", CardID: CardRetaliate, ManaCost: 2, Ignition: 0, Cooldown: 9}
	mb := gameplay.CardInstance{InstanceID: "mb1", CardID: CardManaBurn, ManaCost: 1, Ignition: 0, Cooldown: 3}
	copiedEnergy := gameplay.CardInstance{InstanceID: "eg-cd", CardID: CardEnergyGain, ManaCost: 0, Ignition: 1, Cooldown: 2}
	state, err := gameplay.NewMatchState(testDeckWith(eg), testDeckWith(mb))
	if err != nil {
		t.Fatal(err)
	}
	state.CurrentTurn = gameplay.PlayerA
	state.Players[gameplay.PlayerA].Hand = []gameplay.CardInstance{eg, ret}
	state.Players[gameplay.PlayerA].Mana = 8
	state.Players[gameplay.PlayerB].Hand = []gameplay.CardInstance{mb}
	state.Players[gameplay.PlayerB].Mana = 10
	state.Players[gameplay.PlayerB].Cooldowns = []gameplay.CooldownEntry{{Card: copiedEnergy, TurnsRemaining: 1}}
	e := NewEngine(state, chess.NewEmptyGame(chess.White))
	markInPlayForTest(state)

	if err := e.ActivateCard(gameplay.PlayerA, 0); err != nil {
		t.Fatalf("activate energy-gain: %v", err)
	}
	if err := e.QueueReactionCard(gameplay.PlayerB, 0, -1, EffectTarget{}); err != nil {
		t.Fatalf("queue mana-burn: %v", err)
	}
	target := CardEnergyGain
	if err := e.QueueReactionCard(gameplay.PlayerA, 0, -1, EffectTarget{TargetCard: &target}); err != nil {
		t.Fatalf("queue retaliate above mana-burn: %v", err)
	}
	if err := e.ResolveReactionStack(); err != nil {
		t.Fatalf("resolve reactions: %v", err)
	}
	if got := state.Players[gameplay.PlayerA].Mana; got != 4 {
		t.Fatalf("expected copied Energy Gain to resolve before lower Mana Burn, got PlayerA mana %d", got)
	}
	if got := state.Players[gameplay.PlayerB].Mana; got != 9 {
		t.Fatalf("expected PlayerB to only pay Mana Burn when copied target costs 0, got mana %d", got)
	}
	events := e.PullActivationFXEvents()
	if len(events) < 3 {
		t.Fatalf("expected activation events for mana-burn, copied energy-gain, and retaliate, got %+v", events)
	}
	if events[0].CardID != CardEnergyGain || events[1].CardID != CardRetaliate || events[2].CardID != CardManaBurn {
		t.Fatalf("expected copied effect to resolve before lower chain card, got %+v", events)
	}
}

func TestRetaliatePausesChainForCopiedPowerTargetsThenResumes(t *testing.T) {
	eg := gameplay.CardInstance{InstanceID: "eg1", CardID: CardEnergyGain, ManaCost: 0, Ignition: 1, Cooldown: 2}
	ret := gameplay.CardInstance{InstanceID: "ret1", CardID: CardRetaliate, ManaCost: 2, Ignition: 0, Cooldown: 9}
	mb := gameplay.CardInstance{InstanceID: "mb1", CardID: CardManaBurn, ManaCost: 1, Ignition: 0, Cooldown: 3}
	kt := gameplay.CardInstance{InstanceID: "kt-cd", CardID: CardKnightTouch, ManaCost: 3, Ignition: 0, Cooldown: 2}
	state, err := gameplay.NewMatchState(testDeckWith(eg), testDeckWith(ret))
	if err != nil {
		t.Fatal(err)
	}
	state.CurrentTurn = gameplay.PlayerA
	state.Players[gameplay.PlayerA].Hand = []gameplay.CardInstance{eg, mb}
	state.Players[gameplay.PlayerA].Mana = 10
	state.Players[gameplay.PlayerA].Cooldowns = []gameplay.CooldownEntry{{Card: kt, TurnsRemaining: 1}}
	state.Players[gameplay.PlayerB].Hand = []gameplay.CardInstance{ret}
	state.Players[gameplay.PlayerB].Mana = 10
	board := chess.NewEmptyGame(chess.White)
	board.SetPiece(chess.Pos{Row: 7, Col: 4}, chess.Piece{Type: chess.King, Color: chess.White})
	board.SetPiece(chess.Pos{Row: 0, Col: 4}, chess.Piece{Type: chess.King, Color: chess.Black})
	board.SetPiece(chess.Pos{Row: 3, Col: 3}, chess.Piece{Type: chess.Rook, Color: chess.Black})
	e := NewEngine(state, board)
	markInPlayForTest(state)

	if err := e.ActivateCard(gameplay.PlayerA, 0); err != nil {
		t.Fatalf("activate energy-gain: %v", err)
	}
	target := CardKnightTouch
	if err := e.QueueReactionCard(gameplay.PlayerB, 0, -1, EffectTarget{TargetCard: &target}); err != nil {
		t.Fatalf("queue retaliate: %v", err)
	}
	if err := e.QueueReactionCard(gameplay.PlayerA, 0, -1, EffectTarget{}); err != nil {
		t.Fatalf("queue mana-burn above retaliate: %v", err)
	}
	if err := e.ResolveReactionStack(); err != nil {
		t.Fatalf("resolve first part of reactions: %v", err)
	}
	if pending := e.PendingEffects(); len(pending) != 1 || pending[0].Owner != gameplay.PlayerB || pending[0].CardID != CardKnightTouch {
		t.Fatalf("expected copied Knight Touch pending effect for PlayerB, got %+v", pending)
	}
	if got := len(e.ReactionStackEntries()); got != 0 {
		t.Fatalf("expected no remaining reaction cards after Retaliate paused, got %d", got)
	}
	if rw, _, ok := e.ReactionWindowSnapshot(); !ok || !rw.Open {
		t.Fatalf("reaction window should stay open while copied effect is pending")
	}
	if err := e.ResolvePendingEffect(gameplay.PlayerB, EffectTarget{PiecePos: &chess.Pos{Row: 3, Col: 3}}); err != nil {
		t.Fatalf("resolve copied knight-touch target: %v", err)
	}
	if pending := e.PendingEffects(); len(pending) != 0 {
		t.Fatalf("pending copied effect should be consumed, got %+v", pending)
	}
	if rw, _, ok := e.ReactionWindowSnapshot(); ok && rw.Open {
		t.Fatalf("reaction window should close after pending copied effect completes, got %+v", rw)
	}
	if len(e.movementGrants) != 1 {
		t.Fatalf("expected one copied Knight Touch movement grant, got %+v", e.movementGrants)
	}
	grant := e.movementGrants[0]
	if grant.Owner != gameplay.PlayerB || grant.SourceCardID != CardKnightTouch || grant.Target != (chess.Pos{Row: 3, Col: 3}) {
		t.Fatalf("unexpected copied Knight Touch grant: %+v", grant)
	}
}
