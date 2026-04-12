package match

import (
	"testing"

	"power-chess/internal/chess"
	"power-chess/internal/gameplay"
)

// --- EndTurn (engine-level) ---

func TestEngineEndTurnClearsTurnScopedBuffs(t *testing.T) {
	state, _ := gameplay.NewMatchState(testDeckWith(gameplay.CardInstance{InstanceID: "a", CardID: CardDoubleTurn, ManaCost: 1, Ignition: 1, Cooldown: 1}), testDeckWith(gameplay.CardInstance{InstanceID: "b", CardID: CardDoubleTurn, ManaCost: 1, Ignition: 1, Cooldown: 1}))
	board := chess.NewGame()
	e := NewEngine(state, board)
	markInPlayForTest(state)

	// Simulate buff set.
	pos := chess.Pos{Row: 6, Col: 4}
	e.SetMoveBuffTarget(gameplay.PlayerA, MoveBuffKnight, pos)
	e.extraMoveLeft[gameplay.PlayerA] = 1
	e.movesThisTurn[gameplay.PlayerA] = 2

	if err := e.EndTurn(gameplay.PlayerA); err != nil {
		t.Fatalf("EndTurn failed: %v", err)
	}
	if e.moveBuffTarget[gameplay.PlayerA] != nil {
		t.Fatal("moveBuffTarget should be cleared after EndTurn")
	}
	if e.extraMoveLeft[gameplay.PlayerA] != 0 {
		t.Fatalf("extraMoveLeft should be 0 after EndTurn, got %d", e.extraMoveLeft[gameplay.PlayerA])
	}
	if e.movesThisTurn[gameplay.PlayerA] != 0 {
		t.Fatalf("movesThisTurn should be 0 after EndTurn, got %d", e.movesThisTurn[gameplay.PlayerA])
	}
	if e.State.CurrentTurn != gameplay.PlayerB {
		t.Fatalf("expected PlayerB turn after A EndTurn, got %s", e.State.CurrentTurn)
	}
}

func TestEngineEndTurnRejectsWrongPlayer(t *testing.T) {
	state, _ := gameplay.NewMatchState(testDeckWith(gameplay.CardInstance{InstanceID: "a", CardID: CardDoubleTurn, ManaCost: 1, Ignition: 1, Cooldown: 1}), testDeckWith(gameplay.CardInstance{InstanceID: "b", CardID: CardDoubleTurn, ManaCost: 1, Ignition: 1, Cooldown: 1}))
	board := chess.NewGame()
	e := NewEngine(state, board)
	markInPlayForTest(state)

	if err := e.EndTurn(gameplay.PlayerB); err == nil {
		t.Fatal("expected error ending wrong player's turn")
	}
}

// --- ActivatePlayerSkill ---

func TestActivatePlayerSkillRequiresStartedMatch(t *testing.T) {
	state, _ := gameplay.NewMatchState(testDeckWith(gameplay.CardInstance{InstanceID: "a", CardID: CardDoubleTurn, ManaCost: 1, Ignition: 1, Cooldown: 1}), testDeckWith(gameplay.CardInstance{InstanceID: "b", CardID: CardDoubleTurn, ManaCost: 1, Ignition: 1, Cooldown: 1}))
	board := chess.NewGame()
	e := NewEngine(state, board)
	// Not started yet.
	state.MulliganPhaseActive = false
	state.Started = false

	if err := e.ActivatePlayerSkill(gameplay.PlayerA); err == nil {
		t.Fatal("expected error when match is not started")
	}
}

func TestActivatePlayerSkillRequiresMulliganComplete(t *testing.T) {
	state, _ := gameplay.NewMatchState(testDeckWith(gameplay.CardInstance{InstanceID: "a", CardID: CardDoubleTurn, ManaCost: 1, Ignition: 1, Cooldown: 1}), testDeckWith(gameplay.CardInstance{InstanceID: "b", CardID: CardDoubleTurn, ManaCost: 1, Ignition: 1, Cooldown: 1}))
	board := chess.NewGame()
	e := NewEngine(state, board)
	state.MulliganPhaseActive = true
	state.Started = false

	if err := e.ActivatePlayerSkill(gameplay.PlayerA); err == nil {
		t.Fatal("expected error when mulligan is in progress")
	}
}

