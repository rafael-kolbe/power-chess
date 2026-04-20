package match

import (
	"testing"

	"power-chess/internal/chess"
	"power-chess/internal/gameplay"
)

// --- EndTurn (engine-level) ---

func TestEngineEndTurnAdvancesGameplayTurn(t *testing.T) {
	state, _ := gameplay.NewMatchState(testDeckWith(gameplay.CardInstance{InstanceID: "a", CardID: CardDoubleTurn, ManaCost: 1, Ignition: 1, Cooldown: 1}), testDeckWith(gameplay.CardInstance{InstanceID: "b", CardID: CardDoubleTurn, ManaCost: 1, Ignition: 1, Cooldown: 1}))
	board := chess.NewGame()
	e := NewEngine(state, board)
	markInPlayForTest(state)

	if err := e.EndTurn(gameplay.PlayerA); err != nil {
		t.Fatalf("EndTurn failed: %v", err)
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
	card := gameplay.CardInstance{InstanceID: "dt1", CardID: CardDoubleTurn, ManaCost: 6, Ignition: 2, Cooldown: 9}
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

	nr := noopResolver{}
	e.pendingEffects[gameplay.PlayerA] = []PendingEffect{
		{Owner: gameplay.PlayerA, CardID: CardKnightTouch, Resolver: nr},
	}
	e.pendingEffects[gameplay.PlayerB] = []PendingEffect{
		{Owner: gameplay.PlayerB, CardID: CardRookTouch, Resolver: nr},
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

func TestReactionStackTopSnapshotEmptyUnlessQueued(t *testing.T) {
	state, _ := gameplay.NewMatchState(testDeckWith(gameplay.CardInstance{InstanceID: "a", CardID: CardDoubleTurn, ManaCost: 1, Ignition: 1, Cooldown: 1}), testDeckWith(gameplay.CardInstance{InstanceID: "b", CardID: CardDoubleTurn, ManaCost: 1, Ignition: 1, Cooldown: 1}))
	e := NewEngine(state, chess.NewGame())
	e.OpenReactionWindow("capture_attempt", gameplay.PlayerA, []gameplay.CardType{gameplay.CardTypeCounter})

	if _, ok := e.ReactionStackTopSnapshot(); ok {
		t.Fatal("expected no stack top before queue")
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

	if err := e.QueueReactionCard(gameplay.PlayerA, 0, -1, EffectTarget{}); err == nil {
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
	if err := e.QueueReactionCard(gameplay.PlayerA, 0, -1, EffectTarget{}); err == nil {
		t.Fatal("expected error: capture chain must be started by the opponent")
	}
}

func TestLoneCounterOnStackResolvesNoOp(t *testing.T) {
	blockade := gameplay.CardInstance{InstanceID: "b1", CardID: CardBlockade, ManaCost: 0, Ignition: 0, Cooldown: 3}
	state, _ := gameplay.NewMatchState(testDeckWith(blockade), testDeckWith(blockade))
	e := NewEngine(state, chess.NewGame())
	markInPlayForTest(state)

	resolver := e.resolvers[CardBlockade]
	e.reactions.Push(ReactionAction{Owner: gameplay.PlayerA, Card: blockade, Resolver: resolver})
	e.ReactionWindow = &ReactionWindow{Open: true, Trigger: "capture_attempt", Actor: gameplay.PlayerA}

	if err := e.ResolveReactionStack(); err != nil {
		t.Fatalf("noop counter on stack should resolve: %v", err)
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