func TestActivatePlayerSkillRequiresChessTurnAlignment(t *testing.T) {
	state, _ := gameplay.NewMatchState(testDeckWith(gameplay.CardInstance{InstanceID: "a", CardID: CardDoubleTurn, ManaCost: 1, Ignition: 1, Cooldown: 1}), testDeckWith(gameplay.CardInstance{InstanceID: "b", CardID: CardDoubleTurn, ManaCost: 1, Ignition: 1, Cooldown: 1}))
	board := chess.NewEmptyGame(chess.Black) // mismatch: chess says Black, gameplay says A (White)
	e := NewEngine(state, board)
	markInPlayForTest(state)
	_ = state.SelectPlayerSkill(gameplay.PlayerA, "reinforcements")
	state.Players[gameplay.PlayerA].EnergizedMana = state.Players[gameplay.PlayerA].MaxEnergizedMana

	if err := e.ActivatePlayerSkill(gameplay.PlayerA); err == nil {
		t.Fatal("expected error when chess turn is out of sync")
	}
}

func TestActivatePlayerSkillSuccessAndAdvancesTurn(t *testing.T) {
	card := gameplay.CardInstance{InstanceID: "dt1", CardID: CardDoubleTurn, ManaCost: 4, Ignition: 1, Cooldown: 5}
	state, _ := gameplay.NewMatchState(testDeckWith(card), testDeckWith(card))
	board := chess.NewGame()
	e := NewEngine(state, board)

	// Select skill before marking match as started.
	if err := state.SelectPlayerSkill(gameplay.PlayerA, "reinforcements"); err != nil {
		t.Fatalf("SelectPlayerSkill: %v", err)
	}
	markInPlayForTest(state)
	state.Players[gameplay.PlayerA].EnergizedMana = state.Players[gameplay.PlayerA].MaxEnergizedMana

	if err := e.ActivatePlayerSkill(gameplay.PlayerA); err != nil {
		t.Fatalf("ActivatePlayerSkill failed: %v", err)
	}
	// After skill: turn advances to B, chess turn should also be Black.
	if e.State.CurrentTurn != gameplay.PlayerB {
		t.Fatalf("expected PlayerB after skill activation, got %s", e.State.CurrentTurn)
	}
	if board.Turn != chess.Black {
		t.Fatalf("expected chess Black turn after skill activation, got %v", board.Turn)
	}
}

// --- PendingEffects ---

func TestPendingEffectsReturnsAllQueued(t *testing.T) {
	state, _ := gameplay.NewMatchState(testDeckWith(gameplay.CardInstance{InstanceID: "a", CardID: CardDoubleTurn, ManaCost: 1, Ignition: 1, Cooldown: 1}), testDeckWith(gameplay.CardInstance{InstanceID: "b", CardID: CardDoubleTurn, ManaCost: 1, Ignition: 1, Cooldown: 1}))
	e := NewEngine(state, chess.NewGame())

	e.pendingEffects[gameplay.PlayerA] = []PendingEffect{
		{Owner: gameplay.PlayerA, CardID: CardKnightTouch, Resolver: knightTouchResolver{}},
	}
	e.pendingEffects[gameplay.PlayerB] = []PendingEffect{
		{Owner: gameplay.PlayerB, CardID: CardRookTouch, Resolver: rookTouchResolver{}},
	}

	all := e.PendingEffects()
	if len(all) != 2 {
		t.Fatalf("expected 2 pending effects, got %d", len(all))
	}
}

func TestPendingEffectsEmptyWhenNone(t *testing.T) {
	state, _ := gameplay.NewMatchState(testDeckWith(gameplay.CardInstance{InstanceID: "a", CardID: CardDoubleTurn, ManaCost: 1, Ignition: 1, Cooldown: 1}), testDeckWith(gameplay.CardInstance{InstanceID: "b", CardID: CardDoubleTurn, ManaCost: 1, Ignition: 1, Cooldown: 1}))
	e := NewEngine(state, chess.NewGame())

	all := e.PendingEffects()
	if len(all) != 0 {
		t.Fatalf("expected 0 pending effects, got %d", len(all))
	}
}

// --- ReactionWindowSnapshot ---

func TestReactionWindowSnapshotReturnsFalseWhenClosed(t *testing.T) {
	state, _ := gameplay.NewMatchState(testDeckWith(gameplay.CardInstance{InstanceID: "a", CardID: CardDoubleTurn, ManaCost: 1, Ignition: 1, Cooldown: 1}), testDeckWith(gameplay.CardInstance{InstanceID: "b", CardID: CardDoubleTurn, ManaCost: 1, Ignition: 1, Cooldown: 1}))
	e := NewEngine(state, chess.NewGame())

	_, _, ok := e.ReactionWindowSnapshot()
	if ok {
		t.Fatal("expected ok=false when no reaction window is open")
	}
}

func TestReactionWindowSnapshotReturnsCopyWhenOpen(t *testing.T) {
	state, _ := gameplay.NewMatchState(testDeckWith(gameplay.CardInstance{InstanceID: "a", CardID: CardDoubleTurn, ManaCost: 1, Ignition: 1, Cooldown: 1}), testDeckWith(gameplay.CardInstance{InstanceID: "b", CardID: CardDoubleTurn, ManaCost: 1, Ignition: 1, Cooldown: 1}))
	e := NewEngine(state, chess.NewGame())
	e.OpenReactionWindow("capture_attempt", gameplay.PlayerA, []gameplay.CardType{gameplay.CardTypeCounter})

	snap, stackSize, ok := e.ReactionWindowSnapshot()
	if !ok {
		t.Fatal("expected ok=true when reaction window is open")
	}
	if snap.Trigger != "capture_attempt" {
		t.Fatalf("expected trigger capture_attempt, got %s", snap.Trigger)
	}
	if stackSize != 0 {
		t.Fatalf("expected stack size 0, got %d", stackSize)
	}

	// Mutating the snapshot should not affect the engine's window.
	snap.Open = false
	if !e.ReactionWindow.Open {
		t.Fatal("snapshot mutation should not affect original reaction window")
	}
}

// --- errIfOpeningBlocksGameplay ---

func TestDrawCardBlockedDuringMulligan(t *testing.T) {
	state, _ := gameplay.NewMatchState(testDeckWith(gameplay.CardInstance{InstanceID: "a", CardID: CardDoubleTurn, ManaCost: 1, Ignition: 1, Cooldown: 1}), testDeckWith(gameplay.CardInstance{InstanceID: "b", CardID: CardDoubleTurn, ManaCost: 1, Ignition: 1, Cooldown: 1}))
	e := NewEngine(state, chess.NewGame())
	state.MulliganPhaseActive = true
	state.Started = false

	if err := e.DrawCard(gameplay.PlayerA); err == nil {
		t.Fatal("expected error drawing during mulligan phase")
	}
}

func TestDrawCardBlockedBeforeMatchStarted(t *testing.T) {
	state, _ := gameplay.NewMatchState(testDeckWith(gameplay.CardInstance{InstanceID: "a", CardID: CardDoubleTurn, ManaCost: 1, Ignition: 1, Cooldown: 1}), testDeckWith(gameplay.CardInstance{InstanceID: "b", CardID: CardDoubleTurn, ManaCost: 1, Ignition: 1, Cooldown: 1}))
	e := NewEngine(state, chess.NewGame())
	state.MulliganPhaseActive = false
	state.Started = false

	if err := e.DrawCard(gameplay.PlayerA); err == nil {
		t.Fatal("expected error drawing before match start")
	}
}

// --- QueueReactionCard validation ---

func TestQueueReactionCardRequiresOpenWindow(t *testing.T) {
	ct := gameplay.CardInstance{InstanceID: "ct1", CardID: CardCounterattack, ManaCost: 1, Ignition: 0, Cooldown: 6}
	state, _ := gameplay.NewMatchState(testDeckWith(ct), testDeckWith(ct))
	state.Players[gameplay.PlayerA].Hand = []gameplay.CardInstance{ct}
	e := NewEngine(state, chess.NewGame())
	markInPlayForTest(state)

	if err := e.QueueReactionCard(gameplay.PlayerA, 0, EffectTarget{}); err == nil {
		t.Fatal("expected error queuing reaction without open window")
	}
}

func TestQueueReactionCardRejectsCaptureChainStartByActor(t *testing.T) {
	ct := gameplay.CardInstance{InstanceID: "ct1", CardID: CardCounterattack, ManaCost: 1, Ignition: 0, Cooldown: 6}
	state, _ := gameplay.NewMatchState(testDeckWith(ct), testDeckWith(ct))
	state.Players[gameplay.PlayerA].Hand = []gameplay.CardInstance{ct}
	state.Players[gameplay.PlayerA].Mana = 10
	e := NewEngine(state, chess.NewGame())
	markInPlayForTest(state)

	// A is the actor, B is the opponent.
	e.OpenReactionWindow("capture_attempt", gameplay.PlayerA, []gameplay.CardType{gameplay.CardTypeCounter})
	if err := e.QueueReactionCard(gameplay.PlayerA, 0, EffectTarget{}); err == nil {
		t.Fatal("expected error: capture chain must be started by the opponent")
	}
}

// --- ResolveReactionStack: blockade without counterattack fails ---

func TestBlockadeWithoutPrecedingCounterattackFails(t *testing.T) {
	blockade := gameplay.CardInstance{InstanceID: "b1", CardID: CardBlockade, ManaCost: 0, Ignition: 0, Cooldown: 3}
	state, _ := gameplay.NewMatchState(testDeckWith(blockade), testDeckWith(blockade))
	state.Players[gameplay.PlayerA].Hand = []gameplay.CardInstance{blockade}
	state.Players[gameplay.PlayerA].Mana = 10
	e := NewEngine(state, chess.NewGame())
	markInPlayForTest(state)

	// Directly insert blockade into stack without counterattack preceding it.
	resolver := e.resolvers[CardBlockade]
	e.reactionStack = []ReactionAction{{Owner: gameplay.PlayerA, Card: blockade, Resolver: resolver}}
	e.ReactionWindow = &ReactionWindow{Open: true, Trigger: "capture_attempt", Actor: gameplay.PlayerA}

	if err := e.ResolveReactionStack(); err == nil {
		t.Fatal("expected error: blockade without preceding counterattack")
	}
}

// --- stopRightThere resolver ---

func TestStopRightThereNegatesOwnSlotError(t *testing.T) {
	srt := gameplay.CardInstance{InstanceID: "s1", CardID: CardStopRightThere, ManaCost: 3, Ignition: 0, Cooldown: 5}
	state, _ := gameplay.NewMatchState(testDeckWith(srt), testDeckWith(srt))
	board := chess.NewEmptyGame(chess.White)
	board.SetPiece(chess.Pos{Row: 7, Col: 4}, chess.Piece{Type: chess.King, Color: chess.White})
	board.SetPiece(chess.Pos{Row: 0, Col: 4}, chess.Piece{Type: chess.King, Color: chess.Black})
	e := NewEngine(state, board)
	markInPlayForTest(state)

	// Set ignition slot as owned by A, try stop-right-there from A (should fail).
	occupied := gameplay.CardInstance{InstanceID: "dt1", CardID: CardDoubleTurn, ManaCost: 4, Ignition: 1, Cooldown: 5}
	state.IgnitionSlot = gameplay.IgnitionSlot{Card: occupied, TurnsRemaining: 1, Occupied: true, ActivationOwner: gameplay.PlayerA}
	r := stopRightThereResolver{}
	if err := r.Apply(e, gameplay.PlayerA, EffectTarget{}); err == nil {
		t.Fatal("expected error: cannot negate your own ignited card")
	}
}

func TestStopRightThereNegatesOpponentSlot(t *testing.T) {
	state, _ := gameplay.NewMatchState(testDeckWith(gameplay.CardInstance{InstanceID: "a", CardID: CardDoubleTurn, ManaCost: 1, Ignition: 1, Cooldown: 1}), testDeckWith(gameplay.CardInstance{InstanceID: "b", CardID: CardDoubleTurn, ManaCost: 1, Ignition: 1, Cooldown: 1}))
	board := chess.NewEmptyGame(chess.White)
	board.SetPiece(chess.Pos{Row: 7, Col: 4}, chess.Piece{Type: chess.King, Color: chess.White})
	board.SetPiece(chess.Pos{Row: 0, Col: 4}, chess.Piece{Type: chess.King, Color: chess.Black})
	e := NewEngine(state, board)
	markInPlayForTest(state)

	// Slot owned by A, resolver applied by B.
	occupied := gameplay.CardInstance{InstanceID: "dt1", CardID: CardDoubleTurn, ManaCost: 4, Ignition: 1, Cooldown: 5}
	state.IgnitionSlot = gameplay.IgnitionSlot{Card: occupied, TurnsRemaining: 1, Occupied: true, ActivationOwner: gameplay.PlayerA}
	r := stopRightThereResolver{}
	if err := r.Apply(e, gameplay.PlayerB, EffectTarget{}); err != nil {
		t.Fatalf("stop-right-there should negate opponent's ignition: %v", err)
	}
	if state.IgnitionSlot.Occupied {
		t.Fatal("ignition slot should be cleared")
	}
}

// --- PendingMove helpers ---

func TestPendingMoveReturnsFalseWhenNil(t *testing.T) {
	state, _ := gameplay.NewMatchState(testDeckWith(gameplay.CardInstance{InstanceID: "a", CardID: CardDoubleTurn, ManaCost: 1, Ignition: 1, Cooldown: 1}), testDeckWith(gameplay.CardInstance{InstanceID: "b", CardID: CardDoubleTurn, ManaCost: 1, Ignition: 1, Cooldown: 1}))
	e := NewEngine(state, chess.NewGame())

	_, ok := e.PendingMove()
	if ok {
		t.Fatal("expected no pending move")
	}
}

func TestIsPendingCaptureFromBuffedAttackerFalseWithoutPending(t *testing.T) {
	state, _ := gameplay.NewMatchState(testDeckWith(gameplay.CardInstance{InstanceID: "a", CardID: CardDoubleTurn, ManaCost: 1, Ignition: 1, Cooldown: 1}), testDeckWith(gameplay.CardInstance{InstanceID: "b", CardID: CardDoubleTurn, ManaCost: 1, Ignition: 1, Cooldown: 1}))
	e := NewEngine(state, chess.NewGame())

	if e.IsPendingCaptureFromBuffedAttacker() {
		t.Fatal("expected false with no pending move")
	}
}

// --- CancelPendingCaptureAndCaptureAttacker ---

func TestCancelPendingCaptureErrorsWhenNoPending(t *testing.T) {
	state, _ := gameplay.NewMatchState(testDeckWith(gameplay.CardInstance{InstanceID: "a", CardID: CardDoubleTurn, ManaCost: 1, Ignition: 1, Cooldown: 1}), testDeckWith(gameplay.CardInstance{InstanceID: "b", CardID: CardDoubleTurn, ManaCost: 1, Ignition: 1, Cooldown: 1}))
	e := NewEngine(state, chess.NewGame())

	if err := e.CancelPendingCaptureAndCaptureAttacker(); err == nil {
		t.Fatal("expected error when no pending capture")
	}
}
